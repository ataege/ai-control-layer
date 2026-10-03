package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
)

// readInvoiceArguments are the X-09 arguments of read_invoice.
type readInvoiceArguments struct {
	InvoiceID string `json:"invoice_id"`
}

// InvoiceResult is the model-facing result of read_invoice; its fields are the GO-07 allowlist.
type InvoiceResult struct {
	InvoiceID         string       `json:"invoice_id"`
	Version           int          `json:"version"`
	VendorID          string       `json:"vendor_id"`
	ExternalReference string       `json:"external_reference"`
	Currency          string       `json:"currency"`
	TotalMinorUnits   int64        `json:"total_minor_units"`
	IssuedOn          string       `json:"issued_on"`
	DueOn             string       `json:"due_on"`
	InternalNote      *InvoiceNote `json:"internal_note,omitempty"`
}

// InvoiceNote is the authorized internal note with its trusted classification. It is readable
// for the internal investigation; the classification travels with it and is never cleared.
type InvoiceNote struct {
	Text           string `json:"text"`
	Classification string `json:"classification"`
}

// readInvoice returns the allowlisted fields of one passport-scoped invoice of the organization.
func readInvoice(ctx context.Context, tx pgx.Tx, current scope, rawArguments json.RawMessage) (adapterOutcome, error) {
	var arguments readInvoiceArguments
	if err := decodeArguments(rawArguments, &arguments); err != nil {
		return adapterOutcome{}, err
	}
	if arguments.InvoiceID == "" {
		return adapterOutcome{}, fmt.Errorf("%w: invoice_id is required", errPrecondition)
	}
	if !current.allowsInvoice(arguments.InvoiceID) {
		return failed(ReasonResourceOutOfScope), nil
	}

	var result InvoiceResult
	var noteText, noteClassification *string
	err := tx.QueryRow(ctx,
		`SELECT id, version, vendor_id, external_reference, currency, total_minor_units,
		        issued_on::text, due_on::text, internal_note, internal_note_classification
		   FROM demo.invoices
		  WHERE id = $1 AND organization_id = $2`,
		arguments.InvoiceID, current.organizationID,
	).Scan(&result.InvoiceID, &result.Version, &result.VendorID, &result.ExternalReference,
		&result.Currency, &result.TotalMinorUnits, &result.IssuedOn, &result.DueOn,
		&noteText, &noteClassification)
	if errors.Is(err, pgx.ErrNoRows) {
		// Another organization's invoice and a missing one look the same: no existence leak.
		return failed(ReasonResourceOutOfScope), nil
	}
	if err != nil {
		return adapterOutcome{}, fmt.Errorf("tools: read invoice: %w", err)
	}
	if noteText != nil && current.passport.InternalNoteReadable {
		if noteClassification == nil {
			// An unclassified note never reaches the model (fail closed).
			return adapterOutcome{}, fmt.Errorf("%w: note without classification", errPrecondition)
		}
		result.InternalNote = &InvoiceNote{Text: *noteText, Classification: *noteClassification}
	}
	return succeeded(result, contracts.EventActionSucceeded, contracts.MaskedSummary{Effect: text("read")}), nil
}
