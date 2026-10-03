package provenance

import (
	"fmt"
	"sort"
	"strings"
)

// InvoiceSnapshot is one source invoice as Go read it from demo.invoices for rendering.
type InvoiceSnapshot struct {
	ID, VendorID, ExternalReference, Currency, IssuedOn, DueOn string
	Version                                                    int
	TotalMinorUnits                                            int64
	// Note is set only when the note was authorized for this report; NoteClassification with it.
	Note, NoteClassification *string
}

// currencyExponents gives the minor-unit exponent of the currencies the fixtures use. Minor units
// are not always cents; an unlisted currency is rendered in minor units, never guessed.
var currencyExponents = map[string]int{"EUR": 2, "USD": 2, "GBP": 2, "PLN": 2, "JPY": 0}

// FormatAmount renders an integer minor-unit amount without floating point.
func FormatAmount(currency string, minorUnits int64) string {
	exponent, known := currencyExponents[currency]
	if !known {
		return fmt.Sprintf("%d %s minor units", minorUnits, currency)
	}
	if exponent == 0 {
		return fmt.Sprintf("%s %d", currency, minorUnits)
	}
	sign := ""
	if minorUnits < 0 {
		sign, minorUnits = "-", -minorUnits
	}
	divisor := int64(1)
	for range exponent {
		divisor *= 10
	}
	return fmt.Sprintf("%s %s%d.%0*d", currency, sign, minorUnits/divisor, exponent, minorUnits%divisor)
}

// RepeatedReferences returns, for every external reference that appears on more than one of the
// invoices, the sorted ids of those invoices. It is the duplicate-reference finding.
func RepeatedReferences(invoices []InvoiceSnapshot) map[string][]string {
	byReference := make(map[string][]string)
	for _, invoice := range invoices {
		byReference[invoice.ExternalReference] = append(byReference[invoice.ExternalReference], invoice.ID)
	}
	repeated := make(map[string][]string)
	for reference, invoiceIDs := range byReference {
		if len(invoiceIDs) > 1 {
			sort.Strings(invoiceIDs)
			repeated[reference] = invoiceIDs
		}
	}
	return repeated
}

// sortedByID returns the invoices in id order, so identical inputs render identical bytes.
func sortedByID(invoices []InvoiceSnapshot) []InvoiceSnapshot {
	sorted := append([]InvoiceSnapshot(nil), invoices...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left].ID < sorted[right].ID })
	return sorted
}

// RenderInternalInvestigation renders internal_investigation_v1 deterministically from the
// trusted invoices and their authorized notes; no model prose enters it.
func RenderInternalInvestigation(invoices []InvoiceSnapshot) string {
	var text strings.Builder
	sorted := sortedByID(invoices)
	text.WriteString("Internal investigation report (internal_investigation_v1)\n")
	text.WriteString("Classification: Internal only\n\nInvoices:\n")
	for _, invoice := range sorted {
		fmt.Fprintf(&text, "- %s (version %d): external reference %s, total %s, issued %s, due %s\n",
			invoice.ID, invoice.Version, invoice.ExternalReference,
			FormatAmount(invoice.Currency, invoice.TotalMinorUnits), invoice.IssuedOn, invoice.DueOn)
	}
	text.WriteString("\nRepeated external references:\n")
	repeated := RepeatedReferences(sorted)
	if len(repeated) == 0 {
		text.WriteString("- none\n")
	}
	references := make([]string, 0, len(repeated))
	for reference := range repeated {
		references = append(references, reference)
	}
	sort.Strings(references)
	for _, reference := range references {
		fmt.Fprintf(&text, "- %s: %s\n", reference, strings.Join(repeated[reference], ", "))
	}
	text.WriteString("\nInternal notes:\n")
	noted := false
	for _, invoice := range sorted {
		if invoice.Note != nil {
			fmt.Fprintf(&text, "- %s: %s\n", invoice.ID, *invoice.Note)
			noted = true
		}
	}
	if !noted {
		text.WriteString("- none\n")
	}
	text.WriteString("\nNo fraud is determined and no payment is authorized by this report.\n")
	return text.String()
}

// RenderVendorReconciliation renders vendor_reconciliation_v1 directly from the approved
// projection (vendor_invoice_fields_v1): invoice reference, external reference, the
// duplicate-reference flag, currency and total, due date. It has no parameter for the note, the
// internal report or model prose, and identical inputs give identical bytes.
func RenderVendorReconciliation(invoices []InvoiceSnapshot) string {
	var text strings.Builder
	sorted := sortedByID(invoices)
	repeated := RepeatedReferences(sorted)
	text.WriteString("Vendor reconciliation (vendor_reconciliation_v1, projection vendor_invoice_fields_v1)\n\n")
	for _, invoice := range sorted {
		duplicate := "no"
		if _, isRepeated := repeated[invoice.ExternalReference]; isRepeated {
			duplicate = "yes"
		}
		fmt.Fprintf(&text, "- %s: external reference %s, total %s, due %s, duplicate reference: %s\n",
			invoice.ID, invoice.ExternalReference, FormatAmount(invoice.Currency, invoice.TotalMinorUnits),
			invoice.DueOn, duplicate)
	}
	if len(repeated) > 0 {
		references := make([]string, 0, len(repeated))
		for reference := range repeated {
			references = append(references, reference)
		}
		sort.Strings(references)
		fmt.Fprintf(&text, "\nPlease confirm whether these external references were submitted more than once: %s.\n",
			strings.Join(references, ", "))
	}
	return text.String()
}
