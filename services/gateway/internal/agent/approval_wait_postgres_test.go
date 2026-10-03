package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/worker"
)

// GO-40: an approval wait holds no worker, survives a restart and resumes the original stored action.

// reviewWait is a run waiting for review of its queue_report action, plus a seeded reviewer.
type reviewWait struct {
	world    *loopWorld
	reviewer contracts.OperatorContext
	actionID string
	reportID string
}

// newReviewWait runs a claim that creates a vendor report and proposes queuing it, which the gate
// sends to review.
func newReviewWait(t *testing.T) reviewWait {
	t.Helper()
	world := newLoopWorld(t, passportOptions{})
	world.results = PoolFinalResults{Pool: world.pool}
	wait := reviewWait{world: world, reviewer: seedReviewer(t, world)}
	createVendorReport := func([]model.Message) (StepResult, error) {
		arguments, _ := json.Marshal(map[string]any{"template": "vendor_reconciliation_v1",
			"source_invoice_ids": []string{world.invoiceWithNote, world.invoiceClean}})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolCreateReport, Arguments: arguments}}, nil
	}
	queueIt := func(messages []model.Message) (StepResult, error) {
		var created struct {
			ReportID string `json:"report_id"`
		}
		_ = json.Unmarshal([]byte(messages[len(messages)-1].Content), &created)
		wait.reportID = created.ReportID
		arguments, _ := json.Marshal(map[string]string{"report_id": created.ReportID,
			"recipient_reference": "recipient:" + world.runID + ":" + world.vendorID})
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: contracts.ToolQueueReport, Arguments: arguments}}, nil
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){createVendorReport, queueIt}}
	outcome, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job())
	if err != nil || outcome != worker.Completed() {
		t.Fatalf("first claim: %v %v", outcome, err)
	}
	assertRunEnded(t, world, contracts.RunAwaitingApproval, "")
	if err = world.pool.QueryRow(context.Background(), `SELECT id::text FROM runtime.actions
		WHERE organization_id = $1 AND tool = 'queue_report' AND status = 'awaiting_approval'`, world.organizationID).Scan(&wait.actionID); err != nil {
		t.Fatalf("no queue_report waiting for review: %v", err)
	}
	return wait
}

// seedReviewer adds an organization, a user and a reviewer membership for the world.
func seedReviewer(t *testing.T, world *loopWorld) contracts.OperatorContext {
	t.Helper()
	userID := testdb.ID(t)
	world.exec(t, `INSERT INTO app.organizations (id, name) VALUES ($1, $2)`, world.organizationID, "Org "+world.organizationID[:8])
	world.exec(t, `INSERT INTO app.users (id, email, name) VALUES ($1, $2, 'Reviewer')`, userID, userID+"@example.test")
	world.exec(t, `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, $3)`,
		userID, world.organizationID, []string{policy.ReviewerRole})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, statement := range []string{
			`DELETE FROM app.memberships WHERE "organizationId" = $1`,
			`DELETE FROM app.organizations WHERE id = $1`,
		} {
			if _, err := world.pool.Exec(ctx, statement, world.organizationID); err != nil {
				t.Errorf("clean reviewer fixture: %v", err)
			}
		}
		if _, err := world.pool.Exec(ctx, `DELETE FROM app.users WHERE id = $1`, userID); err != nil {
			t.Errorf("clean reviewer user: %v", err)
		}
	})
	return contracts.OperatorContext{UserID: userID, OrganizationID: world.organizationID, Roles: []string{policy.ReviewerRole}}
}

// continuationJob returns the queued job that resumes the run.
func (wait reviewWait) continuationJob(t *testing.T) worker.Job {
	t.Helper()
	var jobID string
	if err := wait.world.pool.QueryRow(context.Background(), `SELECT id::text FROM runtime.jobs
		WHERE organization_id = $1 AND run_id = $2 AND status = 'queued'`, wait.world.organizationID, wait.world.runID).Scan(&jobID); err != nil {
		t.Fatalf("no continuation job: %v", err)
	}
	return worker.Job{ID: jobID, OrganizationID: wait.world.organizationID, RunID: wait.world.runID, Kind: contracts.JobKindAgentStep}
}

