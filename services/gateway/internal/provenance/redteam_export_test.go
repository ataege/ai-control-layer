package provenance

import (
	"strings"
	"testing"
)

// TestAuthorizeExportRefusesEveryTamperedOrLookalikeRecord (lane w2): the export check compares
// stored values exactly. Each row changes one thing in an otherwise exportable vendor report, with
// a case change, a stray space, a look-alike or a changed byte, and must be denied.
func TestAuthorizeExportRefusesEveryTamperedOrLookalikeRecord(t *testing.T) {
	current := map[string]int{"invoice_A01": 1}
	if decision := AuthorizeExport(storedVendorReport(VendorShareable), DestinationRegisteredVendor, current); !decision.Allowed {
		t.Fatalf("baseline: %+v", decision)
	}
	text := func(value string) *string { return &value }
	number := func(value int) *int { return &value }
	rows := []struct {
		name        string
		change      func(report *StoredReport)
		destination string
		versions    map[string]int
	}{
		{"source label other case", func(r *StoredReport) { r.Lineage[0].Classification = "Vendor_Shareable" }, DestinationRegisteredVendor, current},
		{"source label trailing space", func(r *StoredReport) { r.Lineage[0].Classification = VendorShareable + " " }, DestinationRegisteredVendor, current},
		{"source label upper case", func(r *StoredReport) { r.Lineage[0].Classification = strings.ToUpper(VendorShareable) }, DestinationRegisteredVendor, current},
		{"source label empty", func(r *StoredReport) { r.Lineage[0].Classification = "" }, DestinationRegisteredVendor, current},
		{"source kind other case", func(r *StoredReport) { r.Lineage[0].Kind = "Invoice" }, DestinationRegisteredVendor, current},
		{"source kind trailing space", func(r *StoredReport) { r.Lineage[0].Kind = SourceInvoice + " " }, DestinationRegisteredVendor, current},
		{"source id empty", func(r *StoredReport) { r.Lineage[0].ID = "" }, DestinationRegisteredVendor, current},
		{"source version zero", func(r *StoredReport) { r.Lineage[0].Version = 0 }, DestinationRegisteredVendor, current},
		{"source version negative", func(r *StoredReport) { r.Lineage[0].Version = -1 }, DestinationRegisteredVendor, current},
		{"consumed field outside the projection", func(r *StoredReport) {
			r.Lineage[0].ConsumedFields = append(r.Lineage[0].ConsumedFields, FieldInternalNote)
		}, DestinationRegisteredVendor, current},
		{"consumed field other case", func(r *StoredReport) { r.Lineage[0].ConsumedFields = []string{"External_Reference"} }, DestinationRegisteredVendor, current},
		{"consumed field trailing space", func(r *StoredReport) { r.Lineage[0].ConsumedFields = []string{FieldExternalReference + " "} }, DestinationRegisteredVendor, current},
		{"no consumed fields", func(r *StoredReport) { r.Lineage[0].ConsumedFields = nil }, DestinationRegisteredVendor, current},
		{"one internal source among shareable ones", func(r *StoredReport) {
			extra := r.Lineage[0]
			extra.Source = invoiceSource("invoice_A02", InternalOnly, FieldExternalReference)
			r.Lineage = append(r.Lineage, extra)
		}, DestinationRegisteredVendor, map[string]int{"invoice_A01": 1, "invoice_A02": 1}},
		{"template name other case", func(r *StoredReport) { r.TemplateName = "Vendor_Reconciliation_V1" }, DestinationRegisteredVendor, current},
		{"template name trailing space", func(r *StoredReport) { r.TemplateName = VendorReconciliationV1.Name + " " }, DestinationRegisteredVendor, current},
		{"lineage template name other case", func(r *StoredReport) { r.Lineage[0].TemplateName = "Vendor_Reconciliation_V1" }, DestinationRegisteredVendor, current},
		{"lineage template version zero", func(r *StoredReport) { r.Lineage[0].TemplateVersion = 0 }, DestinationRegisteredVendor, current},
		{"lineage template version ahead", func(r *StoredReport) { r.Lineage[0].TemplateVersion = 2 }, DestinationRegisteredVendor, current},
		{"projection rule other case", func(r *StoredReport) { r.Lineage[0].ProjectionRule = text("Vendor_Invoice_Fields_V1") }, DestinationRegisteredVendor, current},
		{"projection rule missing", func(r *StoredReport) { r.Lineage[0].ProjectionRule = nil }, DestinationRegisteredVendor, current},
		{"projection version changed", func(r *StoredReport) { r.Lineage[0].ProjectionRuleVersion = number(2) }, DestinationRegisteredVendor, current},
		{"projection version missing", func(r *StoredReport) { r.Lineage[0].ProjectionRuleVersion = nil }, DestinationRegisteredVendor, current},
		{"projection policy missing", func(r *StoredReport) { r.ProjectionPolicy = nil }, DestinationRegisteredVendor, current},
		{"projection policy other case", func(r *StoredReport) { r.ProjectionPolicy = text(strings.ToUpper(*r.ProjectionPolicy)) }, DestinationRegisteredVendor, current},
		{"projection policy empty", func(r *StoredReport) { r.ProjectionPolicy = text("") }, DestinationRegisteredVendor, current},
		{"content with a trailing newline", func(r *StoredReport) { r.Content += "\n" }, DestinationRegisteredVendor, current},
		{"content with a zero-width space", func(r *StoredReport) { r.Content += "​" }, DestinationRegisteredVendor, current},
		{"content hash zero", func(r *StoredReport) { r.ContentHash = [32]byte{} }, DestinationRegisteredVendor, current},
		{"content hash of another text", func(r *StoredReport) { r.ContentHash = ContentHash("another body") }, DestinationRegisteredVendor, current},
		{"destination other case", func(*StoredReport) {}, "Registered_Vendor", current},
		{"destination trailing space", func(*StoredReport) {}, DestinationRegisteredVendor + " ", current},
		{"destination empty", func(*StoredReport) {}, "", current},
		{"destination internal reviewers", func(*StoredReport) {}, DestinationInternalReviewers, current},
		{"destination unknown", func(*StoredReport) {}, "public_web", current},
		{"source version unknown", func(*StoredReport) {}, DestinationRegisteredVendor, map[string]int{}},
		{"source id other case in the version map", func(*StoredReport) {}, DestinationRegisteredVendor, map[string]int{"Invoice_A01": 1}},
		{"source version newer", func(*StoredReport) {}, DestinationRegisteredVendor, map[string]int{"invoice_A01": 2}},
		{"source version nil map", func(*StoredReport) {}, DestinationRegisteredVendor, nil},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			report := storedVendorReport(VendorShareable)
			report.Lineage = append([]LineageEntry(nil), report.Lineage...)
			report.Lineage[0].ConsumedFields = append([]string(nil), report.Lineage[0].ConsumedFields...)
			row.change(&report)
			if decision := AuthorizeExport(report, row.destination, row.versions); decision.Allowed {
				t.Fatalf("exported: %+v", decision)
			}
		})
	}
}

