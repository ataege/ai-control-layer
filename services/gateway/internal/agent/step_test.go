package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
)

// fakeCaller is a labelled test double for the accounted model gateway.
type fakeCaller struct {
	result   model.AccountedResult
	err      error
	requests []model.Request
	callIDs  []string
}

func (caller *fakeCaller) Call(_ context.Context, _, callID string, request model.Request) (model.AccountedResult, error) {
	caller.requests = append(caller.requests, request)
	caller.callIDs = append(caller.callIDs, callID)
	return caller.result, caller.err
}

// fakeRecorder records dispatches and outcomes in memory.
type fakeRecorder struct {
	dispatchErr error
	dispatches  []string
	outcomes    map[string]budget.CallOutcome
}

func (recorder *fakeRecorder) RecordDispatch(_ context.Context, _, _, purpose, _ string) (string, error) {
	if recorder.dispatchErr != nil {
		return "", recorder.dispatchErr
	}
	recorder.dispatches = append(recorder.dispatches, purpose)
	return "call-1", nil
}

func (recorder *fakeRecorder) RecordOutcome(_ context.Context, _, callID string, outcome budget.CallOutcome) error {
	if recorder.outcomes == nil {
		recorder.outcomes = map[string]budget.CallOutcome{}
	}
	recorder.outcomes[callID] = outcome
	return nil
}

func int64Pointer(value int64) *int64 { return &value }

var testRun = Run{OrganizationID: "org-1", RunID: "run-1", AllowedModels: []string{"qwen3.5:4b"}}

var testContext = []model.Message{{Role: "user", Content: "Reconcile invoice_A01 and invoice_A02."}}

func toolCall(name, arguments string) model.ToolCall {
	return model.ToolCall{Function: model.FunctionCall{Name: name, Arguments: json.RawMessage(arguments)}}
}

func stepWith(t *testing.T, message model.Message, callErr error, usageUnknown bool) (StepResult, error, *fakeCaller, *fakeRecorder) {
	t.Helper()
	caller := &fakeCaller{err: callErr, result: model.AccountedResult{
		Provider:     model.Result{Message: message, Usage: model.Usage{InputTokens: int64Pointer(40), OutputTokens: int64Pointer(9)}},
		UsageUnknown: usageUnknown,
	}}
	recorder := &fakeRecorder{}
	stepper, err := NewStepper(caller, recorder, "qwen3.5:4b")
	if err != nil {
		t.Fatal(err)
	}
	result, stepErr := stepper.Step(context.Background(), testRun, testContext)
	return result, stepErr, caller, recorder
}

func TestOneToolCallBecomesOneProposal(t *testing.T) {
	result, err, caller, recorder := stepWith(t, model.Message{Role: "assistant", ToolCalls: []model.ToolCall{
		toolCall("read_invoice", `{"invoice_id":"invoice_A01"}`),
	}}, nil, false)
	if err != nil || result.Kind != StepAction {
		t.Fatalf("step: %+v %v", result, err)
	}
	if result.Proposal.Tool != contracts.ToolReadInvoice || string(result.Proposal.Arguments) != `{"invoice_id":"invoice_A01"}` {
		t.Fatalf("proposal: %+v", result.Proposal)
	}
	if result.CallID != "call-1" || *result.Usage.InputTokens != 40 || *result.Usage.OutputTokens != 9 {
		t.Fatalf("call record: %+v", result)
	}
	if len(caller.requests) != 1 || recorder.outcomes["call-1"] != budget.CallCompleted || len(recorder.dispatches) != 1 || recorder.dispatches[0] != "agent" {
		t.Fatalf("dispatches %v outcomes %v", recorder.dispatches, recorder.outcomes)
	}

	// The request holds the fixed instruction, then exactly the given context, and the four
	// registered tools only.
	request := caller.requests[0]
	if request.Purpose != model.AgentPurpose || len(request.Messages) != 2 || request.Messages[0].Role != "system" ||
		request.Messages[0].Content != systemInstruction || !reflect.DeepEqual(request.Messages[1:], testContext) {
		t.Fatalf("request messages: %+v", request.Messages)
	}
	var offered []string
	for _, tool := range request.Tools {
		offered = append(offered, tool.Function.Name)
	}
	if !reflect.DeepEqual(offered, []string{"read_invoice", "read_vendor", "create_report", "queue_report"}) {
		t.Fatalf("offered tools: %v", offered)
	}
}

func TestMalformedOrUnknownToolCallIsPassedToTheGateUnchanged(t *testing.T) {
	// The gate stores and denies these; the stepper neither repairs nor executes them.
	for _, call := range []model.ToolCall{
		toolCall("read_invoice", `{"invoice_id":"invoice_A01","extra":true}`),
		toolCall("run_shell", `{"command":"ls"}`),
	} {
		result, err, _, _ := stepWith(t, model.Message{Role: "assistant", ToolCalls: []model.ToolCall{call}}, nil, false)
		if err != nil || result.Kind != StepAction || string(result.Proposal.Tool) != call.Function.Name ||
			string(result.Proposal.Arguments) != string(call.Function.Arguments) {
			t.Fatalf("%s: %+v %v", call.Function.Name, result, err)
		}
	}
}

