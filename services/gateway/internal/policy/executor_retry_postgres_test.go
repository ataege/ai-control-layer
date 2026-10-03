package policy

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/tools"
)

// flakyRunner fails its first call with a plain storage error (a known no-effect failure once the
// executor rolls back), then runs the real adapters.
type flakyRunner struct{ calls int }

func (runner *flakyRunner) RunEffect(ctx context.Context, tx pgx.Tx, request tools.EffectRequest) (tools.EffectResult, error) {
	runner.calls++
	if runner.calls == 1 {
		return tools.EffectResult{}, errors.New("injected storage error")
	}
	return tools.Runner{}.RunEffect(ctx, tx, request)
}

// closingRunner closes the attempt inside the effect's transaction before the real adapter runs,
// so the adapter finds no open attempt: a precondition failure.
type closingRunner struct{}

func (closingRunner) RunEffect(ctx context.Context, tx pgx.Tx, request tools.EffectRequest) (tools.EffectResult, error) {
	if _, err := tx.Exec(ctx, `UPDATE runtime.execution_attempts SET outcome = 'aborted', completed_at = now() WHERE id = $1`, request.AttemptID); err != nil {
		return tools.EffectResult{}, err
	}
	return tools.Runner{}.RunEffect(ctx, tx, request)
}

func TestSafeRetryUnderTheSameActionQueuesOneMessage(t *testing.T) {
	world := openApprovalWorld(t)
	world.approve(t)
	runner := &flakyRunner{}
	executor := NewExecutor(world.pool, &fakeScopes{scope: world.scope, revision: 1}, runner)
	result := executor.Execute(context.Background(), world.run, world.actionID)
	if result.Status != ExecutionSucceeded || runner.calls != 2 {
		t.Fatalf("result = %s/%s after %d adapter calls; want succeeded after 2", result.Status, result.ReasonCode, runner.calls)
	}
	if world.outboxRows(t) != 1 || world.attemptCount(t) != 2 {
		t.Fatalf("outbox rows %d, attempts %d; want 1 and 2", world.outboxRows(t), world.attemptCount(t))
	}
	var consumedBy string
	if err := world.pool.QueryRow(context.Background(), `SELECT consumed_by_attempt_id::text FROM runtime.approvals WHERE action_id = $1`,
		world.actionID).Scan(&consumedBy); err != nil || consumedBy != result.AttemptID {
		t.Fatalf("grant consumed by %q (err %v), want the retry's attempt %s", consumedBy, err, result.AttemptID)
	}
}

func TestPreconditionFailureFailsTheActionWithoutRetry(t *testing.T) {
	world := openExecutorWorld(t, 12)
	actionID := world.allowRead(t, world.invoiceA01)
	result := world.executor(closingRunner{}).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionStopped || result.ReasonCode != ReasonActionChanged {
		t.Fatalf("result = %s/%s, want stopped/action_changed", result.Status, result.ReasonCode)
	}
	if status := world.actionStatus(t, actionID); status != actionStatusFailed {
		t.Fatalf("action status = %s, want failed", status)
	}
	if attempts := world.attempts(t, actionID); len(attempts) != 1 {
		t.Fatalf("attempts = %v, want exactly one (no retry)", attempts)
	}
	var eventType string
	if err := world.pool.QueryRow(context.Background(), `SELECT event_type FROM runtime.audit_events WHERE action_id = $1 ORDER BY id DESC LIMIT 1`,
		actionID).Scan(&eventType); err != nil || eventType != string(contracts.EventActionFailed) {
		t.Fatalf("last event = %q, err %v; want action.failed", eventType, err)
	}
}

func TestFailedCommitRecordsAnUnknownOutcome(t *testing.T) {
	world := openExecutorWorld(t, 12)
	actionID := world.allowRead(t, world.invoiceA01)
	// The request is cancelled right before the commit, so the commit fails and its outcome
	// cannot be taken for granted.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executor := world.executor(tools.Runner{})
	executor.beforeCommit = cancel
	result := executor.Execute(ctx, world.run, actionID)
	if result.Status != ExecutionPaused || result.ReasonCode != ReasonOutcomeUnknown {
		t.Fatalf("result = %s/%s, want paused/outcome_unknown", result.Status, result.ReasonCode)
	}
	if status := world.actionStatus(t, actionID); status != string(contracts.ActionUnknown) {
		t.Fatalf("action status = %s, want unknown", status)
	}
	if attempts := world.attempts(t, actionID); len(attempts) != 1 || attempts[0] != "open" {
		t.Fatalf("attempts = %v, want one open attempt", attempts)
	}
	var eventType string
	if err := world.pool.QueryRow(context.Background(), `SELECT event_type FROM runtime.audit_events WHERE action_id = $1 ORDER BY id DESC LIMIT 1`,
		actionID).Scan(&eventType); err != nil || eventType != string(contracts.EventActionUnknown) {
		t.Fatalf("last event = %q, err %v; want action.unknown", eventType, err)
	}
	// Nothing re-runs an action whose outcome is unknown.
	again := world.executor(tools.Runner{}).Execute(context.Background(), world.run, actionID)
	if again.Status != ExecutionRefused {
		t.Fatalf("second execution = %s, want refused", again.Status)
	}
	t.Logf("evidence X-61 (SIMULATION, labelled rehearsal): an unknown commit outcome -> action %s with action.unknown and one open attempt; a second dispatch refused (%s)",
		contracts.ActionUnknown, again.ReasonCode)
}
