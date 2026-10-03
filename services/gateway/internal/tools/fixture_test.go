package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/testdb"
)

// testWorld is a set of synthetic rows inside one transaction that the test rolls back, so no
// fixture survives (passports cannot be deleted; rollback is the only cleanup).
type testWorld struct {
	tx                  pgx.Tx
	organizationID      string
	otherOrganizationID string
	runID               string
	passportID          string
	// Ids are suffixed per test so parallel tests never collide on text primary keys.
	atlasID, borealisID, invoiceA01, invoiceA02, invoiceB01, invoiceC01 string
	steps                                                               int
}

// nextStep returns the next free step number for an action of this run.
func (world *testWorld) nextStep() int {
	world.steps++
	return world.steps
}

// openWorld opens a transaction on the test database, seeds the Atlas scenario into it and
// issues the run's passport with the scope buildScope returns (a passport cannot change later).
func openWorld(t *testing.T, buildScope func(*testWorld) passportScope) *testWorld {
	t.Helper()
	pool := testdb.Open(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })

	suffix := testdb.ID(t)[:8]
	world := &testWorld{
		tx:                  tx,
		organizationID:      testdb.ID(t),
		otherOrganizationID: testdb.ID(t),
		atlasID:             "vendor_Atlas_" + suffix,
		borealisID:          "vendor_Borealis_" + suffix,
		invoiceA01:          "invoice_A01_" + suffix,
		invoiceA02:          "invoice_A02_" + suffix,
		invoiceB01:          "invoice_B01_" + suffix,
		invoiceC01:          "invoice_C01_" + suffix,
	}
	world.exec(t, `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address)
	               VALUES ($1, $2, 'Atlas', 'reports@atlas.example.com'), ($3, $4, 'Borealis', NULL)`,
		world.atlasID, world.organizationID, world.borealisID, world.otherOrganizationID)
	world.exec(t, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency,
	                 total_minor_units, issued_on, due_on, internal_note, internal_note_classification)
	               VALUES ($1, $5, $6, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31',
	                       'Investigation note: INV104 appears twice.', 'internal_only'),
	                      ($2, $5, $6, 'INV104', 'EUR', 125000, '2026-09-08', '2026-10-31', NULL, NULL),
	                      ($3, $5, $6, 'INV211', 'EUR', 48000, '2026-09-15', '2026-11-15', NULL, NULL),
	                      ($4, $7, $8, 'BOR-0007', 'EUR', 99000, '2026-09-03', '2026-10-03', NULL, NULL)`,
		world.invoiceA01, world.invoiceA02, world.invoiceB01, world.invoiceC01,
		world.organizationID, world.atlasID, world.otherOrganizationID, world.borealisID)

	world.passportID = testdb.ID(t)
	world.runID = testdb.ID(t)
	scopeJSON, err := json.Marshal(buildScope(world))
	if err != nil {
		t.Fatalf("encode scope: %v", err)
	}
	world.exec(t, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
	                 admission_catalog_revision_id, scope, limits, expires_at)
	               VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, $4, '{}', now() + interval '15 minutes')`,
		world.passportID, world.organizationID, testdb.ID(t), scopeJSON)
	world.exec(t, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		world.runID, world.organizationID, world.passportID)
	return world
}

// scenarioScope returns the scenario's passport scope with the test's suffixed ids.
func scenarioScope(world *testWorld, noteReadable bool) passportScope {
	return passportScope{
		InvoiceIDs:           []string{world.invoiceA01, world.invoiceA02},
		VendorIDs:            []string{world.atlasID},
		ReportTemplates:      []string{"internal_investigation_v1", "vendor_reconciliation_v1"},
		InternalNoteReadable: noteReadable,
	}
}

func (world *testWorld) exec(t *testing.T, sql string, arguments ...any) {
	t.Helper()
	if _, err := world.tx.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("fixture: %v", err)
	}
}

// proposeAction stores an action and its open attempt, as the executor would, and returns the
// request for RunEffect.
func (world *testWorld) proposeAction(t *testing.T, step int, tool string, arguments any) EffectRequest {
	t.Helper()
	rawArguments, err := json.Marshal(arguments)
	if err != nil {
		t.Fatalf("encode arguments: %v", err)
	}
	digest := sha256.Sum256(append([]byte(tool), rawArguments...))
	request := EffectRequest{
		OrganizationID:     world.organizationID,
		RunID:              world.runID,
		PassportID:         world.passportID,
		ActionID:           testdb.ID(t),
		AttemptID:          testdb.ID(t),
		Tool:               tool,
		CanonicalArguments: rawArguments,
		ActionDigest:       digest,
		CatalogRevisionID:  1,
	}
	world.exec(t, `INSERT INTO runtime.actions (id, organization_id, run_id, step_number, tool, canonical_arguments,
	                 canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
	               VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8, 1, 'executing')`,
		request.ActionID, world.organizationID, world.runID, step, tool, rawArguments, digest[:], testdb.ID(t))
	world.exec(t, `INSERT INTO runtime.execution_attempts (id, organization_id, action_id, attempt_number) VALUES ($1, $2, $3, 1)`,
		request.AttemptID, world.organizationID, request.ActionID)
	return request
}

// newAttempt adds another open attempt for an existing action, as a safe retry would.
func (world *testWorld) newAttempt(t *testing.T, actionID string, number int) string {
	t.Helper()
	attemptID := testdb.ID(t)
	world.exec(t, `INSERT INTO runtime.execution_attempts (id, organization_id, action_id, attempt_number) VALUES ($1, $2, $3, $4)`,
		attemptID, world.organizationID, actionID, number)
	return attemptID
}

// count returns a single integer from a query.
func (world *testWorld) count(t *testing.T, sql string, arguments ...any) int {
	t.Helper()
	var value int
	if err := world.tx.QueryRow(context.Background(), sql, arguments...).Scan(&value); err != nil {
		t.Fatalf("count: %v", err)
	}
	return value
}
