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

// Reason codes from the proposed vocabulary (docs/product/README.md) that the adapters return.
const (
	ReasonResourceOutOfScope     = "resource_out_of_scope"
	ReasonTemplateNotAllowed     = "template_not_allowed"
	ReasonDestinationNotAllowed  = "destination_not_allowed"
	ReasonReportExportRestricted = "report_export_restricted"
	ReasonReportLineageMissing   = "report_lineage_missing"
	ReasonResourceVersionChanged = "resource_version_changed"
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

// adapterOutcome is what one adapter returns to the runner.
type adapterOutcome struct {
	result     EffectResult
	eventType  string
	resourceID string
}

func failed(reasonCode, eventType, resourceID string) adapterOutcome {
	return adapterOutcome{
		result:     EffectResult{Outcome: OutcomeFailed, ReasonCode: reasonCode},
		eventType:  eventType,
		resourceID: resourceID,
	}
}

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
		    WHERE id = $1 AND organization_id = $2 AND run_id = $3 AND tool = $4 AND action_digest = $5)`,
		request.ActionID, request.OrganizationID, request.RunID, request.Tool, request.ActionDigest[:],
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

// completeAttempt records the outcome on the attempt and writes one audit event with masked
// metadata only (references, never values).
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
	summary, err := json.Marshal(map[string]string{"tool": request.Tool, "resource": outcome.resourceID})
	if err != nil {
		return fmt.Errorf("tools: encode event summary: %w", err)
	}
	var reasonCode *string
	if outcome.result.ReasonCode != "" {
		reasonCode = &outcome.result.ReasonCode
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO runtime.audit_events
		   (organization_id, run_id, action_id, event_type, decision, reason_code, catalog_revision_id, masked_summary)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		request.OrganizationID, request.RunID, request.ActionID, outcome.eventType,
		outcome.result.Outcome, reasonCode, request.CatalogRevisionID, summary,
	)
	if err != nil {
		return fmt.Errorf("tools: write audit event: %w", err)
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
