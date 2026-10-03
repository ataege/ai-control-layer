package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
)

// GO-29 through the loop: the correction after a denied internal export comes from
// policy.CorrectionCounter.Feedback, so it carries the fixed do-not-retry sentence, and once the
// run's vendor report is queued it says the permitted alternative is done (lane 3c's clean-clone
// run 8b59f19f retried the denied export until the corrections ran out).

// createInternalReport proposes the internal investigation report over the invoice with the note.
func createInternalReport(world *loopWorld) func([]model.Message) (StepResult, error) {
	return func([]model.Message) (StepResult, error) {
		arguments, _ := json.Marshal(map[string]any{"template": "internal_investigation_v1", "source_invoice_ids": []string{world.invoiceWithNote}})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolCreateReport, Arguments: arguments}}, nil
	}
}

// queueInternalReport proposes queueing the run's Internal only report to the registered recipient.
func queueInternalReport(t *testing.T, world *loopWorld) func([]model.Message) (StepResult, error) {
	return func([]model.Message) (StepResult, error) {
		var reportID string
		if err := world.pool.QueryRow(context.Background(), `SELECT id::text FROM demo.reports
			WHERE run_id = $1 AND classification = 'internal_only'`, world.runID).Scan(&reportID); err != nil {
			t.Errorf("no internal report to queue: %v", err)
			return StepResult{}, err
		}
		arguments, _ := json.Marshal(map[string]string{"report_id": reportID, "recipient_reference": "recipient:" + world.runID + ":" + world.vendorID})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolQueueReport, Arguments: arguments}}, nil
	}
}

// exportCorrection returns the stored correction after the denied internal export.
func exportCorrection(t *testing.T, world *loopWorld) policy.DenialFeedback {
	t.Helper()
	entries, err := NewContextStore(world.pool).List(context.Background(), world.organizationID, world.runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Kind != entryCorrection {
			continue
		}
		var feedback policy.DenialFeedback
		if json.Unmarshal(entry.Content, &feedback) == nil && feedback.ReasonCode == policy.ReasonReportExportRestricted {
			return feedback
		}
	}
	t.Fatalf("no report_export_restricted correction among %d context entries", len(entries))
	return policy.DenialFeedback{}
}

func TestDeniedExportFeedbackSaysDoNotRetryAndOffersTheVendorReport(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	stepper.script = []func([]model.Message) (StepResult, error){createInternalReport(world), queueInternalReport(t, world)}
	_, _ = newTestLoop(t, world, stepper).Handle(context.Background(), world.job()) // the exhausted script ends the run

	feedback := exportCorrection(t, world)
	if !strings.Contains(feedback.SafeMessage, "Do not propose this action again.") || feedback.AlternativeTemplate != "vendor_reconciliation_v1" ||
		strings.Contains(feedback.SafeMessage, "already been completed") {
		t.Fatalf("correction = %+v", feedback)
	}
}

func TestDeniedExportAfterTheVendorReportIsQueuedSaysFinish(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	ctx := context.Background()
	reviewer := addReviewer(t, world)
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	stepper.script = []func([]model.Message) (StepResult, error){
		createVendorReport(world), queueStoredReport(t, world),
		// After the approved vendor report is queued, the model creates and tries to export the internal one.
		createInternalReport(world), queueInternalReport(t, world),
	}
	actionID := waitForReview(t, world, stepper)
	if _, err := policy.NewApprovals(world.pool).Decide(ctx, reviewer, actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	_, _ = newTestLoop(t, world, stepper).Handle(ctx, world.job()) // resumes, queues, then the internal export is denied

	if outbox := takeDispatchRecord(t, world).outbox; outbox != 1 {
		t.Fatalf("outbox rows = %d, want the one approved vendor report", outbox)
	}
	feedback := exportCorrection(t, world)
	if feedback.AlternativeTemplate != "" || !strings.Contains(feedback.SafeMessage, "The permitted alternative has already been completed; finish with your final answer.") ||
		!strings.Contains(feedback.SafeMessage, "Do not propose this action again.") {
		t.Fatalf("correction = %+v", feedback)
	}
}
