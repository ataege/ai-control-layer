package budget_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/budget/budgettest"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/testdb"
)

func TestValidationAndUnavailableFailClosed(t *testing.T) {
	store := budget.NewPostgresStore(nil)
	ctx := context.Background()
	if _, err := store.Reserve(ctx, "run", "call", "agent", 100); !errors.Is(err, budget.ErrUnavailable) {
		t.Fatal(err)
	}
	for _, purpose := range []string{"", "guard", "other"} {
		if _, err := store.Reserve(ctx, "run", "call", purpose, 100); !errors.Is(err, budget.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if _, err := store.Settle(ctx, "run", "call", math.MaxInt64, 1); !errors.Is(err, budget.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := store.Settle(ctx, "run", "call", -1, 1); !errors.Is(err, budget.ErrInvalid) {
		t.Fatal(err)
	}
	var missingContext context.Context
	if _, err := store.Snapshot(missingContext, "run"); !errors.Is(err, budget.ErrInvalid) {
		t.Fatal(err)
	}
	if err := store.MarkUnknown(missingContext, "run", "call"); !errors.Is(err, budget.ErrInvalid) {
		t.Fatal(err)
	}
}

// ledger is one seeded run with its open ledger and a helper that writes dispatch records.
type ledger struct {
	t     *testing.T
	pool  *pgxpool.Pool
	store *budget.PostgresStore
	run   budgettest.Run
}

func newLedger(t *testing.T, limits contracts.PassportLimits) ledger {
	t.Helper()
	pool := testdb.Open(t)
	return ledger{t: t, pool: pool, store: budget.NewPostgresStore(pool), run: budgettest.OpenRun(t, pool, limits)}
}

// call writes a dispatch record of the purpose and returns its id.
func (fixture ledger) call(purpose string) string {
	fixture.t.Helper()
	callID := testdb.ID(fixture.t)
	budgettest.RecordDispatch(fixture.t, fixture.pool, fixture.run, callID, purpose)
	return callID
}

func TestPostgresSettlementDurabilityAndUnknown(t *testing.T) {
	fixture := newLedger(t, budgettest.Limits(20000))
	ctx := context.Background()
	normal, late := fixture.call("agent"), fixture.call("security")
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, normal, "agent", 2000); err != nil {
		t.Fatal(err)
	}
	got, err := fixture.store.Settle(ctx, fixture.run.RunID, normal, 700, 50)
	if err != nil || got.ActualTokens != 750 || got.RefundedTokens != 1250 {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err = fixture.store.Reserve(ctx, fixture.run.RunID, late, "security", 2000); err != nil {
		t.Fatal(err)
	}
	if err = fixture.store.MarkUnknown(ctx, fixture.run.RunID, late); err != nil {
		t.Fatal(err)
	}
	// A fresh store reads the persisted counters; unknown usage stays reserved, never zero.
	restarted := budget.NewPostgresStore(fixture.pool)
	snapshot, err := restarted.Snapshot(ctx, fixture.run.RunID)
	if err != nil || snapshot.Reserved != 2000 || snapshot.Used != 750 || snapshot.Security.UnresolvedCalls != 1 ||
		snapshot.Security.ReservedTokens != 2000 || snapshot.Agent.UsedTokens != 750 || snapshot.CallsInFlight != 1 {
		t.Fatalf("%+v %v", snapshot, err)
	}
	got, err = restarted.Settle(ctx, fixture.run.RunID, late, 300, 100)
	if err != nil || got.RefundedTokens != 1600 {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err = fixture.store.Settle(ctx, fixture.run.RunID, late, 300, 100); err != nil || !got.AlreadySettled {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err = fixture.store.Settle(ctx, fixture.run.RunID, late, 301, 100); !errors.Is(err, budget.ErrConflict) {
		t.Fatal(err)
	}
	if _, err = fixture.store.Reserve(ctx, fixture.run.RunID, late, "security", 2000); !errors.Is(err, budget.ErrDuplicate) {
		t.Fatal(err)
	}
	snapshot, err = fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil || snapshot.Reserved != 0 || snapshot.Used != 1150 || snapshot.CallsInFlight != 0 || snapshot.Security.UnresolvedCalls != 0 ||
		snapshot.Agent.Calls != 1 || snapshot.Security.Calls != 1 {
		t.Fatalf("%+v %v", snapshot, err)
	}
}

func TestPostgresConcurrentSharedBudget(t *testing.T) {
	limits := budgettest.Limits(20000)
	limits.CallsTotal, limits.CallsAgent, limits.CallsSecurity, limits.LocalMaxConcurrency = 40, 40, 40, 40
	fixture := newLedger(t, limits)
	ctx := context.Background()
	callIDs := make([]string, 40)
	purposes := make([]string, 40)
	for index := range callIDs {
		purposes[index] = "agent"
		if index%2 == 1 {
			purposes[index] = "security"
		}
		callIDs[index] = fixture.call(purposes[index])
	}
	var waitGroup sync.WaitGroup
	results := make(chan error, 40)
	for index := range callIDs {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			_, err := fixture.store.Reserve(ctx, fixture.run.RunID, callIDs[index], purposes[index], 1000)
			results <- err
		}(index)
	}
	waitGroup.Wait()
	close(results)
	granted := 0
	for err := range results {
		if err == nil {
			granted++
		} else if !errors.Is(err, budget.ErrExhausted) {
			t.Fatal(err)
		}
	}
	snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil || granted != 20 || snapshot.Reserved != 20000 || snapshot.Agent.Calls+snapshot.Security.Calls != 20 {
		t.Fatalf("granted=%d %+v %v", granted, snapshot, err)
	}
}

func TestPostgresCallLimitsSubBudgetsAndConcurrency(t *testing.T) {
	ctx := context.Background()
	t.Run("purpose and shared call limits", func(t *testing.T) {
		limits := budgettest.Limits(20000)
		limits.CallsTotal, limits.CallsAgent, limits.CallsSecurity = 3, 2, 3
		fixture := newLedger(t, limits)
		for range 2 {
			callID := fixture.call("agent")
			if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, callID, "agent", 10); err != nil {
				t.Fatal(err)
			}
			if _, err := fixture.store.Settle(ctx, fixture.run.RunID, callID, 5, 5); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("agent"), "agent", 10); !errors.Is(err, budget.ErrExhausted) {
			t.Fatalf("third agent call: %v", err)
		}
		securityCall := fixture.call("security")
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, securityCall, "security", 10); err != nil {
			t.Fatalf("security call within the shared limit: %v", err)
		}
		if _, err := fixture.store.Settle(ctx, fixture.run.RunID, securityCall, 5, 5); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("security"), "security", 10); !errors.Is(err, budget.ErrExhausted) {
			t.Fatalf("call beyond the shared limit: %v", err)
		}
	})
	t.Run("security token sub-budget inside the shared total", func(t *testing.T) {
		limits := budgettest.Limits(20000)
		securityTokens := int64(1000)
		limits.TokensSecurity = &securityTokens
		fixture := newLedger(t, limits)
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("security"), "security", 1001); !errors.Is(err, budget.ErrExhausted) {
			t.Fatalf("security reservation above its sub-budget: %v", err)
		}
		// The agent's share is not limited by the security sub-budget.
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("agent"), "agent", 5000); err != nil {
			t.Fatalf("agent reservation: %v", err)
		}
	})
	t.Run("concurrency slot held through unknown usage", func(t *testing.T) {
		limits := budgettest.Limits(20000)
		limits.LocalMaxConcurrency = 1
		fixture := newLedger(t, limits)
		first := fixture.call("agent")
		granted, err := fixture.store.Reserve(ctx, fixture.run.RunID, first, "agent", 100)
		if err != nil || granted.RequestTimeout != 20*time.Second {
			t.Fatalf("first reservation: %+v %v", granted, err)
		}
		if err = fixture.store.MarkUnknown(ctx, fixture.run.RunID, first); err != nil {
			t.Fatal(err)
		}
		if _, err = fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("security"), "security", 100); !errors.Is(err, budget.ErrConcurrencyLimit) {
			t.Fatalf("second call while the slot is held: %v", err)
		}
		// The one late reconciliation releases the slot.
		if _, err = fixture.store.Settle(ctx, fixture.run.RunID, first, 50, 10); err != nil {
			t.Fatal(err)
		}
		if _, err = fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("security"), "security", 100); err != nil {
			t.Fatalf("call after the slot was released: %v", err)
		}
	})
	t.Run("a call cannot spend the other purpose", func(t *testing.T) {
		fixture := newLedger(t, budgettest.Limits(20000))
		agentCall := fixture.call("agent")
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, agentCall, "security", 100); !errors.Is(err, budget.ErrUnavailable) {
			t.Fatalf("agent call reserved as security: %v", err)
		}
		if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, testdb.ID(t), "agent", 100); !errors.Is(err, budget.ErrUnavailable) {
			t.Fatalf("call without a dispatch record: %v", err)
		}
		snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
		if err != nil || snapshot.Reserved != 0 || snapshot.Agent.Calls != 0 || snapshot.Security.Calls != 0 {
			t.Fatalf("a refused reservation changed the ledger: %+v %v", snapshot, err)
		}
	})
}

