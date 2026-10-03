package contracts

import "regexp"

// Record identifier shapes (X-09), shared by admission and the gate's decoder so the two never
// drift: a fixed prefix and letters, digits, underscores and hyphens, so no prose fits inside an
// id. Both stay within the semantic action check's constrained identifier format (at most 128
// characters). They are compiled once; callers use them as read-only values.
var (
	InvoiceIDPattern = regexp.MustCompile(`^invoice_[A-Za-z0-9_-]{1,120}$`)
	VendorIDPattern  = regexp.MustCompile(`^vendor_[A-Za-z0-9_-]{1,121}$`)
)

// ValidInvoiceID reports whether value has the shape of an invoice identifier.
func ValidInvoiceID(value string) bool { return InvoiceIDPattern.MatchString(value) }

// ValidVendorID reports whether value has the shape of a vendor identifier.
func ValidVendorID(value string) bool { return VendorIDPattern.MatchString(value) }
