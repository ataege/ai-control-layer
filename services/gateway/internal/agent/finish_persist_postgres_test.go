package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
)

// Live runs of 4 October: after the approved queue_report succeeded the model proposed tools again
// (queue_report for the internal report, queue_report for the vendor report, a tool named status), and
// the single finish message disappeared from the next request. Once the run has queued its report,
// every later request ends with the finish message and every correction says the task is finished.
// The model is a labelled test double; the run, its approval, its queue and its corrections are real.

func TestAStrayProposalAfterTheQueueStillGetsTheFinishMessageOnTheNextRequest(t *testing.T) {
	wait := newReviewWait(t)
	world := wait.world
	if _, err := policy.NewApprovals(world.pool).Decide(context.Background(), wait.reviewer, wait.actionID, policy.ApprovalApprove); err != nil {
		t.Fatalf("approve: %v", err)
	}
	strayMissingTool := func([]model.Message) (StepResult, error) {
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: "status", Arguments: json.RawMessage(`{}`)}}, nil
	}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){strayMissingTool, answer("The report is queued, all done."), wait.finishNamingTheReport}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), wait.continuationJob(t)); err != nil {
		t.Fatal(err)
	}
	assertRunEnded(t, world, contracts.RunCompleted, "")
	if len(stepper.contexts) != 3 {
		t.Fatalf("%d model requests, want 3 (resume, after the stray call, after the prose answer)", len(stepper.contexts))
	}
	endsWithFinish := func(index int) []model.Message {
		t.Helper()
		messages := stepper.contexts[index]
		last := messages[len(messages)-1]
		if last.Role != "user" || last.Content != reportQueuedMessage {
			t.Fatalf("request %d ends with %+v, want the finish message", index+1, last)
		}
		return messages
	}
	// The request that follows the approval, then the one after a denied stray call, then the one after
	// a rejected final answer: each ends with the finish message.
	endsWithFinish(0)
	afterStray := endsWithFinish(1)
	feedback := afterStray[len(afterStray)-2]
	if feedback.Role != "tool" || !strings.Contains(feedback.Content, `"reason_code":"tool_not_registered"`) ||
		!strings.Contains(feedback.Content, "already queued and the task is finished") {
		t.Fatalf("correction after the stray call: %+v", feedback)
	}
	afterProse := endsWithFinish(2)
	rejection := afterProse[len(afterProse)-2]
	if rejection.Role != "user" || !strings.Contains(rejection.Content, "final answer was not accepted") ||
		!strings.Contains(rejection.Content, "already queued and the task is finished") {
		t.Fatalf("correction after the prose answer: %+v", rejection)
	}
	// Nothing else changed: one queued message, one queue_report action, the corrections stayed counted.
	if outbox := world.count(t, "SELECT count(*) FROM demo.outbox_messages WHERE organization_id = $1"); outbox != 1 {
		t.Fatalf("%d outbox messages, want exactly one", outbox)
	}
	if queues := world.count(t, "SELECT count(*) FROM runtime.actions WHERE organization_id = $1 AND tool = 'queue_report'"); queues != 1 {
		t.Fatalf("%d queue_report actions, want 1", queues)
	}
	if denials := world.count(t, "SELECT count(*) FROM runtime.audit_events WHERE organization_id = $1 AND event_type = 'action.denied'"); denials != 2 {
		t.Fatalf("%d denial events, want 2 (the stray call and the prose answer)", denials)
	}
}

// A denial before the queue is not told the task is finished.
func TestACorrectionBeforeTheQueueDoesNotSayTheTaskIsFinished(t *testing.T) {
	world := newLoopWorld(t, passportOptions{})
	world.results = PoolFinalResults{Pool: world.pool}
	stepper := &scriptedStepper{callLog: budget.NewCallLog(world.pool), ledger: budget.NewPostgresStore(world.pool),
		script: []func([]model.Message) (StepResult, error){
			func([]model.Message) (StepResult, error) {
				return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{Tool: "status", Arguments: json.RawMessage(`{}`)}}, nil
			},
			answer("done"), answer("done"),
		}}
	if _, err := newTestLoop(t, world, stepper).Handle(context.Background(), world.job()); err != nil {
		t.Fatal(err)
	}
	for index, messages := range stepper.contexts {
		for _, message := range messages {
			if message.Content == reportQueuedMessage || strings.Contains(message.Content, "already queued") {
				t.Fatalf("request %d says the task is finished before any report was queued: %+v", index+1, message)
			}
		}
	}
	if len(stepper.contexts) < 2 {
		t.Fatalf("%d requests, want the correction to reach the model", len(stepper.contexts))
	}
}
