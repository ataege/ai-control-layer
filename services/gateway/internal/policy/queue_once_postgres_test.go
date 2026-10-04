package policy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/testdb"
)

// A report is queued once per run: once a queue_report of this run has succeeded for a report (its
// outbox row exists), another queue_report for that report is denied before review as
// tool_not_allowed, so a second approval can never queue a second outbox row. Scope, destination
// and export checks still decide first, and a cancelled or rejected first queue blocks nothing.

// proposeQueue evaluates a queue_report proposal of the world's run and returns the decision and the
// stored status of the action.
func (world *approvalWorld) proposeQueue(t *testing.T, reportID, recipientReference string) (Decision, string, string) {
	t.Helper()
	ctx := context.Background()
	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
		NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
	world.nextStep++
	actionID := testdb.ID(t)
	decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: actionID, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "queue_report", RawArguments: json.RawMessage(`{"report_id":"` + reportID + `","recipient_reference":"` + recipientReference + `"}`)})
	var status string
	mustScan(t, world.pool.QueryRow(ctx, `SELECT status FROM runtime.actions WHERE id = $1`, actionID), &status)
	return decision, status, actionID
}

func (world *approvalWorld) outboxRowsForReport(t *testing.T, reportID string) int {
	t.Helper()
	var rows int
	mustScan(t, world.pool.QueryRow(context.Background(), `SELECT count(*) FROM demo.outbox_messages WHERE report_id = $1`, reportID), &rows)
	return rows
}

// queuedOnce runs the world's awaiting queue_report through approval and execution.
func (world *approvalWorld) queuedOnce(t *testing.T) {
	t.Helper()
	world.approve(t)
	if result := world.executor(1).Execute(context.Background(), world.run, world.actionID); result.Status != ExecutionSucceeded {
		t.Fatalf("the first queue = %s/%s, want succeeded", result.Status, result.ReasonCode)
	}
	if rows := world.outboxRowsForReport(t, world.reportID); rows != 1 {
		t.Fatalf("outbox rows after the first queue = %d, want 1", rows)
	}
}

