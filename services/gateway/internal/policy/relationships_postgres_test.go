package policy

import (
	"context"
	"testing"

	"starter/services/gateway/internal/provenance"
	"starter/services/gateway/internal/testdb"
)

func TestPostgresRelationships(t *testing.T) {
	world := openExecutorWorld(t, 12)
	ctx := context.Background()
	relationships := NewPostgresRelationships(world.pool)
	suffix := testdb.ID(t)[:8]

	// A second vendor of the same organization whose only invoice is outside the passport, and a
	// vendor of another organization.
	otherVendor := "vendor_Borealis_" + suffix
	otherInvoice := "invoice_C01_" + suffix
	foreignOrganization := testdb.ID(t)
	foreignVendor := "vendor_Foreign_" + suffix
	mustExec(t, world.pool, `INSERT INTO demo.vendors (id, organization_id, name) VALUES ($1, $2, 'Borealis'), ($3, $4, 'Foreign')`,
		otherVendor, world.run.OrganizationID, foreignVendor, foreignOrganization)
	mustExec(t, world.pool, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency,
	                           total_minor_units, issued_on, due_on)
	                         VALUES ($1, $2, $3, 'BOR-7', 'EUR', 1000, '2026-09-01', '2026-10-01')`,
		otherInvoice, world.run.OrganizationID, otherVendor)
	atlasVendor := "vendor_Atlas_" + world.invoiceA01[len("invoice_A01_"):]
	passportInvoices := []string{world.invoiceA01}

	vendorCases := []struct {
		name           string
		organizationID string
		vendorID       string
		want           bool
	}{
		{"vendor of a passport invoice", world.run.OrganizationID, atlasVendor, true},
		{"vendor whose invoices are outside the passport", world.run.OrganizationID, otherVendor, false},
		{"vendor of another organization", world.run.OrganizationID, foreignVendor, false},
		{"right vendor, wrong organization", foreignOrganization, atlasVendor, false},
		{"unknown vendor", world.run.OrganizationID, "vendor_nobody", false},
	}
	for _, testCase := range vendorCases {
		t.Run(testCase.name, func(t *testing.T) {
			linked, err := relationships.VendorLinkedToInvoices(ctx, testCase.organizationID, testCase.vendorID, passportInvoices)
			if err != nil || linked != testCase.want {
				t.Fatalf("linked = %v, err %v; want %v", linked, err, testCase.want)
			}
		})
	}

	// Reports stored through provenance.StoreReport, so their lineage is the real thing.
	internalReport := storeTestReport(t, world, provenance.InternalInvestigationV1, provenance.InternalOnly,
		[]string{provenance.FieldInvoiceID, provenance.FieldInternalNote})
	vendorReport := storeTestReport(t, world, provenance.VendorReconciliationV1, provenance.VendorShareable,
		provenance.VendorInvoiceFieldsV1.Fields)
	// A report row without lineage, inserted directly.
	unlinedReport := testdb.ID(t)
	mustExec(t, world.pool, `INSERT INTO demo.reports (id, organization_id, run_id, created_by_action_id, template,
	                           classification, destination_class, title, content, content_hash)
	                         VALUES ($1, $2, $3, $4, 'internal_investigation_v1', 'internal_only', 'internal_reviewers',
	                                 'No lineage', 'x', sha256(convert_to('x', 'UTF8')))`,
		unlinedReport, world.run.OrganizationID, world.run.RunID, world.allowRead(t, world.invoiceA01))

	exportCases := []struct {
		name            string
		organizationID  string
		runID           string
		reportID        string
		wantFound       bool
		wantAllowed     bool
		wantReason      ReasonCode
		wantAlternative string
	}{
		{"vendor report may leave", world.run.OrganizationID, world.run.RunID, vendorReport, true, true, "", ""},
		{"internal report is restricted", world.run.OrganizationID, world.run.RunID, internalReport, true, false, ReasonReportExportRestricted, TemplateVendorReconciliation},
		{"report without lineage", world.run.OrganizationID, world.run.RunID, unlinedReport, true, false, ReasonReportLineageMissing, ""},
		{"report of another run", world.run.OrganizationID, testdb.ID(t), vendorReport, false, false, "", ""},
		{"report of another organization", foreignOrganization, world.run.RunID, vendorReport, false, false, "", ""},
		{"unknown report", world.run.OrganizationID, world.run.RunID, testdb.ID(t), false, false, "", ""},
	}
	for _, testCase := range exportCases {
		t.Run(testCase.name, func(t *testing.T) {
			verdict, err := relationships.ReportExport(ctx, testCase.organizationID, testCase.runID, testCase.reportID)
			if err != nil {
				t.Fatal(err)
			}
			if verdict.Found != testCase.wantFound || verdict.Allowed != testCase.wantAllowed ||
				verdict.ReasonCode != testCase.wantReason || verdict.AlternativeTemplate != testCase.wantAlternative {
				t.Fatalf("verdict = %+v", verdict)
			}
		})
	}

	// A changed source version makes the stored vendor report stale.
	mustExec(t, world.pool, `UPDATE demo.invoices SET version = version + 1 WHERE id = $1`, world.invoiceA01)
	verdict, err := relationships.ReportExport(ctx, world.run.OrganizationID, world.run.RunID, vendorReport)
	if err != nil || verdict.Allowed || verdict.ReasonCode != ReasonCode(provenance.ReasonResourceVersionChanged) {
		t.Fatalf("after a source change: verdict %+v, err %v; want resource_version_changed", verdict, err)
	}
}

// storeTestReport stores a report of this run with one invoice source and returns its id.
func storeTestReport(t *testing.T, world *executorWorld, template provenance.Template, classification string, fields []string) string {
	t.Helper()
	ctx := context.Background()
	tx, err := world.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stored, err := provenance.StoreReport(ctx, tx, provenance.NewReport{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID,
		CreatedByActionID: world.allowRead(t, world.invoiceA01), Template: template,
		Sources: []provenance.Source{{Kind: provenance.SourceInvoice, ID: world.invoiceA01, Version: 1,
			Classification: classification, ConsumedFields: fields}},
		Title: "Test report", Content: "INV104 appears twice.",
	})
	if err != nil {
		t.Fatalf("store report: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return stored.ID
}
