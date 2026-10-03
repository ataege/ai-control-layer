package agent

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
)

// contextTokens is the context window requested for every agent step (Ollama num_ctx). The
// output ceiling comes from the trusted accounting settings inside the accounted caller.
const contextTokens = 8192

var (
	// ErrInvalid rejects an unusable step input before any dispatch.
	ErrInvalid = errors.New("invalid agent step")
	// ErrModelNotAllowed: the configured model is outside the passport's allowed models; nothing
	// is dispatched.
	ErrModelNotAllowed = errors.New("model not allowed for this run")
	// ErrModelCallFailed: the dispatched call failed or its usage is unknown. Under the
	// `model call retries` default (Figure 5) the run fails; no retry is made.
	ErrModelCallFailed = errors.New("model call failed")
	// ErrUnusableResponse: the model answered with neither a tool call nor text.
	ErrUnusableResponse = errors.New("model response is neither an action nor a final answer")
	// ErrRecording: the dispatch record could not be written; nothing was dispatched.
	ErrRecording = errors.New("model call could not be recorded")
)

// StepKind says how a model response is to be handled.
type StepKind int

const (
	// StepAction carries exactly one proposal for the action gate. Its arguments are not
	// validated here: a malformed proposal is stored and denied at the gate.
	StepAction StepKind = iota + 1
	// StepFinal carries a final answer for the final-result check (GO-26).
	StepFinal
	// StepRejected: the whole response is denied (GO-01: several actions); no subset runs.
	StepRejected
)

// StepResult is the outcome of one model step. Usage stays nil-valued when the provider did not
// report it; missing counts are never zero.
type StepResult struct {
	Kind         StepKind
	Proposal     contracts.ActionProposal
	FinalAnswer  string
	RejectReason contracts.ReasonCode
	CallID       string
	Usage        model.Usage
	// Duration is Go's wall time for the provider request.
	Duration time.Duration
}

// ModelCaller is the accounted model gateway (model.AccountedCaller).
type ModelCaller interface {
	Call(ctx context.Context, runID, callID string, request model.Request) (model.AccountedResult, error)
}

// CallRecorder writes the pre-dispatch record of each model call (budget.CallLog).
type CallRecorder interface {
	RecordDispatch(ctx context.Context, organizationID, runID, purpose, model string) (string, error)
	RecordOutcome(ctx context.Context, organizationID, callID string, outcome budget.CallOutcome) error
}

// Run identifies the run a step belongs to; every value comes from the stored passport.
type Run struct {
	OrganizationID string
	RunID          string
	AllowedModels  []string
}

// Stepper performs one agent-purpose model step.
type Stepper struct {
	caller   ModelCaller
	recorder CallRecorder
	model    string
}

// NewStepper returns a stepper for one configured model.
func NewStepper(caller ModelCaller, recorder CallRecorder, modelName string) (*Stepper, error) {
	if caller == nil || recorder == nil || strings.TrimSpace(modelName) == "" {
		return nil, ErrInvalid
	}
	return &Stepper{caller: caller, recorder: recorder, model: modelName}, nil
}

// Step sends the fixed instruction and the minimized task context to the model, offering the four
// registered tools only, and reads the response. The context must already be minimized and
// checked (GO-23, GO-76); Step adds nothing to it but the fixed instruction.
func (stepper *Stepper) Step(ctx context.Context, run Run, taskContext []model.Message) (StepResult, error) {
	if run.OrganizationID == "" || run.RunID == "" || len(taskContext) == 0 {
		return StepResult{}, ErrInvalid
	}
	if !slices.Contains(run.AllowedModels, stepper.model) {
		return StepResult{}, ErrModelNotAllowed
	}
	callID, err := stepper.recorder.RecordDispatch(ctx, run.OrganizationID, run.RunID, string(model.AgentPurpose), stepper.model)
	if err != nil {
		return StepResult{}, ErrRecording
	}
	messages := append([]model.Message{{Role: "system", Content: systemInstruction}}, taskContext...)
	accounted, callErr := stepper.caller.Call(ctx, run.RunID, callID, model.Request{
		Purpose:       model.AgentPurpose,
		Messages:      messages,
		ContextTokens: contextTokens,
		Tools:         ToolDefinitions(),
	})
	result := StepResult{CallID: callID, Usage: accounted.Provider.Usage, Duration: accounted.Provider.Duration}

	outcome := budget.CallCompleted
	if callErr != nil {
		outcome = budget.CallFailed
		if accounted.UsageUnknown {
			outcome = budget.CallUsageUnknown
		}
	}
	interpreted, interpretErr := interpret(accounted.Provider.Message)
	if callErr == nil && interpretErr != nil {
		outcome = budget.CallFailed
	}
	// The outcome is written even if the step's context ended: the call was dispatched.
	recordContext, cancelRecord := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelRecord()
	recordErr := stepper.recorder.RecordOutcome(recordContext, run.OrganizationID, callID, outcome)

	switch {
	case callErr != nil:
		return result, errors.Join(ErrModelCallFailed, callErr)
	case recordErr != nil:
		return result, ErrRecording
	case interpretErr != nil:
		return result, interpretErr
	}
	interpreted.CallID, interpreted.Usage, interpreted.Duration = result.CallID, result.Usage, result.Duration
	return interpreted, nil
}

// interpret reads one assistant message as one action, a final answer or a rejection.
func interpret(message model.Message) (StepResult, error) {
	switch {
	case len(message.ToolCalls) > 1:
		// GO-01: never select or execute a subset of several proposed actions.
		return StepResult{Kind: StepRejected, RejectReason: contracts.ReasonMultipleActionsNotSupported}, nil
	case len(message.ToolCalls) == 1:
		call := message.ToolCalls[0].Function
		return StepResult{Kind: StepAction, Proposal: contracts.ActionProposal{
			Tool:      contracts.ToolName(call.Name),
			Arguments: append([]byte(nil), call.Arguments...),
		}}, nil
	case strings.TrimSpace(message.Content) != "":
		return StepResult{Kind: StepFinal, FinalAnswer: message.Content}, nil
	default:
		return StepResult{}, ErrUnusableResponse
	}
}
