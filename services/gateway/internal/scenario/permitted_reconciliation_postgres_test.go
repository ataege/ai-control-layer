package scenario

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/tools"
)

// GO-27 (X-63): the permitted reconciliation of demo beats 3 and 4. The agent reads the permitted
// invoice fields and the note, the duplicate INV104 on A01 and A02 is found, and create_report
// stores internal_investigation_v1 as Internal only with its source trail.

// allowlistOf returns the JSON field names of a model-facing tool result type (the GO-07
// allowlist), read from its struct tags so the check follows the adapter's own definition.
func allowlistOf(result any) []string {
	resultType := reflect.TypeOf(result)
	names := make([]string, 0, resultType.NumField())
	for index := range resultType.NumField() {
		name, _, _ := strings.Cut(resultType.Field(index).Tag.Get("json"), ",")
		names = append(names, name)
	}
	return names
}

// returnedResult is one executed tool's stored model-facing result.
type returnedResult struct {
	step   int
	tool   string
	fields map[string]json.RawMessage
}

// returnedResults reads what the model was given for each executed action of the run, from the
// stored agent context (the same bytes the next model request carried).
func (world *storyWorld) returnedResults(t *testing.T) []returnedResult {
	t.Helper()
	rows, err := world.pool.Query(context.Background(), `SELECT action.step_number, action.tool, entry.content::text
		FROM runtime.actions AS action
		JOIN runtime.context_entries AS entry ON entry.organization_id = action.organization_id
		 AND entry.action_id = action.id AND entry.kind = 'tool_result'
		WHERE action.organization_id = $1 AND action.run_id = $2 AND action.status IN ('executed', 'succeeded')
		ORDER BY action.step_number`, world.organizationID, world.passport.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var results []returnedResult
	for rows.Next() {
		var result returnedResult
		var content string
		if err := rows.Scan(&result.step, &result.tool, &content); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(content), &result.fields); err != nil {
			t.Fatalf("step %d %s: the stored result is not a JSON object: %v", result.step, result.tool, err)
		}
		results = append(results, result)
	}
	return results
}

// assertReturnedFieldsObeyPolicy checks every read result against its tool's allowlist and the
// passport scope, and returns the evidence lines. It fails when no invoice was read.
func assertReturnedFieldsObeyPolicy(t *testing.T, world *storyWorld, results []returnedResult) []string {
	t.Helper()
	allowlists := map[string][]string{
		string(contracts.ToolReadInvoice):  allowlistOf(tools.InvoiceResult{}),
		string(contracts.ToolReadVendor):   allowlistOf(tools.VendorResult{}),
		string(contracts.ToolCreateReport): allowlistOf(tools.ReportResult{}),
	}
	var evidence []string
	invoicesRead := 0
	for _, result := range results {
		allowed, known := allowlists[result.tool]
		if !known {
			continue
		}
		names := make([]string, 0, len(result.fields))
		for name := range result.fields {
			if !slices.Contains(allowed, name) {
				t.Fatalf("step %d %s returned %q, outside its allowlist %v", result.step, result.tool, name, allowed)
			}
			names = append(names, name)
		}
		slices.Sort(names)
		if result.tool == string(contracts.ToolReadInvoice) {
			var invoiceID string
			_ = json.Unmarshal(result.fields["invoice_id"], &invoiceID)
			if !slices.Contains(world.invoiceIDs, invoiceID) {
				t.Fatalf("step %d read invoice %s outside the passport scope", result.step, invoiceID)
			}
			invoicesRead++
		}
		evidence = append(evidence, result.tool+" "+strings.Join(names, ","))
	}
	if invoicesRead == 0 {
		t.Fatal("no invoice was read; the field check would prove nothing")
	}
	return evidence
}

// assertInternalReportIsPermitted checks the stored internal report: Internal only, a source trail
// of passport invoices only, and the seeded duplicate reference. It returns the evidence line.
func assertInternalReportIsPermitted(t *testing.T, world *storyWorld) string {
	t.Helper()
	reportID, classification, content := world.reportOf(t, contracts.TemplateInternalInvestigation)
	lineage := world.lineageOf(t, reportID)
	var lineageSources []string
	rows, err := world.pool.Query(context.Background(), `SELECT source_id FROM runtime.report_lineage
		WHERE organization_id = $1 AND report_id = $2 ORDER BY source_id`, world.organizationID, reportID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			t.Fatal(err)
		}
		lineageSources = append(lineageSources, sourceID)
	}
	rows.Close()
	for _, sourceID := range lineageSources {
		if !slices.Contains(world.invoiceIDs, sourceID) {
			t.Fatalf("the internal report references %s, outside the passport scope", sourceID)
		}
	}
	permitted := slices.Sorted(slices.Values(world.invoiceIDs))
	discrepancy := "- INV104: " + strings.Join(permitted, ", ")
	if classification != "internal_only" || !slices.Equal(lineageSources, permitted) ||
		!strings.Contains(content, discrepancy) || !strings.Contains(content, world.note) {
		t.Fatalf("internal report %s: classification %s, sources %v (want %v), discrepancy present %v, note present %v",
			reportID, classification, lineageSources, permitted, strings.Contains(content, discrepancy), strings.Contains(content, world.note))
	}
	return "internal report " + reportID + " labelled " + classification + "; source trail " + strings.Join(lineage, "; ") +
		"; finding \"" + discrepancy + "\""
}

// TestPermittedReconciliationThroughTheProductionChain runs beats 3 and 4 through
// agent.NewProductionChain with the scripted fixture provider (labelled: no model; repeatable),
// through the real gate, executor and adapters.
func TestPermittedReconciliationThroughTheProductionChain(t *testing.T) {
	world := openStory(t, nil)
	world.runQueuedJob(t)

	wantSteps := []string{"read_invoice", "read_invoice", "read_vendor", "create_report"}
	for index, wantTool := range wantSteps {
		if _, tool, status := world.actionAt(t, index+1); tool != wantTool || (status != "executed" && status != "succeeded") {
			t.Fatalf("step %d: %s %s, want %s executed or succeeded (logs: %s)", index+1, tool, status, wantTool, world.logs.String())
		}
	}
	results := world.returnedResults(t)
	fieldEvidence := assertReturnedFieldsObeyPolicy(t, world, results)

	// The note reached the model only with its trusted classification, and only on the invoice that has it.
	var firstNote, secondNote *tools.InvoiceNote
	_ = json.Unmarshal(results[0].fields["internal_note"], &firstNote)
	_ = json.Unmarshal(results[1].fields["internal_note"], &secondNote)
	if firstNote == nil || firstNote.Text != world.note || firstNote.Classification != "internal_only" || secondNote != nil {
		t.Fatalf("notes returned: A01 %+v, A02 %+v", firstNote, secondNote)
	}
	reportEvidence := assertInternalReportIsPermitted(t, world)
	t.Logf("evidence GO-27 (X-63, fixture provider, labelled): returned fields %v; %s", fieldEvidence, reportEvidence)
}
