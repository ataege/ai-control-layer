package admission

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
)

// The display text of the registered template and approval rule (GO-25).
const (
	reconcileAtlasName           = "Reconcile Atlas invoices"
	reviewQueueReportDescription = "A reviewer approves the exact report and recipient before the report is queued."
)

// OptionsDatabase reads the organization's demo records and the active catalog; the gateway pool.
type OptionsDatabase interface {
	catalog.Querier
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
}

// ErrOptionsUnavailable means the options could not be read: no enforceable active catalog or a
// storage failure. The form gets no options rather than limits admission would not honour.
var ErrOptionsUnavailable = errors.New("task form options unavailable")

// TaskOptions returns the task form's choices for the operator's organization (GO-25): the
// registered template and approval rule, the organization's vendors (each also a destination,
// as admission resolves the destination to a vendor), its invoices by display fields only, and
// the highest limits admission accepts under the active catalog. Records whose ids admission
// would refuse are not offered.
func TaskOptions(ctx context.Context, database OptionsDatabase, loader *catalog.Loader, organizationID string) (contracts.TaskFormOptions, error) {
	if ctx == nil || database == nil || loader == nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}
	snapshot, err := loader.Active(ctx, database)
	if err != nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}
	options := contracts.TaskFormOptions{
		Templates:            []contracts.TaskFormOption{{ID: TaskTemplateReconcileAtlas, Name: reconcileAtlasName}},
		Vendors:              []contracts.TaskFormOption{},
		Invoices:             []contracts.TaskFormInvoice{},
		Destinations:         []contracts.TaskFormOption{},
		ApprovalRequirements: []contracts.TaskFormApprovalRequirement{{ID: ApprovalRuleReviewQueueReport, Description: reviewQueueReportDescription}},
		Limits: contracts.TaskFormLimits{
			MaxModelCalls:     snapshot.Limits.CallsTotal,
			MaxTimeoutSeconds: snapshot.Limits.RunExpiryMinutes * 60,
		},
	}

	vendorRows, err := database.Query(ctx, `SELECT id, name FROM demo.vendors
		WHERE organization_id = $1 ORDER BY name, id`, organizationID)
	if err != nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}
	for vendorRows.Next() {
		var vendor contracts.TaskFormOption
		if err := vendorRows.Scan(&vendor.ID, &vendor.Name); err != nil {
			vendorRows.Close()
			return contracts.TaskFormOptions{}, ErrOptionsUnavailable
		}
		if contracts.ValidVendorID(vendor.ID) {
			options.Vendors = append(options.Vendors, vendor)
			options.Destinations = append(options.Destinations, vendor)
		}
	}
	vendorRows.Close()
	if vendorRows.Err() != nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}

	invoiceRows, err := database.Query(ctx, `SELECT id, external_reference, to_char(issued_on, 'YYYY-MM-DD'),
		total_minor_units, vendor_id FROM demo.invoices WHERE organization_id = $1 ORDER BY issued_on, id`, organizationID)
	if err != nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}
	for invoiceRows.Next() {
		var invoice contracts.TaskFormInvoice
		if err := invoiceRows.Scan(&invoice.ID, &invoice.Number, &invoice.Date, &invoice.Amount, &invoice.VendorID); err != nil {
			invoiceRows.Close()
			return contracts.TaskFormOptions{}, ErrOptionsUnavailable
		}
		if contracts.ValidInvoiceID(invoice.ID) && contracts.ValidVendorID(invoice.VendorID) {
			options.Invoices = append(options.Invoices, invoice)
		}
	}
	invoiceRows.Close()
	if invoiceRows.Err() != nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}
	return options, nil
}

// OptionsReader serves TaskOptions on one database and catalog loader.
type OptionsReader struct {
	database OptionsDatabase
	loader   *catalog.Loader
}

// NewOptionsReader returns a reader on the gateway pool and the shared catalog loader.
func NewOptionsReader(database OptionsDatabase, loader *catalog.Loader) *OptionsReader {
	return &OptionsReader{database: database, loader: loader}
}

// TaskOptions returns the options of the organization; see the package function.
func (reader *OptionsReader) TaskOptions(ctx context.Context, organizationID string) (contracts.TaskFormOptions, error) {
	if reader == nil {
		return contracts.TaskFormOptions{}, ErrOptionsUnavailable
	}
	return TaskOptions(ctx, reader.database, reader.loader, organizationID)
}
