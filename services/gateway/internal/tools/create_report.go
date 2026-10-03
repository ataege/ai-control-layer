package tools

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/provenance"
)

// createReportArguments are the X-09 arguments of create_report. The model chooses a registered
// template and authorized invoice references only; it supplies no classification, title, source
// list of its own or content.
type createReportArguments struct {
	Template         string   `json:"template"`
	SourceInvoiceIDs []string `json:"source_invoice_ids"`
}

// ReportResult is the model-facing result of create_report; its fields are the GO-07 allowlist.
type ReportResult struct {
	ReportID         string   `json:"report_id"`
	Version          int      `json:"version"`
	Template         string   `json:"template"`
	Classification   string   `json:"classification"`
	ContentHash      string   `json:"content_hash"`
	SourceInvoiceIDs []string `json:"source_invoice_ids"`
}

// createReport resolves the trusted source invoices, renders the registered template on the
// server and stores the report with its derived classification and lineage (GO-63), all in the
// executor's transaction. One report per action: a repeat fails at demo.reports' uniqueness.
func createReport(ctx context.Context, tx pgx.Tx, current scope, request EffectRequest) (adapterOutcome, error) {
	var arguments createReportArguments
	if err := decodeArguments(request.CanonicalArguments, &arguments); err != nil {
		return adapterOutcome{}, err
	}
	template, registered := provenance.LookupTemplate(arguments.Template)
	if !registered || !current.allowsTemplate(template.Name) {
		return failed(ReasonTemplateNotAllowed), nil
	}
	sourceIDs := slices.Clone(arguments.SourceInvoiceIDs)
	slices.Sort(sourceIDs)
	if len(sourceIDs) == 0 || len(slices.Compact(slices.Clone(sourceIDs))) != len(sourceIDs) {
		return adapterOutcome{}, fmt.Errorf("%w: source_invoice_ids must be non-empty and distinct", errPrecondition)
	}
	for _, invoiceID := range sourceIDs {
		if !current.allowsInvoice(invoiceID) {
			return failed(ReasonResourceOutOfScope), nil
		}
	}
	invoices, err := loadInvoiceSnapshots(ctx, tx, current, sourceIDs)
	if err != nil {
		return adapterOutcome{}, err
	}
	if len(invoices) != len(sourceIDs) {
		return failed(ReasonResourceOutOfScope), nil
	}

	var content string
	var sources []provenance.Source
	switch template.Name {
	case provenance.InternalInvestigationV1.Name:
		content, sources = renderInternal(invoices, current.passport.InternalNoteReadable)
	case provenance.VendorReconciliationV1.Name:
		var vendorID string
		content, sources, vendorID = renderVendor(invoices)
		if vendorID == "" || !current.allowsVendor(vendorID) {
			return failed(ReasonResourceOutOfScope), nil
		}
	default:
		return failed(ReasonTemplateNotAllowed), nil
	}

	stored, err := provenance.StoreReport(ctx, tx, provenance.NewReport{
		OrganizationID: current.organizationID, RunID: current.runID, CreatedByActionID: request.ActionID,
		Template: template, Sources: sources, Title: reportTitle(template), Content: content,
	})
	if errors.Is(err, provenance.ErrLineage) {
		outcome := failed(ReasonReportLineageMissing)
		outcome.events[0].summary.LineageCheck = text("missing")
		return outcome, nil
	}
	if err != nil {
		return adapterOutcome{}, err
	}
	storedTemplate := contracts.ReportTemplate(stored.TemplateName)
	return succeeded(ReportResult{
		ReportID: stored.ID, Version: stored.Version, Template: stored.TemplateName,
		Classification: stored.Classification, ContentHash: hex.EncodeToString(stored.ContentHash[:]),
		SourceInvoiceIDs: sourceIDs,
	}, contracts.EventReportCreated, contracts.MaskedSummary{
		ReportID: &stored.ID, Template: &storedTemplate, Classification: &stored.Classification,
		LineageCheck: text("passed"), Effect: text("report_created"),
	}), nil
}

