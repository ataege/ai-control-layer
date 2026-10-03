//go:build model_live

package agent

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/catalog/catalogtest"
	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/contracts"
)

// TestLiveProductionChainExecutesAPermittedTool is GO-11's live evidence: the production chain
// (NewProductionChain, as cmd/gateway builds it) runs an admitted run on the local model, with
// the run check before every model request, the real gate, executor, read_invoice adapter, hybrid
// tool-result inspection and ledger. It activates the repository's policy and signature feed on
// the test database (committed; the previous pointer is restored afterwards), so run it alone:
// -tags=model_live, GO_AGENT_LIVE=1, MODEL_BASE_URL, MODEL_NAME and a superuser test database.
func TestLiveProductionChainExecutesAPermittedTool(t *testing.T) {
	if os.Getenv("GO_AGENT_LIVE") != "1" {
		t.Skip("live chain test skipped: set GO_AGENT_LIVE=1")
	}
	modelConfig, err := config.LoadModel()
	if err != nil {
		t.Fatalf("model configuration: %v", err)
	}
	world := newLoopWorld(t, passportOptions{allowedModel: modelConfig.Name, openLedger: true})
	activateRepositoryPolicy(t, world)

	logs := &bytes.Buffer{}
	chain, err := NewProductionChain(world.pool, catalog.NewLoader(), ChainConfig{Model: modelConfig, ModelConfigured: true,
		Logger: slog.New(slog.NewJSONHandler(logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	outcome, err := chain.Loop.Handle(ctx, world.job())
	if err != nil {
		t.Fatalf("handle: %v (logs: %s)", err, logs.String())
	}
	state := world.runState(t)
	reason := ""
	if state.TerminalReason != nil {
		reason = string(*state.TerminalReason)
	}
	executed := world.count(t, `SELECT count(*) FROM runtime.execution_attempts AS attempt
		JOIN runtime.actions AS action ON action.id = attempt.action_id AND action.organization_id = attempt.organization_id
		WHERE attempt.organization_id = $1 AND attempt.outcome = 'succeeded' AND action.tool = 'read_invoice'`)
	agentCalls := world.count(t, `SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1 AND purpose = 'agent'`)
	securityCalls := world.count(t, `SELECT count(*) FROM runtime.model_calls WHERE organization_id = $1 AND purpose = 'security'`)
	lookups := world.count(t, `SELECT count(*) FROM runtime.timing_records WHERE organization_id = $1 AND phase = 'policy_lookup'`)
	snapshot, _ := budget.NewPostgresStore(world.pool).Snapshot(context.Background(), world.runID)
	t.Logf("LIVE model=%s outcome=%v run_status=%s reason=%q read_invoice_executed=%d agent_calls=%d security_calls=%d policy_lookups=%d tokens_used=%d tokens_reserved=%d limit=%d",
		modelConfig.Name, outcome, state.Status, reason, executed, agentCalls, securityCalls, lookups, snapshot.Used, snapshot.Reserved, snapshot.Limit)
	logRows(t, world, "actions", `SELECT step_number || ' ' || tool || ' ' || status || ' ' || canonical_arguments::text FROM runtime.actions WHERE organization_id = $1 ORDER BY step_number`)
	logRows(t, world, "events", `SELECT event_type || ' ' || COALESCE(decision, '-') || ' ' || COALESCE(reason_code, '-') FROM runtime.audit_events WHERE organization_id = $1 ORDER BY id`)
	logRows(t, world, "context", `SELECT step_number || ' ' || kind || ' ' || COALESCE(inspection_outcome, '-') || ' ' || COALESCE(reason_code, '-') FROM runtime.context_entries WHERE organization_id = $1 ORDER BY id`)
	logRows(t, world, "attempts", `SELECT attempt_number || ' ' || COALESCE(outcome, 'open') FROM runtime.execution_attempts WHERE organization_id = $1 ORDER BY dispatch_recorded_at`)
	if executed < 1 {
		t.Fatalf("no permitted read_invoice was executed (run %s %q; logs: %s)", state.Status, reason, logs.String())
	}
	// Every agent model request was preceded by the run check (one policy lookup per checked step).
	if lookups < agentCalls {
		t.Fatalf("%d agent calls but only %d run checks", agentCalls, lookups)
	}
	if state.Status == contracts.RunRunning || state.Status == contracts.RunQueued {
		t.Logf("run still %s after one claim (step bound or requeue)", state.Status)
	}
}

// activateRepositoryPolicy activates the repository's policy and the real signature feed and
// restores the previous active pointer when the test ends.
func activateRepositoryPolicy(t *testing.T, world *loopWorld) {
	t.Helper()
	ctx := context.Background()
	var previousRevision, previousFeed *int64
	_ = world.pool.QueryRow(ctx, `SELECT active_revision_id, active_feed_revision_id FROM app.control_catalog_pointer WHERE id = 1`).
		Scan(&previousRevision, &previousFeed)
	transaction, err := world.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	feedID := catalogtest.InsertFeed(t, transaction)
	catalogtest.Activate(t, transaction, catalogtest.PolicyContent, &feedID)
	if err = transaction.Commit(ctx); err != nil {
		t.Fatalf("activate policy: %v", err)
	}
	t.Cleanup(func() {
		if _, err := world.pool.Exec(context.Background(), `UPDATE app.control_catalog_pointer
			SET active_revision_id = $1, active_feed_revision_id = $2 WHERE id = 1`, previousRevision, previousFeed); err != nil {
			t.Logf("previous catalog pointer not restored: %v", err)
		}
	})
}

// logRows writes safe metadata of the run (ids, tools, statuses, codes; no content) to the test log.
func logRows(t *testing.T, world *loopWorld, label, query string) {
	t.Helper()
	rows, err := world.pool.Query(context.Background(), query, world.organizationID)
	if err != nil {
		t.Logf("%s: %v", label, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if rows.Scan(&line) == nil {
			t.Logf("%s: %s", label, line)
		}
	}
}
