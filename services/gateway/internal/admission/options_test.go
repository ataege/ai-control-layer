package admission

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// GO-25: the form offers the organization's own records by display fields only, and limits that
// admission accepts under the active catalog.
func TestPostgresTaskOptionsOfferOnlyTheOrganizationsRecords(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	options, err := TaskOptions(ctx, fixture.outer, catalog.NewLoader(), fixture.operator.OrganizationID)
	if err != nil {
		t.Fatalf("TaskOptions: %v", err)
	}

	vendorIDs := func(choices []contracts.TaskFormOption) []string {
		ids := make([]string, 0, len(choices))
		for _, choice := range choices {
			ids = append(ids, choice.ID)
		}
		return ids
	}
	// Sorted by name: Atlas, Borealis, Silent; another organization's vendor is not offered.
	wantVendors := []string{fixture.vendorID, fixture.otherVendorID, fixture.silentVendorID}
	if got := vendorIDs(options.Vendors); !slices.Equal(got, wantVendors) {
		t.Errorf("vendors %v, want %v", got, wantVendors)
	}
	if !slices.Equal(options.Destinations, options.Vendors) {
		t.Errorf("destinations %v differ from vendors %v", options.Destinations, options.Vendors)
	}
	var invoiceIDs []string
	for _, invoice := range options.Invoices {
		invoiceIDs = append(invoiceIDs, invoice.ID)
		if invoice.Number != "INV104" || invoice.Date != "2026-09-01" || invoice.Amount != 125000 {
			t.Errorf("invoice %+v: want INV104, 2026-09-01, 125000", invoice)
		}
	}
	if len(invoiceIDs) != 4 || slices.Contains(invoiceIDs, fixture.foreignInvoice) ||
		!slices.Contains(invoiceIDs, fixture.invoiceIDs[0]) || !slices.Contains(invoiceIDs, fixture.otherInvoiceID) {
		t.Errorf("invoices %v: want the organization's four, not %s", invoiceIDs, fixture.foreignInvoice)
	}
	if len(options.Templates) != 1 || options.Templates[0].ID != TaskTemplateReconcileAtlas ||
		len(options.ApprovalRequirements) != 1 || options.ApprovalRequirements[0].ID != ApprovalRuleReviewQueueReport {
		t.Errorf("templates %v, approval requirements %v", options.Templates, options.ApprovalRequirements)
	}

	// The limits are exactly the highest values admission accepts under the active catalog.
	snapshot, err := catalog.NewLoader().Active(ctx, fixture.outer)
	if err != nil {
		t.Fatal(err)
	}
	if options.Limits.MaxModelCalls != snapshot.Limits.CallsTotal || options.Limits.MaxTimeoutSeconds != snapshot.Limits.RunExpiryMinutes*60 {
		t.Errorf("limits %+v, want calls_total %d and %d s", options.Limits, snapshot.Limits.CallsTotal, snapshot.Limits.RunExpiryMinutes*60)
	}

	// Every offered choice is admitted (GO-13) for the same operator: the template, the approval
	// rule, a vendor as destination with its own invoices, and the maximum limits.
	modelCalls, timeoutSeconds := options.Limits.MaxModelCalls, options.Limits.MaxTimeoutSeconds
	approval := options.ApprovalRequirements[0].ID
	vendorID := options.Vendors[0].ID
	if _, err := fixture.admitter.Admit(ctx, fixture.operator, contracts.StartRunRequest{
		Template: options.Templates[0].ID, VendorID: &vendorID, InvoiceIDs: fixture.invoiceIDs,
		Destination: options.Destinations[0].ID, ApprovalRequirement: &approval,
		Limits: &contracts.StartRunLimits{ModelCalls: &modelCalls, TimeoutSeconds: &timeoutSeconds},
	}); err != nil {
		t.Errorf("admission refused the offered choices at their maximum limits: %v", err)
	}

	// No protected vendor field reaches the form.
	encoded, _ := json.Marshal(options)
	if strings.Contains(string(encoded), "reports@") || strings.Contains(string(encoded), "registered_reporting_address") {
		t.Errorf("a protected field reached the options: %s", encoded)
	}
	var decoded contracts.TaskFormOptions
	if err := contracts.DecodeStrict(encoded, &decoded); err != nil {
		t.Errorf("the options do not round-trip: %v", err)
	}
}

func TestPostgresTaskOptionsFailClosedWithoutAnActiveCatalog(t *testing.T) {
	fixture := newFixture(t)
	exec(t, fixture.outer, `UPDATE app.control_catalog_pointer SET active_revision_id = NULL WHERE id = 1`)
	if _, err := TaskOptions(context.Background(), fixture.outer, catalog.NewLoader(), fixture.operator.OrganizationID); !errors.Is(err, ErrOptionsUnavailable) {
		t.Fatalf("got %v, want ErrOptionsUnavailable", err)
	}
}

func TestPostgresTaskOptionsOfAnOrganizationWithoutRecordsAreEmptyLists(t *testing.T) {
	fixture := newFixture(t)
	options, err := TaskOptions(context.Background(), fixture.outer, catalog.NewLoader(), testdb.ID(t))
	if err != nil {
		t.Fatalf("TaskOptions: %v", err)
	}
	encoded, _ := json.Marshal(options)
	if options.Vendors == nil || options.Invoices == nil || options.Destinations == nil ||
		!strings.Contains(string(encoded), `"vendors":[]`) || !strings.Contains(string(encoded), `"invoices":[]`) {
		t.Errorf("an organization without records must get empty lists, not null: %s", encoded)
	}
}
