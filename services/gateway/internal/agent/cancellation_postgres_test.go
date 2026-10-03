package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/tools"
)

// GO-51 (X-55, Go half): cancellation and expiry stop every later dispatch, also after a review
// wait, while effects committed before the stop stay recorded. Each scenario runs the production
// loop (newTestLoop: the gate with the review freezer, the executor, adapters, recovery and the
// GO-40 continuation) on the X-34-shaped loop world; only the model is a scripted test double, and
// its dispatches are counted on the run's ledger.

// dispatchRecord is what X-55 asks to capture: dispatches, attempts, effects and the stop.
type dispatchRecord struct {
	modelCalls, attempts, succeededAttempts, reports, outbox int
}

func takeDispatchRecord(t *testing.T, world *loopWorld) dispatchRecord {
	t.Helper()
	return dispatchRecord{
		modelCalls:        world.count(t, "SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1"),
		attempts:          world.count(t, "SELECT count(*) FROM runtime.execution_attempts WHERE organization_id = $1"),
		succeededAttempts: world.count(t, "SELECT count(*) FROM runtime.execution_attempts WHERE organization_id = $1 AND outcome = 'succeeded'"),
		reports:           world.count(t, "SELECT count(*) FROM demo.reports WHERE organization_id = $1"),
		outbox:            world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"),
	}
}

// stopTimestamps returns when the cancellation was requested and when the run stopped.
func stopTimestamps(t *testing.T, world *loopWorld) (cancelRequestedAt *time.Time, stoppedAt time.Time) {
	t.Helper()
	if err := world.pool.QueryRow(context.Background(),
		`SELECT r.cancel_requested_at, (SELECT max(e.occurred_at) FROM runtime.audit_events e WHERE e.run_id = r.id AND e.event_type = 'run.stopped')
		   FROM runtime.runs r WHERE r.id = $1`, world.runID).Scan(&cancelRequestedAt, &stoppedAt); err != nil {
		t.Fatalf("stop timestamps: %v", err)
	}
	return cancelRequestedAt, stoppedAt
}

// addReviewer gives the world's organization a reviewer, as the API's membership records do.
func addReviewer(t *testing.T, world *loopWorld) contracts.OperatorContext {
	t.Helper()
	reviewerID := testdb.ID(t)
	world.exec(t, `INSERT INTO app.organizations (id, name) VALUES ($1, 'Cancellation scenario')`, world.organizationID)
	world.exec(t, `INSERT INTO app.users (id, email, name) VALUES ($1, $2, 'Reviewer')`, reviewerID, reviewerID+"@example.test")
	world.exec(t, `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, $3)`,
		reviewerID, world.organizationID, []string{"operator", "reviewer"})
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = world.pool.Exec(ctx, `DELETE FROM app.memberships WHERE "organizationId" = $1`, world.organizationID)
		_, _ = world.pool.Exec(ctx, `DELETE FROM app.users WHERE id = $1`, reviewerID)
		_, _ = world.pool.Exec(ctx, `DELETE FROM app.organizations WHERE id = $1`, world.organizationID)
	})
	return contracts.OperatorContext{UserID: reviewerID, OrganizationID: world.organizationID, Roles: []string{"operator", "reviewer"}}
}

// createVendorReport proposes the vendor report over the clean invoice.
func createVendorReport(world *loopWorld) func([]model.Message) (StepResult, error) {
	return func([]model.Message) (StepResult, error) {
		arguments, _ := json.Marshal(map[string]any{"template": "vendor_reconciliation_v1", "source_invoice_ids": []string{world.invoiceClean}})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolCreateReport, Arguments: arguments}}, nil
	}
}

// queueStoredReport proposes queueing the run's stored report to its registered recipient.
func queueStoredReport(t *testing.T, world *loopWorld) func([]model.Message) (StepResult, error) {
	return func([]model.Message) (StepResult, error) {
		var reportID string
		if err := world.pool.QueryRow(context.Background(), `SELECT id::text FROM demo.reports WHERE run_id = $1`, world.runID).Scan(&reportID); err != nil {
			t.Errorf("no stored report to queue: %v", err)
			return StepResult{}, err
		}
		arguments, _ := json.Marshal(map[string]string{"report_id": reportID, "recipient_reference": "recipient:" + world.runID + ":" + world.vendorID})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolQueueReport, Arguments: arguments}}, nil
	}
}

