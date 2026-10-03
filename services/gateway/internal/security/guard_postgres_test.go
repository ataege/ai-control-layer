package security

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/budget/budgettest"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/testdb"
)

// Evidence X-98, guard failure and the security ceiling, on the real PostgreSQL run ledger and the
// real Ollama transport. The model is a labelled local HTTP provider double; its answers are
// fixtures, not semantic verdicts of a real model.

// guardLedger seeds a run with an open ledger (budgettest); its store writes the semantic
// evaluator's dispatch record before reserving, as agent.RecordingCaller does in production.
func guardLedger(t *testing.T, limit int64) (*budgettest.DispatchRecordingStore, *pgxpool.Pool, string) {
	t.Helper()
	pool := testdb.Open(t)
	run := budgettest.OpenRun(t, pool, budgettest.Limits(limit))
	return budgettest.NewDispatchRecordingStore(pool), pool, run.RunID
}

// guardInspector serves answer (or sleeps past the timeout) from a labelled HTTP provider double.
func guardInspector(t *testing.T, store budget.Store, answer func() (content string, outputTokens int, delay time.Duration)) (*Inspector, *atomic.Int64) {
	t.Helper()
	hits := new(atomic.Int64)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		hits.Add(1)
		content, outputTokens, delay := answer()
		select {
		case <-time.After(delay):
		case <-request.Context().Done():
			return
		}
		fmt.Fprintf(writer, `{"model":"test-fixture","done":true,"message":{"role":"assistant","content":%s},"prompt_eval_count":300,"eval_count":%d}`,
			strconv.Quote(content), outputTokens)
	}))
	t.Cleanup(server.Close)
	provider, err := model.NewOllama(model.Options{BaseURL: server.URL, Model: "test-fixture", Timeout: 200 * time.Millisecond, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := model.NewAccountedCaller(provider, store, model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewSemanticEvaluator(caller, EvaluatorOptions{Model: "test-fixture", ContextTokens: MinEvaluatorContextTokens, Source: VerdictFixture})
	if err != nil {
		t.Fatal(err)
	}
	return NewInspector(evaluator), hits
}

func guardInput(t *testing.T, runID string) ToolResultInput {
	input := invoiceInput(t, cleanNoteText)
	input.RunID = runID
	return input
}

func reservationStatuses(t *testing.T, pool *pgxpool.Pool, runID string) map[string]int {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT status, count(*) FROM runtime.model_token_reservations WHERE run_id=$1 AND purpose='security' GROUP BY status", runID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	statuses := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			t.Fatal(err)
		}
		statuses[status] = count
	}
	return statuses
}

// A timeout pauses with no result, keeps the whole reservation as unknown usage and is not retried.
func TestPostgresGuardTimeoutRetainsReservation(t *testing.T) {
	store, pool, runID := guardLedger(t, 20000)
	inspector, hits := guardInspector(t, store, func() (string, int, time.Duration) { return passVerdict, 12, time.Second })
	inspection, err := inspector.InspectToolResult(context.Background(), guardInput(t, runID), fullSettings(t, ModeBlock))
	snapshot, snapshotErr := store.Snapshot(context.Background(), runID)
	if err == nil || inspection.Outcome != ResultPaused || inspection.ResultJSON != nil || inspection.ReasonCode != ReasonSecurityEvaluatorUnavailable {
		t.Fatalf("inspection = %+v, err = %v", inspection, err)
	}
	if snapshotErr != nil || snapshot.Reserved == 0 || snapshot.Used != 0 || hits.Load() != 1 || reservationStatuses(t, pool, runID)["usage_unknown"] != 1 {
		t.Fatalf("snapshot = %+v, hits = %d, statuses = %v", snapshot, hits.Load(), reservationStatuses(t, pool, runID))
	}
	t.Logf("evidence X-98: timeout -> paused %s (failure %s); result withheld; security reservation %d tokens retained as usage_unknown; provider requests 1",
		inspection.ReasonCode, inspection.Records[len(inspection.Records)-1].Failure, snapshot.Reserved)
}

// A malformed verdict pauses even though the call completed and was metered.
func TestPostgresGuardMalformedVerdictPauses(t *testing.T) {
	store, pool, runID := guardLedger(t, 20000)
	inspector, hits := guardInspector(t, store, func() (string, int, time.Duration) {
		return `{"risk_category":"none","score":7,"reason_code":"no_risk_found"}`, 12, 0
	})
	inspection, err := inspector.InspectToolResult(context.Background(), guardInput(t, runID), fullSettings(t, ModeBlock))
	snapshot, _ := store.Snapshot(context.Background(), runID)
	if err == nil || inspection.Outcome != ResultPaused || inspection.ResultJSON != nil || snapshot.Used != 312 || snapshot.Reserved != 0 || hits.Load() != 1 {
		t.Fatalf("inspection = %+v, snapshot = %+v, hits = %d, err = %v", inspection, snapshot, hits.Load(), err)
	}
	t.Logf("evidence X-98: malformed verdict (score 7) -> paused %s; result withheld; usage settled %d tokens; statuses %v",
		inspection.ReasonCode, snapshot.Used, reservationStatuses(t, pool, runID))
}

// An exhausted or paused allowance dispatches nothing and pauses.
func TestPostgresGuardAllowanceCeiling(t *testing.T) {
	store, _, runID := guardLedger(t, 100)
	inspector, hits := guardInspector(t, store, func() (string, int, time.Duration) { return passVerdict, 12, 0 })
	inspection, err := inspector.InspectToolResult(context.Background(), guardInput(t, runID), fullSettings(t, ModeBlock))
	snapshot, _ := store.Snapshot(context.Background(), runID)
	if err == nil || inspection.Outcome != ResultPaused || inspection.ReasonCode != ReasonSecurityAllowanceExhausted || hits.Load() != 0 || snapshot.Reserved != 0 || snapshot.Used != 0 {
		t.Fatalf("exhausted: inspection = %+v, snapshot = %+v, hits = %d", inspection, snapshot, hits.Load())
	}
	t.Logf("evidence X-98: allowance 100 tokens -> paused %s; provider requests 0; reserved 0", inspection.ReasonCode)

	// An overrun pauses the ledger; the next security check then dispatches nothing.
	store, _, runID = guardLedger(t, 20000)
	inspector, hits = guardInspector(t, store, func() (string, int, time.Duration) { return passVerdict, 50000, 0 })
	first, err := inspector.InspectToolResult(context.Background(), guardInput(t, runID), fullSettings(t, ModeBlock))
	if err == nil || first.Outcome != ResultPaused {
		t.Fatalf("overrun: inspection = %+v, err = %v", first, err)
	}
	second, err := inspector.InspectToolResult(context.Background(), guardInput(t, runID), fullSettings(t, ModeBlock))
	snapshot, _ = store.Snapshot(context.Background(), runID)
	if err == nil || second.Outcome != ResultPaused || second.ReasonCode != ReasonSecurityAllowanceExhausted || hits.Load() != 1 || !snapshot.Paused {
		t.Fatalf("after overrun: inspection = %+v, snapshot = %+v, hits = %d", second, snapshot, hits.Load())
	}
	t.Logf("evidence X-98: overrun (%d tokens used of 20000) pauses the ledger -> next check paused %s; provider requests stay 1",
		snapshot.Used, second.ReasonCode)
}