func TestPostgresOverrunPausesWithoutClipping(t *testing.T) {
	fixture := newLedger(t, budgettest.Limits(20000))
	ctx := context.Background()
	callID := fixture.call("agent")
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, callID, "agent", 100); err != nil {
		t.Fatal(err)
	}
	got, err := fixture.store.Settle(ctx, fixture.run.RunID, callID, 150, 75)
	if err != nil || !got.Paused || got.ActualTokens != 225 {
		t.Fatalf("%+v %v", got, err)
	}
	snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil || snapshot.Used != 225 || snapshot.Reserved != 0 || !snapshot.Paused || snapshot.Agent.UsedTokens != 225 {
		t.Fatalf("%+v %v", snapshot, err)
	}
	if _, err = fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("security"), "security", 1); !errors.Is(err, budget.ErrPaused) {
		t.Fatal(err)
	}
}

func TestPostgresConcurrentLateSettlementOnce(t *testing.T) {
	fixture := newLedger(t, budgettest.Limits(20000))
	ctx := context.Background()
	late := fixture.call("agent")
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, late, "agent", 2000); err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.MarkUnknown(ctx, fixture.run.RunID, late); err != nil {
		t.Fatal(err)
	}
	var waitGroup sync.WaitGroup
	results := make(chan budget.Settlement, 12)
	failures := make(chan error, 12)
	for range 12 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, err := fixture.store.Settle(ctx, fixture.run.RunID, late, 700, 50)
			if err != nil {
				failures <- err
				return
			}
			results <- result
		}()
	}
	waitGroup.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	first := 0
	for result := range results {
		if !result.AlreadySettled {
			first++
		}
	}
	snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil || first != 1 || snapshot.Used != 750 || snapshot.Reserved != 0 || snapshot.CallsInFlight != 0 {
		t.Fatalf("first=%d %+v %v", first, snapshot, err)
	}
}

