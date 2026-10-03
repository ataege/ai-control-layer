package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"
)

// passportScope is the part of the passport's stored `scope` the adapters check. The key names
// follow the X-08 passport contract's camelCase convention and wait for its freeze; unknown keys
// are ignored, a missing key means "nothing allowed".
type passportScope struct {
	InvoiceIDs           []string `json:"invoiceIds"`
	VendorIDs            []string `json:"vendorIds"`
	ReportTemplates      []string `json:"reportTemplates"`
	RecipientReferences  []string `json:"recipientReferences"`
	InternalNoteReadable bool     `json:"internalNoteReadable"`
}

// scope is the verified context one adapter call works under.
type scope struct {
	organizationID string
	runID          string
	passport       passportScope
}

func (current scope) allowsInvoice(invoiceID string) bool {
	return slices.Contains(current.passport.InvoiceIDs, invoiceID)
}

func (current scope) allowsVendor(vendorID string) bool {
	return slices.Contains(current.passport.VendorIDs, vendorID)
}

func (current scope) allowsTemplate(template string) bool {
	return slices.Contains(current.passport.ReportTemplates, template)
}

func (current scope) allowsRecipientReference(reference string) bool {
	return slices.Contains(current.passport.RecipientReferences, reference)
}

// loadScope reads the passport of this run inside tx: it must belong to the organization, be the
// run's passport and not be expired. A missing or unreadable passport fails closed.
func loadScope(ctx context.Context, tx pgx.Tx, request EffectRequest) (scope, error) {
	var rawScope []byte
	var unexpired bool
	err := tx.QueryRow(ctx,
		`SELECT passport.scope, passport.expires_at > now()
		   FROM runtime.passports AS passport
		   JOIN runtime.runs AS run
		     ON run.passport_id = passport.id AND run.organization_id = passport.organization_id
		  WHERE passport.id = $1 AND passport.organization_id = $2 AND run.id = $3`,
		request.PassportID, request.OrganizationID, request.RunID,
	).Scan(&rawScope, &unexpired)
	if errors.Is(err, pgx.ErrNoRows) {
		return scope{}, fmt.Errorf("%w: no passport for this run and organization", errPrecondition)
	}
	if err != nil {
		return scope{}, fmt.Errorf("tools: read passport: %w", err)
	}
	if !unexpired {
		return scope{}, fmt.Errorf("%w: passport expired", errPrecondition)
	}
	var decoded passportScope
	if err := json.Unmarshal(rawScope, &decoded); err != nil {
		return scope{}, fmt.Errorf("%w: unreadable passport scope", errPrecondition)
	}
	return scope{organizationID: request.OrganizationID, runID: request.RunID, passport: decoded}, nil
}
