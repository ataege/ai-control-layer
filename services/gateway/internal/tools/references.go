package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// resolvedRecipient is a recipient reference resolved inside an adapter after its checks. The
// address never leaves the adapter: it goes into the simulated outbox row only.
type resolvedRecipient struct {
	vendorID string
	address  string
}

// resolveRecipient turns an opaque recipient reference into the vendor's registered reporting
// address (GO-35). The reference must name this run, be listed in the passport's
// recipientReferences, and point to a vendor in the passport scope and the organization that is
// linked to a passport-scoped invoice and has a registered address. Any other reference returns
// destination_not_allowed, so a reference from another run or organization means nothing here.
func resolveRecipient(ctx context.Context, tx pgx.Tx, current scope, reference string) (resolvedRecipient, string, error) {
	parts := strings.Split(reference, ":")
	if len(parts) != 3 || parts[0] != "recipient" || parts[1] == "" || parts[2] == "" {
		return resolvedRecipient{}, ReasonDestinationNotAllowed, nil
	}
	runID, vendorID := parts[1], parts[2]
	if runID != current.runID || !current.allowsRecipientReference(reference) || !current.allowsVendor(vendorID) {
		return resolvedRecipient{}, ReasonDestinationNotAllowed, nil
	}

	var address *string
	err := tx.QueryRow(ctx,
		`SELECT vendor.registered_reporting_address
		   FROM demo.vendors AS vendor
		  WHERE vendor.id = $1 AND vendor.organization_id = $2
		    AND EXISTS (SELECT 1 FROM demo.invoices AS invoice
		                 WHERE invoice.vendor_id = vendor.id AND invoice.organization_id = vendor.organization_id
		                   AND invoice.id = ANY($3))`,
		vendorID, current.organizationID, current.passport.InvoiceIDs,
	).Scan(&address)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && address == nil) {
		return resolvedRecipient{}, ReasonDestinationNotAllowed, nil
	}
	if err != nil {
		return resolvedRecipient{}, "", fmt.Errorf("tools: resolve recipient: %w", err)
	}
	return resolvedRecipient{vendorID: vendorID, address: *address}, "", nil
}

// ResolveRecipientForReview resolves a recipient reference for the reviewer's frozen review
// payload (GO-43), with exactly the checks queue_report applies (resolveRecipient). It loads the
// scope of the run's stored passport itself, so a caller cannot widen it: the reference must name
// this active run, be listed in the passport's recipientReferences, and point to a vendor of the
// organization in the passport's vendorIds that is linked to a passport-scoped invoice and has a
// registered address. It only reads. The address is for the reviewer's payload only and must
// never reach the model. A refusal returns reasonCode destination_not_allowed and empty values; a
// run without an active passport of this organization is a precondition error.
func ResolveRecipientForReview(ctx context.Context, tx pgx.Tx, organizationID, runID, reference string) (vendorID, address, reasonCode string, err error) {
	var passportID string
	err = tx.QueryRow(ctx,
		`SELECT passport_id::text FROM runtime.runs WHERE id = $1 AND organization_id = $2`,
		runID, organizationID,
	).Scan(&passportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", fmt.Errorf("%w: no run of this organization", errPrecondition)
	}
	if err != nil {
		return "", "", "", fmt.Errorf("tools: read run: %w", err)
	}
	current, err := loadScope(ctx, tx, EffectRequest{OrganizationID: organizationID, RunID: runID, PassportID: passportID})
	if err != nil {
		return "", "", "", err
	}
	recipient, reason, err := resolveRecipient(ctx, tx, current, reference)
	if err != nil || reason != "" {
		return "", "", reason, err
	}
	return recipient.vendorID, recipient.address, "", nil
}
