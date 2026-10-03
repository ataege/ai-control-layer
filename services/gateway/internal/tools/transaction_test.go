package tools

import (
	"context"
	"testing"

	"starter/services/gateway/internal/provenance"
)

// effectRows counts the four kinds of row one create_report effect writes for its action.
func effectRows(t *testing.T, world *testWorld, request EffectRequest) [4]int {
	t.Helper()
	return [4]int{
		world.count(t, `SELECT count(*) FROM demo.reports WHERE created_by_action_id = $1`, request.ActionID),
		world.count(t, `SELECT count(*) FROM runtime.report_lineage AS lineage JOIN demo.reports AS report ON report.id = lineage.report_id
		                 WHERE report.created_by_action_id = $1`, request.ActionID),
		world.count(t, `SELECT count(*) FROM runtime.execution_attempts WHERE action_id = $1 AND completed_at IS NOT NULL`, request.ActionID),
		world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1`, request.ActionID),
	}
}

// The executor begins the transaction, calls RunEffect and commits or rolls back. A savepoint
// stands in for that transaction here, so the test's outer transaction still cleans up.
func TestEffectLineageCompletionAndEventCommitTogether(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	arguments := map[string]any{"template": provenance.InternalInvestigationV1.Name, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02}}

	rolledBack := world.proposeAction(t, world.nextStep(), ToolCreateReport, arguments)
	executorTx, err := world.tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Runner{}).RunEffect(context.Background(), executorTx, rolledBack); err != nil {
		t.Fatalf("RunEffect: %v", err)
	}
	if err := executorTx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := effectRows(t, world, rolledBack); rows != [4]int{0, 0, 0, 0} {
		t.Fatalf("after rollback: report, lineage, completion, event = %v, want none", rows)
	}

	committed := world.proposeAction(t, world.nextStep(), ToolCreateReport, arguments)
	executorTx, err = world.tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Runner{}).RunEffect(context.Background(), executorTx, committed); err != nil {
		t.Fatalf("RunEffect: %v", err)
	}
	if err := executorTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := effectRows(t, world, committed); rows != [4]int{1, 2, 1, 1} {
		t.Fatalf("after commit: report, lineage, completion, event = %v, want 1, 2 (one per source), 1, 1", rows)
	}
}