// finishNamingTheReport is the final answer naming the run's report.
func (wait *reviewWait) finishNamingTheReport([]model.Message) (StepResult, error) {
	return StepResult{Kind: StepFinal, FinalAnswer: `{"status":"completed","report_ids":["` + wait.reportID + `"]}`}, nil
}

func TestReviewWaitHoldsNoWorkerAndResumesTheApprovedActionAfterARestart(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	// No worker holds the run while it waits.
	if leased := world.count(t, "SELECT count(*) FROM runtime.jobs WHERE organization_id = $1 AND lease_owner IS NOT NULL"); leased != 0 {
		t.Fatalf("%d jobs hold a lease during the review wait", leased)
	}
	if waiting := world.count(t, "SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'run.awaiting_approval'"); waiting != 1 {
		t.Fatalf("%d run.awaiting_approval events", waiting)
	}
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var digestBefore []byte
	mustScan(t, world.pool.QueryRow(context.Background(), "SELECT action_digest FROM runtime.actions WHERE id = $1", wait.actionID), &digestBefore)

	// A new worker process claims the continuation.
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){wait.finishNamingTheReport}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	var status string
	var digestAfter []byte
	if err := world.pool.QueryRow(context.Background(), "SELECT status, action_digest FROM runtime.actions WHERE id = $1", wait.actionID).
		Scan(&status, &digestAfter); err != nil || status != "succeeded" || string(digestAfter) != string(digestBefore) {
		t.Fatalf("the approved stored action: status %s, digest changed %v, %v", status, string(digestAfter) != string(digestBefore), err)
	}
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 1 {
		t.Fatalf("%d outbox messages, want exactly one", outbox)
	}
	if queues := world.count(t, "SELECT count(*) FROM runtime.actions WHERE organization_id = $1 AND tool = 'queue_report'"); queues != 1 {
		t.Fatalf("%d queue_report actions: the resume made a new proposal", queues)
	}
	if resumed := world.count(t, "SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'run.resumed'"); resumed != 1 {
		t.Fatalf("%d run.resumed events", resumed)
	}
}

func TestRejectedActionExecutesNothingAndIsACountedCorrection(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalReject); err != nil {
		t.Fatalf("reject: %v", err)
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){wait.finishNamingTheReport}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 0 {
		t.Fatalf("a rejected action queued %d messages", outbox)
	}
	if denials := world.count(t, "SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'action.denied'"); denials != 1 {
		t.Fatalf("%d denial events, want the rejection counted once", denials)
	}
	request := stepper.contexts[0]
	if feedback := request[len(request)-1]; feedback.Role != "tool" || !strings.Contains(feedback.Content, "reviewer rejected") {
		t.Fatalf("rejection feedback: %+v", feedback)
	}
}

