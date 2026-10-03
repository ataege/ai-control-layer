package provenance

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/testdb"
)

// reportWorld holds a run with a create_report action inside a transaction the test rolls back.
type reportWorld struct {
	tx                                pgx.Tx
	organizationID, runID, invoiceA01 string
}

func openReportWorld(t *testing.T) *reportWorld {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	world := &reportWorld{tx: tx, organizationID: testdb.ID(t), runID: testdb.ID(t), invoiceA01: "invoice_A01_" + testdb.ID(t)[:8]}
	passportID := testdb.ID(t)
	world.exec(t, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
	               VALUES ($1, $2, $3, 't', 1, '{}', '{}', now() + interval '15 minutes')`, passportID, world.organizationID, testdb.ID(t))
	world.exec(t, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		world.runID, world.organizationID, passportID)
	return world
}

func (world *reportWorld) exec(t *testing.T, sql string, arguments ...any) {
	t.Helper()
	if _, err := world.tx.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

// action stores a create_report action and returns its id.
func (world *reportWorld) action(t *testing.T, step int) string {
	t.Helper()
	actionID := testdb.ID(t)
	digest := sha256.Sum256([]byte(actionID))
	world.exec(t, `INSERT INTO runtime.actions (id, organization_id, run_id, step_number, tool, canonical_arguments,
	                 canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
	               VALUES ($1, $2, $3, $4, 'create_report', '{}', 1, $5, $6, 1, 'executing')`,
		actionID, world.organizationID, world.runID, step, digest[:], testdb.ID(t))
	return actionID
}

func TestStoreReportPersistsTheDerivedClassificationAndLineage(t *testing.T) {
	world := openReportWorld(t)
	stored, err := StoreReport(context.Background(), world.tx, NewReport{
		OrganizationID: world.organizationID, RunID: world.runID, CreatedByActionID: world.action(t, 1),
		Template: InternalInvestigationV1, Title: "Investigation", Content: "internal body",
		Sources: []Source{invoiceSource(world.invoiceA01, InternalOnly, FieldExternalReference, FieldInternalNote)},
	})
	if err != nil {
		t.Fatalf("StoreReport: %v", err)
	}
	loaded, err := LoadReport(context.Background(), world.tx, world.organizationID, world.runID, stored.ID)
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}
	if loaded.Classification != InternalOnly || len(loaded.Lineage) != 1 || loaded.Lineage[0].Classification != InternalOnly ||
		loaded.Lineage[0].TemplateVersion != 1 || loaded.ContentHash != ContentHash("internal body") {
		t.Fatalf("loaded = %+v", loaded)
	}
	decision := AuthorizeExport(loaded, DestinationRegisteredVendor, map[string]int{world.invoiceA01: 1})
	assertDenied(t, "internal report to the vendor", decision, ReasonReportExportRestricted)

	if _, err := LoadReport(context.Background(), world.tx, testdb.ID(t), world.runID, stored.ID); !errors.Is(err, ErrReportNotFound) {
		t.Errorf("another organization loaded the report: %v", err)
	}
}

func TestStoreReportWithoutLineageStoresNothing(t *testing.T) {
	world := openReportWorld(t)
	_, err := StoreReport(context.Background(), world.tx, NewReport{
		OrganizationID: world.organizationID, RunID: world.runID, CreatedByActionID: world.action(t, 1),
		Template: VendorReconciliationV1, Title: "Public summary", Content: "body",
	})
	if !errors.Is(err, ErrLineage) {
		t.Fatalf("err = %v, want ErrLineage", err)
	}
	var reports int
	if err := world.tx.QueryRow(context.Background(), `SELECT count(*) FROM demo.reports WHERE run_id = $1`, world.runID).Scan(&reports); err != nil {
		t.Fatal(err)
	}
	if reports != 0 {
		t.Fatalf("reports = %d, want 0", reports)
	}
}

func TestStoredLabelAndTitleCannotOverrideTheLineage(t *testing.T) {
	world := openReportWorld(t)
	// A row written outside the provenance path: vendor template, label vendor_shareable, title
	// "Public summary", but its lineage records an Internal only source; and a second row with no
	// lineage at all.
	tampered := insertRawVendorReport(t, world, 1, "Public summary")
	projection := VendorInvoiceFieldsV1.Name
	world.exec(t, `INSERT INTO runtime.report_lineage (organization_id, run_id, report_id, source_kind, source_id, source_version,
	                 source_classification, consumed_fields, template, template_version, projection_rule, projection_rule_version)
	               VALUES ($1, $2, $3, 'invoice', $4, 1, 'internal_only', ARRAY['external_reference'], 'vendor_reconciliation_v1', 1, $5, 1)`,
		world.organizationID, world.runID, tampered, world.invoiceA01, projection)
	withoutLineage := insertRawVendorReport(t, world, 2, "Vendor reconciliation")

	current := map[string]int{world.invoiceA01: 1}
	for name, reportID := range map[string]string{"tampered label": tampered, "missing lineage": withoutLineage} {
		loaded, err := LoadReport(context.Background(), world.tx, world.organizationID, world.runID, reportID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if decision := AuthorizeExport(loaded, DestinationRegisteredVendor, current); decision.Allowed {
			t.Errorf("%s: export allowed, want denied", name)
		}
	}
}

func insertRawVendorReport(t *testing.T, world *reportWorld, step int, title string) string {
	t.Helper()
	var reportID string
	err := world.tx.QueryRow(context.Background(),
		`INSERT INTO demo.reports (organization_id, run_id, created_by_action_id, template, projection_rule, projection_policy_version,
		   classification, destination_class, title, content, content_hash)
		 VALUES ($1, $2, $3, 'vendor_reconciliation_v1', 'vendor_invoice_fields_v1', 'report_projection_policy_v1',
		   'vendor_shareable', 'registered_vendor_recipient', $4, 'body', sha256(convert_to('body', 'UTF8'))) RETURNING id`,
		world.organizationID, world.runID, world.action(t, step), title,
	).Scan(&reportID)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}
	return reportID
}