// reportTitle is the fixed display title of a template; a title never grants export authority.
func reportTitle(template provenance.Template) string {
	if template.Name == provenance.VendorReconciliationV1.Name {
		return "Vendor reconciliation"
	}
	return "Internal investigation"
}

// renderInternal renders internal_investigation_v1 and its lineage. The note enters only where
// the passport's field rule allows it, and then the source carries the note's classification.
func renderInternal(invoices []provenance.InvoiceSnapshot, noteReadable bool) (string, []provenance.Source) {
	sources := make([]provenance.Source, 0, len(invoices))
	rendered := make([]provenance.InvoiceSnapshot, 0, len(invoices))
	for _, invoice := range invoices {
		fields := []string{provenance.FieldInvoiceID, provenance.FieldExternalReference, provenance.FieldCurrency,
			provenance.FieldTotalMinorUnits, provenance.FieldIssuedOn, provenance.FieldDueOn}
		classification := provenance.VendorShareable
		if !noteReadable || invoice.Note == nil {
			invoice.Note, invoice.NoteClassification = nil, nil
		} else {
			fields = append(fields, provenance.FieldInternalNote)
			classification = *invoice.NoteClassification
		}
		rendered = append(rendered, invoice)
		sources = append(sources, provenance.Source{Kind: provenance.SourceInvoice, ID: invoice.ID,
			Version: invoice.Version, Classification: classification, ConsumedFields: fields})
	}
	return provenance.RenderInternalInvestigation(rendered), sources
}

// renderVendor renders vendor_reconciliation_v1 from the approved projection (GO-65). Every
// source must have the same vendor, which it returns ("" when they differ). The note is dropped
// before rendering and never consumed, so each source contributes exactly the projection fields
// of Vendor shareable data.
func renderVendor(invoices []provenance.InvoiceSnapshot) (string, []provenance.Source, string) {
	vendorID := invoices[0].VendorID
	projected := make([]provenance.InvoiceSnapshot, 0, len(invoices))
	sources := make([]provenance.Source, 0, len(invoices))
	for _, invoice := range invoices {
		if invoice.VendorID != vendorID {
			return "", nil, ""
		}
		invoice.Note, invoice.NoteClassification = nil, nil
		projected = append(projected, invoice)
		sources = append(sources, provenance.Source{Kind: provenance.SourceInvoice, ID: invoice.ID,
			Version: invoice.Version, Classification: provenance.VendorShareable,
			ConsumedFields: slices.Clone(provenance.VendorInvoiceFieldsV1.Fields)})
	}
	return provenance.RenderVendorReconciliation(projected), sources, vendorID
}

// loadInvoiceSnapshots reads the organization's invoices with these ids.
func loadInvoiceSnapshots(ctx context.Context, tx pgx.Tx, current scope, invoiceIDs []string) ([]provenance.InvoiceSnapshot, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, version, vendor_id, external_reference, currency, total_minor_units, issued_on::text, due_on::text,
		        internal_note, internal_note_classification
		   FROM demo.invoices
		  WHERE organization_id = $1 AND id = ANY($2)
		  ORDER BY id`,
		current.organizationID, invoiceIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("tools: read source invoices: %w", err)
	}
	defer rows.Close()
	var invoices []provenance.InvoiceSnapshot
	for rows.Next() {
		var invoice provenance.InvoiceSnapshot
		if err := rows.Scan(&invoice.ID, &invoice.Version, &invoice.VendorID, &invoice.ExternalReference,
			&invoice.Currency, &invoice.TotalMinorUnits, &invoice.IssuedOn, &invoice.DueOn,
			&invoice.Note, &invoice.NoteClassification); err != nil {
			return nil, fmt.Errorf("tools: read source invoice: %w", err)
		}
		if invoice.Note != nil && invoice.NoteClassification == nil {
			// An unclassified note leaves the lineage unresolved: fail closed.
			return nil, fmt.Errorf("%w: note without classification", errPrecondition)
		}
		invoices = append(invoices, invoice)
	}
	return invoices, rows.Err()
}
