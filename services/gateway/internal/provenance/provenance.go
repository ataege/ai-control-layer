// Package provenance holds the report provenance rules (GO-63): the Go-registered templates and
// projection rule, classification derived from trusted sources only, and the export decision
// made from a report's stored lineage. Nothing here accepts a classification, a title or a source
// list from the model: "an agent-provided title, declared label, or request to call a report
// 'Public summary' would not reset them".
package provenance

import (
	"errors"
	"fmt"
	"slices"

	"starter/services/gateway/internal/contracts"
)

// Classifications: the prototype's two trusted labels (demo.reports.classification values).
const (
	InternalOnly    = "internal_only"
	VendorShareable = "vendor_shareable"
)

// Destination classes stored with a report.
const (
	DestinationInternalReviewers = "internal_reviewers"
	DestinationRegisteredVendor  = "registered_vendor_recipient"
)

// Source kinds recorded in runtime.report_lineage.
const SourceInvoice = "invoice"

// Fields a source can contribute. FieldInternalNote is the authorized internal note.
const (
	FieldInvoiceID          = "invoice_id"
	FieldExternalReference  = "external_reference"
	FieldDuplicateReference = "duplicate_reference"
	FieldCurrency           = "currency"
	FieldTotalMinorUnits    = "total_minor_units"
	FieldIssuedOn           = "issued_on"
	FieldDueOn              = "due_on"
	FieldInternalNote       = "internal_note"
)

// Projection is a Go-registered, versioned field projection.
type Projection struct {
	Name          string
	Version       int
	PolicyVersion string
	Fields        []string
}

// VendorInvoiceFieldsV1 is the approved vendor projection (lead's decision on `vendor projection
// fields`): invoice reference, external reference, the duplicate-reference flag, currency, total
// and due date; never the internal note.
var VendorInvoiceFieldsV1 = Projection{
	Name:          "vendor_invoice_fields_v1",
	Version:       1,
	PolicyVersion: "report_projection_policy_v1",
	Fields: []string{FieldInvoiceID, FieldExternalReference, FieldDuplicateReference,
		FieldCurrency, FieldTotalMinorUnits, FieldDueOn},
}

// Template is a Go-registered, versioned report template.
type Template struct {
	Name             string
	Version          int
	DestinationClass string
	// AlwaysInternalOnly: the template's reports are Internal only whatever their sources.
	AlwaysInternalOnly bool
	// Projection is the only field set the template may consume; nil means the template renders
	// authorized investigation fields, including the internal note.
	Projection *Projection
}

// The two registered templates. The internal template "would always be classified Internal only
// and retain the trusted restrictions of all consumed sources"; the vendor template renders "a
// deterministic server projection of explicitly approved invoice fields".
var (
	InternalInvestigationV1 = Template{
		Name: "internal_investigation_v1", Version: 1,
		DestinationClass: DestinationInternalReviewers, AlwaysInternalOnly: true,
	}
	VendorReconciliationV1 = Template{
		Name: "vendor_reconciliation_v1", Version: 1,
		DestinationClass: DestinationRegisteredVendor, Projection: &VendorInvoiceFieldsV1,
	}
)

var registeredTemplates = map[string]Template{
	InternalInvestigationV1.Name: InternalInvestigationV1,
	VendorReconciliationV1.Name:  VendorReconciliationV1,
}

// LookupTemplate returns a registered template; any other name is not a template.
func LookupTemplate(name string) (Template, bool) {
	template, found := registeredTemplates[name]
	return template, found
}

// Reason codes of the export decision, from the shared X-13 vocabulary (contracts.ReasonCode),
// as plain strings so callers can convert them to their own types.
const (
	ReasonReportExportRestricted = string(contracts.ReasonReportExportRestricted)
	ReasonReportLineageMissing   = string(contracts.ReasonReportLineageMissing)
	ReasonTemplateNotAllowed     = string(contracts.ReasonTemplateNotAllowed)
	ReasonResourceVersionChanged = string(contracts.ReasonResourceVersionChanged)
	ReasonDestinationNotAllowed  = string(contracts.ReasonDestinationNotAllowed)
)

// ErrLineage marks sources from which no trusted classification can be derived.
var ErrLineage = errors.New("provenance: no trusted lineage")

// Source is one trusted source a report consumes, as Go resolved it from the database: never a
// model-supplied record.
type Source struct {
	Kind           string
	ID             string
	Version        int
	Classification string
	ConsumedFields []string
}

// DeriveClassification derives a report's classification from its template and the trusted
// sources it consumes. Missing, unclassified or unresolved lineage fails closed.
func DeriveClassification(template Template, sources []Source) (string, error) {
	if len(sources) == 0 {
		return "", fmt.Errorf("%w: a report needs at least one trusted source", ErrLineage)
	}
	if _, registered := registeredTemplates[template.Name]; !registered {
		return "", fmt.Errorf("%w: unregistered template", ErrLineage)
	}
	for _, source := range sources {
		if source.ID == "" || source.Version <= 0 || len(source.ConsumedFields) == 0 {
			return "", fmt.Errorf("%w: unresolved source", ErrLineage)
		}
		if source.Classification != InternalOnly && source.Classification != VendorShareable {
			return "", fmt.Errorf("%w: unclassified source %s", ErrLineage, source.ID)
		}
	}
	if template.AlwaysInternalOnly {
		return InternalOnly, nil
	}
	// A projection template may consume only its approved fields of Vendor shareable sources.
	for _, source := range sources {
		if source.Classification != VendorShareable {
			return "", fmt.Errorf("%w: %s is %s, the vendor projection cannot consume it", ErrLineage, source.ID, source.Classification)
		}
		for _, field := range source.ConsumedFields {
			if !slices.Contains(template.Projection.Fields, field) {
				return "", fmt.Errorf("%w: field %s is outside %s", ErrLineage, field, template.Projection.Name)
			}
		}
	}
	return VendorShareable, nil
}
