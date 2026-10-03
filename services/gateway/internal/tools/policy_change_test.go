package tools

import (
	"context"
	"testing"

	"starter/services/gateway/internal/provenance"
)

// TestNoStaleReportIsQueuedAfterASourceOrTemplateChange is the "no stale report is queued" half
// of GO-70 (X-76), on the direct adapter path. The "old approval is rejected" half needs the
// executor's approval flow (GO-45, GO-52) and is added when it lands; revocations need the
// revocation records (SH-38).
func TestNoStaleReportIsQueuedAfterASourceOrTemplateChange(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	atlasRecipient := recipientReference(world.runID, world.atlasID)
	queue := func(reportID string) EffectResult {
		t.Helper()
		result, err := Runner{}.RunEffect(context.Background(), world.tx, world.proposeAction(t, world.nextStep(), ToolQueueReport,
			map[string]string{"report_id": reportID, "recipient_reference": atlasRecipient}))
		if err != nil {
			t.Fatal(err)
		}
		return result
	}

	// A source invoice changes while the vendor report waits for review.
	vendorReport := createReportFor(t, world, provenance.VendorReconciliationV1.Name)
	world.exec(t, `UPDATE demo.invoices SET version = version + 1, total_minor_units = 99999 WHERE id = $1`, world.invoiceA01)
	sourceChanged := queue(vendorReport)
	t.Logf("evidence X-76: source version changed during review -> %s %s; outbox rows %d", sourceChanged.Outcome, sourceChanged.ReasonCode, world.outboxRows(t))

	// A report whose lineage records an older projection version than the registered one.
	oldProjectionAction := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{"template": "old"})
	var oldReportID string
	err := world.tx.QueryRow(context.Background(),
		`INSERT INTO demo.reports (organization_id, run_id, created_by_action_id, template, projection_rule, projection_policy_version,
		   classification, destination_class, title, content, content_hash)
		 VALUES ($1, $2, $3, 'vendor_reconciliation_v1', 'vendor_invoice_fields_v1', 'report_projection_policy_v0',
		   'vendor_shareable', 'registered_vendor_recipient', 'Vendor reconciliation', 'old body', sha256(convert_to('old body', 'UTF8'))) RETURNING id`,
		world.organizationID, world.runID, oldProjectionAction.ActionID).Scan(&oldReportID)
	if err != nil {
		t.Fatal(err)
	}
	world.exec(t, `INSERT INTO runtime.report_lineage (organization_id, run_id, report_id, source_kind, source_id, source_version,
	                 source_classification, consumed_fields, template, template_version, projection_rule, projection_rule_version)
	               VALUES ($1, $2, $3, 'invoice', $4, 1, 'vendor_shareable', ARRAY['currency'], 'vendor_reconciliation_v1', 1, 'vendor_invoice_fields_v1', 0)`,
		world.organizationID, world.runID, oldReportID, world.invoiceA02)
	projectionChanged := queue(oldReportID)
	t.Logf("evidence X-76: projection version no longer registered -> %s %s; outbox rows %d", projectionChanged.Outcome, projectionChanged.ReasonCode, world.outboxRows(t))

	if sourceChanged.ReasonCode != ReasonResourceVersionChanged || projectionChanged.ReasonCode != ReasonTemplateNotAllowed || world.outboxRows(t) != 0 {
		t.Fatalf("source change %+v, projection change %+v, outbox %d", sourceChanged, projectionChanged, world.outboxRows(t))
	}
}
