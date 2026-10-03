package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRelationshipsUnavailable means a relationship could not be read; the gate then denies.
var ErrRelationshipsUnavailable = errors.New("relationship records unavailable")

// PostgresRelationships reads the GO-28 relationships from the demo records, always scoped by the
// verified organization. Every query is schema-qualified.
type PostgresRelationships struct{ pool *pgxpool.Pool }

// NewPostgresRelationships returns a reader on the given pool. It creates nothing.
func NewPostgresRelationships(pool *pgxpool.Pool) *PostgresRelationships {
	return &PostgresRelationships{pool: pool}
}

// VendorLinkedToInvoices is true when the vendor belongs to the organization and is the vendor of
// at least one of the given invoices of that organization.
func (relationships *PostgresRelationships) VendorLinkedToInvoices(ctx context.Context, organizationID, vendorID string, invoiceIDs []string) (bool, error) {
	if relationships.pool == nil {
		return false, ErrRelationshipsUnavailable
	}
	var linked bool
	err := relationships.pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM demo.invoices AS invoice
		     JOIN demo.vendors AS vendor
		       ON vendor.id = invoice.vendor_id AND vendor.organization_id = invoice.organization_id
		    WHERE invoice.organization_id = $1 AND vendor.id = $2 AND invoice.id = ANY($3))`,
		organizationID, vendorID, invoiceIDs).Scan(&linked)
	if err != nil {
		return false, ErrRelationshipsUnavailable
	}
	return linked, nil
}

// ReportOfRun is true when the report belongs to the organization and was created in the run.
func (relationships *PostgresRelationships) ReportOfRun(ctx context.Context, organizationID, runID, reportID string) (bool, error) {
	if relationships.pool == nil {
		return false, ErrRelationshipsUnavailable
	}
	var ofRun bool
	err := relationships.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM demo.reports WHERE id = $1 AND organization_id = $2 AND run_id = $3)`,
		reportID, organizationID, runID).Scan(&ofRun)
	if err != nil {
		return false, ErrRelationshipsUnavailable
	}
	return ofRun, nil
}
