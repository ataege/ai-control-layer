package policy

import (
	"context"
	"encoding/json"
	"testing"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/tools"
)

// switchableSnapshots is a catalog source the test can move to another revision, as a judge's
// policy.yaml import and activation does between two decisions of a running task.
type switchableSnapshots struct{ current fakeSnapshots }

func (snapshots *switchableSnapshots) Active(ctx context.Context, querier catalog.Querier) (catalog.Snapshot, error) {
	return snapshots.current.Active(ctx, querier)
}

// recordedAttempts counts the execution attempts of the review world's run.
func recordedAttempts(t *testing.T, world *reviewWorld) int {
	t.Helper()
	var attempts int
	err := world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.execution_attempts attempt
		JOIN runtime.actions action ON action.id = attempt.action_id WHERE action.run_id = $1`, world.run.RunID).Scan(&attempts)
	if err != nil {
		t.Fatal(err)
	}
	return attempts
}

// config/README.md, "budgets.tool_attempts": a ceiling lowered through the catalog applies to a task
// that is already running, counting the attempts it has used. End to end, with the real scope reader,
// gate and executor: the stored passport allows 12 attempts, so nothing but the lowered catalog value
// can refuse the second execution.
func TestLoweredToolAttemptsRefuseTheNextExecutionWithAllowanceExhausted(t *testing.T) {
	world := openReviewWorld(t)
	ctx := context.Background()
	catalogs := &switchableSnapshots{current: limitingTools(2, 12)} // the passport's own ceiling
	reader := scopeReaderOn(world, catalogs)
	gate := NewGate(reader, NewPostgresRecorder(world.pool), nil, nil)
	runner := &countingRunner{inner: tools.Runner{}}
	executor := NewExecutor(world.pool, reader, runner)

	// Each action is proposed under the revision that is active when it executes, so the catalog's
	// revision never differs between evaluation and execution and only the ceiling can refuse it.
	proposeRead := func() string {
		t.Helper()
		world.nextStep++
		actionID := testdb.ID(t)
		decision := gate.Evaluate(ctx, world.run, Proposal{ActionID: actionID, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
			Tool: "read_invoice", RawArguments: json.RawMessage(`{"invoice_id":"` + world.invoiceID + `"}`)})
		if decision.Outcome != OutcomeAllow {
			t.Fatalf("gate decision = %s/%s, want allow: the attempt ceiling is the executor's to enforce", decision.Outcome, decision.ReasonCode)
		}
		return actionID
	}

	first := proposeRead()
	if result := executor.Execute(ctx, world.run, first); result.Status != ExecutionSucceeded {
		t.Fatalf("first execution = %s/%s, want succeeded", result.Status, result.ReasonCode)
	}
	if attempts := recordedAttempts(t, world); attempts != 1 {
		t.Fatalf("attempts after the first execution = %d, want 1", attempts)
	}

	// The judge lowers budgets.tool_attempts to 1. One attempt is already used.
	catalogs.current = limitingTools(3, 1)
	second := proposeRead()
	result := executor.Execute(ctx, world.run, second)
	if result.Status != ExecutionRefused || result.ReasonCode != ReasonAllowanceExhausted {
		t.Fatalf("second execution = %s/%s, want refused/allowance_exhausted at the lowered ceiling", result.Status, result.ReasonCode)
	}
	if runner.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1: the refused action must not reach the adapter", runner.calls)
	}
	if attempts := recordedAttempts(t, world); attempts != 1 {
		t.Fatalf("attempts after the refusal = %d, want 1: a refused action uses no attempt", attempts)
	}

	// The ceiling follows the catalog: raised to 2 (still below the passport's 12) the same
	// proposal may run, because the refusal consumed nothing.
	catalogs.current = limitingTools(4, 2)
	third := proposeRead()
	if result := executor.Execute(ctx, world.run, third); result.Status != ExecutionSucceeded {
		t.Fatalf("execution under the raised ceiling = %s/%s, want succeeded", result.Status, result.ReasonCode)
	}
	if attempts := recordedAttempts(t, world); attempts != 2 {
		t.Fatalf("attempts after the third execution = %d, want 2", attempts)
	}
}
