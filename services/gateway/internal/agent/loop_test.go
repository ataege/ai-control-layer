package agent

import (
	"encoding/json"
	"errors"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
)

func TestNewLoopRequiresEveryDependency(t *testing.T) {
	if _, err := NewLoop(LoopDependencies{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty dependencies: %v", err)
	}
}

func TestStepErrorsNeverContinueTheRun(t *testing.T) {
	for _, testCase := range []struct {
		err  error
		want runEnd
	}{
		{errors.Join(ErrModelCallFailed, budget.ErrPaused), runEnd{status: contracts.RunPaused, reason: contracts.ReasonAllowanceExhausted}},
		{errors.Join(ErrModelCallFailed, model.ErrOverspend), runEnd{status: contracts.RunPaused, reason: contracts.ReasonAllowanceExhausted}},
		{errors.Join(ErrModelCallFailed, model.ErrUsageUnknown), runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown}},
		{ErrUnusableResponse, runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}},
		{ErrRecording, runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}},
	} {
		if got := stepErrorEnd(testCase.err); got != testCase.want {
			t.Errorf("%v: got %+v, want %+v", testCase.err, got, testCase.want)
		}
	}
}

func TestReasonsOutsideTheVocabularyBecomeDecisionUnavailable(t *testing.T) {
	if got := contractReason(policy.ReasonCode("made_up_reason")); got != contracts.ReasonDecisionUnavailable {
		t.Fatalf("unknown reason mapped to %s", got)
	}
	if got := contractReason(policy.ReasonResourceOutOfScope); got != contracts.ReasonResourceOutOfScope {
		t.Fatalf("known reason mapped to %s", got)
	}
	if got := refusalEnd(contracts.ReasonActionChanged); got.status != contracts.RunFailed {
		t.Fatalf("changed action ended as %s", got.status)
	}
}

func TestUnusableInspectionsAreNeverReleased(t *testing.T) {
	if validInspection(Inspection{Outcome: InspectionPass}) || validInspection(Inspection{Outcome: "other", Content: json.RawMessage(`{}`)}) {
		t.Fatal("an unusable inspection was accepted")
	}
	if !validInspection(Inspection{Outcome: InspectionBlocked, Content: json.RawMessage(`{"withheld":true}`)}) {
		t.Fatal("a withheld marker was refused")
	}
}

func TestProposedCallKeepsOnlyASmallJSONObject(t *testing.T) {
	for raw, want := range map[string]string{
		`{"invoice_id":"x"}`: `{"tool":"read_invoice","arguments":{"invoice_id":"x"}}`,
		`[1,2]`:              `{"tool":"read_invoice","arguments":{}}`,
		`not json`:           `{"tool":"read_invoice","arguments":{}}`,
	} {
		got := proposedCall(contracts.ActionProposal{Tool: contracts.ToolReadInvoice, Arguments: json.RawMessage(raw)})
		if string(got) != want {
			t.Errorf("%s: got %s, want %s", raw, got, want)
		}
	}
}
