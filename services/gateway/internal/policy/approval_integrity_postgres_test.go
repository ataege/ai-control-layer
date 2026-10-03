package policy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// grantState returns whether the action's grant exists and whether it was consumed.
func (world *approvalWorld) grantState(t *testing.T) (exists, consumed bool) {
	t.Helper()
	var consumedAt *string
	err := world.pool.QueryRow(context.Background(), `SELECT consumed_at::text FROM runtime.approvals WHERE action_id = $1`, world.actionID).Scan(&consumedAt)
	if err != nil {
		return false, false
	}
	return true, consumedAt != nil
}

// TestApprovalIntegrity is the X-45 evidence: after an approval, each tampering is refused with
// its reason, no outbox row exists and the original grant stays unconsumed.
func TestApprovalIntegrity(t *testing.T) {
	cases := []struct {
		name       string
		tamper     func(*testing.T, *approvalWorld)
		wantReason contracts.ReasonCode
	}{
		{"changed recipient address", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE demo.vendors SET registered_reporting_address = 'changed@atlas.example.com' WHERE organization_id = $1`, world.run.OrganizationID)
		}, contracts.ReasonActionChanged},
		{"changed recipient reference", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE runtime.actions SET canonical_arguments = $1 WHERE id = $2`,
				`{"report_id":"`+world.reportID+`","recipient_reference":"recipient:`+world.run.RunID+`:vendor_other"}`, world.actionID)
		}, contracts.ReasonActionChanged},
		{"changed content (another report)", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE runtime.actions SET canonical_arguments = $1 WHERE id = $2`,
				`{"report_id":"`+testdb.ID(t)+`","recipient_reference":"`+world.reference+`"}`, world.actionID)
		}, contracts.ReasonActionChanged},
		{"changed source record version", func(t *testing.T, world *approvalWorld) {
			mustExec(t, world.pool, `UPDATE demo.invoices SET version = version + 1 WHERE id = $1`, world.invoiceID)
		}, contracts.ReasonResourceVersionChanged},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			world := openApprovalWorld(t)
			world.approve(t)
			testCase.tamper(t, world)
			result := world.executor(1).Execute(context.Background(), world.run, world.actionID)
			exists, consumed := world.grantState(t)
			if result.Status != ExecutionRefused || result.ReasonCode != testCase.wantReason || world.outboxRows(t) != 0 || !exists || consumed {
				t.Fatalf("result %s/%s, outbox %d, grant exists %v consumed %v", result.Status, result.ReasonCode, world.outboxRows(t), exists, consumed)
			}
			t.Logf("evidence X-45: %s -> refused %s; outbox rows 0; original grant unconsumed", testCase.name, result.ReasonCode)
		})
	}

	t.Run("expired approval", func(t *testing.T) {
		world := openApprovalWorld(t)
		approvals := NewApprovals(world.pool)
		approvals.now = func() time.Time { return world.scope.ExpiresAt.Add(time.Second) }
		_, err := approvals.Decide(context.Background(), world.reviewer, world.actionID, ApprovalApprove)
		exists, _ := world.grantState(t)
		if !errors.Is(err, ErrApprovalExpired) || exists || world.outboxRows(t) != 0 {
			t.Fatalf("err %v, grant exists %v, outbox %d", err, exists, world.outboxRows(t))
		}
		t.Logf("evidence X-45: expired approval -> %s; no grant; outbox rows 0", contracts.ReasonApprovalExpired)
	})

	t.Run("approval cannot enlarge the passport", func(t *testing.T) {
		world := openApprovalWorld(t)
		// A proposal the gate denied (an invoice outside the passport) has nothing to approve.
		gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
			NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
		world.nextStep++
		deniedAction := testdb.ID(t)
		decision := gate.Evaluate(context.Background(), world.run, Proposal{ActionID: deniedAction, StepNumber: world.nextStep,
			IdempotencyKey: testdb.ID(t), Tool: "create_report",
			RawArguments: json.RawMessage(`{"template":"vendor_reconciliation_v1","source_invoice_ids":["invoice_outside_the_passport"]}`)})
		if decision.Outcome != OutcomeDeny {
			t.Fatalf("gate decision = %s, want deny", decision.Outcome)
		}
		_, err := NewApprovals(world.pool).Decide(context.Background(), world.reviewer, deniedAction, ApprovalApprove)
		var grants int
		mustScan(t, world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.approvals WHERE action_id = $1`, deniedAction), &grants)
		if err == nil || grants != 0 {
			t.Fatalf("approving a denied action: err %v, grants %d", err, grants)
		}
		t.Logf("evidence X-45: approving a gate-denied (%s) action -> refused (%v); no grant", decision.ReasonCode, err)
	})
}
