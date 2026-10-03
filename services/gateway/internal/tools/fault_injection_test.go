package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/provenance"
)

// actionRows counts every row one effect may write for its action: report, lineage, outbox,
// completed attempt and events.
func actionRows(t *testing.T, world *testWorld, actionID string) [5]int {
	t.Helper()
	return [5]int{
		world.count(t, `SELECT count(*) FROM demo.reports WHERE created_by_action_id = $1`, actionID),
		world.count(t, `SELECT count(*) FROM runtime.report_lineage AS lineage JOIN demo.reports AS report ON report.id = lineage.report_id
		                 WHERE report.created_by_action_id = $1`, actionID),
		world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE action_id = $1`, actionID),
		world.count(t, `SELECT count(*) FROM runtime.execution_attempts WHERE action_id = $1 AND completed_at IS NOT NULL`, actionID),
		world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1`, actionID),
	}
}

// injectFailure makes the next matching write inside executorTx fail. The trigger and its function
// are created in that transaction and disappear with its rollback.
func injectFailure(t *testing.T, executorTx pgx.Tx, table, operation string) {
	t.Helper()
	for _, statement := range []string{
		`CREATE FUNCTION runtime.inject_test_failure() RETURNS trigger LANGUAGE plpgsql AS $$
		 BEGIN RAISE EXCEPTION 'injected failure'; END; $$`,
		fmt.Sprintf(`CREATE TRIGGER inject_test_failure BEFORE %s ON %s FOR EACH ROW EXECUTE FUNCTION runtime.inject_test_failure()`, operation, table),
	} {
		if _, err := executorTx.Exec(context.Background(), statement); err != nil {
			t.Fatalf("inject failure: %v", err)
		}
	}
}

// TestEffectsCommitTogetherOrNotAtAll is GO-55's fault-injection proof (X-57): a failure injected
// between the effect, its lineage, the attempt's completion and the event leaves none of them.
func TestEffectsCommitTogetherOrNotAtAll(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	atlasRecipient := recipientReference(world.runID, world.atlasID)
	vendorReportArguments := map[string]any{"template": provenance.VendorReconciliationV1.Name, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02}}

	// A committed vendor report for the queue_report cases.
	setupRequest := world.proposeAction(t, world.nextStep(), ToolCreateReport, vendorReportArguments)
	created, err := Runner{}.RunEffect(context.Background(), world.tx, setupRequest)
	if err != nil {
		t.Fatal(err)
	}
	vendorReportID := created.ModelFacing.(ReportResult).ReportID

	faults := []struct {
		name, tool, table, operation string
		arguments                    any
	}{
		{"create_report: lineage insert", ToolCreateReport, "runtime.report_lineage", "INSERT", vendorReportArguments},
		{"create_report: attempt completion", ToolCreateReport, "runtime.execution_attempts", "UPDATE", vendorReportArguments},
		{"create_report: event", ToolCreateReport, "runtime.audit_events", "INSERT", vendorReportArguments},
		{"queue_report: attempt completion", ToolQueueReport, "runtime.execution_attempts", "UPDATE",
			map[string]string{"report_id": vendorReportID, "recipient_reference": atlasRecipient}},
		{"queue_report: event", ToolQueueReport, "runtime.audit_events", "INSERT",
			map[string]string{"report_id": vendorReportID, "recipient_reference": atlasRecipient}},
	}
	for _, fault := range faults {
		request := world.proposeAction(t, world.nextStep(), fault.tool, fault.arguments)
		executorTx, err := world.tx.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		injectFailure(t, executorTx, fault.table, fault.operation)
		_, runErr := Runner{}.RunEffect(context.Background(), executorTx, request)
		// The executor rolls back on any error.
		if err := executorTx.Rollback(context.Background()); err != nil {
			t.Fatal(err)
		}
		rows := actionRows(t, world, request.ActionID)
		t.Logf("evidence X-57: %s -> error %v; report, lineage, outbox, completion, events = %v", fault.name, runErr != nil, rows)
		if runErr == nil || rows != [5]int{} {
			t.Fatalf("%s: error %v, rows %v; want an error and no rows", fault.name, runErr, rows)
		}
	}

	// Without a fault the same queue effect commits all of its rows once.
	request := world.proposeAction(t, world.nextStep(), ToolQueueReport, map[string]string{"report_id": vendorReportID, "recipient_reference": atlasRecipient})
	executorTx, err := world.tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Runner{}).RunEffect(context.Background(), executorTx, request); err != nil {
		t.Fatal(err)
	}
	if err := executorTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := actionRows(t, world, request.ActionID)
	t.Logf("evidence X-57: queue_report without a fault -> report, lineage, outbox, completion, events = %v", rows)
	if rows != [5]int{0, 0, 1, 1, 1} {
		t.Fatalf("committed rows = %v, want outbox, completion and event once", rows)
	}
}
