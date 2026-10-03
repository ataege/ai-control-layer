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
