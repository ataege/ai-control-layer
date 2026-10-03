package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/provenance"
)

// createReportFor stores a report of the given template from A01 and A02 and returns its id.
func createReportFor(t *testing.T, world *testWorld, template string) string {
	t.Helper()
	request := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{
		"template": template, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02},
	})
	result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
	if err != nil || result.Outcome != OutcomeSucceeded {
		t.Fatalf("create %s: %+v %v", template, result, err)
	}
	return result.ModelFacing.(ReportResult).ReportID
}

func (world *testWorld) queue(t *testing.T, reportID, recipient string) (EffectRequest, EffectResult) {
	t.Helper()
	request := world.proposeAction(t, world.nextStep(), ToolQueueReport, map[string]string{"report_id": reportID, "recipient_reference": recipient})
	result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
	if err != nil {
		t.Fatalf("queue_report: %v", err)
	}
	return request, result
}

func (world *testWorld) outboxRows(t *testing.T) int {
	return world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1`, world.organizationID)
}

func TestQueueInternalReportToTheRegisteredVendorIsDenied(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	internalReport := createReportFor(t, world, provenance.InternalInvestigationV1.Name)
	before := world.outboxRows(t)
	request, result := world.queue(t, internalReport, recipientReference(world.runID, world.atlasID))
	if result.Outcome != OutcomeFailed || result.ReasonCode != ReasonReportExportRestricted || result.ModelFacing != nil {
		t.Fatalf("result = %+v, want failed report_export_restricted", result)
	}
	if after := world.outboxRows(t); after != before {
		t.Fatalf("outbox rows %d -> %d, want unchanged", before, after)
	}
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1 AND event_type = 'report.export_denied' AND reason_code = 'report_export_restricted'
	                            AND masked_summary->>'safeMessage' = 'This report inherits an Internal only restriction and cannot be sent to the vendor.'`, request.ActionID); got != 1 {
		t.Fatalf("export_denied events with their X-13 safe message = %d, want 1", got)
	}
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1 AND event_type = 'report.safe_template_offered'
	                            AND masked_summary->>'alternativeTemplate' = 'vendor_reconciliation_v1'`, request.ActionID); got != 1 {
		t.Fatalf("safe_template_offered events = %d, want 1", got)
	}
}

func TestQueueVendorReportCreatesOneSimulatedOutboxRow(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	vendorReport := createReportFor(t, world, provenance.VendorReconciliationV1.Name)
	request, result := world.queue(t, vendorReport, recipientReference(world.runID, world.atlasID))
	if result.Outcome != OutcomeSucceeded {
		t.Fatalf("result = %+v", result)
	}
	minimized, err := MinimizeForModel(ToolQueueReport, result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(minimized.JSON, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 3 || fields["status"] != StatusQueuedSimulated || strings.Contains(string(minimized.JSON), "@") {
		t.Fatalf("model-facing result = %s", minimized.JSON)
	}
	var recipient string
	var hashMatches bool
	err = world.tx.QueryRow(context.Background(),
		`SELECT outbox.recipient, outbox.report_content_hash = report.content_hash
		   FROM demo.outbox_messages AS outbox JOIN demo.reports AS report ON report.id = outbox.report_id
		  WHERE outbox.action_id = $1`, request.ActionID).Scan(&recipient, &hashMatches)
	if err != nil {
		t.Fatal(err)
	}
	if recipient != "reports@atlas.example.com" || !hashMatches || world.outboxRows(t) != 1 {
		t.Fatalf("outbox recipient %q, hash matches %v, rows %d", recipient, hashMatches, world.outboxRows(t))
	}

	// A retry under the same action leaves the one row.
	retry := request
	retry.AttemptID = world.newAttempt(t, request.ActionID, 2)
	if _, err := (Runner{}).RunEffect(context.Background(), world.tx, retry); err == nil || !strings.Contains(err.Error(), "outbox_messages_one_per_action") {
		t.Fatalf("retry err = %v, want the outbox uniqueness to refuse it", err)
	}
}

func TestQueueRefusesUnrelatedReportsUntrustedRecipientsAndChangedSources(t *testing.T) {
	// Borealis is listed in vendorIds and recipientReferences but belongs to the other
	// organization, so only the database's organization filter can refuse it.
	world := openWorld(t, func(world *testWorld) passportScope {
		passport := scenarioScope(world, true)
		passport.VendorIDs = append(passport.VendorIDs, world.borealisID)
		passport.RecipientReferences = append(passport.RecipientReferences, recipientReference(world.runID, world.borealisID))
		return passport
	})
	vendorReport := createReportFor(t, world, provenance.VendorReconciliationV1.Name)
	atlas := recipientReference(world.runID, world.atlasID)

	cases := []struct {
		name, reportID, recipient, reason string
	}{
		{"unknown report", world.passportID, atlas, ReasonResourceOutOfScope},
		{"raw address", vendorReport, "reports@atlas.example.com", ReasonDestinationNotAllowed},
		{"other run's reference", vendorReport, recipientReference(world.passportID, world.atlasID), ReasonDestinationNotAllowed},
		{"other organization's vendor", vendorReport, recipientReference(world.runID, world.borealisID), ReasonDestinationNotAllowed},
	}
	for _, testCase := range cases {
		_, result := world.queue(t, testCase.reportID, testCase.recipient)
		if result.Outcome != OutcomeFailed || result.ReasonCode != testCase.reason {
			t.Errorf("%s: %+v, want failed %s", testCase.name, result, testCase.reason)
		}
	}
	// A source changes after the report was rendered: the stale report is not queued.
	world.exec(t, `UPDATE demo.invoices SET version = version + 1 WHERE id = $1`, world.invoiceA02)
	if _, result := world.queue(t, vendorReport, atlas); result.ReasonCode != ReasonResourceVersionChanged {
		t.Errorf("changed source: %+v, want resource_version_changed", result)
	}
	if rows := world.outboxRows(t); rows != 0 {
		t.Fatalf("outbox rows = %d, want 0", rows)
	}
}

func TestDenialEventsClaimOnlyWhatWasChecked(t *testing.T) {
	// The passport does not permit the vendor template: no alternative may be offered.
	world := openWorld(t, func(world *testWorld) passportScope {
		passport := scenarioScope(world, true)
		passport.ReportTemplates = []string{provenance.InternalInvestigationV1.Name}
		return passport
	})
	internalReport := createReportFor(t, world, provenance.InternalInvestigationV1.Name)
	restricted, _ := world.queue(t, internalReport, recipientReference(world.runID, world.atlasID))
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1
	                            AND (event_type = 'report.safe_template_offered' OR masked_summary->>'alternativeTemplate' IS NOT NULL)`,
		restricted.ActionID); got != 0 {
		t.Fatalf("an alternative the passport does not permit was offered (%d events)", got)
	}
	// A refusal before the lineage check claims no lineage result.
	rawAddress, _ := world.queue(t, internalReport, "reports@atlas.example.com")
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE action_id = $1 AND event_type = 'report.export_denied'
	                            AND masked_summary->>'lineageCheck' IS NULL`, rawAddress.ActionID); got != 1 {
		t.Fatal("a recipient refusal claims a lineage check that never ran")
	}
}