// TestInternalReportNeverLeavesForAnyVendorSpelling: an internal report is denied for the vendor
// destination, and for every other spelling of a destination it is denied too (only the exact
// internal class is a destination for it).
func TestInternalReportNeverLeavesForAnyVendorSpelling(t *testing.T) {
	content := "internal body"
	internal := StoredReport{
		TemplateName: InternalInvestigationV1.Name, Classification: VendorShareable, // a lying stored label
		DestinationClass: DestinationRegisteredVendor, Title: "Public summary", Content: content, ContentHash: ContentHash(content),
		Lineage: []LineageEntry{{
			Source:       invoiceSource("invoice_A01", InternalOnly, FieldExternalReference, FieldInternalNote),
			TemplateName: InternalInvestigationV1.Name, TemplateVersion: 1,
		}},
	}
	current := map[string]int{"invoice_A01": 1}
	for _, destination := range []string{DestinationRegisteredVendor, "Registered_Vendor", DestinationRegisteredVendor + " ", "", "public_web", "vendor"} {
		if decision := AuthorizeExport(internal, destination, current); decision.Allowed {
			t.Errorf("destination %q: %+v", destination, decision)
		}
	}
	if decision := AuthorizeExport(internal, DestinationInternalReviewers, current); !decision.Allowed {
		t.Errorf("the internal destination: %+v, want allowed", decision)
	}
}
