package policy

import (
	"context"
	"testing"

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

	// A report created by an action of this run.
	creatingAction := world.allowRead(t, world.invoiceA01)
	reportID := testdb.ID(t)
	mustExec(t, world.pool, `INSERT INTO demo.reports (id, organization_id, run_id, created_by_action_id, template,
	                           classification, destination_class, title, content, content_hash)
	                         VALUES ($1, $2, $3, $4, 'internal_investigation_v1', 'internal_only', 'internal',
	                                 'Investigation', 'INV104 appears twice.', sha256(convert_to('INV104 appears twice.', 'UTF8')))`,
		reportID, world.run.OrganizationID, world.run.RunID, creatingAction)

	reportCases := []struct {
		name           string
		organizationID string
		runID          string
		reportID       string
		want           bool
	}{
		{"report of this run", world.run.OrganizationID, world.run.RunID, reportID, true},
		{"report of another run", world.run.OrganizationID, testdb.ID(t), reportID, false},
		{"report of another organization", foreignOrganization, world.run.RunID, reportID, false},
		{"unknown report", world.run.OrganizationID, world.run.RunID, testdb.ID(t), false},
	}
	for _, testCase := range reportCases {
		t.Run(testCase.name, func(t *testing.T) {
			ofRun, err := relationships.ReportOfRun(ctx, testCase.organizationID, testCase.runID, testCase.reportID)
			if err != nil || ofRun != testCase.want {
				t.Fatalf("ofRun = %v, err %v; want %v", ofRun, err, testCase.want)
			}
		})
	}
}
