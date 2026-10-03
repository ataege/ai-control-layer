package tools

import (
	"context"
	"strings"
	"testing"

	"starter/services/gateway/internal/provenance"
)

func createInternalReport(t *testing.T, world *testWorld, sourceIDs ...string) (EffectRequest, EffectResult) {
	t.Helper()
	request := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{
		"template": provenance.InternalInvestigationV1.Name, "source_invoice_ids": sourceIDs,
	})
	result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
	if err != nil {
		t.Fatalf("create_report: %v", err)
	}
	return request, result
}

func TestCreateInternalReportDerivesInternalOnlyAndStoresLineage(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	_, result := createInternalReport(t, world, world.invoiceA02, world.invoiceA01)
	if result.Outcome != OutcomeSucceeded {
		t.Fatalf("result = %+v", result)
	}
	report := result.ModelFacing.(ReportResult)
	if report.Classification != provenance.InternalOnly || report.Template != provenance.InternalInvestigationV1.Name {
		t.Fatalf("report = %+v", report)
	}
	stored, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, report.ReportID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stored.Content, "- INV104: "+world.invoiceA01+", "+world.invoiceA02) ||
		!strings.Contains(stored.Content, "Investigation note: INV104 appears twice.") {
		t.Fatalf("content lacks the finding or the authorized note:\n%s", stored.Content)
	}
	classifications := map[string]string{}
	for _, entry := range stored.Lineage {
		classifications[entry.ID] = entry.Classification
	}
	if classifications[world.invoiceA01] != provenance.InternalOnly || classifications[world.invoiceA02] != provenance.VendorShareable {
		t.Fatalf("lineage = %v, want A01 internal_only (note) and A02 vendor_shareable", classifications)
	}
	if got := world.count(t, `SELECT count(*) FROM runtime.audit_events WHERE event_type = 'report.created' AND run_id = $1`, world.runID); got != 1 {
		t.Fatalf("report.created events = %d, want 1", got)
	}
}

func TestCreateReportWithoutTheNoteIsStillInternalOnly(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, false) })
	_, result := createInternalReport(t, world, world.invoiceA01)
	report := result.ModelFacing.(ReportResult)
	stored, err := provenance.LoadReport(context.Background(), world.tx, world.organizationID, world.runID, report.ReportID)
	if err != nil {
		t.Fatal(err)
	}
	if report.Classification != provenance.InternalOnly || strings.Contains(stored.Content, "Investigation note") {
		t.Fatalf("classification %s, content:\n%s", report.Classification, stored.Content)
	}
}

func TestCreateReportRefusesUnauthorizedSourcesTemplatesAndLabels(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	cases := map[string]struct {
		arguments map[string]any
		reason    string
	}{
		"out-of-scope invoice":  {map[string]any{"template": "internal_investigation_v1", "source_invoice_ids": []string{world.invoiceA01, world.invoiceB01}}, ReasonResourceOutOfScope},
		"other organization":    {map[string]any{"template": "internal_investigation_v1", "source_invoice_ids": []string{world.invoiceC01}}, ReasonResourceOutOfScope},
		"unregistered template": {map[string]any{"template": "public_summary_v1", "source_invoice_ids": []string{world.invoiceA01}}, ReasonTemplateNotAllowed},
	}
	for name, testCase := range cases {
		request := world.proposeAction(t, world.nextStep(), ToolCreateReport, testCase.arguments)
		result, err := Runner{}.RunEffect(context.Background(), world.tx, request)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Outcome != OutcomeFailed || result.ReasonCode != testCase.reason {
			t.Errorf("%s: %+v, want failed %s", name, result, testCase.reason)
		}
	}
	// A model-supplied classification or source list of its own is not an argument at all.
	labelled := world.proposeAction(t, world.nextStep(), ToolCreateReport, map[string]any{
		"template": "internal_investigation_v1", "source_invoice_ids": []string{world.invoiceA01}, "classification": "vendor_shareable",
	})
	if _, err := (Runner{}).RunEffect(context.Background(), world.tx, labelled); err == nil {
		t.Error("a model-supplied classification was accepted")
	}
	if got := world.count(t, `SELECT count(*) FROM demo.reports WHERE run_id = $1`, world.runID); got != 0 {
		t.Fatalf("reports stored = %d, want 0", got)
	}
}

func TestCreateReportTwiceUnderOneActionCreatesOneReport(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	request, _ := createInternalReport(t, world, world.invoiceA01)
	// A second attempt for the same action, as a retry would create.
	retry := request
	retry.AttemptID = world.newAttempt(t, request.ActionID, 2)
	_, err := (Runner{}).RunEffect(context.Background(), world.tx, retry)
	if err == nil || !strings.Contains(err.Error(), "reports_one_per_action") {
		t.Fatalf("err = %v, want the reports_one_per_action uniqueness to refuse a second report", err)
	}
}
