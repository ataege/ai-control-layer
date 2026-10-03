// Package tools holds the four registered tool adapters (read_invoice, read_vendor,
// create_report, queue_report) and the effect runner the executor calls (GO-16, GO-34).
// Every adapter checks the organization and the passport scope itself; an upstream check never
// replaces its own. All SQL is schema-qualified.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/repository"
)

// Tool names: the four registered adapters, nothing else is executable.
const (
	ToolReadInvoice  = "read_invoice"
	ToolReadVendor   = "read_vendor"
	ToolCreateReport = "create_report"
	ToolQueueReport  = "queue_report"
)

// Outcomes of an effect. An unknown outcome is never a value here: it is an error.
const (
	OutcomeSucceeded = "succeeded"
	OutcomeFailed    = "failed"
)

// Reason codes the adapters return, from the shared X-13 vocabulary (contracts.ReasonCode).
// EffectResult carries them as plain strings so callers can convert them to their own types.
const (
	ReasonResourceOutOfScope     = string(contracts.ReasonResourceOutOfScope)
	ReasonTemplateNotAllowed     = string(contracts.ReasonTemplateNotAllowed)
	ReasonDestinationNotAllowed  = string(contracts.ReasonDestinationNotAllowed)
	ReasonReportExportRestricted = string(contracts.ReasonReportExportRestricted)
	ReasonReportLineageMissing   = string(contracts.ReasonReportLineageMissing)
	ReasonResourceVersionChanged = string(contracts.ReasonResourceVersionChanged)
)

// EffectRequest is what the executor passes for one claimed attempt. Identity comes from the
// executor's verified records, never from model output.
type EffectRequest struct {
	OrganizationID, RunID, PassportID, ActionID, AttemptID string
	Tool                                                   string
	CanonicalArguments                                     json.RawMessage
	ActionDigest                                           [32]byte
	CatalogRevisionID                                      int64
}

// EffectResult is the adapter's decision. ModelFacing is the typed, allowlisted result
// (nil when Outcome is "failed"); ReasonCode is set exactly when Outcome is "failed".
type EffectResult struct {
	Outcome     string
	ReasonCode  string
	ModelFacing any
}

// EffectRunner is the interface the executor calls.
type EffectRunner interface {
	RunEffect(ctx context.Context, tx pgx.Tx, request EffectRequest) (EffectResult, error)
}

// Runner runs the registered adapters. It holds no state: everything comes from the request and
// the database transaction.
type Runner struct{}

var _ EffectRunner = Runner{}

// errPrecondition marks a request that does not match the stored action, attempt or passport.
// It is an error, not a failed outcome: the executor rolls back and pauses.
var errPrecondition = errors.New("tools: effect precondition not met")

// adapterOutcome is what one adapter returns to the runner: its result and the safe events
// (X-12) that record it. Events carry references and Go-derived labels only, never values.
type adapterOutcome struct {
	result EffectResult
	events []eventRecord
}

// eventRecord is one safe event of an outcome, before the runner adds the identifiers.
type eventRecord struct {
	eventType contracts.EventType
	decision  *contracts.EventDecision
	summary   contracts.MaskedSummary
}

// failed is an adapter's own refusal: no effect, one action.failed event.
func failed(reasonCode string) adapterOutcome {
	return adapterOutcome{
		result: EffectResult{Outcome: OutcomeFailed, ReasonCode: reasonCode},
		events: []eventRecord{{eventType: contracts.EventActionFailed, summary: contracts.MaskedSummary{Effect: text("none")}}},
	}
}

// succeeded is an adapter's success with one event.
func succeeded(modelFacing any, eventType contracts.EventType, summary contracts.MaskedSummary) adapterOutcome {
	return adapterOutcome{
		result: EffectResult{Outcome: OutcomeSucceeded, ModelFacing: modelFacing},
		events: []eventRecord{{eventType: eventType, summary: summary}},
	}
}

func text(value string) *string { return &value }

