package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Action status values the gate writes. Execution states follow with the executor (GO-16).
const (
	actionStatusProposed         = "proposed"
	actionStatusAllowed          = "allowed"
	actionStatusDenied           = "denied"
	actionStatusAwaitingApproval = "awaiting_approval"
)

// decisionEventType is the safe event of one gate decision (X-12, provisional name). It switches
// to 3c's GO-22 event writer once that lands.
const decisionEventType = "action.decided"

// ErrRecorderUnavailable means the gate's records could not be written; the gate then denies.
var ErrRecorderUnavailable = errors.New("action records unavailable")

// PostgresRecorder writes the gate's records: runtime.actions and the decision's
// runtime.audit_events row. Every query is schema-qualified.
type PostgresRecorder struct{ pool *pgxpool.Pool }

// NewPostgresRecorder returns a recorder on the given pool. It creates nothing at construction.
func NewPostgresRecorder(pool *pgxpool.Pool) *PostgresRecorder { return &PostgresRecorder{pool: pool} }

// StoreAction inserts the immutable action and commits it before any evaluation. Storing the same
// action again (same id, digest and key, for example after a worker restart) is accepted; any
// other conflict is an error.
func (recorder *PostgresRecorder) StoreAction(ctx context.Context, action StoredAction) error {
	if recorder.pool == nil {
		return ErrRecorderUnavailable
	}
	_, err := recorder.pool.Exec(ctx,
		`INSERT INTO runtime.actions
		   (id, organization_id, run_id, step_number, tool, canonical_arguments,
		    canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		action.ActionID, action.OrganizationID, action.RunID, action.StepNumber, string(action.Tool),
		[]byte(action.CanonicalArguments), action.CanonicalizationVersion, action.ActionDigest[:],
		action.IdempotencyKey, action.EvaluatedRevisionID, actionStatusProposed)
	if err == nil {
		return nil
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return recorder.confirmSameAction(ctx, action)
	}
	return ErrRecorderUnavailable
}

// confirmSameAction accepts a repeated store only when the existing row is the same action.
func (recorder *PostgresRecorder) confirmSameAction(ctx context.Context, action StoredAction) error {
	var storedDigest []byte
	var storedKey string
	err := recorder.pool.QueryRow(ctx,
		`SELECT action_digest, idempotency_key FROM runtime.actions
		  WHERE id = $1 AND organization_id = $2 AND run_id = $3`,
		action.ActionID, action.OrganizationID, action.RunID).Scan(&storedDigest, &storedKey)
	if err != nil || !bytes.Equal(storedDigest, action.ActionDigest[:]) || storedKey != action.IdempotencyKey {
		return ErrRecorderUnavailable
	}
	return nil
}

// RecordDecision sets the stored action's status and inserts the decision event in one
// transaction. A proposal without an action row (malformed arguments) gets the event only.
func (recorder *PostgresRecorder) RecordDecision(ctx context.Context, run RunIdentity, decision Decision) error {
	if recorder.pool == nil {
		return ErrRecorderUnavailable
	}
	summary, err := json.Marshal(decisionSummary(decision))
	if err != nil {
		return ErrRecorderUnavailable
	}
	err = pgx.BeginFunc(ctx, recorder.pool, func(tx pgx.Tx) error {
		var eventActionID any // NULL without an action row: the event must not point at a missing action
		var revisionID any
		if decision.ActionStored {
			tag, err := tx.Exec(ctx,
				`UPDATE runtime.actions SET status = $1, updated_at = now()
				  WHERE id = $2 AND organization_id = $3 AND run_id = $4 AND status = $5`,
				actionStatusFor(decision.Outcome), decision.ActionID, run.OrganizationID, run.RunID, actionStatusProposed)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrRecorderUnavailable // already decided or not this run's action
			}
			eventActionID = decision.ActionID
			revisionID = decision.EvaluatedRevisionID
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO runtime.audit_events
			   (organization_id, run_id, action_id, event_type, decision, reason_code, catalog_revision_id, masked_summary)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			run.OrganizationID, run.RunID, eventActionID, decisionEventType, string(decision.Outcome),
			nullableReason(decision.ReasonCode), revisionID, summary)
		return err
	})
	if err != nil {
		return ErrRecorderUnavailable
	}
	return nil
}

// decisionSummary holds references only: never arguments, content or model text.
func decisionSummary(decision Decision) map[string]any {
	summary := map[string]any{"outcome": string(decision.Outcome), "action_stored": decision.ActionStored}
	if decision.ReasonCode != "" {
		summary["reason_code"] = string(decision.ReasonCode)
	}
	return summary
}

func actionStatusFor(outcome Outcome) string {
	switch outcome {
	case OutcomeAllow:
		return actionStatusAllowed
	case OutcomeApprovalRequired:
		return actionStatusAwaitingApproval
	default:
		return actionStatusDenied
	}
}

func nullableReason(reason ReasonCode) any {
	if reason == "" {
		return nil
	}
	return string(reason)
}