func TestPostgresAggregateOverflowPauses(t *testing.T) {
	fixture := newLedger(t, budgettest.Limits(20000))
	ctx := context.Background()
	prior, huge := fixture.call("agent"), fixture.call("security")
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, prior, "agent", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.Settle(ctx, fixture.run.RunID, prior, 1, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, huge, "security", 100); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.store.Settle(ctx, fixture.run.RunID, huge, math.MaxInt64, 0)
	if !errors.Is(err, budget.ErrInvalid) || !result.Paused || result.ActualTokens != math.MaxInt64 {
		t.Fatalf("%+v %v", result, err)
	}
	snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil || !snapshot.Paused || snapshot.Used != 1 || snapshot.Reserved != 100 || snapshot.Security.UnresolvedCalls != 1 {
		t.Fatalf("%+v %v", snapshot, err)
	}
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("agent"), "agent", 1); !errors.Is(err, budget.ErrPaused) {
		t.Fatal(err)
	}
}

func TestCountAgentCallsReadsTheLedger(t *testing.T) {
	fixture := newLedger(t, budgettest.Limits(20000))
	ctx := context.Background()
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("agent"), "agent", 10); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.Reserve(ctx, fixture.run.RunID, fixture.call("security"), "security", 10); err != nil {
		t.Fatal(err)
	}
	calls, err := fixture.store.CountAgentCalls(ctx, fixture.run.OrganizationID, fixture.run.RunID)
	if err != nil || calls != 1 {
		t.Fatalf("agent calls %d %v", calls, err)
	}
	if _, err = fixture.store.CountAgentCalls(ctx, testdb.ID(t), fixture.run.RunID); !errors.Is(err, budget.ErrNotFound) {
		t.Fatalf("another organization read the ledger: %v", err)
	}
}