// internalReport stores a genuine Internal only report of this run.
func (world *approvalWorld) internalReport(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	creatingAction := world.storeAction(t, ToolCreateReport, `{"template":"internal_investigation_v1","source_invoice_ids":["`+world.invoiceID+`"]}`)
	tx, err := world.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	internal, err := provenance.StoreReport(ctx, tx, provenance.NewReport{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, CreatedByActionID: creatingAction,
		Template: provenance.InternalInvestigationV1,
		Sources: []provenance.Source{{Kind: provenance.SourceInvoice, ID: world.invoiceID, Version: 1,
			Classification: provenance.InternalOnly, ConsumedFields: []string{provenance.FieldInvoiceID, provenance.FieldInternalNote}}},
		Title: "Internal note", Content: "Investigation note: INV104 appears twice.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return internal.ID
}

func TestSecondQueueOfAnAlreadyQueuedReportIsDeniedBeforeReview(t *testing.T) {
	world := openApprovalWorld(t)
	world.queuedOnce(t)

	decision, status, actionID := world.proposeQueue(t, world.reportID, world.reference)
	if decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonToolNotAllowed || !decision.ReportAlreadyQueued {
		t.Fatalf("second queue = %s/%s (already queued %v), want deny/tool_not_allowed", decision.Outcome, decision.ReasonCode, decision.ReportAlreadyQueued)
	}
	if status == actionStatusAwaitingApproval {
		t.Fatal("the second queue was left awaiting a second approval")
	}
	var approvals int
	mustScan(t, world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.approvals WHERE action_id = $1`, actionID), &approvals)
	if approvals != 0 || world.outboxRowsForReport(t, world.reportID) != 1 {
		t.Fatalf("approvals for the second queue = %d, outbox rows = %d; want 0 and 1", approvals, world.outboxRowsForReport(t, world.reportID))
	}
	// The model is told why, in words that name the cause, not only that the tool is not permitted.
	if message := BuildDenialFeedback(decision, world.scope).SafeMessage; !strings.Contains(message, "already queued") {
		t.Errorf("feedback %q does not say the report was already queued", message)
	}
}

func TestTheFirstQueueStillGoesToReviewAndANonSucceededOneBlocksNothing(t *testing.T) {
	world := openApprovalWorld(t) // its queue_report is awaiting approval (the first queue went to review)

	// Still awaiting review: a further proposal for the report is not blocked.
	if decision, status, _ := world.proposeQueue(t, world.reportID, world.reference); decision.Outcome != OutcomeApprovalRequired || status != actionStatusAwaitingApproval {
		t.Fatalf("proposal while the first awaits = %s/%s (%s), want approval_required", decision.Outcome, decision.ReasonCode, status)
	}

	// A rejected first queue blocks nothing: no outbox row exists.
	if _, err := NewApprovals(world.pool).Decide(context.Background(), world.reviewer, world.actionID, ApprovalReject); err != nil {
		t.Fatal(err)
	}
	if decision, status, _ := world.proposeQueue(t, world.reportID, world.reference); decision.Outcome != OutcomeApprovalRequired || status != actionStatusAwaitingApproval {
		t.Fatalf("proposal after a rejected first queue = %s/%s (%s), want approval_required", decision.Outcome, decision.ReasonCode, status)
	}
	if world.outboxRowsForReport(t, world.reportID) != 0 {
		t.Fatal("an outbox row exists although nothing was queued")
	}
}

// After the vendor report is queued, a replay or a hostile re-proposal keeps its own reason: the
// scope, destination and export checks run first (beats 5 and 9 of the demonstration).
func TestScopeDestinationAndExportReasonsWinAfterAQueue(t *testing.T) {
	world := openApprovalWorld(t)
	world.queuedOnce(t)

	internal := world.internalReport(t)
	if decision, _, _ := world.proposeQueue(t, internal, world.reference); decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonReportExportRestricted {
		t.Errorf("internal report after a queue = %s/%s, want deny/report_export_restricted", decision.Outcome, decision.ReasonCode)
	}
	otherRecipient := "recipient:" + world.run.RunID + ":vendor_nobody"
	if decision, _, _ := world.proposeQueue(t, world.reportID, otherRecipient); decision.Outcome != OutcomeDeny || decision.ReasonCode != ReasonDestinationNotAllowed {
		t.Errorf("another recipient after a queue = %s/%s, want deny/destination_not_allowed", decision.Outcome, decision.ReasonCode)
	}
}

// Two proposals for the same report can be awaiting at once; the first is approved and executed, and
// the second, approved afterwards, is refused at execution: no second outbox row.
func TestAnApprovedSecondQueueIsRefusedAtExecution(t *testing.T) {
	world := openApprovalWorld(t)
	ctx := context.Background()
	decision, _, secondAction := world.proposeQueue(t, world.reportID, world.reference)
	if decision.Outcome != OutcomeApprovalRequired {
		t.Fatalf("setup: the second proposal = %s/%s, want approval_required", decision.Outcome, decision.ReasonCode)
	}
	world.queuedOnce(t)
	if _, err := NewApprovals(world.pool).Decide(ctx, world.reviewer, secondAction, ApprovalApprove); err != nil {
		t.Fatalf("approving the second queue: %v", err)
	}
	result := world.executor(1).Execute(ctx, world.run, secondAction)
	if result.Status != ExecutionRefused || result.ReasonCode != ReasonToolNotAllowed {
		t.Fatalf("second approved queue = %s/%s, want refused/tool_not_allowed", result.Status, result.ReasonCode)
	}
	if rows := world.outboxRowsForReport(t, world.reportID); rows != 1 {
		t.Fatalf("outbox rows = %d, want 1", rows)
	}
	var attempts int
	mustScan(t, world.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.execution_attempts WHERE action_id = $1`, secondAction), &attempts)
	if attempts != 0 {
		t.Fatalf("the refused second queue consumed %d attempts", attempts)
	}
}
