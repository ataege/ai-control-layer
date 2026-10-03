package tools

import (
	"context"
	"strings"
	"testing"

	"starter/services/gateway/internal/provenance"
)

// TestProtectedFieldsStayInTheirAllowedChannels is GO-56's inspection (Go half of X-50) for the
// scenario that reaches the queued vendor report. Protected values and where they may appear:
//   - the registered reporting address: only the simulated outbox row's recipient;
//   - the internal note text: the model context (authorized for the investigation, labelled
//     internal_only) and the stored internal report; never an event, the vendor report or the
//     queued content.
//
// What it does not cover: it searches for the literal values, so a value the model transforms or
// encodes "in an allowed channel" is not found ("without claiming universal detection of
// personally identifiable information or encoded disclosure"). Model requests are built by the
// worker (f3); the tool results checked here are what MinimizeForModel hands to it.
func TestProtectedFieldsStayInTheirAllowedChannels(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	const address = "reports@atlas.example.com"
	const noteText = "Investigation note: INV104 appears twice."
	atlasRecipient := recipientReference(world.runID, world.atlasID)

	var modelFacing []string
	run := func(tool string, arguments any) EffectResult {
		t.Helper()
		result, err := Runner{}.RunEffect(context.Background(), world.tx, world.proposeAction(t, world.nextStep(), tool, arguments))
		if err != nil {
			t.Fatalf("%s: %v", tool, err)
		}
		minimized, err := MinimizeForModel(tool, result)
		if err != nil {
			t.Fatal(err)
		}
		modelFacing = append(modelFacing, tool+": "+string(minimized.JSON))
		return result
	}
	run(ToolReadInvoice, map[string]string{"invoice_id": world.invoiceA01})
	run(ToolReadVendor, map[string]string{"vendor_id": world.atlasID})
	sources := []string{world.invoiceA01, world.invoiceA02}
	internalID := run(ToolCreateReport, map[string]any{"template": provenance.InternalInvestigationV1.Name, "source_invoice_ids": sources}).ModelFacing.(ReportResult).ReportID
	run(ToolQueueReport, map[string]string{"report_id": internalID, "recipient_reference": atlasRecipient})
	vendorID := run(ToolCreateReport, map[string]any{"template": provenance.VendorReconciliationV1.Name, "source_invoice_ids": sources}).ModelFacing.(ReportResult).ReportID
	run(ToolQueueReport, map[string]string{"report_id": vendorID, "recipient_reference": atlasRecipient})

	// Model-facing tool results: the address never; the note only in read_invoice's result.
	for _, result := range modelFacing {
		if strings.Contains(result, address) {
			t.Errorf("address in a model-facing result: %s", result)
		}
		if strings.Contains(result, noteText) && !strings.HasPrefix(result, ToolReadInvoice+":") {
			t.Errorf("note in a model-facing result other than read_invoice: %s", result)
		}
	}

	// Safe events (the activity feed and audit export source): neither value, in any field.
	events := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE run_id = $1`, world.runID)
	leaking := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE run_id = $1
	                             AND (row_to_json(audit_events)::text LIKE '%' || $2 || '%' OR row_to_json(audit_events)::text LIKE '%' || $3 || '%')`,
		world.runID, address, noteText)
	if events == 0 || leaking != 0 {
		t.Errorf("events %d, events holding a protected value %d", events, leaking)
	}

	// Stored reports: the note only in the internal one; the address in neither.
	internal, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, internalID)
	if err != nil {
		t.Fatal(err)
	}
	vendor, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, vendorID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(internal.Content, noteText) || strings.Contains(vendor.Content, noteText) ||
		strings.Contains(internal.Content, address) || strings.Contains(vendor.Content, address) {
		t.Errorf("report contents break the field rules")
	}

	// The simulated outbox: one row, the address only as its recipient, the queued bytes are the
	// vendor report's (which hold no note), and no field the vendor template does not render.
	var outboxJSON, recipient string
	var queuesVendorReport bool
	err = world.tx.QueryRow(context.Background(),
		`SELECT row_to_json(outbox)::text, outbox.recipient, outbox.report_id = $2
		   FROM demo.outbox_messages AS outbox WHERE outbox.organization_id = $1`,
		world.organizationID, vendorID).Scan(&outboxJSON, &recipient, &queuesVendorReport)
	if err != nil {
		t.Fatal(err)
	}
	if recipient != address || !queuesVendorReport || strings.Contains(outboxJSON, noteText) ||
		strings.Count(outboxJSON, address) != 1 {
		t.Errorf("outbox row %s", outboxJSON)
	}
	t.Logf("evidence X-50: inspected %d model-facing results, %d events, 2 stored reports and 1 outbox row; "+
		"address only as the outbox recipient, note only in read_invoice's result and the internal report", len(modelFacing), events)
}
