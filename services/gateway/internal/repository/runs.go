package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"starter/services/gateway/internal/contracts"
)

// allowedRunTransitions lists, per stored status, the statuses a run may move to. Completed,
// failed and stopped are terminal. A paused run resumes only after its cause is resolved.
var allowedRunTransitions = map[contracts.RunStatus][]contracts.RunStatus{
	contracts.RunQueued:           {contracts.RunRunning, contracts.RunPaused, contracts.RunFailed, contracts.RunStopped},
	contracts.RunRunning:          {contracts.RunAwaitingApproval, contracts.RunPaused, contracts.RunCompleted, contracts.RunFailed, contracts.RunStopped},
	contracts.RunAwaitingApproval: {contracts.RunRunning, contracts.RunPaused, contracts.RunFailed, contracts.RunStopped},
	contracts.RunPaused:           {contracts.RunRunning, contracts.RunFailed, contracts.RunStopped},
}

// sourceStatusesFor returns the stored statuses from which a run may move to target.
func sourceStatusesFor(target contracts.RunStatus) []string {
	var sources []string
	for source, targets := range allowedRunTransitions {
		for _, allowedTarget := range targets {
			if allowedTarget == target {
				sources = append(sources, string(source))
			}
		}
	}
	return sources
}

// NewJob is a durable job admission creates together with its passport and run.
type NewJob struct {
	ID   string
	Kind string
}

