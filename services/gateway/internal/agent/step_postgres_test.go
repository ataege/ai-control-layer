package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/testdb"
)

// newRunFixture inserts a synthetic passport and run plus the run's token ledger, and removes
// them afterwards. Passports reject DELETE by trigger, so the passport is removed with triggers
// disabled for that transaction (superuser test database); otherwise it is left and logged.
func newRunFixture(t *testing.T, pool *pgxpool.Pool, tokenLimit int64) Run {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	run := Run{OrganizationID: testdb.ID(t), RunID: testdb.ID(t), AllowedModels: []string{"test-fixture"}}
	passportID := testdb.ID(t)
	if _, err := pool.Exec(ctx, `INSERT INTO runtime.passports(id, organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
		VALUES ($1, $2, $3, 'agent_test_v1', 1, '{}', '{}', now() + interval '1 hour')`, passportID, run.OrganizationID, testdb.ID(t)); err != nil {
		t.Fatalf("insert passport fixture: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO runtime.runs(id, organization_id, passport_id, status) VALUES ($1, $2, $3, 'running')`,
		run.RunID, run.OrganizationID, passportID); err != nil {
		t.Fatalf("insert run fixture: %v", err)
	}
	if err := budget.NewPostgresStore(pool).CreateRun(ctx, run.RunID, tokenLimit); err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext, cancelCleanup := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCleanup()
		for _, statement := range []string{
			"DELETE FROM runtime.model_token_reservations WHERE run_id = $1",
			"DELETE FROM runtime.model_token_budgets WHERE run_id = $1",
			"DELETE FROM runtime.model_calls WHERE run_id = $1",
			"DELETE FROM runtime.runs WHERE id = $1",
		} {
			if _, err := pool.Exec(cleanupContext, statement, run.RunID); err != nil {
				t.Errorf("clean run fixture: %v", err)
				return
			}
		}
		transaction, err := pool.Begin(cleanupContext)
		if err == nil {
			defer func() { _ = transaction.Rollback(cleanupContext) }()
			if _, err = transaction.Exec(cleanupContext, "SET LOCAL session_replication_role = replica"); err == nil {
				if _, err = transaction.Exec(cleanupContext, "DELETE FROM runtime.passports WHERE id = $1", passportID); err == nil {
					err = transaction.Commit(cleanupContext)
				}
			}
		}
		if err != nil {
			t.Logf("passport fixture %s left in place (removing it needs a superuser)", passportID)
		}
	})
	return run
}

// TestStepRecordsTheDispatchAndSettlesUsage runs one step through the real Ollama client, the
// PostgreSQL ledger and the call log, against a labelled HTTP provider test double.
func TestStepRecordsTheDispatchAndSettlesUsage(t *testing.T) {
	pool := testdb.Open(t)
	run := newRunFixture(t, pool, 20000)

	var requestsSeen atomic.Int32
	var requestBody map[string]any
	providerDouble := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestsSeen.Add(1)
		_ = json.NewDecoder(request.Body).Decode(&requestBody)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"test-fixture","done":true,"prompt_eval_count":40,"eval_count":9,` +
			`"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"read_invoice","arguments":{"invoice_id":"invoice_A01"}}}]}}`))
	}))
	t.Cleanup(providerDouble.Close)
	provider, err := model.NewOllama(model.Options{BaseURL: providerDouble.URL, Model: "test-fixture", Timeout: 5 * time.Second,
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	caller, err := model.NewAccountedCaller(provider, budget.NewPostgresStore(pool), model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	stepper, err := NewStepper(caller, budget.NewCallLog(pool), "test-fixture")
	if err != nil {
		t.Fatal(err)
	}

	result, err := stepper.Step(context.Background(), run, testContext)
	if err != nil || result.Kind != StepAction || result.Proposal.Tool != "read_invoice" {
		t.Fatalf("step: %+v %v", result, err)
	}
	if requestsSeen.Load() != 1 {
		t.Fatalf("%d provider requests, want 1", requestsSeen.Load())
	}
	if requestBody["think"] != false || requestBody["stream"] != false {
		t.Fatalf("think/stream not disabled: think=%v stream=%v", requestBody["think"], requestBody["stream"])
	}

	ctx := context.Background()
	var callCount int
	var purpose, outcome, reservationStatus string
	var actualTokens int64
	if err = pool.QueryRow(ctx, `SELECT count(*), min(purpose), min(outcome) FROM runtime.model_calls WHERE run_id = $1`, run.RunID).
		Scan(&callCount, &purpose, &outcome); err != nil {
		t.Fatal(err)
	}
	if callCount != 1 || purpose != "agent" || outcome != string(budget.CallCompleted) {
		t.Fatalf("model_calls: %d %s %s", callCount, purpose, outcome)
	}
	// The ledger reservation is keyed by the dispatch record's id and settled to the reported usage.
	if err = pool.QueryRow(ctx, `SELECT status, actual_tokens FROM runtime.model_token_reservations WHERE run_id = $1 AND call_id = $2`,
		run.RunID, result.CallID).Scan(&reservationStatus, &actualTokens); err != nil {
		t.Fatalf("ledger reservation for call %s: %v", result.CallID, err)
	}
	snapshot, err := budget.NewPostgresStore(pool).Snapshot(ctx, run.RunID)
	if err != nil || reservationStatus != "settled" || actualTokens != 49 || snapshot.Used != 49 || snapshot.Reserved != 0 {
		t.Fatalf("ledger: %s %d %+v %v", reservationStatus, actualTokens, snapshot, err)
	}
}

func TestCallLogRejectsASecondOutcome(t *testing.T) {
	pool := testdb.Open(t)
	run := newRunFixture(t, pool, 20000)
	callLog := budget.NewCallLog(pool)
	ctx := context.Background()
	callID, err := callLog.RecordDispatch(ctx, run.OrganizationID, run.RunID, "agent", "test-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err = callLog.RecordOutcome(ctx, run.OrganizationID, callID, budget.CallUsageUnknown); err != nil {
		t.Fatal(err)
	}
	if err = callLog.RecordOutcome(ctx, run.OrganizationID, callID, budget.CallCompleted); err == nil {
		t.Fatal("a recorded outcome was overwritten")
	}
	// Another organization cannot set the outcome of this call.
	otherCallID, _ := callLog.RecordDispatch(ctx, run.OrganizationID, run.RunID, "security", "test-fixture")
	if err = callLog.RecordOutcome(ctx, testdb.ID(t), otherCallID, budget.CallCompleted); err == nil {
		t.Fatal("an outcome was recorded across organizations")
	}
	if _, err = callLog.RecordDispatch(ctx, run.OrganizationID, run.RunID, "guard", "test-fixture"); err == nil {
		t.Fatal("an unknown purpose was recorded")
	}
}
