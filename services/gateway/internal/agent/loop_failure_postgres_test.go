package agent

import (
	"context"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
)

// GO-58: "On provider failure, show the actual failure state". The real stepper, accounted caller,
// token ledger and call log meet a provider that cannot be reached: the call is recorded with an
// unknown outcome, its reservation stays held, the run pauses and its event says why.
func TestUnreachableProviderRecordsTheActualFailureState(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	ctx := context.Background()
	// newLoopWorld opens the run's ledger with the passport, as admission does.
	ledger := budget.NewPostgresStore(world.pool)
	// A closed loopback port: the connection is refused, nothing answers.
	provider, err := model.NewOllama(model.Options{BaseURL: "http://127.0.0.1:1", Model: "test-fixture", Timeout: 5 * time.Second,
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := model.NewAccountedCaller(provider, ledger, model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	stepper, err := NewStepper(caller, budget.NewCallLog(world.pool), "test-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil {
		t.Fatal(err)
	}

	assertRunEnded(t, world, contracts.RunPaused, contracts.ReasonOutcomeUnknown)
	var callOutcome, reservationStatus string
	if err := world.pool.QueryRow(ctx, `SELECT outcome FROM runtime.model_calls WHERE organization_id = $1 AND purpose = 'agent'`,
		world.organizationID).Scan(&callOutcome); err != nil {
		t.Fatal(err)
	}
	if err := world.pool.QueryRow(ctx, `SELECT status FROM runtime.model_token_reservations WHERE run_id = $1`, world.runID).
		Scan(&reservationStatus); err != nil {
		t.Fatal(err)
	}
	var safeMessage, purpose string
	if err := world.pool.QueryRow(ctx, `SELECT masked_summary->>'safeMessage', masked_summary->>'purpose' FROM runtime.audit_events
		WHERE organization_id = $1 AND event_type = 'run.paused'`, world.organizationID).Scan(&safeMessage, &purpose); err != nil {
		t.Fatal(err)
	}
	if callOutcome != string(budget.CallUsageUnknown) || reservationStatus != "usage_unknown" ||
		safeMessage != messageModelUnreachable || purpose != string(model.AgentPurpose) {
		t.Fatalf("call %s, reservation %s, event message %q purpose %q", callOutcome, reservationStatus, safeMessage, purpose)
	}
	t.Logf("evidence GO-58: provider unreachable -> model_calls.outcome %s, reservation %s, run paused/%s, event safeMessage %q",
		callOutcome, reservationStatus, contracts.ReasonOutcomeUnknown, safeMessage)
}
