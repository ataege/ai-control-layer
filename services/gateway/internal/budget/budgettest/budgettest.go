// Package budgettest seeds synthetic runs with an open model ledger for database-backed tests.
// It is test support only: production code opens ledgers through admission (OpenRunLedger) and
// writes dispatch records through budget.CallLog.
package budgettest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

// Run identifies one seeded synthetic run.
type Run struct {
	OrganizationID string
	RunID          string
}

// Limits returns the report's illustrative passport limits with the given shared token total.
func Limits(tokensTotal int64) contracts.PassportLimits {
	return contracts.PassportLimits{CallsTotal: 24, CallsAgent: 12, CallsSecurity: 12, TokensTotal: tokensTotal,
		RequestTimeoutSeconds: 20, LocalMaxConcurrency: 2, ToolAttempts: 12, Corrections: 2, RunExpiryMinutes: 15}
}

// OpenRun inserts a synthetic passport (allowing the model "test-fixture"), run and open ledger and removes them, with every dispatch
// record and reservation of the run, when the test ends. Passports reject DELETE by trigger, so
// cleanup runs with triggers disabled for its transaction (superuser test database); otherwise it
// leaves the synthetic rows and logs them.
func OpenRun(t testing.TB, pool *pgxpool.Pool, limits contracts.PassportLimits) Run {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run := Run{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)}
	passportID := testdb.ID(t)
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal("begin ledger fixture")
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, `INSERT INTO runtime.passports(id, organization_id, actor_id, task_version,
		admission_catalog_revision_id, scope, limits, expires_at)
		VALUES ($1, $2, $3, 'ledger_test_v1', 1, '{"allowedModels": ["test-fixture"]}', '{}', now() + interval '1 hour')`,
		passportID, run.OrganizationID, testdb.ID(t)); err != nil {
		t.Fatalf("insert passport fixture: %v", err)
	}
	if _, err = transaction.Exec(ctx, `INSERT INTO runtime.runs(id, organization_id, passport_id, status)
		VALUES ($1, $2, $3, 'running')`, run.RunID, run.OrganizationID, passportID); err != nil {
		t.Fatalf("insert run fixture: %v", err)
	}
	if err = budget.OpenRunLedger(ctx, transaction, run.OrganizationID, run.RunID, limits); err != nil {
		t.Fatalf("open ledger fixture: %v", err)
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Fatal("commit ledger fixture")
	}
	t.Cleanup(func() { remove(t, pool, run.OrganizationID) })
	return run
}

// remove deletes every runtime row of the synthetic organization.
func remove(t testing.TB, pool *pgxpool.Pool, organizationID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Error("clean ledger fixture")
		return
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
		t.Logf("ledger fixture of organization %s left in place (cleanup needs a superuser)", organizationID)
		return
	}
	for _, table := range []string{
		"runtime.model_token_reservations", "runtime.model_token_budgets", "runtime.control_assessments",
		"runtime.timing_records", "runtime.model_calls", "runtime.runs", "runtime.passports",
	} {
		if _, err = transaction.Exec(ctx, "DELETE FROM "+table+" WHERE organization_id = $1", organizationID); err != nil {
			t.Errorf("clean %s: %v", table, err)
			return
		}
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Error("commit ledger fixture cleanup")
	}
}

// RecordDispatch writes the dispatch record a reservation needs, under a chosen call id.
func RecordDispatch(t testing.TB, pool *pgxpool.Pool, run Run, callID, purpose string) {
	t.Helper()
	if _, err := budget.NewCallLog(pool).RecordDispatchForRun(context.Background(), callID, run.RunID, purpose, "test-fixture"); err != nil {
		t.Fatalf("record dispatch %s: %v", callID, err)
	}
}

// DispatchRecordingStore is a ledger for tests whose caller picks its own call ids without a call
// log (for example the semantic evaluator driven directly): Reserve first writes the missing
// dispatch record, as RecordingCaller does in production.
type DispatchRecordingStore struct {
	*budget.PostgresStore
	callLog *budget.CallLog
}

// NewDispatchRecordingStore wraps the ledger on the pool.
func NewDispatchRecordingStore(pool *pgxpool.Pool) *DispatchRecordingStore {
	return &DispatchRecordingStore{PostgresStore: budget.NewPostgresStore(pool), callLog: budget.NewCallLog(pool)}
}

// Reserve records the dispatch, ignoring an existing record, then reserves.
func (store *DispatchRecordingStore) Reserve(ctx context.Context, runID, callID, purpose string, tokens int64) (budget.Reservation, error) {
	_, _ = store.callLog.RecordDispatchForRun(ctx, callID, runID, purpose, "test-fixture")
	return store.PostgresStore.Reserve(ctx, runID, callID, purpose, tokens)
}

// ReserveWithin records the dispatch like Reserve, then reserves under the ceiling.
func (store *DispatchRecordingStore) ReserveWithin(ctx context.Context, runID, callID, purpose string, tokens int64,
	ceiling budget.Ceiling) (budget.Reservation, error) {
	_, _ = store.callLog.RecordDispatchForRun(ctx, callID, runID, purpose, "test-fixture")
	return store.PostgresStore.ReserveWithin(ctx, runID, callID, purpose, tokens, ceiling)
}
