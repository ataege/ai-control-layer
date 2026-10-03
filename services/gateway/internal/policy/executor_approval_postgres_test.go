package policy

import (
	"context"
	"sync"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/tools"
)

func (world *approvalWorld) approve(t *testing.T) {
	t.Helper()
	if _, err := NewApprovals(world.pool).Decide(context.Background(), world.reviewer, world.actionID, ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
}

func (world *approvalWorld) executor(revision int64) *Executor {
	return NewExecutor(world.pool, &fakeScopes{scope: world.scope, revision: revision}, tools.Runner{})
}

func (world *approvalWorld) outboxRows(t *testing.T) int {
	t.Helper()
	var rows int
	if err := world.pool.QueryRow(context.Background(), `SELECT count(*) FROM demo.outbox_messages WHERE action_id = $1`, world.actionID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func (world *approvalWorld) attemptCount(t *testing.T) int {
	t.Helper()
	var attempts int
	_ = world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.execution_attempts WHERE action_id = $1`, world.actionID).Scan(&attempts)
	return attempts
}

func TestApprovedActionExecutesOnceAndConsumesItsGrant(t *testing.T) {
	world := openApprovalWorld(t)
	world.approve(t)
	ctx := context.Background()
	result := world.executor(1).Execute(ctx, world.run, world.actionID)
	if result.Status != ExecutionSucceeded {
		t.Fatalf("status = %s/%s, want succeeded", result.Status, result.ReasonCode)
	}
	if rows := world.outboxRows(t); rows != 1 {
		t.Fatalf("outbox rows = %d, want 1", rows)
	}
	var consumedBy *string
	if err := world.pool.QueryRow(ctx, `SELECT consumed_by_attempt_id::text FROM runtime.approvals WHERE action_id = $1`, world.actionID).
		Scan(&consumedBy); err != nil || consumedBy == nil || *consumedBy != result.AttemptID {
		t.Fatalf("grant consumed by %v (err %v), want attempt %s", consumedBy, err, result.AttemptID)
	}
	again := world.executor(1).Execute(ctx, world.run, world.actionID)
	if again.Status != ExecutionRefused || world.outboxRows(t) != 1 {
		t.Fatalf("second execution = %s with %d outbox rows; want refused and still 1", again.Status, world.outboxRows(t))
	}
}

func TestApprovedActionRechecksBeforeExecution(t *testing.T) {
	cases := []struct {
		name       string
		change     func(*testing.T, *approvalWorld)
		revision   int64
		wantReason contracts.ReasonCode
	}{
		{"source version changed after approval", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE demo.invoices SET version = version + 1 WHERE id = $1`, world.invoiceID)
		}, 1, contracts.ReasonResourceVersionChanged},
		{"recipient address changed after approval", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE demo.vendors SET registered_reporting_address = 'other@atlas.example.com'
			                         WHERE organization_id = $1`, world.run.OrganizationID)
		}, 1, contracts.ReasonActionChanged},
		{"stored arguments changed after approval", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE runtime.actions SET canonical_arguments = $1 WHERE id = $2`,
				`{"report_id":"`+world.reportID+`","recipient_reference":"recipient:`+world.run.RunID+`:vendor_other"}`, world.actionID)
		}, 1, contracts.ReasonActionChanged},
		{"cancelled run", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE runtime.runs SET cancel_requested_at = now() WHERE id = $1`, world.run.RunID)
		}, 1, contracts.ReasonRunCancelled},
		{"active catalog revision changed", func(*testing.T, *approvalWorld) {}, 2, contracts.ReasonSourcePolicyChanged},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			world := openApprovalWorld(t)
			world.approve(t)
			testCase.change(t, world)
			result := world.executor(testCase.revision).Execute(context.Background(), world.run, world.actionID)
			if result.Status != ExecutionRefused || result.ReasonCode != testCase.wantReason {
				t.Fatalf("result = %s/%s, want refused/%s", result.Status, result.ReasonCode, testCase.wantReason)
			}
			if world.outboxRows(t) != 0 || world.attemptCount(t) != 0 {
				t.Fatalf("outbox %d, attempts %d; want nothing dispatched", world.outboxRows(t), world.attemptCount(t))
			}
		})
	}
}

func TestExpiredOrRejectedGrantExecutesNothing(t *testing.T) {
	t.Run("expired grant", func(t *testing.T) {
		world := openApprovalWorld(t)
		// An approval decided in the past whose expiry has passed, inserted as the owner.
		var payloadID string
		var digest []byte
		if err := world.pool.QueryRow(context.Background(),
			`SELECT p.id::text, a.action_digest FROM runtime.review_payloads p JOIN runtime.actions a ON a.id = p.action_id WHERE p.action_id = $1`,
			world.actionID).Scan(&payloadID, &digest); err != nil {
			t.Fatal(err)
		}
		mustExec(t, world.pool, `INSERT INTO runtime.approvals (organization_id, action_id, action_digest, review_payload_reference,
		                           reviewer_id, decision, decided_at, expires_at)
		                         VALUES ($1, $2, $3, $4, $5, 'approved', now() - interval '2 hours', now() - interval '1 hour')`,
			world.run.OrganizationID, world.actionID, digest, payloadID, world.reviewer.UserID)
		mustExec(t, world.pool, `UPDATE runtime.actions SET status = 'approved' WHERE id = $1`, world.actionID)
		result := world.executor(1).Execute(context.Background(), world.run, world.actionID)
		if result.Status != ExecutionRefused || result.ReasonCode != contracts.ReasonApprovalExpired || world.outboxRows(t) != 0 {
			t.Fatalf("result = %s/%s with %d outbox rows", result.Status, result.ReasonCode, world.outboxRows(t))
		}
	})
	t.Run("rejected grant", func(t *testing.T) {
		world := openApprovalWorld(t)
		if _, err := NewApprovals(world.pool).Decide(context.Background(), world.reviewer, world.actionID, ApprovalReject); err != nil {
			t.Fatal(err)
		}
		result := world.executor(1).Execute(context.Background(), world.run, world.actionID)
		if result.Status != ExecutionRefused || world.outboxRows(t) != 0 {
			t.Fatalf("result = %s with %d outbox rows", result.Status, world.outboxRows(t))
		}
	})
}

func TestConcurrentExecutionsConsumeTheGrantOnce(t *testing.T) {
	world := openApprovalWorld(t)
	world.approve(t)
	const executions = 4
	results := make([]ExecutionResult, executions)
	var waitGroup sync.WaitGroup
	for index := range executions {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			results[index] = world.executor(1).Execute(context.Background(), world.run, world.actionID)
		}()
	}
	waitGroup.Wait()
	succeeded := 0
	for _, result := range results {
		if result.Status == ExecutionSucceeded {
			succeeded++
		}
	}
	var consumed int
	_ = world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.approvals WHERE action_id = $1 AND consumed_at IS NOT NULL`, world.actionID).Scan(&consumed)
	if succeeded != 1 || world.outboxRows(t) != 1 || consumed != 1 {
		t.Fatalf("succeeded %d, outbox rows %d, consumed grants %d; want 1, 1, 1 (results %+v)", succeeded, world.outboxRows(t), consumed, results)
	}
}
