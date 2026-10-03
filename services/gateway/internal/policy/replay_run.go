package policy

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
)

// HostileNoteInvoiceID is the out-of-scope invoice the redirect-record hostile note names.
const HostileNoteInvoiceID = "invoice_B01"

// Errors of a demo-triggered replay. None of them writes anything.
var (
	ErrReplayRunNotFound    = errors.New("replay run not found")
	ErrReplayRunNotFinished = errors.New("replay run is not finished")
	ErrReplayTargetMissing  = errors.New("replay run has no record for this fixture")
	ErrReplayUnavailable    = errors.New("replay could not be prepared")
)

// ReplayOutcome is what one demo-triggered replay did: the labelled proposal and the gate's
// stored decision, with the reason its live equivalent is denied with.
type ReplayOutcome struct {
	Run            RunIdentity
	FixtureID      string
	ReplaySource   string
	ActionID       string
	StepNumber     int
	Tool           string
	ExpectedReason ReasonCode
	Decision       Decision
}

// ReplayRunner submits a labelled replay for a finished run to the gate it is given, which is
// the production gate in cmd/replay. It never executes the action: a replayed prohibited
// proposal has nothing to run, and an unexpected allow is reported, not acted on.
type ReplayRunner struct {
	pool       *pgxpool.Pool
	repository *repository.Repository
	gate       *Gate
}

// NewReplayRunner returns a runner on the pool and gate. It creates nothing.
func NewReplayRunner(pool *pgxpool.Pool, gate *Gate) *ReplayRunner {
	return &ReplayRunner{pool: pool, repository: repository.New(pool), gate: gate}
}

// Replay submits the fixture's labelled proposal as the next step of the run. Only a finished
// run (completed, failed or stopped) is accepted, so the replay can never take a step number
// the agent loop of a live run would use next.
func (runner *ReplayRunner) Replay(ctx context.Context, runID, fixtureID string) (ReplayOutcome, error) {
	if _, defined := replayFixtures[fixtureID]; !defined {
		return ReplayOutcome{}, ErrUnknownReplayFixture
	}
	if runner.pool == nil || runner.gate == nil {
		return ReplayOutcome{}, ErrReplayUnavailable
	}
	run, nextStep, err := runner.finishedRun(ctx, runID)
	if err != nil {
		return ReplayOutcome{}, err
	}
	targets, err := runner.targets(ctx, run)
	if err != nil {
		return ReplayOutcome{}, err
	}
	if err = requireFixtureTargets(fixtureID, targets); err != nil {
		return ReplayOutcome{}, err
	}
	actionID, err := newUUID()
	if err != nil {
		return ReplayOutcome{}, ErrReplayUnavailable
	}
	idempotencyKey, err := newUUID()
	if err != nil {
		return ReplayOutcome{}, ErrReplayUnavailable
	}
	proposal, expectedReason, err := ReplayProposal(fixtureID, targets, actionID, nextStep, idempotencyKey)
	if err != nil {
		return ReplayOutcome{}, err
	}
	return ReplayOutcome{
		Run: run, FixtureID: fixtureID, ReplaySource: proposal.ReplaySource, ActionID: actionID,
		StepNumber: nextStep, Tool: proposal.Tool, ExpectedReason: expectedReason,
		Decision: runner.gate.Evaluate(ctx, run, proposal),
	}, nil
}

// finishedRun reads the run's organization from its own row, requires a terminal status and
// returns the step after every step the run has recorded.
func (runner *ReplayRunner) finishedRun(ctx context.Context, runID string) (RunIdentity, int, error) {
	if !uuidPattern.MatchString(runID) {
		return RunIdentity{}, 0, ErrReplayRunNotFound
	}
	var organizationID string
	var status contracts.RunStatus
	var lastStep int
	err := runner.pool.QueryRow(ctx,
		`SELECT r.organization_id::text, r.status,
		        greatest(coalesce((SELECT max(step_number) FROM runtime.actions WHERE run_id = r.id), 0),
		                 coalesce((SELECT max(step_number) FROM runtime.context_entries WHERE run_id = r.id), 0))
		   FROM runtime.runs r WHERE r.id = $1`, runID).Scan(&organizationID, &status, &lastStep)
	if errors.Is(err, pgx.ErrNoRows) {
		return RunIdentity{}, 0, ErrReplayRunNotFound
	}
	if err != nil {
		return RunIdentity{}, 0, ErrReplayUnavailable
	}
	if status != contracts.RunCompleted && status != contracts.RunFailed && status != contracts.RunStopped {
		return RunIdentity{}, 0, ErrReplayRunNotFinished
	}
	return RunIdentity{OrganizationID: organizationID, RunID: runID}, lastStep + 1, nil
}

// targets reads the run's trusted records: the run's latest report of each classification and
// its first registered recipient reference. A record the run lacks stays empty.
func (runner *ReplayRunner) targets(ctx context.Context, run RunIdentity) (ReplayTargets, error) {
	passport, err := runner.repository.Passport(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return ReplayTargets{}, ErrReplayUnavailable
	}
	targets := ReplayTargets{OutOfScopeInvoiceID: HostileNoteInvoiceID}
	if references := slices.Sorted(slices.Values(passport.Scope.RecipientReferences)); len(references) > 0 {
		targets.RecipientReference = references[0]
	}
	rows, err := runner.pool.Query(ctx,
		`SELECT DISTINCT ON (classification) classification, id::text FROM demo.reports
		  WHERE organization_id = $1 AND run_id = $2 ORDER BY classification, created_at DESC, id`,
		run.OrganizationID, run.RunID)
	if err != nil {
		return ReplayTargets{}, ErrReplayUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var classification, reportID string
		if err = rows.Scan(&classification, &reportID); err != nil {
			return ReplayTargets{}, ErrReplayUnavailable
		}
		switch classification {
		case "vendor_shareable":
			targets.VendorReportID = reportID
		case "internal_only":
			targets.InternalReportID = reportID
		}
	}
	if rows.Err() != nil {
		return ReplayTargets{}, ErrReplayUnavailable
	}
	return targets, nil
}

// requireFixtureTargets refuses a replay whose proposal would point at a record the run does not
// have, so a missing report is never shown as an argument error of the gate.
func requireFixtureTargets(fixtureID string, targets ReplayTargets) error {
	switch fixtureID {
	case "hostile_note_redirect_recipient_v1":
		if targets.VendorReportID == "" {
			return ErrReplayTargetMissing
		}
	case "hostile_note_internal_disclosure_v1":
		if targets.InternalReportID == "" || targets.RecipientReference == "" {
			return ErrReplayTargetMissing
		}
	}
	return nil
}
