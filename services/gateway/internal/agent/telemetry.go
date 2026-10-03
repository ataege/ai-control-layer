package agent

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/security"
)

// Timing phases (runtime.timing_records.phase). Observed monotonic durations only, never
// estimates or cost.
const (
	PhasePolicyLookup  = "policy_lookup"
	PhaseDeterministic = "deterministic"
	PhaseSemantic      = "semantic"
	PhaseProvider      = "provider"
	PhaseApprovalWait  = "approval_wait"
	PhaseCommit        = "commit"
	PhaseTotal         = "total"
)

// ErrTelemetry hides driver errors of the telemetry store.
var ErrTelemetry = errors.New("telemetry storage unavailable")

// Span is one measured duration attached to what it measured.
type Span struct {
	Phase        string
	Duration     time.Duration
	Failed       bool
	EvaluationID string
	ActionID     string
	ModelCallID  string
}

// Telemetry writes timing records and control assessments (GO-80). It never stores inspected
// text, prompts, model output or protected values: only ids, outcomes, codes and durations.
type Telemetry struct{ pool *pgxpool.Pool }

// NewTelemetry returns a store over the shared gateway pool.
func NewTelemetry(pool *pgxpool.Pool) *Telemetry { return &Telemetry{pool: pool} }

// RecordSpans writes the spans of one run in one transaction.
func (telemetry *Telemetry) RecordSpans(ctx context.Context, organizationID, runID string, spans []Span) error {
	if telemetry == nil || telemetry.pool == nil {
		return ErrTelemetry
	}
	if len(spans) == 0 {
		return nil
	}
	transaction, err := telemetry.pool.Begin(ctx)
	if err != nil {
		return ErrTelemetry
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err = insertSpans(ctx, transaction, organizationID, runID, spans); err != nil {
		return err
	}
	if err = transaction.Commit(ctx); err != nil {
		return ErrTelemetry
	}
	return nil
}

// RecordInspection writes the control assessments and spans of an inspection that released
// nothing (a paused run), in its own transaction. A released inspection is written together
// with its context entries instead (ContextStore.AppendToolStep).
func (telemetry *Telemetry) RecordInspection(ctx context.Context, organizationID, runID, actionID string, admissionRevisionID int64,
	evaluationID string, evidence *security.ToolResultInspection) error {
	if telemetry == nil || telemetry.pool == nil {
		return ErrTelemetry
	}
	if evidence == nil {
		return nil
	}
	transaction, err := telemetry.pool.Begin(ctx)
	if err != nil {
		return ErrTelemetry
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err = insertInspectionEvidence(ctx, transaction, organizationID, runID, actionID, admissionRevisionID, evaluationID, evidence); err != nil {
		return err
	}
	if err = transaction.Commit(ctx); err != nil {
		return ErrTelemetry
	}
	return nil
}

// insertInspectionEvidence writes one control assessment per control decision and one span per
// measured control and security provider call of one inspection.
func insertInspectionEvidence(ctx context.Context, transaction pgx.Tx, organizationID, runID, actionID string, admissionRevisionID int64,
	evaluationID string, evidence *security.ToolResultInspection) error {
	if evidence == nil {
		return nil
	}
	// The repository is the one control_assessments writer (3c, GO-80): it validates every record.
	evaluatedRevisionID := admissionRevisionID
	for _, record := range evidence.Records {
		if record.EvaluatedCatalogRevisionID > 0 {
			evaluatedRevisionID = record.EvaluatedCatalogRevisionID
		}
	}
	keys := repository.ControlKeys{OrganizationID: organizationID, RunID: runID, EvaluationID: evaluationID, ActionID: actionID,
		AdmissionCatalogRevisionID: admissionRevisionID, EvaluatedCatalogRevisionID: evaluatedRevisionID}
	if err := repository.Join(transaction).InsertControlRecords(ctx, keys, evidence.Records); err != nil {
		return ErrTelemetry
	}
	return insertSpans(ctx, transaction, organizationID, runID, inspectionSpans(evaluationID, actionID, evidence))
}

// inspectionSpans turns an inspection's records into spans: each deterministic or semantic control,
// and the provider time of each metered security call.
func inspectionSpans(evaluationID, actionID string, evidence *security.ToolResultInspection) []Span {
	var spans []Span
	for _, record := range evidence.Records {
		phase := PhaseDeterministic
		if record.ControlClass == security.ClassSemantic {
			phase = PhaseSemantic
		}
		spans = append(spans, Span{Phase: phase, Duration: record.Duration, Failed: record.Outcome == security.OutcomeError,
			EvaluationID: evaluationID, ActionID: actionID, ModelCallID: record.SecurityModelCallID})
	}
	for _, call := range evidence.SemanticCalls {
		if call.Record.SecurityModelCallID == "" {
			continue // not dispatched: no provider time to record
		}
		spans = append(spans, Span{Phase: PhaseProvider, Duration: call.ProviderDuration, Failed: call.UsageUnknown,
			EvaluationID: evaluationID, ActionID: actionID, ModelCallID: call.Record.SecurityModelCallID})
	}
	return spans
}

func insertSpans(ctx context.Context, transaction pgx.Tx, organizationID, runID string, spans []Span) error {
	for _, span := range spans {
		if span.Duration < 0 {
			span.Duration = 0
		}
		if _, err := transaction.Exec(ctx, `
			INSERT INTO runtime.timing_records(organization_id, run_id, evaluation_id, action_id, model_call_id, phase, duration_microseconds, failed)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			organizationID, runID, optionalText(span.EvaluationID), optionalText(span.ActionID), optionalText(span.ModelCallID),
			span.Phase, span.Duration.Microseconds(), span.Failed); err != nil {
			return ErrTelemetry
		}
	}
	return nil
}

// optionalText stores an empty string as NULL.
func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