// RunEffect runs one adapter inside tx, then completes the attempt and writes its audit event in
// the same transaction. The executor begins and commits tx; on an error it rolls back and pauses.
func (Runner) RunEffect(ctx context.Context, tx pgx.Tx, request EffectRequest) (EffectResult, error) {
	if ctx == nil || tx == nil {
		return EffectResult{}, fmt.Errorf("%w: missing context or transaction", errPrecondition)
	}
	if err := verifyActionAndAttempt(ctx, tx, request); err != nil {
		return EffectResult{}, err
	}
	scope, err := loadScope(ctx, tx, request)
	if err != nil {
		return EffectResult{}, err
	}

	var outcome adapterOutcome
	switch request.Tool {
	case ToolReadInvoice:
		outcome, err = readInvoice(ctx, tx, scope, request.CanonicalArguments)
	case ToolReadVendor:
		outcome, err = readVendor(ctx, tx, scope, request.CanonicalArguments)
	case ToolCreateReport:
		outcome, err = createReport(ctx, tx, scope, request)
	case ToolQueueReport:
		outcome, err = queueReport(ctx, tx, scope, request)
	default:
		return EffectResult{}, fmt.Errorf("%w: unregistered tool", errPrecondition)
	}
	if err != nil {
		return EffectResult{}, err
	}
	if err := completeAttempt(ctx, tx, request, outcome); err != nil {
		return EffectResult{}, err
	}
	return outcome.result, nil
}

// verifyActionAndAttempt checks that the stored action matches the request and that the attempt
// is this action's open attempt, and locks the attempt row.
func verifyActionAndAttempt(ctx context.Context, tx pgx.Tx, request EffectRequest) error {
	var matches bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM runtime.actions
		    WHERE id = $1 AND organization_id = $2 AND run_id = $3 AND tool = $4 AND action_digest = $5
		      AND canonical_arguments = $6::jsonb)`,
		request.ActionID, request.OrganizationID, request.RunID, request.Tool, request.ActionDigest[:],
		string(request.CanonicalArguments),
	).Scan(&matches)
	if err != nil {
		return fmt.Errorf("tools: read stored action: %w", err)
	}
	if !matches {
		return fmt.Errorf("%w: stored action does not match the request", errPrecondition)
	}
	var attemptID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM runtime.execution_attempts
		  WHERE id = $1 AND action_id = $2 AND organization_id = $3 AND completed_at IS NULL
		  FOR UPDATE`,
		request.AttemptID, request.ActionID, request.OrganizationID,
	).Scan(&attemptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: no open attempt for this action", errPrecondition)
	}
	if err != nil {
		return fmt.Errorf("tools: lock attempt: %w", err)
	}
	return nil
}

// completeAttempt records the outcome on the attempt and appends the outcome's safe events through
// the shared X-12 event writer, all inside tx. repository.Join(tx) writes in tx itself (taking
// the run lock that keeps a run's events in cursor order), so they commit with the effect or not
// at all.
func completeAttempt(ctx context.Context, tx pgx.Tx, request EffectRequest, outcome adapterOutcome) error {
	tag, err := tx.Exec(ctx,
		`UPDATE runtime.execution_attempts SET outcome = $1, completed_at = now()
		  WHERE id = $2 AND organization_id = $3 AND completed_at IS NULL`,
		outcome.result.Outcome, request.AttemptID, request.OrganizationID,
	)
	if err != nil {
		return fmt.Errorf("tools: complete attempt: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: attempt was completed concurrently", errPrecondition)
	}
	runID, actionID := request.RunID, request.ActionID
	var catalogRevisionID *int64
	if request.CatalogRevisionID > 0 {
		catalogRevisionID = &request.CatalogRevisionID
	}
	var reasonCode *contracts.ReasonCode
	if outcome.result.ReasonCode != "" {
		code := contracts.ReasonCode(outcome.result.ReasonCode)
		reasonCode = &code
	}
	eventTx := repository.Join(tx)
	for _, record := range outcome.events {
		// A denied or failed effect says why in its X-13 safe message (GO-58).
		if reasonCode != nil && record.summary.SafeMessage == nil {
			message := reasonCode.SafeMessage()
			record.summary.SafeMessage = &message
		}
		if _, err := eventTx.AppendEvent(ctx, repository.NewEvent{
			OrganizationID: request.OrganizationID, RunID: &runID, ActionID: &actionID,
			EventType: record.eventType, Decision: record.decision, ReasonCode: reasonCode,
			CatalogRevisionID: catalogRevisionID, MaskedSummary: record.summary,
		}); err != nil {
			return fmt.Errorf("tools: append event: %w", err)
		}
	}
	return nil
}

// decodeArguments decodes one JSON object strictly: unknown fields, trailing data and a missing
// required field are rejected by the caller's validation.
func decodeArguments(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid arguments", errPrecondition)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data after arguments", errPrecondition)
	}
	return nil
}
