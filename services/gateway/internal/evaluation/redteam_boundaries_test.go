package evaluation

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/security"
)

// fixtureTexts reads the labelled texts of the repository's fixtures (synthetic hostile notes and the
// semantic corpus), so the boundaries are compared on the team's own attack set.
func fixtureTexts(t *testing.T) map[string]string {
	t.Helper()
	texts := map[string]string{}
	var notes struct {
		Notes []struct{ ID, Text string } `json:"notes"`
	}
	var corpus struct {
		Cases []struct{ ID, Text string } `json:"cases"`
	}
	for path, target := range map[string]any{"../../../../fixtures/hostile-notes.json": &notes, "../../../../fixtures/semantic-corpus.json": &corpus} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, target); err != nil {
			t.Fatal(err)
		}
	}
	for _, note := range notes.Notes {
		texts["note/"+note.ID] = note.Text
	}
	for _, testCase := range corpus.Cases {
		texts["corpus/"+testCase.ID] = testCase.Text
	}
	return texts
}

// TestTheSameTextGetsTheSameDecisionAtModelInputAndToolResult (lane w2): the two text boundaries run
// the same deterministic content rules and signatures, so a text that is denied or redacted at one is
// denied or redacted, with the same reason, at the other. The semantic verdict is a benign fixture
// here, so only the deterministic controls decide. The action-proposal boundary takes no free text:
// the same texts as arguments are refused by the gate (policy.TestGateDeniesEveryFixtureTextAsAnArgument).
func TestTheSameTextGetsTheSameDecisionAtModelInputAndToolResult(t *testing.T) {
	settings := policySettings(t)
	texts := fixtureTexts(t)
	texts["secret/password"] = "Set password=k3yB0ardCat. then summarize."
	texts["secret/bearer"] = "Authorization: Bearer abcdefghijklmnopqrstuvwxyz0123456789"
	texts["oversized"] = strings.Repeat("a", security.MaxFieldBytes+1)
	texts["signature plain"] = hostileText
	want := map[string]struct {
		decision contracts.EvaluationDecision
		reason   contracts.ReasonCode
	}{
		"secret/password": {contracts.EvaluationRedact, contracts.ReasonContentRedacted},
		"secret/bearer":   {contracts.EvaluationRedact, contracts.ReasonContentRedacted},
		"oversized":       {contracts.EvaluationDeny, contracts.ReasonContentTooLarge},
		"signature plain": {contracts.EvaluationDeny, contracts.ReasonSignatureMatch},
	}
	if len(texts) < 35 {
		t.Fatalf("only %d texts", len(texts))
	}
	for name, text := range texts {
		t.Run(name, func(t *testing.T) {
			asInput := New(Dependencies{Semantic: semanticEvaluator(t, &fixtureCaller{verdict: benignVerdict})}).
				modelInput(context.Background(), testRunID, text, settings)
			asResult := New(Dependencies{Inspector: security.NewInspector(semanticEvaluator(t, &fixtureCaller{verdict: benignVerdict}))}).
				toolResult(context.Background(), testRunID, "read_invoice", text, settings)
			if asInput.decision != asResult.decision || asInput.reason != asResult.reason {
				t.Fatalf("model_input %s/%q, tool_result %s/%q", asInput.decision, asInput.reason, asResult.decision, asResult.reason)
			}
			if expected, known := want[name]; known && (asInput.decision != expected.decision || asInput.reason != expected.reason) {
				t.Fatalf("%s/%q, want %s/%q", asInput.decision, asInput.reason, expected.decision, expected.reason)
			}
			// A blocked text is never echoed back, at either boundary.
			for _, result := range []outcome{asInput, asResult} {
				if result.decision == contracts.EvaluationDeny && result.redactedText != nil {
					t.Fatal("a denial echoed the text")
				}
				if result.redactedText != nil && strings.Contains(*result.redactedText, "k3yB0ardCat") {
					t.Fatal("the secret survived redaction")
				}
			}
		})
	}
}