// waitForReview runs the loop until the queue_report proposal waits for a reviewer and returns
// the awaiting action.
func waitForReview(t *testing.T, world *loopWorld, stepper *scriptedStepper) string {
	t.Helper()
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if state := world.runState(t); state.Status != contracts.RunAwaitingApproval {
		t.Fatalf("run status %s (reason %v), want awaiting_approval", state.Status, state.TerminalReason)
	}
	var actionID string
	if err := world.pool.QueryRow(context.Background(), `SELECT id::text FROM runtime.actions WHERE run_id = $1 AND status = 'awaiting_approval'`,
		world.runID).Scan(&actionID); err != nil {
		t.Fatalf("awaiting action: %v", err)
	}
	return actionID
}

func TestCancellationDuringAModelRequestExecutesNothingAfterIt(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	ctx := context.Background()
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	stepper.script = []func([]model.Message) (StepResult, error){
		readInvoice(world.invoiceClean),
		// The operator cancels while the second model request is in flight; the response still
		// proposes a permitted read.
		func(messages []model.Message) (StepResult, error) {
			if _, err := world.repository.CancelRun(ctx, world.organizationID, world.runID); err != nil {
				t.Errorf("cancel: %v", err)
			}
			return readInvoice(world.invoiceWithNote)(messages)
		},
		finalAnswer,
	}
	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil {
		t.Fatalf("handle: %v", err)
	}
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunCancelled)
	record := takeDispatchRecord(t, world)
	// Two model requests, one executed read before the cancellation, nothing after it.
	if len(stepper.contexts) != 2 || record.modelCalls != 2 || record.attempts != 1 || record.succeededAttempts != 1 {
		t.Fatalf("requests %d, record %+v; want 2 requests, 2 model calls, 1 succeeded attempt", len(stepper.contexts), record)
	}
	cancelRequestedAt, stoppedAt := stopTimestamps(t, world)
	if cancelRequestedAt == nil || stoppedAt.Before(*cancelRequestedAt) {
		t.Fatalf("cancel requested %v, stopped %v", cancelRequestedAt, stoppedAt)
	}
	// A late continuation finds a stopped run and dispatches nothing.
	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil || len(stepper.contexts) != 2 {
		t.Fatalf("late continuation: err %v, %d requests", err, len(stepper.contexts))
	}
	t.Logf("evidence X-55 cancel in flight: cancel_requested_at %s, run.stopped %s, model calls %d, attempts %d (succeeded %d, before the cancel), proposal after the cancel not executed",
		cancelRequestedAt.Format(time.RFC3339Nano), stoppedAt.Format(time.RFC3339Nano), record.modelCalls, record.attempts, record.succeededAttempts)
}

func TestCancellationDuringAReviewWaitStopsBeforeAnyDecisionOrDispatch(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	ctx := context.Background()
	reviewer := addReviewer(t, world)
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	stepper.script = []func([]model.Message) (StepResult, error){createVendorReport(world), queueStoredReport(t, world), finalAnswer}
	actionID := waitForReview(t, world, stepper)
	before := takeDispatchRecord(t, world)

	if _, err := world.repository.CancelRun(ctx, world.organizationID, world.runID); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunCancelled)
	// No grant can be stored for a cancelled run, and the stored action cannot be executed.
	if _, err := policy.NewApprovals(world.pool).Decide(ctx, reviewer, actionID, policy.ApprovalApprove); !errors.Is(err, policy.ErrApprovalRunStopped) {
		t.Fatalf("decision after cancel: err %v, want ErrApprovalRunStopped", err)
	}
	execution := policy.NewExecutor(world.pool, testScopes{repository: world.repository}, tools.Runner{}).
		Execute(ctx, policy.RunIdentity{OrganizationID: world.organizationID, RunID: world.runID}, actionID)
	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil {
		t.Fatal(err)
	}
	after := takeDispatchRecord(t, world)
	if execution.Status != policy.ExecutionRefused || after != before || len(stepper.contexts) != 2 ||
		world.count(t, "SELECT count(*) FROM runtime.approvals WHERE organization_id = $1") != 0 {
		t.Fatalf("execution %s/%s, record %+v -> %+v, %d requests", execution.Status, execution.ReasonCode, before, after, len(stepper.contexts))
	}
	cancelRequestedAt, stoppedAt := stopTimestamps(t, world)
	t.Logf("evidence X-55 cancel in review wait: cancel_requested_at %s, run.stopped %s, decision refused (run stopped), execution %s/%s, record %+v unchanged (report kept, outbox 0)",
		cancelRequestedAt.Format(time.RFC3339Nano), stoppedAt.Format(time.RFC3339Nano), execution.Status, execution.ReasonCode, after)
}

