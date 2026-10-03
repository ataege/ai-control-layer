package policy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/security"
)

// Action status values the gate writes. Execution states follow with the executor (GO-16).
const (
	actionStatusProposed         = "proposed"
	actionStatusAllowed          = "allowed"
	actionStatusDenied           = "denied"
	actionStatusAwaitingApproval = "awaiting_approval"
)

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

// RecordDecision sets the stored action's status and appends the decision's safe event (X-12)
// through the runtime repository, in one transaction. A proposal without an action row
// (malformed arguments) gets the event only.
func (recorder *PostgresRecorder) RecordDecision(ctx context.Context, run RunIdentity, decision Decision) error {
	if recorder.pool == nil {
		return ErrRecorderUnavailable
	}
	err := repository.New(recorder.pool).InTransaction(ctx, func(tx repository.Tx) error {
		event := decisionEvent(run, decision)
		if decision.ActionStored {
			tag, err := tx.Raw().Exec(ctx,
				`UPDATE runtime.actions SET status = $1, updated_at = now()
				  WHERE id = $2 AND organization_id = $3 AND run_id = $4 AND status = $5`,
				actionStatusFor(decision.Outcome), decision.ActionID, run.OrganizationID, run.RunID, actionStatusProposed)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrRecorderUnavailable // already decided or not this run's action
			}
		}
		if err := insertControlRecords(ctx, tx, run, decision); err != nil {
			return err
		}
		_, err := tx.AppendEvent(ctx, event)
		return err
	})
	if err != nil {
		return ErrRecorderUnavailable
	}
	return nil
}

// decisionEvent maps a decision to its closed X-12 event type, with references only: never
// arguments, content or model text.
func decisionEvent(run RunIdentity, decision Decision) repository.NewEvent {
	runID := run.RunID
	event := repository.NewEvent{OrganizationID: run.OrganizationID, RunID: &runID}
	if decision.ActionStored {
		actionID := decision.ActionID
		revisionID := decision.EvaluatedRevisionID
		event.ActionID, event.CatalogRevisionID = &actionID, &revisionID
	}
	var eventDecision contracts.EventDecision
	switch decision.Outcome {
	case OutcomeAllow:
		event.EventType, eventDecision = contracts.EventActionAllowed, contracts.DecisionAllow
	case OutcomeApprovalRequired:
		event.EventType, eventDecision = contracts.EventApprovalRequested, contracts.DecisionApprovalRequired
		// References only: never the exact content or the recipient address.
		if decision.Review != nil && decision.Review.ReportID != "" {
			reportID := decision.Review.ReportID
			template := contracts.ReportTemplate(decision.Review.Template)
			classification := decision.Review.Classification
			event.MaskedSummary.ReportID, event.MaskedSummary.Template = &reportID, &template
			event.MaskedSummary.Classification = &classification
		}
	default:
		event.EventType, eventDecision = contracts.EventActionDenied, contracts.DecisionDeny
		if isExportDenial(decision.ReasonCode) {
			event.EventType = contracts.EventReportExportDenied
			lineageCheck := "failed"
			if decision.ReasonCode == ReasonReportLineageMissing {
				lineageCheck = "missing"
			}
			event.MaskedSummary.LineageCheck = &lineageCheck
		}
	}
	event.Decision = &eventDecision
	if decision.ReasonCode != "" {
		reason := decision.ReasonCode
		event.ReasonCode = &reason
	}
	effect := "none"
	event.MaskedSummary.Effect = &effect
	if decision.AlternativeTemplate != "" {
		alternative := contracts.ReportTemplate(decision.AlternativeTemplate)
		event.MaskedSummary.AlternativeTemplate = &alternative
	}
	return event
}

// isExportDenial is true for the provenance reasons of a refused report export (GO-64).
func isExportDenial(reason ReasonCode) bool {
	return reason == ReasonReportExportRestricted || reason == ReasonReportLineageMissing ||
		reason == contracts.ReasonResourceVersionChanged
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

// insertControlRecords writes the decision's security evidence (GO-77) to
// runtime.control_assessments: one evaluation id for the decision, the action when it was stored,
// and never any inspected text.
func insertControlRecords(ctx context.Context, tx repository.Tx, run RunIdentity, decision Decision) error {
	if len(decision.ControlRecords) == 0 {
		return nil
	}
	evaluationID, err := newUUID()
	if err != nil {
		return err
	}
	var actionID any
	if decision.ActionStored {
		actionID = decision.ActionID
	}
	for _, record := range decision.ControlRecords {
		var verdict, verdictSource, modelCallID, matchedRule, feedRevision, reason any
		if record.ControlClass == security.ClassSemantic {
			verdictSource = string(record.VerdictSource)
			if record.Verdict != nil {
				encoded, err := json.Marshal(record.Verdict)
				if err != nil {
					return err
				}
				verdict = encoded
			}
			if record.SecurityModelCallID != "" {
				modelCallID = record.SecurityModelCallID
			}
		}
		if record.MatchedRuleID != "" {
			matchedRule = record.MatchedRuleID
		}
		if record.FeedRevision != "" {
			feedRevision = record.FeedRevision
		}
		if record.ReasonCode != "" {
			reason = record.ReasonCode
		}
		evaluatedRevision := record.EvaluatedCatalogRevisionID
		if evaluatedRevision <= 0 {
			evaluatedRevision = decision.EvaluatedRevisionID
		}
		if _, err := tx.Raw().Exec(ctx,
			`INSERT INTO runtime.control_assessments
			   (organization_id, run_id, evaluation_id, action_id, security_model_call_id, boundary, control_class,
			    control_id, outcome, reason_code, admission_catalog_revision_id, evaluated_catalog_revision_id,
			    matched_rule_id, feed_revision, verdict_source, verdict)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			run.OrganizationID, run.RunID, evaluationID, actionID, modelCallID, string(record.Boundary),
			string(record.ControlClass), record.ControlID, string(record.Outcome), reason,
			decision.AdmissionCatalogRevisionID, evaluatedRevision, matchedRule, feedRevision, verdictSource, verdict,
		); err != nil {
			return err
		}
	}
	return nil
}

// newUUID returns a random version 4 UUID.
func newUUID() (string, error) {
	var identifier [16]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return "", err
	}
	identifier[6] = (identifier[6] & 0x0f) | 0x40
	identifier[8] = (identifier[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", identifier[0:4], identifier[4:6], identifier[6:8], identifier[8:10], identifier[10:16]), nil
}
