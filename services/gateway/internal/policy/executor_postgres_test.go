package policy

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/testdb"
	"starter/services/gateway/internal/tools"
)

// executorWorld is one seeded Atlas run. The executor commits, so the rows stay in the test
// database; every id is random, so runs never collide.
type executorWorld struct {
	pool       *pgxpool.Pool
	run        RunIdentity
	passportID string
	invoiceA01 string
	invoiceB01 string
	scope      PassportScope
	nextStep   int
}

func openExecutorWorld(t *testing.T, attemptLimit int) *executorWorld {
	return openExecutorWorldWithExpiry(t, attemptLimit, "now() + interval '15 minutes'", "now()")
}

// openExecutorWorldWithExpiry seeds the passport with the given SQL expressions for its expiry and
// issue time (a passport cannot be changed later, so an expired one is seeded expired).
func openExecutorWorldWithExpiry(t *testing.T, attemptLimit int, expiresAtSQL, issuedAtSQL string) *executorWorld {
	t.Helper()
	pool := testdb.Open(t)
	ctx := context.Background()
	suffix := testdb.ID(t)[:8]
	world := &executorWorld{
		pool:       pool,
		run:        RunIdentity{OrganizationID: testdb.ID(t), RunID: testdb.ID(t)},
		passportID: testdb.ID(t),
		invoiceA01: "invoice_A01_" + suffix,
		invoiceB01: "invoice_B01_" + suffix,
	}
	vendorID := "vendor_Atlas_" + suffix
	mustExec(t, pool, `INSERT INTO demo.vendors (id, organization_id, name, registered_reporting_address)
	                   VALUES ($1, $2, 'Atlas', 'reports@atlas.example.com')`, vendorID, world.run.OrganizationID)
	mustExec(t, pool, `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency,
	                     total_minor_units, issued_on, due_on)
	                   VALUES ($1, $3, $4, 'INV104', 'EUR', 125000, '2026-09-01', '2026-10-31'),
	                          ($2, $3, $4, 'INV211', 'EUR', 48000, '2026-09-15', '2026-11-15')`,
		world.invoiceA01, world.invoiceB01, world.run.OrganizationID, vendorID)
	// The stored scope in the tools package's X-08 shape: the adapter re-checks it itself.
	storedScope, _ := json.Marshal(map[string]any{"invoiceIds": []string{world.invoiceA01}, "vendorIds": []string{vendorID}})
	mustExec(t, pool, `INSERT INTO runtime.passports (id, organization_id, actor_id, task_version,
	                     admission_catalog_revision_id, scope, limits, issued_at, expires_at)
	                   VALUES ($1, $2, $3, 'reconcile_atlas_v1', 1, $4, '{}', `+issuedAtSQL+`, `+expiresAtSQL+`)`,
		world.passportID, world.run.OrganizationID, testdb.ID(t), storedScope)
	mustExec(t, pool, `INSERT INTO runtime.runs (id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		world.run.RunID, world.run.OrganizationID, world.passportID)
	world.scope = PassportScope{
		OrganizationID: world.run.OrganizationID, RunID: world.run.RunID, PassportID: world.passportID,
		AllowedTools: []ToolName{ToolReadInvoice}, AllowedInvoiceIDs: []string{world.invoiceA01},
		ApprovalRequiredTools: []ToolName{ToolQueueReport}, ToolAttemptLimit: attemptLimit,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	_ = ctx
	return world
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, arguments ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, arguments...); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// allowRead runs a read_invoice proposal through the real gate and recorder and returns its id.
func (world *executorWorld) allowRead(t *testing.T, invoiceID string) string {
	t.Helper()
	world.nextStep++
	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool), nil)
	actionID := testdb.ID(t)
	decision := gate.Evaluate(context.Background(), world.run, Proposal{
		ActionID: actionID, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "read_invoice", RawArguments: json.RawMessage(`{"invoice_id":"` + invoiceID + `"}`),
	})
	if decision.Outcome != OutcomeAllow {
		t.Fatalf("gate decision = %s/%s, want allow", decision.Outcome, decision.ReasonCode)
	}
	return actionID
}

func (world *executorWorld) executor(runner tools.EffectRunner) *Executor {
	return NewExecutor(world.pool, &fakeScopes{scope: world.scope, revision: 1}, runner)
}

func (world *executorWorld) attempts(t *testing.T, actionID string) []string {
	t.Helper()
	rows, err := world.pool.Query(context.Background(),
		`SELECT coalesce(outcome, 'open') FROM runtime.execution_attempts WHERE action_id = $1 ORDER BY attempt_number`, actionID)
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return outcomes
}

func (world *executorWorld) actionStatus(t *testing.T, actionID string) string {
	t.Helper()
	var status string
	if err := world.pool.QueryRow(context.Background(), `SELECT status FROM runtime.actions WHERE id = $1`, actionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

// countingRunner records whether any adapter was reached.
type countingRunner struct {
	calls int
	inner tools.EffectRunner
	err   error
}

func (runner *countingRunner) RunEffect(ctx context.Context, tx pgx.Tx, request tools.EffectRequest) (tools.EffectResult, error) {
	runner.calls++
	if runner.err != nil {
		return tools.EffectResult{}, runner.err
	}
	return runner.inner.RunEffect(ctx, tx, request)
}

func TestAllowedReadExecutesOnceByItsActionID(t *testing.T) {
	world := openExecutorWorld(t, 12)
	actionID := world.allowRead(t, world.invoiceA01)
	runner := &countingRunner{inner: tools.Runner{}}

	result := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionSucceeded {
		t.Fatalf("status = %s/%s, want succeeded", result.Status, result.ReasonCode)
	}
	if !strings.Contains(string(result.Result.JSON), world.invoiceA01) {
		t.Fatalf("minimized result does not name the invoice: %s", result.Result.JSON)
	}
	if got := world.attempts(t, actionID); len(got) != 1 || got[0] != tools.OutcomeSucceeded {
		t.Fatalf("attempts = %v, want exactly one succeeded attempt", got)
	}
	if status := world.actionStatus(t, actionID); status != actionStatusExecuted {
		t.Fatalf("action status = %s, want executed", status)
	}

	again := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if again.Status != ExecutionRefused || runner.calls != 1 {
		t.Fatalf("second execution = %s with %d adapter calls; want refused and one call", again.Status, runner.calls)
	}
}

func TestChangedArgumentsAreNotExecuted(t *testing.T) {
	world := openExecutorWorld(t, 12)
	actionID := world.allowRead(t, world.invoiceA01)
	mustExec(t, world.pool, `UPDATE runtime.actions SET canonical_arguments = $1 WHERE id = $2`,
		`{"invoice_id":"`+world.invoiceB01+`"}`, actionID)
	runner := &countingRunner{inner: tools.Runner{}}

	result := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionRefused || result.ReasonCode != ReasonActionChanged || runner.calls != 0 {
		t.Fatalf("result = %s/%s with %d adapter calls; want refused/action_changed and none", result.Status, result.ReasonCode, runner.calls)
	}
	if got := world.attempts(t, actionID); len(got) != 0 {
		t.Fatalf("attempts = %v, want none", got)
	}
}

func TestDeniedActionReachesNoAdapter(t *testing.T) {
	world := openExecutorWorld(t, 12)
	world.nextStep++
	gate := NewGate(&fakeScopes{scope: world.scope, revision: 1}, NewPostgresRecorder(world.pool), nil)
	actionID := testdb.ID(t)
	decision := gate.Evaluate(context.Background(), world.run, Proposal{
		ActionID: actionID, StepNumber: world.nextStep, IdempotencyKey: testdb.ID(t),
		Tool: "read_invoice", RawArguments: json.RawMessage(`{"invoice_id":"` + world.invoiceB01 + `"}`),
	})
	if decision.Outcome != OutcomeDeny {
		t.Fatalf("gate decision = %s, want deny", decision.Outcome)
	}
	runner := &countingRunner{inner: tools.Runner{}}
	result := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionRefused || runner.calls != 0 {
		t.Fatalf("denied action: %s with %d adapter calls; want refused and none", result.Status, runner.calls)
	}
	unknown := world.executor(runner).Execute(context.Background(), world.run, testdb.ID(t))
	if unknown.Status != ExecutionRefused || runner.calls != 0 {
		t.Fatalf("unknown action: %s with %d adapter calls; want refused and none", unknown.Status, runner.calls)
	}
}

func TestCancelledRunExecutesNothing(t *testing.T) {
	world := openExecutorWorld(t, 12)
	actionID := world.allowRead(t, world.invoiceA01)
	mustExec(t, world.pool, `UPDATE runtime.runs SET cancel_requested_at = now() WHERE id = $1`, world.run.RunID)
	runner := &countingRunner{inner: tools.Runner{}}

	result := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionRefused || result.ReasonCode != ReasonRunCancelled || runner.calls != 0 {
		t.Fatalf("result = %s/%s with %d adapter calls; want refused/run_cancelled and none", result.Status, result.ReasonCode, runner.calls)
	}
}

func TestAttemptLimitIsEnforced(t *testing.T) {
	world := openExecutorWorld(t, 1)
	first := world.allowRead(t, world.invoiceA01)
	second := world.allowRead(t, world.invoiceA01)
	runner := &countingRunner{inner: tools.Runner{}}
	if result := world.executor(runner).Execute(context.Background(), world.run, first); result.Status != ExecutionSucceeded {
		t.Fatalf("first = %s, want succeeded", result.Status)
	}
	result := world.executor(runner).Execute(context.Background(), world.run, second)
	if result.Status != ExecutionRefused || result.ReasonCode != ReasonAllowanceExhausted || runner.calls != 1 {
		t.Fatalf("second = %s/%s with %d adapter calls; want refused/allowance_exhausted and one call", result.Status, result.ReasonCode, runner.calls)
	}
}

func TestAdapterErrorRollsBackAndPauses(t *testing.T) {
	world := openExecutorWorld(t, 12)
	actionID := world.allowRead(t, world.invoiceA01)
	runner := &countingRunner{err: errors.New("precondition not met")}

	result := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionPaused {
		t.Fatalf("status = %s, want paused", result.Status)
	}
	if got := world.attempts(t, actionID); len(got) != 1 || got[0] != attemptOutcomeAborted {
		t.Fatalf("attempts = %v, want exactly one aborted attempt", got)
	}
	if status := world.actionStatus(t, actionID); status != actionStatusExecuting {
		t.Fatalf("action status = %s, want executing (paused for reconciliation)", status)
	}
}

func TestExpiredPassportExecutesNothing(t *testing.T) {
	// The stored passport is expired; the gate's view (a fake scope) still allowed the action, so
	// only the executor's fresh check stands between the action and the adapter.
	world := openExecutorWorldWithExpiry(t, 12, "now() - interval '1 minute'", "now() - interval '1 hour'")
	actionID := world.allowRead(t, world.invoiceA01)
	runner := &countingRunner{inner: tools.Runner{}}

	result := world.executor(runner).Execute(context.Background(), world.run, actionID)
	if result.Status != ExecutionRefused || result.ReasonCode != ReasonRunCancelled || runner.calls != 0 {
		t.Fatalf("result = %s/%s with %d adapter calls; want refused/run_cancelled and none", result.Status, result.ReasonCode, runner.calls)
	}
}