func TestCancellationAfterApprovalStopsTheApprovedAction(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	ctx := context.Background()
	reviewer := addReviewer(t, world)
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	stepper.script = []func([]model.Message) (StepResult, error){createVendorReport(world), queueStoredReport(t, world), finalAnswer}
	actionID := waitForReview(t, world, stepper)
	if _, err := policy.NewApprovals(world.pool).Decide(ctx, reviewer, actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	before := takeDispatchRecord(t, world)

	// The cancellation lands after the approval and before the continuation runs.
	if _, err := world.repository.CancelRun(ctx, world.organizationID, world.runID); err != nil {
		t.Fatal(err)
	}
	execution := policy.NewExecutor(world.pool, testScopes{repository: world.repository}, tools.Runner{}).
		Execute(ctx, policy.RunIdentity{OrganizationID: world.organizationID, RunID: world.runID}, actionID)
	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil {
		t.Fatal(err)
	}
	after := takeDispatchRecord(t, world)
	var grantConsumed bool
	if err := world.pool.QueryRow(ctx, `SELECT consumed_at IS NOT NULL FROM runtime.approvals WHERE action_id = $1`, actionID).Scan(&grantConsumed); err != nil {
		t.Fatal(err)
	}
	if execution.Status != policy.ExecutionRefused || execution.ReasonCode != policy.ReasonRunCancelled ||
		after != before || after.outbox != 0 || grantConsumed || len(stepper.contexts) != 2 {
		t.Fatalf("execution %s/%s, record %+v -> %+v, grant consumed %v, %d requests",
			execution.Status, execution.ReasonCode, before, after, grantConsumed, len(stepper.contexts))
	}
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunCancelled)
	cancelRequestedAt, stoppedAt := stopTimestamps(t, world)
	t.Logf("evidence X-55 cancel after approval: cancel_requested_at %s, run.stopped %s, approved action refused %s, grant unconsumed, record %+v unchanged",
		cancelRequestedAt.Format(time.RFC3339Nano), stoppedAt.Format(time.RFC3339Nano), execution.ReasonCode, after)
}

func TestExpiryBetweenStepsStopsBeforeTheNextModelRequest(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	ctx := context.Background()
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	loop := newTestLoop(t, world, stepper)
	passport, err := world.repository.Passport(ctx, world.organizationID, world.runID)
	if err != nil {
		t.Fatal(err)
	}
	stepper.script = []func([]model.Message) (StepResult, error){
		// The first read executes; the passport's expiry passes before the next step.
		func(messages []model.Message) (StepResult, error) {
			loop.now = func() time.Time { return passport.ExpiresAt.Add(time.Second) }
			return readInvoice(world.invoiceClean)(messages)
		},
		finalAnswer,
	}
	if _, err := loop.Handle(ctx, world.job()); err != nil {
		t.Fatalf("handle: %v", err)
	}
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunExpired)
	record := takeDispatchRecord(t, world)
	if len(stepper.contexts) != 1 || record.modelCalls != 1 || record.attempts != 1 || record.succeededAttempts != 1 {
		t.Fatalf("requests %d, record %+v; want 1 request and the 1 read before the expiry", len(stepper.contexts), record)
	}
	_, stoppedAt := stopTimestamps(t, world)
	t.Logf("evidence X-55 expiry between steps: passport expires_at %s, run.stopped %s (loop clock past expiry), model calls %d, attempts %d before the expiry, no request after",
		passport.ExpiresAt.Format(time.RFC3339Nano), stoppedAt.Format(time.RFC3339Nano), record.modelCalls, record.attempts)
}
