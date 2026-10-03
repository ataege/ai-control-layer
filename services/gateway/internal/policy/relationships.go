package policy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/provenance"
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

// ReportExport loads the report and its lineage in one read-only transaction and asks
// provenance.AuthorizeExport, which decides from the stored lineage only (the stored
// classification column and the title carry no authority), for the registered vendor recipient.
func (relationships *PostgresRelationships) ReportExport(ctx context.Context, organizationID, runID, reportID string) (ExportVerdict, error) {
	if relationships.pool == nil {
		return ExportVerdict{}, ErrRelationshipsUnavailable
	}
	var verdict ExportVerdict
	err := pgx.BeginTxFunc(ctx, relationships.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		report, err := provenance.LoadReport(ctx, tx, organizationID, runID, reportID)
		if errors.Is(err, provenance.ErrReportNotFound) {
			return nil // verdict.Found stays false
		}
		if err != nil {
			return err
		}
		var invoiceIDs []string
		for _, entry := range report.Lineage {
			if entry.Kind == provenance.SourceInvoice {
				invoiceIDs = append(invoiceIDs, entry.ID)
			}
		}
		currentVersions, err := provenance.CurrentInvoiceVersions(ctx, tx, organizationID, invoiceIDs)
		if err != nil {
			return err
		}
		decision := provenance.AuthorizeExport(report, provenance.DestinationRegisteredVendor, currentVersions)
		verdict = ExportVerdict{
			Found: true, Allowed: decision.Allowed, ReasonCode: ReasonCode(decision.ReasonCode),
			AlternativeTemplate: decision.AlternativeTemplate,
			// The stored references of the report; the classification is the stored fact.
			Report: ReportRef{ID: report.ID, Template: report.TemplateName, Classification: report.Classification},
		}
		return nil
	})
	if err != nil {
		return ExportVerdict{}, ErrRelationshipsUnavailable
	}
	return verdict, nil
}
