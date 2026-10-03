package policy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// afterReviewExpiry makes the world's approvals see a clock past the frozen review's expiry.
func afterReviewExpiry(world *approvalWorld) *Approvals {
	approvals := NewApprovals(world.pool)
	approvals.now = func() time.Time { return world.scope.ExpiresAt.Add(time.Second) }
	return approvals
}

func TestOverdueApprovalClosesAsExpiredWithItsEventAndContinuation(t *testing.T) {
	ctx := context.Background()
	world := openApprovalWorld(t)
	organization := world.run.OrganizationID

	// Before the expiry nothing closes.
	if closed, err := NewApprovals(world.pool).expireOverdueIn(ctx, organization); err != nil || closed != 0 {
		t.Fatalf("before expiry: closed %d, err %v", closed, err)
	}
	approvals := afterReviewExpiry(world)
	closed, err := approvals.expireOverdueIn(ctx, organization)
	if err != nil || closed != 1 {
		t.Fatalf("closed %d, err %v; want 1", closed, err)
	}
	approvalRows, jobs, status := world.counts(t)
	if approvalRows != 0 || jobs != 1 || status != string(contracts.ActionExpired) {
		t.Fatalf("approvals %d, jobs %d, status %s; want 0, 1, expired", approvalRows, jobs, status)
	}
	var eventType, reason string
	var decision *string
	if err := world.pool.QueryRow(ctx, `SELECT event_type, decision, reason_code FROM runtime.audit_events
	                                     WHERE action_id = $1 ORDER BY id DESC LIMIT 1`, world.actionID).Scan(&eventType, &decision, &reason); err != nil {
		t.Fatal(err)
	}
	if eventType != string(contracts.EventApprovalDecided) || decision != nil || reason != string(contracts.ReasonApprovalExpired) {
		t.Fatalf("event %s decision %v reason %s", eventType, decision, reason)
	}
	// Idempotent, and a late reviewer decision finds the approval closed.
	if closed, err := approvals.expireOverdueIn(ctx, organization); err != nil || closed != 0 {
		t.Fatalf("second call: closed %d, err %v", closed, err)
	}
	if _, err := NewApprovals(world.pool).Decide(ctx, world.reviewer, world.actionID, ApprovalApprove); err != ErrApprovalClosed {
		t.Fatalf("late decision err = %v, want ErrApprovalClosed", err)
	}
	decided, found, err := approvals.DecidedActionFor(ctx, organization, world.run.RunID)
	if err != nil || !found || decided.ActionID != world.actionID || decided.Status != contracts.ActionExpired || decided.GrantOpen {
		t.Fatalf("decided action %+v, found %v, err %v", decided, found, err)
	}
}

func TestExpirySkipsAnApprovalBeingDecidedAndQueuesNothingForAFinishedRun(t *testing.T) {
	ctx := context.Background()
	world := openApprovalWorld(t)
	approvals := afterReviewExpiry(world)

	// A reviewer's transaction holds the action: the expiry skips it rather than racing it.
	reviewer, err := world.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reviewer.Exec(ctx, `SELECT 1 FROM runtime.actions WHERE id = $1 FOR UPDATE`, world.actionID); err != nil {
		t.Fatal(err)
	}
	closed, err := approvals.expireOverdueIn(ctx, world.run.OrganizationID)
	_ = reviewer.Rollback(ctx)
	if err != nil || closed != 0 {
		t.Fatalf("while held: closed %d, err %v; want 0", closed, err)
	}

	mustExec(t, world.pool, `UPDATE runtime.runs SET status = 'stopped', updated_at = now() WHERE id = $1`, world.run.RunID)
	if closed, err := approvals.expireOverdueIn(ctx, world.run.OrganizationID); err != nil || closed != 1 {
		t.Fatalf("finished run: closed %d, err %v; want 1", closed, err)
	}
	if _, jobs, status := world.counts(t); jobs != 0 || status != string(contracts.ActionExpired) {
		t.Fatalf("finished run: jobs %d, status %s; want 0, expired", jobs, status)
	}
}

func TestDecidedActionForReturnsOnlyTheActionAwaitingItsContinuation(t *testing.T) {
	ctx := context.Background()
	world := openApprovalWorld(t)
	approvals := NewApprovals(world.pool)
	organization, runID := world.run.OrganizationID, world.run.RunID

	// Still awaiting a reviewer: nothing to resume.
	if _, found, err := approvals.DecidedActionFor(ctx, organization, runID); err != nil || found {
		t.Fatalf("awaiting: found %v, err %v", found, err)
	}
	if _, err := approvals.Decide(ctx, world.reviewer, world.actionID, ApprovalApprove); err != nil {
		t.Fatal(err)
	}
	decided, found, err := approvals.DecidedActionFor(ctx, organization, runID)
	if err != nil || !found || decided.Status != contracts.ActionApproved || !decided.GrantOpen ||
		!decided.GrantExpiresAt.Equal(world.scope.ExpiresAt) || decided.Tool != ToolQueueReport || decided.StepNumber != world.nextStep {
		t.Fatalf("approved: %+v, found %v, err %v", decided, found, err)
	}
	var arguments QueueReportArguments
	if err := json.Unmarshal(decided.CanonicalArguments, &arguments); err != nil || arguments.ReportID != world.reportID {
		t.Fatalf("arguments %s, err %v", decided.CanonicalArguments, err)
	}
	// Another organization's view of the run finds nothing.
	if _, found, err := approvals.DecidedActionFor(ctx, testdb.ID(t), runID); err != nil || found {
		t.Fatalf("other organization: found %v, err %v", found, err)
	}
	// A later step supersedes the decided action.
	world.storeAction(t, ToolReadInvoice, `{"invoice_id":"`+world.invoiceID+`"}`)
	if _, found, err := approvals.DecidedActionFor(ctx, organization, runID); err != nil || found {
		t.Fatalf("superseded: found %v, err %v", found, err)
	}
	if _, _, err := approvals.DecidedActionFor(ctx, organization, "not-a-run"); err != ErrApprovalInvalid {
		t.Fatalf("malformed run id err = %v", err)
	}
}