func TestUndecidedApprovalExpiresWhileNoWorkerHoldsTheRun(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	// The frozen review's expiry passes while no worker holds the job.
	// Frozen reviews reject UPDATE by trigger; the test moves the expiry with triggers disabled for one
	// transaction (superuser test database), standing in for the passage of time.
	transaction, err := world.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = transaction.Exec(context.Background(), "SET LOCAL session_replication_role = replica"); err == nil {
		_, err = transaction.Exec(context.Background(), "UPDATE runtime.review_payloads SET created_at = now() - interval '2 seconds', expires_at = now() - interval '1 second' WHERE action_id = $1", wait.actionID)
	}
	if err != nil {
		_ = transaction.Rollback(context.Background())
		t.Fatalf("move the review expiry: %v", err)
	}
	if err = transaction.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = policy.NewApprovals(world.pool).ExpireOverdue(context.Background()); err != nil {
		t.Fatalf("expire overdue: %v", err)
	}
	var status string
	if err := world.pool.QueryRow(context.Background(), "SELECT status FROM runtime.actions WHERE id = $1", wait.actionID).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("action after expiry: %s %v", status, err)
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){wait.finishNamingTheReport}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 0 {
		t.Fatalf("an expired action queued %d messages", outbox)
	}
	if expired := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1
		AND event_type = 'approval.decided' AND reason_code = 'approval_expired'`); expired != 1 {
		t.Fatalf("%d expiry events", expired)
	}
	request := stepper.contexts[0]
	if feedback := request[len(request)-1]; !strings.Contains(feedback.Content, "approval_expired") {
		t.Fatalf("expiry feedback: %+v", feedback)
	}
}

func TestApprovedActionOfACancelledRunDoesNotResume(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// The operator cancels after the approval and before the continuation is claimed.
	world.exec(t, "UPDATE runtime.runs SET cancel_requested_at = now() WHERE id = $1", world.runID)
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunCancelled)
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 0 {
		t.Fatalf("a cancelled run queued %d messages", outbox)
	}
	if len(stepper.contexts) != 0 {
		t.Fatal("a cancelled run made a model request")
	}
}

// refusingExecutor stands in for an executor whose fresh check fails before dispatch.
type refusingExecutor struct{ reason policy.ReasonCode }

func (executor refusingExecutor) Execute(context.Context, policy.RunIdentity, string) policy.ExecutionResult {
	return policy.ExecutionResult{Status: policy.ExecutionRefused, ReasonCode: executor.reason}
}

func TestApprovedActionWhoseRecheckFailsIsACountedCorrection(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){wait.finishNamingTheReport}}
	loop := newTestLoop(t, world, stepper)
	// The catalog revision changed between the approval and the resume.
	loop.dependencies.Executor = refusingExecutor{reason: policy.ReasonCode(contracts.ReasonSourcePolicyChanged)}
	if _, err := loop.Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	var denials int
	if err := world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1
		AND event_type = 'action.denied' AND action_id = $2 AND reason_code = 'source_policy_changed'`, world.organizationID, wait.actionID).
		Scan(&denials); err != nil || denials != 1 {
		t.Fatalf("%d counted denials for the refused approved action (%v)", denials, err)
	}
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 0 {
		t.Fatalf("a refused action queued %d messages", outbox)
	}
	request := stepper.contexts[0]
	if feedback := request[len(request)-1]; feedback.Role != "tool" || !strings.Contains(feedback.Content, "source_policy_changed") {
		t.Fatalf("recheck feedback: %+v", feedback)
	}
}

func TestExpiredRunStopsFromTheWaitWithoutAResumedEvent(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	loop := newTestLoop(t, world, stepper)
	// The passport expired while the run waited.
	loop.now = func() time.Time { return time.Now().Add(24 * time.Hour) }
	if _, err := loop.Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunExpired)
	if resumed := world.count(t, "SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'run.resumed'"); resumed != 0 {
		t.Fatalf("%d run.resumed events for a run that never worked again", resumed)
	}
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 0 || len(stepper.contexts) != 0 {
		t.Fatalf("an expired run queued %d messages or made %d model requests", outbox, len(stepper.contexts))
	}
}

// GO-80: the resume records the review wait as an approval_wait span, from the awaiting transition
// to the resume, measured on the loop's clock.
func TestResumeRecordsTheApprovalWaitSpan(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var awaitingSince time.Time
	if err := world.pool.QueryRow(context.Background(), "SELECT updated_at FROM runtime.runs WHERE id = $1", world.runID).Scan(&awaitingSince); err != nil {
		t.Fatal(err)
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){wait.finishNamingTheReport}}
	loop := newTestLoop(t, world, stepper)
	loop.now = func() time.Time { return awaitingSince.Add(90 * time.Second) }
	if _, err := loop.Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	var spans int
	var duration int64
	var actionID string
	if err := world.pool.QueryRow(context.Background(), `SELECT count(*), coalesce(max(duration_microseconds), 0), coalesce(max(action_id::text), '')
		FROM runtime.timing_records WHERE organization_id = $1 AND phase = 'approval_wait'`, world.organizationID).Scan(&spans, &duration, &actionID); err != nil {
		t.Fatal(err)
	}
	if spans != 1 || duration != (90*time.Second).Microseconds() || actionID != wait.actionID {
		t.Fatalf("approval_wait spans %d, duration %d µs, action %s (want 1, 90 s, %s)", spans, duration, actionID, wait.actionID)
	}
}
