package security

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The judge path (internal/evaluation, X-91) runs a tool result through the full policy: the content
// rules first, then the semantic check on what the content rules left. X-99's test covers the secret
// rules alone, and the live corpus test skips the secret cases, so this is the test of the whole path
// for those cases. Labelled fixture verdicts stand in for the model: it proves how the pieces compose,
// not what the live model answers.

// judgeToolResultInput is what the evaluation adapter builds for a judge's tool_result text.
func judgeToolResultInput(t *testing.T, text string) ToolResultInput {
	t.Helper()
	resultJSON, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		t.Fatal(err)
	}
	source := SourceRef{SourceID: "judge_input"}
	return ToolResultInput{
		RunID: testRunID, Tool: "read_invoice", ResultJSON: resultJSON, Source: source,
		Untrusted: []UntrustedPath{{Path: []string{"text"}, Name: FieldToolResultText, Source: source}},
	}
}

func fullPolicySettings(t *testing.T) Settings {
	t.Helper()
	settings, err := SettingsFromCatalog(4, []byte(samplePolicyContent), loadCommittedFeed(t), committedFeedDigest)
	if err != nil {
		t.Fatal(err)
	}
	return settings
}

func resultText(t *testing.T, resultJSON json.RawMessage) string {
	t.Helper()
	var decoded struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(resultJSON, &decoded); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	return decoded.Text
}

func secretToolResultCases(t *testing.T) []corpusCase {
	t.Helper()
	var cases []corpusCase
	for _, corpusCase := range loadCorpus(t) {
		if corpusCase.Category == "secret_redaction" && corpusCase.Boundary == BoundaryToolResult {
			cases = append(cases, corpusCase)
		}
	}
	if len(cases) == 0 {
		t.Fatal("the corpus has no tool-result secret case")
	}
	return cases
}

// With a benign semantic verdict the secret case ends exactly as the end-to-end check expects:
// redacted, no secret value in the returned text, and the model was shown only the redacted text.
func TestSecretToolResultIsRedactedWhenTheSemanticVerdictIsBenign(t *testing.T) {
	for _, corpusCase := range secretToolResultCases(t) {
		t.Run(corpusCase.ID, func(t *testing.T) {
			provider := &providerDouble{answer: passVerdict}
			inspection, err := inspectorWith(t, provider, &ledgerDouble{limit: 20000}).
				InspectToolResult(context.Background(), judgeToolResultInput(t, corpusCase.Text), fullPolicySettings(t))
			if err != nil || inspection.Outcome != ResultRedacted || inspection.ReasonCode != ReasonContentRedacted {
				t.Fatalf("inspection = %+v, err = %v", inspection, err)
			}
			text := resultText(t, inspection.ResultJSON)
			if !strings.Contains(text, "[REDACTED") {
				t.Fatalf("no redaction marker in %q", text)
			}
			if provider.calls() != 1 {
				t.Fatalf("%d semantic calls, want 1 on the redacted text", provider.calls())
			}
			sent, _ := json.Marshal(provider.requests)
			for _, secret := range corpusCase.Secrets {
				if strings.Contains(text, secret.Value) || strings.Contains(string(sent), secret.Value) {
					t.Fatalf("a secret value of %s reached the result or the model request", secret.Kind)
				}
			}
		})
	}
}

// A semantic verdict restricts, never grants (guardrail 8): when the verdict on the redacted text is
// a block, the redaction does not rescue it. The judge path answers deny, the text is withheld, and
// the secret still never reaches the model or the answer. This is the deny of session 08's e2e run
// when the live model rated the redacted note as risky; it is the designed outcome of the
// composition, not a defect of the secret rule.
func TestSecretToolResultIsDeniedWhenTheSemanticVerdictBlocksTheRedactedText(t *testing.T) {
	for _, corpusCase := range secretToolResultCases(t) {
		t.Run(corpusCase.ID, func(t *testing.T) {
			provider := &providerDouble{answer: blockVerdict}
			inspection, err := inspectorWith(t, provider, &ledgerDouble{limit: 20000}).
				InspectToolResult(context.Background(), judgeToolResultInput(t, corpusCase.Text), fullPolicySettings(t))
			if err != nil || inspection.Outcome != ResultBlocked || inspection.ReasonCode != ReasonSemanticInjectionDetected {
				t.Fatalf("inspection = %+v, err = %v", inspection, err)
			}
			if text := resultText(t, inspection.ResultJSON); text != "[WITHHELD:semantic_injection_detected]" {
				t.Fatalf("blocked text = %q", text)
			}
			sent, _ := json.Marshal(provider.requests)
			for _, secret := range corpusCase.Secrets {
				if strings.Contains(string(sent), secret.Value) || strings.Contains(string(inspection.ResultJSON), secret.Value) {
					t.Fatalf("a secret value of %s reached the model request or the result", secret.Kind)
				}
			}
		})
	}
}

// The deterministic part alone (no semantic call) redacts: so a deny can only come from the
// semantic verdict, a signature, or a guard failure, never from the secret rule.
func TestSecretToolResultWithoutTheSemanticGuardIsRedactedByTheSecretRuleAlone(t *testing.T) {
	settings, err := deterministicSettings(t, 4, "feed_v1", loadCommittedFeed(t), committedFeedDigest)
	if err != nil {
		t.Fatal(err)
	}
	for _, corpusCase := range secretToolResultCases(t) {
		inspection, err := NewInspector(nil).InspectToolResult(context.Background(), judgeToolResultInput(t, corpusCase.Text), settings)
		if err != nil || inspection.Outcome != ResultRedacted {
			t.Fatalf("%s: inspection = %+v, err = %v", corpusCase.ID, inspection, err)
		}
	}
}
