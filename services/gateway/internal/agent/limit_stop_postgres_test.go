package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
)

// TestModelLimitStopsTheRunBeforeTheNextDispatch is the GO-42 evidence (X-46, demo beat 9): a run
// admitted with a small agent call allowance exhausts it. The production stepper, accounted
// caller, Ollama client, token ledger and call log meet a labelled provider double (an httptest
// server speaking the Ollama chat API) that always proposes a permitted read. The next request is
// rejected before it leaves Go, the run shows the terminal reason, and every dispatch has its
// reservation and usage with none left unresolved.
func TestModelLimitStopsTheRunBeforeTheNextDispatch(t *testing.T) {
	const agentCallAllowance = 2
	world := newLoopWorld(t, passportOptions{callsAgent: agentCallAllowance})
	ctx := context.Background()

	var providerRequests atomic.Int32
	providerDouble := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		providerRequests.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"test-fixture","done":true,"prompt_eval_count":40,"eval_count":9,` +
			`"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"read_invoice","arguments":{"invoice_id":"` +
			world.invoiceClean + `"}}}]}}`))
	}))
	t.Cleanup(providerDouble.Close)
	provider, err := model.NewOllama(model.Options{BaseURL: providerDouble.URL, Model: "test-fixture", Timeout: 5 * time.Second,
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	ledger := budget.NewPostgresStore(world.pool)
	caller, err := model.NewAccountedCaller(provider, ledger, model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	stepper, err := NewStepper(caller, budget.NewCallLog(world.pool), "test-fixture")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil {
		t.Fatalf("handle: %v", err)
	}
	assertRunEnded(t, world, contracts.RunPaused, contracts.ReasonAllowanceExhausted)

	// Every dispatch is recorded, reserved and settled; nothing left Go after the limit.
	calls := world.count(t, "SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1 AND purpose = 'agent'")
	completedCalls := world.count(t, "SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1 AND purpose = 'agent' AND outcome = 'completed'")
	var reservations, settledWithUsage, unresolved int
	if err := world.pool.QueryRow(ctx, `SELECT count(*),
	           count(*) FILTER (WHERE status = 'settled' AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL),
	           count(*) FILTER (WHERE status <> 'settled')
	      FROM runtime.model_token_reservations WHERE run_id = $1 AND purpose = 'agent'`, world.runID).
		Scan(&reservations, &settledWithUsage, &unresolved); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ledger.Snapshot(ctx, world.runID)
	if err != nil {
		t.Fatal(err)
	}
	var pausedReason, pausedMessage string
	if err := world.pool.QueryRow(ctx, `SELECT reason_code, coalesce(masked_summary->>'safeMessage', '') FROM runtime.audit_events
	     WHERE organization_id = $1 AND event_type = 'run.paused'`, world.organizationID).Scan(&pausedReason, &pausedMessage); err != nil {
		t.Fatal(err)
	}
	if providerRequests.Load() != agentCallAllowance || calls != agentCallAllowance || completedCalls != agentCallAllowance ||
		reservations != agentCallAllowance || settledWithUsage != agentCallAllowance || unresolved != 0 ||
		snapshot.Reserved != 0 || pausedReason != string(contracts.ReasonAllowanceExhausted) {
		t.Fatalf("provider requests %d, model calls %d (completed %d), reservations %d (settled with usage %d, unresolved %d), ledger %+v, paused event %q",
			providerRequests.Load(), calls, completedCalls, reservations, settledWithUsage, unresolved, snapshot, pausedReason)
	}
	t.Logf("evidence X-46: agent call allowance %d; provider requests %d; model_calls %d completed; reservations %d settled with usage, %d unresolved; "+
		"ledger used %d reserved %d tokens; run paused/%s (event run.paused %q); the next request was rejected before dispatch",
		agentCallAllowance, providerRequests.Load(), completedCalls, settledWithUsage, unresolved, snapshot.Used, snapshot.Reserved, pausedReason, pausedMessage)
}
