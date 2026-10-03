package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/worker"
)

// GO-49: a new claim reconciles what a former claim left at each step boundary, never repeating a
// dispatch or an effect.

// crashedFirstClaim runs one step that executes a read and then stops the claim before the next
// model request, leaving a running run with one executed action and its context.
func crashedFirstClaim(t *testing.T, world *loopWorld) {
	t.Helper()
	ctx, crash := context.WithCancel(context.Background())
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){
			readInvoice(world.invoiceClean),
			func([]model.Message) (StepResult, error) { crash(); return StepResult{}, context.Canceled },
		}}
	if _, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); !errors.Is(err, context.Canceled) {
		t.Fatalf("first claim: %v", err)
	}
}

func TestRecoveryPausesARunWithAnUnresolvedModelCall(t *testing.T) {
	for name, reserve := range map[string]bool{"after the reservation": true, "after the dispatch record only": false} {
		t.Run(name, func(t *testing.T) {
			world := newLoopWorld(t, passportOptions{})
			world.exec(t, "UPDATE runtime.runs SET status = 'running' WHERE id = $1", world.runID)
			ctx := context.Background()
			callID, err := budget.NewCallLog(world.pool).RecordDispatch(ctx, world.organizationID, world.runID, "agent", "test-fixture")
			if err != nil {
				t.Fatal(err)
			}
			ledger := budget.NewPostgresStore(world.pool)
			if reserve {
				if _, err = ledger.Reserve(ctx, world.runID, callID, "agent", 500); err != nil {
					t.Fatal(err)
				}
			}
			stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: ledger}
			if outcome, err := newTestLoop(t, world, stepper).Handle(ctx, world.job()); err != nil || outcome != worker.Completed() {
				t.Fatalf("handle: %v %v", outcome, err)
			}
			assertRunEnded(t, world, contracts.RunPaused, contracts.ReasonOutcomeUnknown)
			if len(stepper.contexts) != 0 {
				t.Fatal("a model request was resent after an unresolved call")
			}
			var outcome string
			if err = world.pool.QueryRow(ctx, "SELECT outcome FROM runtime.model_calls WHERE id = $1", callID).Scan(&outcome); err != nil {
				t.Fatal(err)
			}
			snapshot, err := ledger.Snapshot(ctx, world.runID)
			if err != nil {
				t.Fatal(err)
			}
			if reserve && (outcome != string(budget.CallUsageUnknown) || snapshot.Reserved != 500 || snapshot.Agent.UnresolvedCalls != 1 || snapshot.CallsInFlight != 1) {
				t.Fatalf("reserved call: outcome %s, %+v", outcome, snapshot)
			}
			if !reserve && (outcome != string(budget.CallFailed) || snapshot.Reserved != 0) {
				t.Fatalf("unreserved call: outcome %s, %+v", outcome, snapshot)
			}
		})
	}
}

func TestRecoveryContinuesAfterAnEffectCommittedWithoutItsContext(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	crashedFirstClaim(t, world)
	// The former claim stopped after the effect committed and before the context append.
	world.exec(t, "DELETE FROM runtime.context_entries WHERE run_id = $1", world.runID)

	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){finalAnswer}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	resumed := stepper.contexts[0]
	if len(resumed) != 3 || resumed[1].Role != "assistant" || resumed[1].ToolCalls[0].Function.Name != "read_invoice" ||
		resumed[2].Role != "tool" || !strings.Contains(resumed[2].Content, `"withheld":true`) {
		t.Fatalf("the executed step was not restored as a withheld result: %+v", resumed)
	}
	if attempts := world.count(t, "SELECT count(*) FROM runtime.execution_attempts WHERE organization_id = $1"); attempts != 1 {
		t.Fatalf("the committed read was executed %d times", attempts)
	}
}

func TestRecoveryPausesOnAnActionStillExecuting(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	crashedFirstClaim(t, world)
	// The former claim stopped after dispatching a tool attempt whose outcome never came back.
	world.exec(t, "UPDATE runtime.actions SET status = 'executing' WHERE run_id = $1", world.runID)
	world.exec(t, `INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number)
		SELECT organization_id, id, 2 FROM runtime.actions WHERE run_id = $1`, world.runID)

	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunPaused, contracts.ReasonOutcomeUnknown)
	if len(stepper.contexts) != 0 {
		t.Fatal("a model request was made while a tool outcome was unknown")
	}
	if succeeded := world.count(t, "SELECT count(*) FROM runtime.execution_attempts WHERE organization_id = $1 AND outcome = 'succeeded'"); succeeded != 1 {
		t.Fatalf("%d succeeded attempts, want exactly the original one", succeeded)
	}
}

func TestRestartKeepsWaitingAndEndedRunsAsTheyWere(t *testing.T) {
	for _, testCase := range []struct {
		status contracts.RunStatus
		reason contracts.ReasonCode
	}{
		{contracts.RunAwaitingApproval, ""},
		{contracts.RunPaused, contracts.ReasonOutcomeUnknown},
		{contracts.RunPaused, contracts.ReasonAllowanceExhausted},
		{contracts.RunStopped, contracts.ReasonRunCancelled},
	} {
		world := newLoopWorld(t, passportOptions{})
		var reason any
		if testCase.reason != "" {
			reason = string(testCase.reason)
		}
		world.exec(t, "UPDATE runtime.runs SET status = $2, terminal_reason = $3 WHERE id = $1", world.runID, string(testCase.status), reason)
		stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool)}
		if outcome, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil || outcome != worker.Completed() {
			t.Fatalf("%s: %v %v", testCase.status, outcome, err)
		}
		assertRunEnded(t, world, testCase.status, testCase.reason)
		if len(stepper.contexts) != 0 {
			t.Fatalf("%s: a model request was made", testCase.status)
		}
	}
}
