//go:build model_live

package security

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"starter/services/gateway/internal/config"
	"starter/services/gateway/internal/model"
)

// Evidence X-96: the labelled corpus through the real local evaluator. It needs the model_live
// build tag and GO_SECURITY_LIVE=1. GO_SECURITY_EVIDENCE_FILE names a JSON results file to write;
// GO_SECURITY_LIVE_STRICT=1 also fails the test on a false positive or negative. The run is a
// finite labelled sample: its counts are observations, not a detection rate.

type liveCaseResult struct {
	ID             string   `json:"id"`
	Fixture        string   `json:"fixture"`
	Category       string   `json:"category"`
	Boundary       Boundary `json:"boundary"`
	Expected       string   `json:"expected"`
	Outcome        Outcome  `json:"outcome"`
	Verdict        *Verdict `json:"verdict"`
	Failure        string   `json:"failure,omitempty"`
	Pass           bool     `json:"pass"`
	InputTokens    *int64   `json:"input_tokens"`
	OutputTokens   *int64   `json:"output_tokens"`
	ProviderMillis int64    `json:"provider_ms"`
	// Hostile notes only: the second, independent pass through InspectToolResult.
	PipelineOutcome ResultOutcome `json:"pipeline_outcome,omitempty"`
	PipelineVerdict *Verdict      `json:"pipeline_verdict,omitempty"`
	ContextWithheld *bool         `json:"context_withheld,omitempty"`
}

type liveSummary struct {
	Total          int `json:"total"`
	Pass           int `json:"pass"`
	FalsePositives int `json:"false_positives"`
	FalseNegatives int `json:"false_negatives"`
	GuardFailures  int `json:"guard_failures"`
	// PipelinePasses counts hostile notes that a second evaluation let into the context.
	PipelinePasses int `json:"hostile_pipeline_passes"`
}

