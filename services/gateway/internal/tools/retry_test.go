package tools

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"starter/services/gateway/internal/provenance"
)

func TestClassifyRunErrorAndRetrySafety(t *testing.T) {
	storageFailure := fmt.Errorf("tools: insert simulated outbox row: %w", errors.New("connection reset"))
	precondition := fmt.Errorf("%w: no open attempt for this action", errPrecondition)
	cases := []struct {
		err       error
		class     FailureClass
		retrySafe bool
	}{
		{nil, FailureNone, false},
		{storageFailure, FailureKnownNoEffect, true},
		{precondition, FailurePrecondition, false},
	}
	for _, testCase := range cases {
		class := ClassifyRunError(testCase.err)
		if class != testCase.class || RetrySafe(ToolQueueReport, class) != testCase.retrySafe {
			t.Errorf("%v: class %s retry %v", testCase.err, class, RetrySafe(ToolQueueReport, class))
		}
	}
	if RetrySafe("run_shell", FailureKnownNoEffect) {
		t.Error("an unregistered tool is retry-safe")
	}
}

// A known no-effect failure of queue_report is retried under the same action with a new attempt:
// the retry succeeds and there is still exactly one outbox row.
func TestKnownSafeRetryOfQueueReportYieldsOneOutboxRow(t *testing.T) {
	world := openWorld(t, func(world *testWorld) passportScope { return scenarioScope(world, true) })
	vendorReport := createReportFor(t, world, provenance.VendorReconciliationV1.Name)
	request := world.proposeAction(t, world.nextStep(), ToolQueueReport,
		map[string]string{"report_id": vendorReport, "recipient_reference": recipientReference(world.runID, world.atlasID)})

	// First attempt: the event insert fails, the executor rolls back and closes the attempt.
	executorTx, err := world.tx.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	injectFailure(t, executorTx, "runtime.audit_events", "INSERT")
	_, runErr := Runner{}.RunEffect(context.Background(), executorTx, request)
	if err := executorTx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	class := ClassifyRunError(runErr)
	if class != FailureKnownNoEffect || !RetrySafe(request.Tool, class) {
		t.Fatalf("first attempt: err %v, class %s", runErr, class)
	}
	world.exec(t, `UPDATE runtime.execution_attempts SET outcome = 'aborted', completed_at = now() WHERE id = $1`, request.AttemptID)

	// Retry: same action, a new attempt, fresh checks inside RunEffect.
	retry := request
	retry.AttemptID = world.newAttempt(t, request.ActionID, 2)
	result, err := Runner{}.RunEffect(context.Background(), world.tx, retry)
	if err != nil || result.Outcome != OutcomeSucceeded {
		t.Fatalf("retry: %+v %v", result, err)
	}
	if rows := world.count(t, `SELECT count(*) FROM demo.outbox_messages WHERE action_id = $1`, request.ActionID); rows != 1 {
		t.Fatalf("outbox rows = %d, want 1", rows)
	}
	if attempts := world.count(t, `SELECT count(*) FROM runtime.execution_attempts WHERE action_id = $1`, request.ActionID); attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (each retry is a counted attempt)", attempts)
	}
}
