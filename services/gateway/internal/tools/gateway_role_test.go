package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/provenance"
)

const gatewayRole = "task_passport_gateway"

// asGatewayRole switches the rest of tx to the gateway's own database role (X-35,
// CreateServiceRoles); the fixtures before it were written by the owner.
func asGatewayRole(t *testing.T, tx pgx.Tx) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), `SET LOCAL ROLE `+gatewayRole); err != nil {
		t.Fatalf("set role: %v (run the migrations: CreateServiceRoles creates the role)", err)
	}
}

// TestTheGatewayRoleRunsTheEffectsAndNothingElse proves the gateway role's grants for the tool
// adapters (GO-38): the whole Atlas scenario and an injected fault (GO-55) run as the role, while
// writes outside its authority fail at the database ("Credentials and database roles must enforce
// the intended boundary").
func TestTheGatewayRoleRunsTheEffectsAndNothingElse(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	atlasRecipient := recipientReference(world.runID, world.atlasID)

	// GO-55 on the gateway role: the trigger is created by the owner, the effect runs as the role.
	faulty := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{
		"template": provenance.VendorReconciliationV1.Name, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02}})
	executorTx, err := world.tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	injectFailure(t, executorTx, "runtime.audit_events", "INSERT")
	asGatewayRole(t, executorTx)
	if _, err := (Runner{}).RunEffect(context.Background(), executorTx, faulty); err == nil {
		t.Fatal("the injected failure did not surface")
	}
	if err := executorTx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := actionRows(t, world, faulty.ActionID); rows != [5]int{} {
		t.Fatalf("gateway role, injected fault: rows %v, want none", rows)
	}

	asGatewayRole(t, world.tx)
	var currentRole string
	if err := world.tx.QueryRow(context.Background(), `SELECT current_user`).Scan(&currentRole); err != nil || currentRole != gatewayRole {
		t.Fatalf("current_user = %q, %v", currentRole, err)
	}
	run := func(tool string, arguments any) EffectResult {
		t.Helper()
		result, err := Runner{}.RunEffect(context.Background(), world.tx, world.proposeAction(t, world.nextStep(), tool, arguments))
		if err != nil {
			t.Fatalf("%s as %s: %v", tool, gatewayRole, err)
		}
		return result
	}
	run(ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01})
	run(ToolReadVendor, map[string]string{"vendor_id": world.atlasID})
	internalReport := run(ToolCreateReport, map[string]any{"template": provenance.InternalInvestigationV1.Name,
		"source_invoice_ids": []string{world.invoiceA01, world.invoiceA02}}).ModelFacing.(ReportResult).ReportID
	if denied := run(ToolQueueReport, map[string]string{"report_id": internalReport, "recipient_reference": atlasRecipient}); denied.ReasonCode != ReasonReportExportRestricted {
		t.Fatalf("internal export as the role: %+v", denied)
	}
	vendorReport := run(ToolCreateReport, map[string]any{"template": provenance.VendorReconciliationV1.Name,
		"source_invoice_ids": []string{world.invoiceA01, world.invoiceA02}}).ModelFacing.(ReportResult).ReportID
	if queued := run(ToolQueueReport, map[string]string{"report_id": vendorReport, "recipient_reference": atlasRecipient}); queued.Outcome != OutcomeSucceeded {
		t.Fatalf("vendor queue as the role: %+v", queued)
	}

	// The other Go modules' tables, as the audit of every gateway SQL statement found them used:
	// the active catalog (read), the token ledger and the job/run/action state (read and write).
	for _, statement := range []string{
		`SELECT count(*) FROM app.control_catalog_revisions`,
		`SELECT count(*) FROM app.control_catalog_pointer`,
		`SELECT count(*) FROM app.signature_feed_revisions`,
		`SELECT count(*) FROM runtime.model_token_budgets`,
		`SELECT count(*) FROM runtime.model_token_reservations`,
		`SELECT count(*) FROM runtime.jobs`,
		`SELECT count(*) FROM runtime.control_assessments`,
		`SELECT count(*) FROM runtime.timing_records`,
		`SELECT id FROM runtime.runs WHERE id = '` + world.runID + `' FOR UPDATE`,
	} {
		if _, err := world.tx.Exec(context.Background(), statement); err != nil {
			t.Fatalf("%s as %s: %v", statement, gatewayRole, err)
		}
	}

	// Writes outside the gateway's authority fail at the database.
	for name, statement := range map[string]string{
		"write an app record":     `INSERT INTO app.control_catalog_pointer (id) VALUES (2)`,
		"change a source invoice": `UPDATE demo.invoices SET total_minor_units = 0`,
		"delete a report":         `DELETE FROM demo.reports`,
		"truncate the outbox":     `TRUNCATE demo.outbox_messages`,
		"delete an audit event":   `DELETE FROM runtime.audit_events`,
		"create a table":          `CREATE TABLE runtime.gateway_owned (id int)`,
	} {
		savepoint, err := world.tx.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		_, err = savepoint.Exec(context.Background(), statement)
		_ = savepoint.Rollback(context.Background())
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s as %s: err = %v, want permission denied", name, gatewayRole, err)
		}
	}
}
