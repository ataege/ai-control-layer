package policy

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"starter/services/gateway/internal/tools"
)

// atomicRunner counts adapter calls safely across goroutines.
type atomicRunner struct {
	calls atomic.Int32
	inner tools.EffectRunner
}

func (runner *atomicRunner) RunEffect(ctx context.Context, tx pgx.Tx, request tools.EffectRequest) (tools.EffectResult, error) {
	runner.calls.Add(1)
	return runner.inner.RunEffect(ctx, tx, request)
}

// TestPostgresCompetingExecutionsCannotSpendTheSameToolAttempts is GO-50's tool half (X-52):
// more allowed actions than the run's remaining tool attempts execute at once through the real
// executor. Only as many as the limit covers reach the adapter; the others are refused with
// allowance_exhausted, keep no attempt and stay allowed.
func TestPostgresCompetingExecutionsCannotSpendTheSameToolAttempts(t *testing.T) {
	const competitors, attemptLimit = 6, 2
	world := openExecutorWorld(t, attemptLimit)
	actionIDs := make([]string, competitors)
	for index := range actionIDs {
		actionIDs[index] = world.allowRead(t, world.invoiceA01)
	}
	runner := &atomicRunner{inner: tools.Runner{}}
	start := make(chan struct{})
	results := make([]ExecutionResult, competitors)
	var waitGroup sync.WaitGroup
	for index := range actionIDs {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			<-start
			results[index] = world.executor(runner).Execute(context.Background(), world.run, actionIDs[index])
		}(index)
	}
	close(start)
	waitGroup.Wait()

	succeeded, refused := 0, 0
	for index, result := range results {
		switch {
		case result.Status == ExecutionSucceeded:
			succeeded++
			if attempts := world.attempts(t, actionIDs[index]); len(attempts) != 1 || attempts[0] != tools.OutcomeSucceeded {
				t.Fatalf("winner %d attempts %v", index, attempts)
			}
		case result.Status == ExecutionRefused && result.ReasonCode == ReasonAllowanceExhausted:
			refused++
			if attempts := world.attempts(t, actionIDs[index]); len(attempts) != 0 || world.actionStatus(t, actionIDs[index]) != actionStatusAllowed {
				t.Fatalf("refused %d kept attempts %v or changed status to %s", index, attempts, world.actionStatus(t, actionIDs[index]))
			}
		default:
			t.Fatalf("competitor %d: %s/%s", index, result.Status, result.ReasonCode)
		}
	}
	var attemptRows int
	if err := world.pool.QueryRow(context.Background(), `SELECT count(*) FROM runtime.execution_attempts AS attempt
		JOIN runtime.actions AS action ON action.id = attempt.action_id AND action.organization_id = attempt.organization_id
		WHERE action.run_id = $1 AND action.organization_id = $2`, world.run.RunID, world.run.OrganizationID).Scan(&attemptRows); err != nil {
		t.Fatal(err)
	}
	if succeeded != attemptLimit || refused != competitors-attemptLimit || attemptRows != attemptLimit || int(runner.calls.Load()) != attemptLimit {
		t.Fatalf("succeeded %d, refused %d, attempt rows %d, adapter calls %d; limit %d", succeeded, refused, attemptRows, runner.calls.Load(), attemptLimit)
	}
	t.Logf("evidence GO-50 (X-52, tools): %d allowed actions executed at once against %d remaining tool attempts: %d succeeded, "+
		"%d refused (allowance_exhausted); attempt rows %d, adapter calls %d", competitors, attemptLimit, succeeded, refused, attemptRows, runner.calls.Load())
}
