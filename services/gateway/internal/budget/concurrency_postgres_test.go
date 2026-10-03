package budget_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/budget/budgettest"
)

// TestPostgresCompetingReservationsCannotSpendTheSameAllowance is GO-50's model half (X-52):
// competing reservations of both purposes race for an allowance that covers only some of them.
// Exactly the covered ones are granted, the rest are refused before any dispatch, and after the
// winners settle (one with unknown usage) the ledger reconciles with its reservation rows.
func TestPostgresCompetingReservationsCannotSpendTheSameAllowance(t *testing.T) {
	const (
		competitors        = 12
		reservationTokens  = 1000
		coveredTokens      = 3 * reservationTokens // the remaining allowance covers three
		settledInputTokens = 400
		settledOutput      = 200
	)
	limits := budgettest.Limits(coveredTokens)
	limits.CallsTotal, limits.CallsAgent, limits.CallsSecurity, limits.LocalMaxConcurrency = competitors, competitors, competitors, competitors
	fixture := newLedger(t, limits)
	ctx := context.Background()

	callIDs := make([]string, competitors)
	purposes := make([]string, competitors)
	for index := range callIDs {
		purposes[index] = "agent"
		if index%2 == 1 {
			purposes[index] = "security"
		}
		callIDs[index] = fixture.call(purposes[index])
	}
	// Every competitor waits at the start line, so the reservations really overlap.
	start := make(chan struct{})
	errs := make([]error, competitors)
	var waitGroup sync.WaitGroup
	for index := range callIDs {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			<-start
			_, errs[index] = fixture.store.Reserve(ctx, fixture.run.RunID, callIDs[index], purposes[index], reservationTokens)
		}(index)
	}
	close(start)
	waitGroup.Wait()

	var granted []int
	for index, err := range errs {
		switch {
		case err == nil:
			granted = append(granted, index)
		case !errors.Is(err, budget.ErrExhausted):
			t.Fatalf("competitor %d: %v, want granted or exhausted", index, err)
		}
	}
	if len(granted) != coveredTokens/reservationTokens {
		t.Fatalf("%d reservations granted, the allowance covers %d", len(granted), coveredTokens/reservationTokens)
	}

	// The winners finish: all settle except the last, whose usage stays unknown and held.
	for position, index := range granted {
		if position == len(granted)-1 {
			if err := fixture.store.MarkUnknown(ctx, fixture.run.RunID, callIDs[index]); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if _, err := fixture.store.Settle(ctx, fixture.run.RunID, callIDs[index], settledInputTokens, settledOutput); err != nil {
			t.Fatal(err)
		}
	}

	// Reconcile the ledger with its reservation rows.
	var reservationRows, settledRows, unknownRows int
	var settledTokens, heldTokens int64
	err := fixture.pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE status = 'settled'),
			count(*) FILTER (WHERE status = 'usage_unknown'),
			coalesce(sum(actual_tokens) FILTER (WHERE status = 'settled'), 0),
			coalesce(sum(token_reservation) FILTER (WHERE status IN ('reserved', 'usage_unknown')), 0)
		FROM runtime.model_token_reservations WHERE run_id = $1`, fixture.run.RunID).
		Scan(&reservationRows, &settledRows, &unknownRows, &settledTokens, &heldTokens)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	wantSettled := int64(len(granted)-1) * (settledInputTokens + settledOutput)
	if reservationRows != len(granted) || settledRows != len(granted)-1 || unknownRows != 1 ||
		settledTokens != wantSettled || heldTokens != reservationTokens ||
		snapshot.Used != settledTokens || snapshot.Reserved != heldTokens ||
		snapshot.Agent.Calls+snapshot.Security.Calls != int64(len(granted)) {
		t.Fatalf("rows %d (settled %d, unknown %d), settled tokens %d, held %d; ledger %+v",
			reservationRows, settledRows, unknownRows, settledTokens, heldTokens, snapshot)
	}
	// Refused competitors spent nothing: no reservation row and no counted call.
	for index, err := range errs {
		if err == nil {
			continue
		}
		var rows int
		if err := fixture.pool.QueryRow(ctx, `SELECT count(*) FROM runtime.model_token_reservations WHERE run_id = $1 AND call_id = $2`,
			fixture.run.RunID, callIDs[index]).Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("refused competitor %d left %d reservation rows (%v)", index, rows, err)
		}
	}
	t.Logf("evidence GO-50 (X-52, model): %d competing reservations of %d tokens for an allowance of %d: %d granted, %d refused "+
		"(exhausted); reservation rows %d (settled %d, usage unknown %d); ledger used %d = settled rows, reserved %d = held rows, calls %d",
		competitors, reservationTokens, coveredTokens, len(granted), competitors-len(granted), reservationRows, settledRows, unknownRows,
		snapshot.Used, snapshot.Reserved, snapshot.Agent.Calls+snapshot.Security.Calls)
}

// TestPostgresCompetingCallsCannotExceedTheCallLimit races more calls than the run's call limit
// allows, with tokens to spare: the call count, not the tokens, refuses the surplus.
func TestPostgresCompetingCallsCannotExceedTheCallLimit(t *testing.T) {
	const competitors, callLimit = 10, 4
	limits := budgettest.Limits(20000)
	limits.CallsTotal, limits.CallsAgent, limits.CallsSecurity, limits.LocalMaxConcurrency = callLimit, callLimit, callLimit, competitors
	fixture := newLedger(t, limits)
	ctx := context.Background()
	callIDs := make([]string, competitors)
	for index := range callIDs {
		callIDs[index] = fixture.call("agent")
	}
	start := make(chan struct{})
	errs := make([]error, competitors)
	var waitGroup sync.WaitGroup
	for index := range callIDs {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			<-start
			_, errs[index] = fixture.store.Reserve(ctx, fixture.run.RunID, callIDs[index], "agent", 10)
		}(index)
	}
	close(start)
	waitGroup.Wait()
	granted := 0
	for index, err := range errs {
		switch {
		case err == nil:
			granted++
		case !errors.Is(err, budget.ErrExhausted):
			t.Fatalf("competitor %d: %v", index, err)
		}
	}
	snapshot, err := fixture.store.Snapshot(ctx, fixture.run.RunID)
	if err != nil || granted != callLimit || snapshot.Agent.Calls != callLimit || snapshot.Reserved != callLimit*10 {
		t.Fatalf("granted %d of %d, ledger %+v (%v)", granted, competitors, snapshot, err)
	}
	t.Logf("evidence GO-50 (X-52, calls): %d competing calls for a limit of %d: %d granted, %d refused", competitors, callLimit, granted, competitors-granted)
}
