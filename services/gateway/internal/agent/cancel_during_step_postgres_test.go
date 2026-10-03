package agent

import (
	"context"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
)

// Worker 3's review: a cancel that lands while a step runs stops the run, whatever the step would
// otherwise have ended in (completed, paused, a new action).

// cancelThen returns a script step that requests the run's cancellation during the model request
// and then answers with next.
func cancelThen(t *testing.T, world *loopWorld, next func([]model.Message) (StepResult, error)) func([]model.Message) (StepResult, error) {
	return func(messages []model.Message) (StepResult, error) {
		if _, err := world.repository.CancelRun(context.Background(), world.organizationID, world.runID); err != nil {
			t.Errorf("cancel: %v", err)
		}
		return next(messages)
	}
}

func TestCancelDuringTheModelRequestStopsTheRun(t *testing.T) {
	for name, answer := range map[string]func([]model.Message) (StepResult, error){
		"a final answer does not complete it": finalAnswer,
		"a proposed action does not run":      nil, // set per world below
		"a model timeout does not pause it": func([]model.Message) (StepResult, error) {
			return StepResult{}, model.ErrTimeout
		},
	} {
		t.Run(name, func(t *testing.T) {
			world := newLoopWorld(t, passportOptions{})
			if answer == nil {
				answer = readInvoice(world.invoiceClean)
			}
			stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
				script: []func([]model.Message) (StepResult, error){cancelThen(t, world, answer)}}
			if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
				t.Fatal(err)
			}
			assertRunEnded(t, world, contracts.RunStopped, contracts.ReasonRunCancelled)
			if actions := world.count(t, "SELECT count(*) FROM runtime.actions WHERE organization_id = $1"); actions != 0 {
				t.Fatalf("%d actions stored after the cancel", actions)
			}
			if len(stepper.contexts) != 1 {
				t.Fatalf("%d model requests, want the one in flight only", len(stepper.contexts))
			}
		})
	}
}
