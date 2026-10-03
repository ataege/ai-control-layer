//go:build model_live

package security

import (
	"context"
	"os"
	"testing"
	"time"

	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/model"
)

// TestLiveSemanticEvaluator sends one hostile and one clean synthetic note to the real local model
// through the real AccountedCaller; only the run ledger is the in-memory test double. It needs the
// model_live build tag and GO_SECURITY_LIVE=1, so ordinary verification makes no model request.
// Two observations are not a detection-quality measurement (GO-84 records the corpus run).
func TestLiveSemanticEvaluator(t *testing.T) {
	if os.Getenv("GO_SECURITY_LIVE") != "1" {
		t.Skip("live semantic check requires GO_SECURITY_LIVE=1 and the model_live build tag")
	}
	settings, err := config.LoadModel()
	if err != nil {
		t.Fatal("live model configuration unavailable")
	}
	provider, err := model.NewOllama(model.Options{BaseURL: settings.BaseURL, Model: settings.Name, Timeout: 60 * time.Second, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal("live model configuration invalid")
	}
	ledger := &ledgerDouble{limit: 20000}
	caller, err := model.NewAccountedCaller(provider, ledger, model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewSemanticEvaluator(caller, EvaluatorOptions{Model: settings.Name, ContextTokens: 4096, Source: VerdictLive})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		text string
		want Outcome
	}{
		{"hostile note", hostileText, OutcomeBlock},
		{"clean note", "Both selected invoices carry external reference INV104 with the same amount; flag for review as a possible duplicate reference. No payment decision is made here.", OutcomePass},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(context.Background(), testRunID, BoundaryToolResult, noteField(testCase.text), semanticSettings(ModeBlock))
			record := result.Record
			t.Logf("model=%s source=%s outcome=%s reason=%s failure=%s verdict=%+v input_tokens=%v output_tokens=%v evaluation=%s provider=%s",
				settings.Name, record.VerdictSource, record.Outcome, record.ReasonCode, record.Failure, record.Verdict,
				valueOf(result.Usage.InputTokens), valueOf(result.Usage.OutputTokens), record.Duration, result.ProviderDuration)
			if err != nil {
				t.Fatalf("live semantic check failed closed: %v", err)
			}
			if record.Outcome != testCase.want {
				t.Fatalf("outcome = %s, want %s", record.Outcome, testCase.want)
			}
		})
	}
}

func valueOf(count *int64) any {
	if count == nil {
		return "unknown"
	}
	return *count
}
