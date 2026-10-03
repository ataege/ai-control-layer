package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/provenance"
)

// TestAtlasScenarioDirectAdapterPath captures the evidence of GO-66 (X-72), GO-67 (X-75) and the
// Go half of GO-47 (X-44) on the DIRECT ADAPTER PATH, NOT THE AGENT LOOP: the actions are stored
// by the test, not proposed by a model or a labelled replay, and no exact-action approval is
// consumed (GO-45 is the executor's). Those tasks stay open until the agent path reruns them.
// Run with -v to print the evidence lines.
func TestAtlasScenarioDirectAdapterPath(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	atlasRecipient := recipientReference(world.runID, world.atlasID)
	run := func(tool string, arguments any) EffectResult {
		t.Helper()
		result, err := Runner{}.RunEffect(context.Background(), world.tx, world.proposeAction(t, world.nextStep(), tool, arguments))
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		return result
	}
	t.Log("evidence: direct adapter path, not the agent loop")

	// Beat 3: the permitted reads, the note readable with its Internal only label.
	invoice := run(ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01}).ModelFacing.(InvoiceResult)
	if invoice.InternalNote == nil || invoice.InternalNote.Classification != provenance.InternalOnly {
		t.Fatalf("A01 note = %+v", invoice.InternalNote)
	}
	run(ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA02})
	vendor := run(ToolReadVendor, map[string]string{"vendor_id": world.atlasID}).ModelFacing.(VendorResult)
	if vendor.RecipientReference == nil || *vendor.RecipientReference != atlasRecipient {
		t.Fatalf("vendor reference = %v", vendor.RecipientReference)
	}

	// Beat 4: the internal report inherits Internal only.
	internalReport := run(ToolCreateReport, map[string]any{
		"template": provenance.InternalInvestigationV1.Name, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02},
	}).ModelFacing.(ReportResult)

	t.Run("GO-66 denied internal export", func(t *testing.T) {
		stored, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, internalReport.ReportID)
		if err != nil {
			t.Fatal(err)
		}
		manifest, _ := json.Marshal(stored.Lineage)
		before := world.outboxRows(t)
		denied := run(ToolQueueReport, map[string]string{"report_id": internalReport.ReportID, "recipient_reference": atlasRecipient})
		after := world.outboxRows(t)
		t.Logf("evidence X-72: label=%s template=%s source manifest=%s", stored.Classification, stored.TemplateName, manifest)
		t.Logf("evidence X-72: queue_report to the registered Atlas recipient -> outcome=%s rule=%s; outbox rows before=%d after=%d",
			denied.Outcome, denied.ReasonCode, before, after)
		if denied.ReasonCode != ReasonReportExportRestricted || before != 0 || after != 0 {
			t.Fatalf("denial %+v, outbox %d -> %d", denied, before, after)
		}
	})

	var vendorReport ReportResult
	t.Run("GO-67 approved external projection", func(t *testing.T) {
		vendorReport = run(ToolCreateReport, map[string]any{
			"template": provenance.VendorReconciliationV1.Name, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02},
		}).ModelFacing.(ReportResult)
		stored, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, vendorReport.ReportID)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range stored.Lineage {
			t.Logf("evidence X-75: source %s version %d classification %s fields %v; template %s v%d; projection %s v%d",
				entry.ID, entry.Version, entry.Classification, entry.ConsumedFields, entry.TemplateName, entry.TemplateVersion,
				*entry.ProjectionRule, *entry.ProjectionRuleVersion)
		}
		t.Logf("evidence X-75: serialized vendor report content:\n%s", stored.Content)
		if stored.Classification != provenance.VendorShareable || strings.Contains(stored.Content, "Investigation note") {
			t.Fatalf("vendor report %s holds internal text or the wrong label", stored.Classification)
		}
	})

	t.Run("GO-47 legitimate task, Go half", func(t *testing.T) {
		queued := run(ToolQueueReport, map[string]string{"report_id": vendorReport.ReportID, "recipient_reference": atlasRecipient})
		if queued.Outcome != OutcomeSucceeded {
			t.Fatalf("queue vendor report: %+v", queued)
		}
		var recipient string
		var reviewedBytesMatch bool
		err := world.tx.QueryRow(context.Background(),
			`SELECT outbox.recipient, outbox.report_content_hash = sha256(convert_to(report.content, 'UTF8'))
			   FROM demo.outbox_messages AS outbox JOIN demo.reports AS report ON report.id = outbox.report_id
			  WHERE outbox.organization_id = $1`, world.organizationID).Scan(&recipient, &reviewedBytesMatch)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("evidence X-44: report references internal=%s vendor=%s; discrepancy INV104 on %s and %s; outbox rows=%d recipient=%s hash-matches-stored-content=%v",
			internalReport.ReportID, vendorReport.ReportID, world.invoiceA01, world.invoiceA02, world.outboxRows(t), recipient, reviewedBytesMatch)
		rows, err := world.tx.Query(context.Background(),
			`SELECT event_type, coalesce(reason_code, '') FROM runtime.audit_events WHERE run_id = $1 ORDER BY id`, world.runID)
		if err != nil {
			t.Fatal(err)
		}
		var events []string
		for rows.Next() {
			var eventType, reasonCode string
			if err := rows.Scan(&eventType, &reasonCode); err != nil {
				t.Fatal(err)
			}
			events = append(events, strings.TrimSuffix(eventType+" "+reasonCode, " "))
		}
		rows.Close()
		t.Logf("evidence X-44: ordered events: %s", strings.Join(events, " | "))
		want := []string{"action.succeeded", "action.succeeded", "action.succeeded", "report.created",
			"report.export_denied report_export_restricted", "report.safe_template_offered report_export_restricted",
			"report.created", "action.succeeded"}
		if strings.Join(events, "|") != strings.Join(want, "|") || world.outboxRows(t) != 1 || !reviewedBytesMatch {
			t.Fatalf("events %v, outbox %d, bytes match %v", events, world.outboxRows(t), reviewedBytesMatch)
		}
	})
}