func TestSeveralToolCallsRejectTheWholeResponse(t *testing.T) {
	result, err, _, recorder := stepWith(t, model.Message{Role: "assistant", ToolCalls: []model.ToolCall{
		toolCall("read_invoice", `{"invoice_id":"invoice_A01"}`),
		toolCall("read_invoice", `{"invoice_id":"invoice_A02"}`),
	}}, nil, false)
	if err != nil || result.Kind != StepRejected || result.RejectReason != contracts.ReasonMultipleActionsNotSupported {
		t.Fatalf("step: %+v %v", result, err)
	}
	if result.Proposal.Tool != "" || result.Proposal.Arguments != nil {
		t.Fatalf("a rejected response carried a proposal: %+v", result.Proposal)
	}
	// The dispatched call still counts; its usage was settled normally.
	if recorder.outcomes["call-1"] != budget.CallCompleted || result.Usage.InputTokens == nil {
		t.Fatalf("outcome %v usage %+v", recorder.outcomes, result.Usage)
	}
}

func TestTextWithoutToolCallIsAFinalAnswer(t *testing.T) {
	result, err, _, _ := stepWith(t, model.Message{Role: "assistant", Content: "Done: duplicate reference INV104."}, nil, false)
	if err != nil || result.Kind != StepFinal || result.FinalAnswer != "Done: duplicate reference INV104." {
		t.Fatalf("step: %+v %v", result, err)
	}
}

func TestUnusableResponseAndProviderFailureNeverBecomeAnAction(t *testing.T) {
	result, err, _, recorder := stepWith(t, model.Message{Role: "assistant", Content: "  "}, nil, false)
	if !errors.Is(err, ErrUnusableResponse) || result.Kind != 0 || recorder.outcomes["call-1"] != budget.CallFailed {
		t.Fatalf("empty response: %+v %v %v", result, err, recorder.outcomes)
	}

	// A timeout keeps usage unknown: the outcome says so and no proposal is produced.
	result, err, _, recorder = stepWith(t, model.Message{}, model.ErrTimeout, true)
	if !errors.Is(err, ErrModelCallFailed) || !errors.Is(err, model.ErrTimeout) || result.Kind != 0 ||
		recorder.outcomes["call-1"] != budget.CallUsageUnknown {
		t.Fatalf("timeout: %+v %v %v", result, err, recorder.outcomes)
	}

	// An exhausted allowance stays recognisable for the run's stop reason.
	_, err, _, recorder = stepWith(t, model.Message{}, budget.ErrExhausted, false)
	if !errors.Is(err, budget.ErrExhausted) || recorder.outcomes["call-1"] != budget.CallFailed {
		t.Fatalf("exhausted: %v %v", err, recorder.outcomes)
	}
}

func TestNothingIsDispatchedWithoutAllowedModelOrDispatchRecord(t *testing.T) {
	caller := &fakeCaller{}
	recorder := &fakeRecorder{}
	stepper, _ := NewStepper(caller, recorder, "qwen3.5:4b")
	notAllowed := testRun
	notAllowed.AllowedModels = []string{"qwen3.5:9b"}
	if _, err := stepper.Step(context.Background(), notAllowed, testContext); !errors.Is(err, ErrModelNotAllowed) {
		t.Fatalf("model outside the passport: %v", err)
	}
	if _, err := stepper.Step(context.Background(), Run{}, testContext); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing run: %v", err)
	}
	failingRecorder := &fakeRecorder{dispatchErr: budget.ErrUnavailable}
	unrecorded, _ := NewStepper(caller, failingRecorder, "qwen3.5:4b")
	if _, err := unrecorded.Step(context.Background(), testRun, testContext); !errors.Is(err, ErrRecording) {
		t.Fatalf("dispatch record failure: %v", err)
	}
	if len(caller.requests) != 0 || len(recorder.dispatches) != 0 {
		t.Fatalf("dispatched without authority or record: %d requests", len(caller.requests))
	}
	if _, err := NewStepper(nil, recorder, "model"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil caller: %v", err)
	}
}

// TestToolParametersMatchTheActionContract fails when the offered parameters drift from X-09.
func TestToolParametersMatchTheActionContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "packages", "contracts", "schemas", "action-proposal.schema.json"))
	if err != nil {
		t.Fatalf("read action contract: %v", err)
	}
	var schema struct {
		OneOf []struct {
			Properties struct {
				Tool      struct{ Const string } `json:"tool"`
				Arguments json.RawMessage        `json:"arguments"`
			} `json:"properties"`
		} `json:"oneOf"`
	}
	if err = json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode action contract: %v", err)
	}
	contractArguments := map[string]any{}
	for _, variant := range schema.OneOf {
		var arguments any
		if err = json.Unmarshal(variant.Properties.Arguments, &arguments); err != nil {
			t.Fatal(err)
		}
		contractArguments[variant.Properties.Tool.Const] = arguments
	}
	if len(contractArguments) != len(toolParameters) {
		t.Fatalf("contract has %d tools, the agent offers %d", len(contractArguments), len(toolParameters))
	}
	for _, tool := range ToolDefinitions() {
		var offered any
		if err = json.Unmarshal(tool.Function.Parameters, &offered); err != nil {
			t.Fatalf("%s parameters are not JSON: %v", tool.Function.Name, err)
		}
		if !reflect.DeepEqual(offered, contractArguments[tool.Function.Name]) {
			t.Errorf("%s parameters differ from action-proposal.schema.json", tool.Function.Name)
		}
	}
}
