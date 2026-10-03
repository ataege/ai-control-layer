package provenance

import (
	"errors"
	"testing"
)

func invoiceSource(id string, classification string, fields ...string) Source {
	return Source{Kind: SourceInvoice, ID: id, Version: 1, Classification: classification, ConsumedFields: fields}
}

func TestInternalTemplateIsAlwaysInternalOnly(t *testing.T) {
	// Even when every source is Vendor shareable, the internal template stays Internal only.
	classification, err := DeriveClassification(InternalInvestigationV1, []Source{
		invoiceSource("invoice_A01", VendorShareable, FieldExternalReference),
	})
	if err != nil || classification != InternalOnly {
		t.Fatalf("classification = %q, %v; want internal_only", classification, err)
	}
}

func TestVendorTemplateAcceptsOnlyApprovedFieldsOfShareableSources(t *testing.T) {
	approved := VendorInvoiceFieldsV1.Fields
	classification, err := DeriveClassification(VendorReconciliationV1, []Source{
		invoiceSource("invoice_A01", VendorShareable, approved...),
		invoiceSource("invoice_A02", VendorShareable, approved...),
	})
	if err != nil || classification != VendorShareable {
		t.Fatalf("classification = %q, %v; want vendor_shareable", classification, err)
	}
	for name, sources := range map[string][]Source{
		"internal note consumed":   {invoiceSource("invoice_A01", VendorShareable, FieldInternalNote)},
		"internal only source":     {invoiceSource("invoice_A01", InternalOnly, FieldExternalReference)},
		"field outside projection": {invoiceSource("invoice_A01", VendorShareable, FieldIssuedOn)},
	} {
		if _, err := DeriveClassification(VendorReconciliationV1, sources); !errors.Is(err, ErrLineage) {
			t.Errorf("%s: err = %v, want ErrLineage", name, err)
		}
	}
}

func TestMissingUnclassifiedOrUnresolvedLineageFailsClosed(t *testing.T) {
	cases := map[string][]Source{
		"no sources":    nil,
		"unclassified":  {invoiceSource("invoice_A01", "public", FieldExternalReference)},
		"no version":    {{Kind: SourceInvoice, ID: "invoice_A01", Classification: VendorShareable, ConsumedFields: []string{FieldCurrency}}},
		"no fields":     {invoiceSource("invoice_A01", VendorShareable)},
		"no identifier": {invoiceSource("", VendorShareable, FieldCurrency)},
	}
	for name, sources := range cases {
		for _, template := range []Template{InternalInvestigationV1, VendorReconciliationV1} {
			if _, err := DeriveClassification(template, sources); !errors.Is(err, ErrLineage) {
				t.Errorf("%s / %s: err = %v, want ErrLineage", name, template.Name, err)
			}
		}
	}
	if _, err := DeriveClassification(Template{Name: "public_summary_v1", Version: 1}, []Source{
		invoiceSource("invoice_A01", VendorShareable, FieldCurrency),
	}); !errors.Is(err, ErrLineage) {
		t.Errorf("unregistered template: err = %v, want ErrLineage", err)
	}
}

// storedVendorReport builds a consistent stored vendor report for AuthorizeExport tests.
func storedVendorReport(lineageClassification string) StoredReport {
	content := "vendor body"
	projection := VendorInvoiceFieldsV1.Name
	version := VendorInvoiceFieldsV1.Version
	policy := VendorInvoiceFieldsV1.PolicyVersion
	return StoredReport{
		TemplateName: VendorReconciliationV1.Name, Classification: VendorShareable, ProjectionPolicy: &policy,
		DestinationClass: DestinationRegisteredVendor, Title: "Vendor reconciliation",
		Content: content, ContentHash: ContentHash(content),
		Lineage: []LineageEntry{{
			Source:       invoiceSource("invoice_A01", lineageClassification, FieldExternalReference, FieldCurrency),
			TemplateName: VendorReconciliationV1.Name, TemplateVersion: 1,
			ProjectionRule: &projection, ProjectionRuleVersion: &version,
		}},
	}
}