// InsertAdmission stores the passport, its run in status queued and the first queued job.
// It is called inside the admission transaction so all three commit together (Figure 4).
func (tx Tx) InsertAdmission(ctx context.Context, passport contracts.Passport, job NewJob) error {
	if ctx == nil || !validPassport(passport) || !validUUID(job.ID) || strings.TrimSpace(job.Kind) == "" {
		return ErrInvalid
	}
	scope, scopeErr := json.Marshal(passport.Scope)
	limits, limitsErr := json.Marshal(passport.Limits)
	if scopeErr != nil || limitsErr != nil {
		return ErrInvalid
	}
	statements := []struct {
		sql       string
		arguments []any
	}{
		{`INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
			admission_catalog_revision_id, scope, limits, issued_at, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			[]any{passport.PassportID, passport.OrganizationID, passport.ActorID, passport.TaskVersion,
				passport.AdmissionCatalogRevisionID, scope, limits, passport.IssuedAt, passport.ExpiresAt}},
		{`INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, $4)`,
			[]any{passport.RunID, passport.OrganizationID, passport.PassportID, string(contracts.RunQueued)}},
		{`INSERT INTO runtime.jobs (id, organization_id, run_id, kind, status) VALUES ($1, $2, $3, $4, 'queued')`,
			[]any{job.ID, passport.OrganizationID, passport.RunID, job.Kind}},
	}
	for _, statement := range statements {
		if _, err := tx.transaction.Exec(ctx, statement.sql, statement.arguments...); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

// validPassport checks the identity and lifetime fields the database cannot check itself.
func validPassport(passport contracts.Passport) bool {
	return validUUID(passport.PassportID) && validUUID(passport.RunID) &&
		validUUID(passport.OrganizationID) && validUUID(passport.ActorID) &&
		strings.TrimSpace(passport.TaskVersion) != "" && passport.AdmissionCatalogRevisionID > 0 &&
		!passport.IssuedAt.IsZero() && passport.ExpiresAt.After(passport.IssuedAt)
}

// RunTransition moves one run to a new status and records the event it produces.
type RunTransition struct {
	OrganizationID string
	RunID          string
	To             contracts.RunStatus
	// Reason is required for paused, failed and stopped and forbidden otherwise (X-11).
	Reason *contracts.ReasonCode
	Event  NewEvent
}

// TransitionRun applies a guarded status change and appends its event in this transaction.
// A change the stored status does not allow returns ErrInvalidTransition and writes nothing.
func (tx Tx) TransitionRun(ctx context.Context, transition RunTransition) (contracts.RunState, error) {
	var state contracts.RunState
	if ctx == nil || !validUUID(transition.OrganizationID) || !validUUID(transition.RunID) ||
		!transition.To.Valid() || transition.To.NeedsReason() != (transition.Reason != nil) ||
		(transition.Reason != nil && !transition.Reason.Valid()) {
		return state, ErrInvalid
	}
	// The event belongs to this run; it may not name another run or organization.
	if transition.Event.OrganizationID != transition.OrganizationID ||
		transition.Event.RunID == nil || *transition.Event.RunID != transition.RunID {
		return state, ErrInvalid
	}
	var reason *string
	if transition.Reason != nil {
		reasonText := string(*transition.Reason)
		reason = &reasonText
	}
	// One guarded statement: the status check and the update cannot interleave with another writer.
	row := tx.transaction.QueryRow(ctx, `UPDATE runtime.runs
		SET status = $3, terminal_reason = $4, updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND status = ANY($5)
		RETURNING `+runStateColumns,
		transition.RunID, transition.OrganizationID, string(transition.To), reason,
		sourceStatusesFor(transition.To))
	state, err := scanRunState(row)
	if errors.Is(err, ErrNotFound) {
		// Distinguish a missing run from a refused transition without revealing other organizations.
		if _, readErr := tx.readRunState(ctx, transition.OrganizationID, transition.RunID); readErr == nil {
			return contracts.RunState{}, ErrInvalidTransition
		}
		return contracts.RunState{}, ErrNotFound
	}
	if err != nil {
		return contracts.RunState{}, err
	}
	if _, err := tx.AppendEvent(ctx, transition.Event); err != nil {
		return contracts.RunState{}, err
	}
	return state, nil
}

const runStateColumns = `id::text, passport_id::text, status, terminal_reason, cancel_requested_at, created_at, updated_at`

type rowScanner interface {
	Scan(destinations ...any) error
}

func scanRunState(row rowScanner) (contracts.RunState, error) {
	var state contracts.RunState
	var status string
	var reason *string
	var cancelRequestedAt *time.Time
	err := row.Scan(&state.RunID, &state.PassportID, &status, &reason, &cancelRequestedAt, &state.CreatedAt, &state.UpdatedAt)
	if err != nil {
		return contracts.RunState{}, storageError(err)
	}
	state.Status = contracts.RunStatus(status)
	if !state.Status.Valid() {
		// A status outside the contract is never reported as a valid run state.
		return contracts.RunState{}, ErrUnavailable
	}
	if reason != nil {
		code := contracts.ReasonCode(*reason)
		if !code.Valid() {
			return contracts.RunState{}, ErrUnavailable
		}
		state.TerminalReason = &code
	}
	state.CreatedAt = state.CreatedAt.UTC()
	state.UpdatedAt = state.UpdatedAt.UTC()
	if cancelRequestedAt != nil {
		utc := cancelRequestedAt.UTC()
		state.CancelRequestedAt = &utc
	}
	return state, nil
}

func (tx Tx) readRunState(ctx context.Context, organizationID, runID string) (contracts.RunState, error) {
	return scanRunState(tx.transaction.QueryRow(ctx, `SELECT `+runStateColumns+`
		FROM runtime.runs WHERE id = $1 AND organization_id = $2`, runID, organizationID))
}

// RunState reads one run of the organization.
func (repository *Repository) RunState(ctx context.Context, organizationID, runID string) (contracts.RunState, error) {
	if ctx == nil || !validUUID(organizationID) || !validUUID(runID) {
		return contracts.RunState{}, ErrInvalid
	}
	var state contracts.RunState
	err := repository.InTransaction(ctx, func(tx Tx) error {
		var readErr error
		state, readErr = tx.readRunState(ctx, organizationID, runID)
		return readErr
	})
	return state, err
}

// Passport reads the passport of one run of the organization, decoding its stored scope and
// limits strictly against the X-08 contract.
func (repository *Repository) Passport(ctx context.Context, organizationID, runID string) (contracts.Passport, error) {
	var passport contracts.Passport
	if ctx == nil || !validUUID(organizationID) || !validUUID(runID) {
		return passport, ErrInvalid
	}
	err := repository.InTransaction(ctx, func(tx Tx) error {
		var scope, limits []byte
		err := tx.transaction.QueryRow(ctx, `SELECT passport.id::text, run.id::text,
			passport.organization_id::text, passport.actor_id::text, passport.task_version,
			passport.admission_catalog_revision_id, passport.issued_at, passport.expires_at,
			passport.scope, passport.limits
			FROM runtime.runs AS run
			JOIN runtime.passports AS passport
				ON passport.id = run.passport_id AND passport.organization_id = run.organization_id
			WHERE run.id = $1 AND run.organization_id = $2`, runID, organizationID).
			Scan(&passport.PassportID, &passport.RunID, &passport.OrganizationID, &passport.ActorID,
				&passport.TaskVersion, &passport.AdmissionCatalogRevisionID, &passport.IssuedAt,
				&passport.ExpiresAt, &scope, &limits)
		if err != nil {
			return storageError(err)
		}
		if contracts.DecodeStrict(scope, &passport.Scope) != nil || contracts.DecodeStrict(limits, &passport.Limits) != nil {
			// A stored passport that no longer fits its contract is never used as authority.
			return ErrUnavailable
		}
		passport.IssuedAt = passport.IssuedAt.UTC()
		passport.ExpiresAt = passport.ExpiresAt.UTC()
		return nil
	})
	if err != nil {
		return contracts.Passport{}, err
	}
	return passport, nil
}
