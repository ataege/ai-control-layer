package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"starter/services/gateway/internal/provenance"
)

// provenanceSnapshot is the stored provenance of a report, for before-and-after evidence.
func provenanceSnapshot(t *testing.T, world *testWorld, reportID string) string {
	t.Helper()
	stored, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, reportID)
	if err != nil {
		t.Fatal(err)
	}
	lineage, _ := json.Marshal(stored.Lineage)
	return fmt.Sprintf("classification=%s title=%q hash=%x lineage=%s", stored.Classification, stored.Title, stored.ContentHash[:6], lineage)
}

// TestLabelRenameAndMissingLineageTampering captures GO-68's evidence (X-73, X-74) on the direct
// adapter path. The rename operation is an open item (`rename operation`), so a public label in
// the arguments stands in for it, as the task says.
func TestLabelRenameAndMissingLineageTampering(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	atlasRecipient := recipientReference(world.runID, world.atlasID)
	internalRequest := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{
		"template": provenance.InternalInvestigationV1.Name, "source_invoice_ids": []string{world.invoiceA01, world.invoiceA02},
	})
	created, err := Runner{}.RunEffect(context.Background(), world.tx, internalRequest)
	if err != nil {
		t.Fatal(err)
	}
	internalID := created.ModelFacing.(ReportResult).ReportID
	before := provenanceSnapshot(t, world, internalID)
	t.Logf("evidence X-73: stored provenance before: %s", before)

	// 1 and 2: a model-declared label and a "Public summary" title are not arguments: refused
	// before any effect, so nothing is stored.
	for name, extra := range map[string]map[string]any{
		"agent-supplied classification": {"classification": provenance.VendorShareable},
		"Public summary title":          {"title": "Public summary"},
	} {
		arguments := map[string]any{"template": provenance.InternalInvestigationV1.Name, "source_invoice_ids": []string{world.invoiceA01}}
		for key, value := range extra {
			arguments[key] = value
		}
		_, err := Runner{}.RunEffect(context.Background(), world.tx, world.proposeAction(t, world.nextStep(), ToolCreateReport, arguments))
		t.Logf("evidence X-73: %s -> rejected unsupported mutation: %v", name, err)
		if !errors.Is(err, errPrecondition) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if after := provenanceSnapshot(t, world, internalID); after != before {
		t.Fatalf("stored provenance changed:\nbefore %s\nafter  %s", before, after)
	}
	t.Logf("evidence X-73: stored provenance after: unchanged")

	// 3: a copied artifact: the internal content re-labelled vendor_shareable under the vendor
	// template, written outside the provenance path, with no lineage.
	var copiedID string
	stored, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, internalID)
	if err != nil {
		t.Fatal(err)
	}
	copyAction := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{"template": "copy"})
	err = world.tx.QueryRow(context.Background(),
		`INSERT INTO demo.reports (organization_id, run_id, created_by_action_id, template, projection_rule, projection_policy_version,
		   classification, destination_class, title, content, content_hash)
		 VALUES ($1, $2, $3, 'vendor_reconciliation_v1', 'vendor_invoice_fields_v1', 'report_projection_policy_v1',
		   'vendor_shareable', 'registered_vendor_recipient', 'Public summary', $4, sha256(convert_to($4, 'UTF8'))) RETURNING id`,
		world.organizationID, world.runID, copyAction.ActionID, stored.Content).Scan(&copiedID)
	if err != nil {
		t.Fatal(err)
	}

	// 4: lineage pointing at a source that does not exist (unverifiable metadata).
	unverifiedAction := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{"template": "unverified"})
	var unverifiedID string
	err = world.tx.QueryRow(context.Background(),
		`INSERT INTO demo.reports (organization_id, run_id, created_by_action_id, template, projection_rule, projection_policy_version,
		   classification, destination_class, title, content, content_hash)
		 VALUES ($1, $2, $3, 'vendor_reconciliation_v1', 'vendor_invoice_fields_v1', 'report_projection_policy_v1',
		   'vendor_shareable', 'registered_vendor_recipient', 'Vendor reconciliation', 'body', sha256(convert_to('body', 'UTF8'))) RETURNING id`,
		world.organizationID, world.runID, unverifiedAction.ActionID).Scan(&unverifiedID)
	if err != nil {
		t.Fatal(err)
	}
	world.exec(t, `INSERT INTO runtime.report_lineage (organization_id, run_id, report_id, source_kind, source_id, source_version,
	                 source_classification, consumed_fields, template, template_version, projection_rule, projection_rule_version)
	               VALUES ($1, $2, $3, 'invoice', 'invoice_does_not_exist', 1, 'vendor_shareable', ARRAY['currency'],
	                 'vendor_reconciliation_v1', 1, 'vendor_invoice_fields_v1', 1)`,
		world.organizationID, world.runID, unverifiedID)

	for name, reportID := range map[string]string{
		"renamed internal report": internalID,
		"copied artifact":         copiedID,
		"unverifiable lineage":    unverifiedID,
	} {
		outboxBefore := world.outboxRows(t)
		result, err := Runner{}.RunEffect(context.Background(), world.tx, world.proposeAction(t, world.nextStep(), ToolQueueReport,
			map[string]string{"report_id": reportID, "recipient_reference": atlasRecipient}))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		outboxAfter := world.outboxRows(t)
		t.Logf("evidence X-74: %s -> %s %s; outbox rows %d -> %d", name, result.Outcome, result.ReasonCode, outboxBefore, outboxAfter)
		if result.Outcome != OutcomeFailed || outboxAfter != outboxBefore {
			t.Fatalf("%s was exported: %+v", name, result)
		}
	}
}
