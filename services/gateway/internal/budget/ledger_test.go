package budget

import (
	"context"
	"errors"
	"testing"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

func int64Value(value int64) *int64 { return &value }

func TestOpenRunLedgerRejectsUnusableLimitsBeforeWriting(t *testing.T) {
	organizationID, runID := "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01", "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b"
	valid := contracts.PassportLimits{TokensTotal: 20000}
	for name, testCase := range map[string]struct {
		organizationID, runID string
		limits                contracts.PassportLimits
	}{
		"no total":                {organizationID, runID, contracts.PassportLimits{}},
		"negative total":          {organizationID, runID, contracts.PassportLimits{TokensTotal: -1}},
		"total beyond safe range": {organizationID, runID, contracts.PassportLimits{TokensTotal: maximumSafeInteger + 1}},
		"agent above total":       {organizationID, runID, contracts.PassportLimits{TokensTotal: 100, TokensAgent: int64Value(101)}},
		"security zero":           {organizationID, runID, contracts.PassportLimits{TokensTotal: 100, TokensSecurity: int64Value(0)}},
		"run id not a uuid":       {organizationID, "run-1", valid},
		"organization not a uuid": {"org", runID, valid},
	} {
		// A nil transaction is never reached: validation fails first, except for the uuid cases,
		// which fail on their own check before the transaction is touched.
		if err := OpenRunLedger(context.Background(), nil, testCase.organizationID, testCase.runID, testCase.limits); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestOpenRunLedgerCommitsWithTheCallersTransaction(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()
	organizationID, runID := testdb.ID(t), testdb.ID(t)
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupContext, "DELETE FROM runtime.model_token_budgets WHERE run_id = $1", runID); err != nil {
			t.Error("clean ledger fixture")
		}
	})
	limits := contracts.PassportLimits{TokensTotal: 20000, TokensAgent: int64Value(12000)}

	// Rolled back with the caller's transaction: no ledger row remains.
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = OpenRunLedger(ctx, transaction, organizationID, runID, limits); err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if err = transaction.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = NewPostgresStore(pool).Snapshot(ctx, runID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a rolled-back ledger exists: %v", err)
	}
	// Without a ledger row, nothing can be reserved: the model path fails closed.
	if _, err = NewPostgresStore(pool).Reserve(ctx, runID, "call-1", "agent", 100); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reserve without ledger: %v", err)
	}

	// Committed with the caller's transaction: the limit is the passport's total.
	transaction, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = OpenRunLedger(ctx, transaction, organizationID, runID, limits); err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if err = transaction.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewPostgresStore(pool).Snapshot(ctx, runID)
	if err != nil || snapshot.Limit != 20000 || snapshot.Used != 0 || snapshot.Reserved != 0 || snapshot.Paused {
		t.Fatalf("ledger after commit: %+v %v", snapshot, err)
	}
	// A second ledger for the same run is refused.
	transaction, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err = OpenRunLedger(ctx, transaction, organizationID, runID, limits); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("second ledger: %v", err)
	}
}