func TestAuthorizeExportDecidesFromTheStoredLineage(t *testing.T) {
	current := map[string]int{"invoice_A01": 1}
	if decision := AuthorizeExport(storedVendorReport(VendorShareable), DestinationRegisteredVendor, current); !decision.Allowed {
		t.Fatalf("vendor report: %+v, want allowed", decision)
	}

	// The stored label says vendor_shareable and the title says Public summary, but a source in
	// the lineage is Internal only: the lineage wins.
	tampered := storedVendorReport(InternalOnly)
	tampered.Title = "Public summary"
	if decision := AuthorizeExport(tampered, DestinationRegisteredVendor, current); decision.Allowed {
		t.Fatalf("tampered label: %+v, want denied", decision)
	}

	missing := storedVendorReport(VendorShareable)
	missing.Lineage = nil
	assertDenied(t, "missing lineage", AuthorizeExport(missing, DestinationRegisteredVendor, current), ReasonReportLineageMissing)

	changedContent := storedVendorReport(VendorShareable)
	changedContent.Content = "vendor body, edited"
	assertDenied(t, "changed content", AuthorizeExport(changedContent, DestinationRegisteredVendor, current), ReasonReportLineageMissing)

	assertDenied(t, "changed source version", AuthorizeExport(storedVendorReport(VendorShareable), DestinationRegisteredVendor,
		map[string]int{"invoice_A01": 2}), ReasonResourceVersionChanged)

	oldTemplate := storedVendorReport(VendorShareable)
	oldTemplate.Lineage[0].TemplateVersion = 0
	assertDenied(t, "template version", AuthorizeExport(oldTemplate, DestinationRegisteredVendor, current), ReasonTemplateNotAllowed)
}

func TestInternalReportExportToTheVendorIsRestricted(t *testing.T) {
	content := "internal body"
	internal := StoredReport{
		TemplateName: InternalInvestigationV1.Name, Classification: InternalOnly,
		DestinationClass: DestinationInternalReviewers, Content: content, ContentHash: ContentHash(content),
		Lineage: []LineageEntry{{
			Source:       invoiceSource("invoice_A01", InternalOnly, FieldExternalReference, FieldInternalNote),
			TemplateName: InternalInvestigationV1.Name, TemplateVersion: 1,
		}},
	}
	decision := AuthorizeExport(internal, DestinationRegisteredVendor, map[string]int{"invoice_A01": 1})
	assertDenied(t, "internal export", decision, ReasonReportExportRestricted)
	if decision.AlternativeTemplate != VendorReconciliationV1.Name {
		t.Fatalf("alternative = %q, want the vendor template", decision.AlternativeTemplate)
	}
}

func assertDenied(t *testing.T, name string, decision ExportDecision, reasonCode string) {
	t.Helper()
	if decision.Allowed || decision.ReasonCode != reasonCode {
		t.Errorf("%s: %+v, want denied with %s", name, decision, reasonCode)
	}
}

func TestDerivationUsesTheRegisteredTemplateNotTheCallersCopy(t *testing.T) {
	forged := Template{Name: InternalInvestigationV1.Name, Version: 1} // AlwaysInternalOnly cleared
	classification, err := DeriveClassification(forged, []Source{invoiceSource("invoice_A01", VendorShareable, FieldCurrency)})
	if err != nil || classification != InternalOnly {
		t.Fatalf("classification = %q, %v; want the registered internal_only", classification, err)
	}
	vendorSource := Source{Kind: "vendor", ID: "vendor_Atlas", Version: 1, Classification: VendorShareable, ConsumedFields: []string{FieldCurrency}}
	if _, err := DeriveClassification(InternalInvestigationV1, []Source{vendorSource}); !errors.Is(err, ErrLineage) {
		t.Fatalf("a non-invoice source: err = %v, want ErrLineage", err)
	}
}

func TestAuthorizeExportChecksTheStoredProjectionPolicyVersion(t *testing.T) {
	stale := storedVendorReport(VendorShareable)
	oldPolicy := "report_projection_policy_v0"
	stale.ProjectionPolicy = &oldPolicy
	assertDenied(t, "stale policy", AuthorizeExport(stale, DestinationRegisteredVendor, map[string]int{"invoice_A01": 1}), ReasonTemplateNotAllowed)
}