func TestLiveSemanticCorpus(t *testing.T) {
	if os.Getenv("GO_SECURITY_LIVE") != "1" {
		t.Skip("live corpus run requires GO_SECURITY_LIVE=1 and the model_live build tag")
	}
	modelSettings, err := config.LoadModel()
	if err != nil {
		t.Fatal("live model configuration unavailable")
	}
	provider, err := model.NewOllama(model.Options{BaseURL: modelSettings.BaseURL, Model: modelSettings.Name, Timeout: 60 * time.Second, MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal("live model configuration invalid")
	}
	// The ledger is the labelled in-memory double: this run measures verdicts, not allowances.
	caller, err := model.NewAccountedCaller(provider, &ledgerDouble{limit: 1 << 40}, model.DefaultAccountingSettings())
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewSemanticEvaluator(caller, EvaluatorOptions{Model: modelSettings.Name, ContextTokens: MinEvaluatorContextTokens, Source: VerdictLive})
	if err != nil {
		t.Fatal(err)
	}
	settings := semanticSettings(ModeBlock)

	type liveCase struct {
		id, fixture, category, expected string
		boundary                        Boundary
		text                            string
	}
	var cases []liveCase
	for _, corpusCase := range loadCorpus(t) {
		if corpusCase.Category == "secret_redaction" {
			continue // the deterministic secret rules cover these (X-99)
		}
		cases = append(cases, liveCase{corpusCase.ID, "semantic-corpus.json", corpusCase.Category, corpusCase.ExpectedOutcome, corpusCase.Boundary, corpusCase.Text})
	}
	raw, err := os.ReadFile("../../../../fixtures/hostile-notes.json")
	if err != nil {
		t.Fatal(err)
	}
	var notes struct {
		Notes []struct{ ID, Text string } `json:"notes"`
	}
	if err := json.Unmarshal(raw, &notes); err != nil {
		t.Fatal(err)
	}
	for _, note := range notes.Notes {
		cases = append(cases, liveCase{note.ID, "hostile-notes.json", "hostile_note", "block", BoundaryToolResult, note.Text})
	}

	var results []liveCaseResult
	var summary liveSummary
	for _, liveCase := range cases {
		name := FieldToolResultText
		if liveCase.boundary == BoundaryModelInput {
			name = FieldModelInputText
		}
		semantic, _ := evaluator.Evaluate(context.Background(), testRunID, liveCase.boundary, Field{Name: name, Text: liveCase.text}, settings)
		result := liveCaseResult{
			ID: liveCase.id, Fixture: liveCase.fixture, Category: liveCase.category, Boundary: liveCase.boundary,
			Expected: liveCase.expected, Outcome: semantic.Record.Outcome, Verdict: semantic.Record.Verdict,
			Failure: semantic.Record.Failure, InputTokens: semantic.Usage.InputTokens, OutputTokens: semantic.Usage.OutputTokens,
			ProviderMillis: semantic.ProviderDuration.Milliseconds(),
		}
		blocked := semantic.Record.Outcome == OutcomeBlock
		switch {
		case semantic.Paused():
			summary.GuardFailures++
		case liveCase.expected == "block" && blocked, liveCase.expected == "allow" && semantic.Record.Outcome == OutcomePass:
			result.Pass = true
			summary.Pass++
		case liveCase.expected == "allow":
			summary.FalsePositives++
		default:
			summary.FalseNegatives++
		}
		// A hostile note also runs through the full tool-result pipeline: a blocked note must not
		// appear in what would become agent context.
		if liveCase.fixture == "hostile-notes.json" {
			inspection, _ := NewInspector(evaluator).InspectToolResult(context.Background(), invoiceInput(t, liveCase.text), fullSettings(t, ModeBlock))
			withheld := inspection.ResultJSON == nil || !strings.Contains(string(inspection.ResultJSON), liveCase.text)
			result.ContextWithheld, result.PipelineOutcome = &withheld, inspection.Outcome
			if len(inspection.SemanticCalls) > 0 {
				result.PipelineVerdict = inspection.SemanticCalls[0].Record.Verdict
			}
			if inspection.Outcome == ResultPass {
				summary.PipelinePasses++
			}
			t.Logf("evidence X-96: %s through InspectToolResult -> %s verdict=%+v context_withheld=%v",
				liveCase.id, inspection.Outcome, result.PipelineVerdict, withheld)
			if inspection.Outcome != ResultPass && !withheld {
				t.Errorf("%s: a non-passing inspection leaked the note into the context", liveCase.id)
			}
		}
		summary.Total++
		results = append(results, result)
		t.Logf("evidence X-96: %s (%s, expected %s) -> %s verdict=%+v failure=%s provider=%dms",
			liveCase.id, liveCase.boundary, liveCase.expected, result.Outcome, result.Verdict, result.Failure, result.ProviderMillis)
	}
	t.Logf("evidence X-96: model=%s threshold=%.2f context=%d total=%d pass=%d false_positives=%d false_negatives=%d guard_failures=%d hostile_pipeline_passes=%d",
		modelSettings.Name, settings.SemanticInjection.Threshold, MinEvaluatorContextTokens,
		summary.Total, summary.Pass, summary.FalsePositives, summary.FalseNegatives, summary.GuardFailures, summary.PipelinePasses)

	if path := os.Getenv("GO_SECURITY_EVIDENCE_FILE"); path != "" {
		document := map[string]any{
			"evidence": "X-96", "verdict_source": VerdictLive, "model": modelSettings.Name,
			"threshold": settings.SemanticInjection.Threshold, "context_tokens": MinEvaluatorContextTokens,
			"recorded_at": time.Now().UTC().Format(time.RFC3339), "summary": summary, "cases": results,
			"note": "Finite labelled synthetic sample; counts are observations, not a detection rate.",
		}
		encoded, err := json.MarshalIndent(document, "", "  ")
		if err != nil || os.WriteFile(path, append(encoded, '\n'), 0o600) != nil {
			t.Fatal("could not write the evidence file")
		}
	}
	if summary.GuardFailures > 0 {
		t.Fatalf("%d live checks failed closed (guard failure); see the evidence lines", summary.GuardFailures)
	}
	if os.Getenv("GO_SECURITY_LIVE_STRICT") == "1" && summary.Pass != summary.Total {
		t.Fatalf("strict mode: %d of %d cases did not match their label", summary.Total-summary.Pass, summary.Total)
	}
}
