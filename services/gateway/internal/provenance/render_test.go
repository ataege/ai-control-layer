package provenance

import (
	"strings"
	"testing"
)

func TestFormatAmountUsesIntegerMinorUnits(t *testing.T) {
	cases := map[string]string{
		FormatAmount("EUR", 125000): "EUR 1250.00",
		FormatAmount("EUR", 5):      "EUR 0.05",
		FormatAmount("EUR", -150):   "EUR -1.50",
		FormatAmount("JPY", 900):    "JPY 900",
		FormatAmount("XTS", 42):     "42 XTS minor units",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestInternalRenderingIsDeterministic(t *testing.T) {
	note, classification := "Investigation note.", InternalOnly
	first := []InvoiceSnapshot{
		{ID: "invoice_A02", Version: 1, ExternalReference: "INV104", Currency: "EUR", TotalMinorUnits: 125000, IssuedOn: "2026-09-08", DueOn: "2026-10-31"},
		{ID: "invoice_A01", Version: 1, ExternalReference: "INV104", Currency: "EUR", TotalMinorUnits: 125000, IssuedOn: "2026-09-01", DueOn: "2026-10-31", Note: &note, NoteClassification: &classification},
	}
	second := []InvoiceSnapshot{first[1], first[0]}
	if RenderInternalInvestigation(first) != RenderInternalInvestigation(second) {
		t.Fatal("input order changed the rendered bytes")
	}
	rendered := RenderInternalInvestigation(first)
	if !strings.Contains(rendered, "- INV104: invoice_A01, invoice_A02\n") || !strings.Contains(rendered, "EUR 1250.00") {
		t.Fatalf("rendered:\n%s", rendered)
	}
}

func TestVendorRenderingGoldenBytes(t *testing.T) {
	note, classification := "Investigation note: INV104 appears twice.", InternalOnly
	invoices := []InvoiceSnapshot{
		{ID: "invoice_A02", Version: 1, ExternalReference: "INV104", Currency: "EUR", TotalMinorUnits: 125000, IssuedOn: "2026-09-08", DueOn: "2026-10-31"},
		{ID: "invoice_A01", Version: 1, ExternalReference: "INV104", Currency: "EUR", TotalMinorUnits: 125000, IssuedOn: "2026-09-01", DueOn: "2026-10-31", Note: &note, NoteClassification: &classification},
	}
	want := "Vendor reconciliation (vendor_reconciliation_v1, projection vendor_invoice_fields_v1)\n\n" +
		"- invoice_A01: external reference INV104, total EUR 1250.00, due 2026-10-31, duplicate reference: yes\n" +
		"- invoice_A02: external reference INV104, total EUR 1250.00, due 2026-10-31, duplicate reference: yes\n" +
		"\nPlease confirm whether these external references were submitted more than once: INV104.\n"
	if got := RenderVendorReconciliation(invoices); got != want {
		t.Fatalf("rendered:\n%q\nwant:\n%q", got, want)
	}
	if got := RenderVendorReconciliation([]InvoiceSnapshot{invoices[1], invoices[0]}); got != want {
		t.Fatal("input order changed the rendered bytes")
	}
	rendered := RenderVendorReconciliation(invoices)
	if strings.Contains(rendered, "Investigation note") || strings.Contains(rendered, "2026-09-01") {
		t.Fatal("the vendor report holds the note or a field outside the projection")
	}
}
