//go:build model_live

package agent

import (
	"context"
	"os"
	"testing"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/testdb"
)

// TestLiveModelProposesATypedAction is GO-10's live evidence: the configured local model,
// reached through the accounted gateway, proposes one typed tool action within a recorded
// allowance. It runs only with -tags=model_live, GO_AGENT_LIVE=1, MODEL_NAME and PostgreSQL.
func TestLiveModelProposesATypedAction(t *testing.T) {
	if os.Getenv("GO_AGENT_LIVE") != "1" {
		t.Skip("live model test skipped: set GO_AGENT_LIVE=1")
	}
	modelName := os.Getenv("MODEL_NAME")
	baseURL := os.Getenv("MODEL_BASE_URL")
	if modelName == "" || baseURL == "" {
		t.Fatal("MODEL_NAME and MODEL_BASE_URL are required for the live test")
	}
	pool := testdb.Open(t)
	run := newRunFixture(t, pool, 20000)
	run.AllowedModels = []string{modelName}

	provider, err := model.NewOllama(model.Options{BaseURL: baseURL, Model: modelName, Timeout: 60 * time.Second,
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	store := budget.NewPostgresStore(pool)
	caller, err := model.NewAccountedCaller(provider, store, model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	stepper, err := NewStepper(caller, budget.NewCallLog(pool), modelName)
	if err != nil {
		t.Fatal(err)
	}
	taskContext := []model.Message{{Role: "user", Content: "Task: reconcile the Atlas invoices invoice_A01 and invoice_A02. " +
		"Start by reading invoice invoice_A01."}}
	result, err := stepper.Step(context.Background(), run, taskContext)
	if err != nil {
		t.Fatalf("live step failed: %v", err)
	}
	if result.Kind != StepAction {
		t.Fatalf("live model did not propose one action: kind %d, reject %q, final %q", result.Kind, result.RejectReason, result.FinalAnswer)
	}
	var arguments contracts.ReadInvoiceArguments
	if result.Proposal.Tool != contracts.ToolReadInvoice || contracts.DecodeStrict(result.Proposal.Arguments, &arguments) != nil {
		t.Fatalf("live proposal is not a typed read_invoice: %s %s", result.Proposal.Tool, result.Proposal.Arguments)
	}
	snapshot, err := store.Snapshot(context.Background(), run.RunID)
	if err != nil || snapshot.Reserved != 0 || snapshot.Used <= 0 || snapshot.Used > snapshot.Limit {
		t.Fatalf("allowance not settled within its limit: %+v %v", snapshot, err)
	}
	t.Logf("LIVE model=%s tool=%s invoice_id=%s input_tokens=%d output_tokens=%d used=%d limit=%d wall_ns=%d",
		modelName, result.Proposal.Tool, arguments.InvoiceID, *result.Usage.InputTokens, *result.Usage.OutputTokens,
		snapshot.Used, snapshot.Limit, result.Duration.Nanoseconds())
}
