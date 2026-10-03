package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
)

// readVendorArguments are the X-09 arguments of read_vendor.
type readVendorArguments struct {
	VendorID string `json:"vendor_id"`
}

// VendorResult is the model-facing result of read_vendor; its fields are the GO-07 allowlist.
// The registered reporting address never appears: only its opaque, run-scoped reference.
type VendorResult struct {
	VendorID           string  `json:"vendor_id"`
	Version            int     `json:"version"`
	Name               string  `json:"name"`
	RecipientReference *string `json:"recipient_reference,omitempty"`
}

// readVendor returns the allowlisted fields of a vendor that is in the passport scope, belongs to
// the organization and is the vendor of at least one passport-scoped invoice ("read_vendor should
// not accept an arbitrary vendor identifier simply because the tool name is allowed").
func readVendor(ctx context.Context, tx pgx.Tx, current scope, rawArguments json.RawMessage) (adapterOutcome, error) {
	var arguments readVendorArguments
	if err := decodeArguments(rawArguments, &arguments); err != nil {
		return adapterOutcome{}, err
	}
	if arguments.VendorID == "" {
		return adapterOutcome{}, fmt.Errorf("%w: vendor_id is required", errPrecondition)
	}
	if !current.allowsVendor(arguments.VendorID) {
		return failed(ReasonResourceOutOfScope), nil
	}

	var result VendorResult
	var hasRegisteredAddress bool
	err := tx.QueryRow(ctx,
		`SELECT vendor.id, vendor.version, vendor.name, vendor.registered_reporting_address IS NOT NULL
		   FROM demo.vendors AS vendor
		  WHERE vendor.id = $1 AND vendor.organization_id = $2
		    AND EXISTS (SELECT 1 FROM demo.invoices AS invoice
		                 WHERE invoice.vendor_id = vendor.id AND invoice.organization_id = vendor.organization_id
		                   AND invoice.id = ANY($3))`,
		arguments.VendorID, current.organizationID, current.passport.InvoiceIDs,
	).Scan(&result.VendorID, &result.Version, &result.Name, &hasRegisteredAddress)
	if errors.Is(err, pgx.ErrNoRows) {
		// Another organization's vendor, an unlinked vendor and a missing one look the same.
		return failed(ReasonResourceOutOfScope), nil
	}
	if err != nil {
		return adapterOutcome{}, fmt.Errorf("tools: read vendor: %w", err)
	}
	if hasRegisteredAddress {
		reference := recipientReference(current.runID, result.VendorID)
		result.RecipientReference = &reference
	}
	return succeeded(result, contracts.EventActionSucceeded, contracts.MaskedSummary{Effect: text("read")}), nil
}

// recipientReference is the opaque, run-scoped reference for a vendor's registered reporting
// address (GO-07 format). It holds no address and is not a secret.
func recipientReference(runID, vendorID string) string {
	return "recipient:" + runID + ":" + vendorID
}
