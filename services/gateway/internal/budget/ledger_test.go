package budget_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/budget/budgettest"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

func int64Value(value int64) *int64 { return &value }

func TestOpenRunLedgerRejectsUnusableLimitsBeforeWriting(t *testing.T) {
	organizationID, runID := "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01", "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
	valid := budgettest.Limits(20000)
	adjusted := func(change func(*contracts.PassportLimits)) contracts.PassportLimits {
		limits := budgettest.Limits(20000)
		change(&limits)
		return limits
	}
	for name, testCase := range map[string]struct {
		organizationID, runID string
		limits                contracts.PassportLimits
	}{
		"no total":                {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.TokensTotal = 0 })},
		"total beyond safe range": {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.TokensTotal = 9007199254740992 })},
		"agent above total":       {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.TokensAgent = int64Value(20001) })},
		"security zero":           {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.TokensSecurity = int64Value(0) })},
		"no call limit":           {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.CallsTotal = 0 })},
		"agent calls above total": {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.CallsAgent = 25 })},
		"no request timeout":      {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.RequestTimeoutSeconds = 0 })},
		"no concurrency":          {organizationID, runID, adjusted(func(l *contracts.PassportLimits) { l.LocalMaxConcurrency = 0 })},
		"run id not a uuid":       {organizationID, "run-1", valid},
		"organization not a uuid": {"org", runID, valid},
	} {
		// Validation fails before the (nil) transaction is touched.
		if err := budget.OpenRunLedger(context.Background(), nil, testCase.organizationID, testCase.runID, testCase.limits); !errors.Is(err, budget.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// insertRun adds a synthetic passport and run inside the caller's transaction.
func insertRun(t *testing.T, ctx context.Context, transaction pgx.Tx, organizationID, runID string) {
	t.Helper()
	passportID := testdb.ID(t)
	if _, err := transaction.Exec(ctx, `INSERT INTO runtime.passports(id, organization_id, actor_id, task_version,
		admission_catalog_revision_id, scope, limits, expires_at)
		VALUES ($1, $2, $3, 'ledger_test_v1', 1, '{}', '{}', now() + interval '1 hour')`, passportID, organizationID, testdb.ID(t)); err != nil {
		t.Fatalf("insert passport: %v", err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO runtime.runs(id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'queued')`,
		runID, organizationID, passportID); err != nil {
		t.Fatalf("insert run: %v", err)
	}
}

// removeOrganization deletes the organization's runtime rows (passports need disabled triggers).
func removeOrganization(t *testing.T, pool *pgxpool.Pool, organizationID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Error("clean ledger fixture")
		return
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err = transaction.Exec(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
		t.Logf("fixture of organization %s left in place (cleanup needs a superuser)", organizationID)
		return
	}
	for _, table := range []string{"runtime.model_token_budgets", "runtime.runs", "runtime.passports"} {
		if _, err = transaction.Exec(ctx, "DELETE FROM "+table+" WHERE organization_id = $1", organizationID); err != nil {
			t.Errorf("clean %s: %v", table, err)
			return
		}
	}
	_ = transaction.Commit(ctx)
}

func TestOpenRunLedgerCommitsWithTheCallersTransaction(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	organizationID, runID := testdb.ID(t), testdb.ID(t)
	t.Cleanup(func() { removeOrganization(t, pool, organizationID) })
	agentTokens := int64(12000)
	limits := budgettest.Limits(20000)
	limits.TokensAgent = &agentTokens
	store := budget.NewPostgresStore(pool)

	// Rolled back with the caller's transaction: no ledger row remains.
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	insertRun(t, ctx, transaction, organizationID, runID)
	if err = budget.OpenRunLedger(ctx, transaction, organizationID, runID, limits); err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if err = transaction.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Snapshot(ctx, runID); !errors.Is(err, budget.ErrNotFound) {
		t.Fatalf("a rolled-back ledger exists: %v", err)
	}
	// Without a ledger row nothing can be reserved: the model path fails closed.
	if _, err = store.Reserve(ctx, runID, testdb.ID(t), "agent", 100); !errors.Is(err, budget.ErrNotFound) {
		t.Fatalf("reserve without ledger: %v", err)
	}

	// Committed with the caller's transaction: every limit is the passport's.
	transaction, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	insertRun(t, ctx, transaction, organizationID, runID)
	if err = budget.OpenRunLedger(ctx, transaction, organizationID, runID, limits); err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx, runID)
	if err != nil || snapshot.Limit != 20000 || snapshot.Used != 0 || snapshot.Reserved != 0 || snapshot.Paused ||
		snapshot.CallLimit != 24 || snapshot.Agent.CallLimit != 12 || snapshot.Security.CallLimit != 12 ||
		snapshot.Agent.TokenLimit == nil || *snapshot.Agent.TokenLimit != 12000 || snapshot.Security.TokenLimit != nil ||
		snapshot.MaxConcurrentCalls != 2 || snapshot.RequestTimeout != 20*time.Second {
		t.Fatalf("ledger after commit: %+v %v", snapshot, err)
	}
	// A second ledger for the same run is refused, and a ledger needs an existing run.
	transaction, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err = budget.OpenRunLedger(ctx, transaction, organizationID, runID, limits); !errors.Is(err, budget.ErrUnavailable) {
		t.Fatalf("second ledger: %v", err)
	}
	orphan, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = orphan.Rollback(ctx) }()
	if err = budget.OpenRunLedger(ctx, orphan, organizationID, testdb.ID(t), limits); !errors.Is(err, budget.ErrUnavailable) {
		t.Fatalf("ledger without a run: %v", err)
	}
}
