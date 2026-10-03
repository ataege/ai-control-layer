package policy

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/tools"
)

// boundarySnapshot is the business and execution state a denied proposal must leave unchanged.
type boundarySnapshot struct {
	excludedInvoiceVersion int
	reports, outbox        int
	attempts               int
	passport               string
}

func takeBoundarySnapshot(t *testing.T, world *approvalWorld, excludedInvoice string) boundarySnapshot {
	t.Helper()
	ctx := context.Background()
	var snapshot boundarySnapshot
	// A failed read fails the test, so an unreadable table can never pass as "unchanged".
	reads := []struct {
		query  string
		target any
		arg    string
	}{
		{`SELECT version FROM demo.invoices WHERE id = $1`, &snapshot.excludedInvoiceVersion, excludedInvoice},
		{`SELECT count(*) FROM demo.reports WHERE organization_id = $1`, &snapshot.reports, world.run.OrganizationID},
		{`SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`, &snapshot.outbox, world.run.OrganizationID},
		{`SELECT count(*) FROM runtime.execution_attempts WHERE organization_id = $1`, &snapshot.attempts, world.run.OrganizationID},
		{`SELECT p.actor_id::text || '|' || p.scope::text FROM runtime.passports p
		    JOIN runtime.runs r ON r.passport_id = p.id WHERE r.id = $1`, &snapshot.passport, world.run.RunID},
	}
	for _, read := range reads {
		if err := world.pool.QueryRow(ctx, read.query, read.arg).Scan(read.target); err != nil {
			t.Fatalf("snapshot read %q: %v", read.query, err)
		}
	}
	if snapshot.passport == "" {
		t.Fatal("snapshot read an empty passport")
	}
	return snapshot
}

// countingAdapter records whether any adapter was reached.
type countingAdapter struct{ calls int }

func (adapter *countingAdapter) RunEffect(ctx context.Context, tx pgx.Tx, request tools.EffectRequest) (tools.EffectResult, error) {
	adapter.calls++
	return tools.Runner{}.RunEffect(ctx, tx, request)
}

// TestResourceAndDestinationBoundaries is the X-37 and X-38 evidence: an out-of-scope invoice, a
// vendor outside the task and a changed recipient, two of them from the labelled replay of the
// hostile notes, are denied before any adapter runs, leave every record unchanged, and leave the
// stored passport and operator identity as admitted.
func TestResourceAndDestinationBoundaries(t *testing.T) {
	type boundaryCase struct {
		name     string
		evidence string
		proposal func(*testing.T, *approvalWorld, ReplayTargets) (Proposal, ReasonCode)
	}
	cases := []boundaryCase{
		{"out-of-scope invoice (labelled replay)", "X-37", func(t *testing.T, world *approvalWorld, targets ReplayTargets) (Proposal, ReasonCode) {
			world.nextStep++
			proposal, reason, err := ReplayProposal("hostile_note_redirect_record_v1", targets, testdb.ID(t), world.nextStep, testdb.ID(t))
			if err != nil {
				t.Fatal(err)
			}
			return proposal, reason
		}},
		{"vendor outside the task", "X-37", func(t *testing.T, world *approvalWorld, _ ReplayTargets) (Proposal, ReasonCode) {
			outsider := "vendor_Outside_" + testdb.ID(t)[:8]
			mustExec(t, world.pool, `INSERT INTO demo.vendors (id, organization_id, name) VALUES ($1, $2, 'Outside')`, outsider, world.run.OrganizationID)
			world.nextStep++
			return Proposal{ActionID: testdb.ID(t), StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t), Tool: "read_vendor",
				RawArguments: json.RawMessage(`{"vendor_id":"` + outsider + `"}`)}, ReasonResourceOutOfScope
		}},
		{"changed recipient (labelled replay)", "X-38", func(t *testing.T, world *approvalWorld, targets ReplayTargets) (Proposal, ReasonCode) {
			world.nextStep++
			proposal, reason, err := ReplayProposal("hostile_note_redirect_recipient_v1", targets, testdb.ID(t), world.nextStep, testdb.ID(t))
			if err != nil {
				t.Fatal(err)
			}
			return proposal, reason
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			world := openApprovalWorld(t)
			targets := replayTargets(t, world)
			before := takeBoundarySnapshot(t, world, targets.OutOfScopeInvoiceID)
			adapter := &countingAdapter{}
			gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool),
				NewPostgresRelationships(world.pool), nil).WithReviewFreezer(NewPostgresReviewFreezer(world.pool))
			executor := NewExecutor(world.pool, &fakeScopes{scope: world.scope, revision: 1}, adapter)

			proposal, wantReason := testCase.proposal(t, world, targets)
			decision := gate.Evaluate(ctx, world.run, proposal)
			execution := executor.Execute(ctx, world.run, proposal.ActionID)
			after := takeBoundarySnapshot(t, world, targets.OutOfScopeInvoiceID)

			var storedDecision string
			mustScan(t, world.pool.QueryRow(ctx, `SELECT decision || '/' || reason_code FROM runtime.audit_events WHERE action_id = $1`, proposal.ActionID), &storedDecision)
			if decision.Outcome != OutcomeDeny || decision.ReasonCode != wantReason || execution.Status != ExecutionRefused || adapter.calls != 0 {
				t.Fatalf("decision %s/%s, execution %s, adapter calls %d; want deny/%s, refused, 0", decision.Outcome, decision.ReasonCode, execution.Status, adapter.calls, wantReason)
			}
			if before != after || storedDecision != "deny/"+string(wantReason) {
				t.Fatalf("state changed: before %+v after %+v, stored decision %q", before, after, storedDecision)
			}
			label := proposal.ReplaySource
			if label == "" {
				label = "model-shaped proposal (not a replay)"
			}
			t.Logf("evidence %s: %s [%s] -> stored decision %s; adapter calls 0; execution attempts %d->%d; excluded invoice version %d->%d; reports %d->%d; outbox %d->%d; passport and operator identity unchanged",
				testCase.evidence, testCase.name, label, storedDecision, before.attempts, after.attempts, before.excludedInvoiceVersion, after.excludedInvoiceVersion,
				before.reports, after.reports, before.outbox, after.outbox)
		})
	}
}
