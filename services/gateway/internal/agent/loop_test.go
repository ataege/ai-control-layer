package agent

import (
	"encoding/json"
	"errors"
	"strings"
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
		if got := stepErrorEnd(testCase.err); got.status != testCase.want.status || got.reason != testCase.want.reason {
			t.Errorf("%v: got %+v, want %+v", testCase.err, got, testCase.want)
		}
	}
}

// GO-58: a model failure names which failure it was, in fixed text the event may store.
func TestModelFailuresNameTheActualFailure(t *testing.T) {
	for err, want := range map[error]string{
		errors.Join(ErrModelCallFailed, model.ErrUsageUnknown):                     messageModelUsageUnknown,
		errors.Join(ErrModelCallFailed, model.ErrUsageUnknown, model.ErrTransport): messageModelUnreachable,
		errors.Join(ErrModelCallFailed, model.ErrUsageUnknown, model.ErrResponse):  messageModelBadResponse,
		errors.Join(ErrModelCallFailed, model.ErrTimeout):                          messageModelTimeout,
		ErrUnusableResponse: messageModelUnusable,
		ErrRecording:        messageModelNotRecorded,
		errors.Join(ErrModelCallFailed, budget.ErrNotFound): messageModelCallFailed,
	} {
		end := stepErrorEnd(err)
		if end.message != want || end.purpose != string(model.AgentPurpose) || len(end.message) > 512 {
			t.Errorf("%v: %+v", err, end)
		}
	}
	// An allowance stop keeps the reason's own X-13 message.
	if end := stepErrorEnd(errors.Join(ErrModelCallFailed, budget.ErrExhausted)); end.message != "" || end.purpose != string(model.AgentPurpose) {
		t.Errorf("allowance stop: %+v", end)
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

// A step error that carries ErrUsageUnknown pauses the run for attention even when it also carries
// the failure to persist that fact; the log names the kinds and never the error text.
func TestUnpersistedUnknownUsagePausesTheRunAndTheLogNamesTheKinds(t *testing.T) {
	err := errors.Join(ErrModelCallFailed, errors.Join(model.ErrUsageUnknown, model.ErrAccounting))
	end := stepErrorEnd(err)
	if end.status != contracts.RunPaused || end.reason != contracts.ReasonOutcomeUnknown {
		t.Fatalf("a dispatched call with unknown usage ended the run as %s/%s", end.status, end.reason)
	}
	kinds := strings.Join(stepErrorKinds(errors.Join(err, errors.New("provider said: secret detail"))), ",")
	if kinds != "usage_unknown,accounting,model_call_failed" {
		t.Fatalf("kinds %q", kinds)
	}
	if end := stepErrorEnd(errors.Join(ErrModelCallFailed, model.ErrAccounting)); end.status != contracts.RunFailed {
		t.Fatalf("an accounting failure without a dispatched call must still fail: %s", end.status)
	}
}
