package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"starter/services/gateway/internal/budget"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/worker"
)

// maximumStepsPerClaim bounds one claim of a job; the passport's agent call limit is the real
// step limit. A run that is still going afterwards is requeued and claimed again.
const maximumStepsPerClaim = 64

// ModelStepper performs one agent model step (Stepper).
type ModelStepper interface {
	Step(ctx context.Context, run Run, taskContext []model.Message) (StepResult, error)
}

// ActionGate decides one proposal (policy.Gate).
type ActionGate interface {
	Evaluate(ctx context.Context, run policy.RunIdentity, proposal policy.Proposal) policy.Decision
}

// ActionExecutor runs one allowed action by its identifier (policy.Executor).
type ActionExecutor interface {
	Execute(ctx context.Context, run policy.RunIdentity, actionID string) policy.ExecutionResult
}

// StepCounter counts the run's agent model calls (budget.CallLog).
type StepCounter interface {
	CountAgentCalls(ctx context.Context, organizationID, runID string) (int, error)
}

// LoopDependencies are the components the loop drives. Every one is required.
type LoopDependencies struct {
	Runs      *repository.Repository
	Stepper   ModelStepper
	Gate      ActionGate
	Executor  ActionExecutor
	Inspector ResultInspector
	Steps     StepCounter
	Contexts  *ContextStore
	Logger    *slog.Logger
}

// Loop is the bounded agent loop: the handler of contracts.JobKindAgentStep jobs (GO-11).
type Loop struct {
	dependencies LoopDependencies
	now          func() time.Time
}

// NewLoop validates its dependencies.
func NewLoop(dependencies LoopDependencies) (*Loop, error) {
	if dependencies.Runs == nil || dependencies.Stepper == nil || dependencies.Gate == nil || dependencies.Executor == nil ||
		dependencies.Inspector == nil || dependencies.Steps == nil || dependencies.Contexts == nil || dependencies.Logger == nil {
		return nil, ErrInvalid
	}
	return &Loop{dependencies: dependencies, now: time.Now}, nil
}

// runEnd is a decided end of this claim: a status change to record, or nothing (zero value).
type runEnd struct {
	status   contracts.RunStatus
	reason   contracts.ReasonCode
	actionID string
}

// Handle runs model steps for the job's run until the run ends, waits, or the claim's step bound
// is reached. Before every model request it rereads the run and passport and stops a run that is
// cancelled, expired or out of agent steps. An error leaves the job for lease expiry.
func (loop *Loop) Handle(ctx context.Context, job worker.Job) (worker.Outcome, error) {
	runIdentity := policy.RunIdentity{OrganizationID: job.OrganizationID, RunID: job.RunID}
	for range maximumStepsPerClaim {
		if err := ctx.Err(); err != nil {
			return worker.Outcome{}, err
		}
		end, proceed, err := loop.step(ctx, runIdentity)
		if err != nil {
			return worker.Outcome{}, err
		}
		if end.status != "" {
			if err = loop.endRun(ctx, runIdentity, end); err != nil {
				return worker.Outcome{}, err
			}
		}
		if !proceed {
			return worker.Completed(), nil
		}
	}
	return worker.Requeue(0), nil
}

// step performs the checks and one model step. It returns the run end to record, whether the
// loop continues, or an error that leaves the job unchanged.
func (loop *Loop) step(ctx context.Context, run policy.RunIdentity) (runEnd, bool, error) {
	runs := loop.dependencies.Runs
	state, err := runs.RunState(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	switch state.Status {
	case contracts.RunQueued:
		// The first claim starts the run. Another writer (a cancellation) may win the race.
		if err = loop.transition(ctx, run, runEnd{status: contracts.RunRunning}); errors.Is(err, repository.ErrInvalidTransition) {
			return runEnd{}, true, nil
		} else if err != nil {
			return runEnd{}, false, err
		}
	case contracts.RunRunning:
	default:
		// Terminal, waiting for review or paused: this job has nothing to do.
		return runEnd{}, false, nil
	}

	passport, err := runs.Passport(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	if state.CancelRequestedAt != nil {
		return runEnd{status: contracts.RunStopped, reason: contracts.ReasonRunCancelled}, false, nil
	}
	if !loop.now().Before(passport.ExpiresAt) {
		return runEnd{status: contracts.RunStopped, reason: contracts.ReasonRunExpired}, false, nil
	}
	stepsTaken, err := loop.dependencies.Steps.CountAgentCalls(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	if int64(stepsTaken) >= passport.Limits.CallsAgent {
		// Alignment decision 7: an exhausted allowance pauses the run.
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonAllowanceExhausted}, false, nil
	}
	stepNumber := stepsTaken + 1

	entries, err := loop.dependencies.Contexts.List(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	agentRun := Run{OrganizationID: run.OrganizationID, RunID: run.RunID, AllowedModels: passport.Scope.AllowedModels}
	result, err := loop.dependencies.Stepper.Step(ctx, agentRun, buildTaskContext(passport, entries))
	if err != nil {
		if ctx.Err() != nil {
			return runEnd{}, false, ctx.Err()
		}
		return stepErrorEnd(err), false, nil
	}

	switch result.Kind {
	case StepRejected:
		// GO-01: the whole response is denied and recorded; no subset runs. Until GO-29's
		// bounded correction, the run stops with the reason.
		return runEnd{status: contracts.RunStopped, reason: result.RejectReason}, false, nil
	case StepFinal:
		// GO-26 adds the narrow final-result validation.
		return runEnd{status: contracts.RunCompleted}, false, nil
	case StepAction:
		return loop.act(ctx, run, agentRun, stepNumber, result.Proposal)
	default:
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}, false, nil
	}
}

// act sends one proposal through the gate, executes an allowed one, inspects its result and
// stores the step's context.
func (loop *Loop) act(ctx context.Context, run policy.RunIdentity, agentRun Run, stepNumber int, proposal contracts.ActionProposal) (runEnd, bool, error) {
	actionID, err := newUUID()
	if err != nil {
		return runEnd{}, false, err
	}
	idempotencyKey, err := newUUID()
	if err != nil {
		return runEnd{}, false, err
	}
	decision := loop.dependencies.Gate.Evaluate(ctx, run, policy.Proposal{
		ActionID: actionID, StepNumber: stepNumber, IdempotencyKey: idempotencyKey,
		Tool: string(proposal.Tool), RawArguments: proposal.Arguments,
	})
	switch decision.Outcome {
	case policy.OutcomeAllow:
	case policy.OutcomeApprovalRequired:
		// Figure 7: persist the wait; the worker releases the job. GO-40 resumes the stored action.
		return runEnd{status: contracts.RunAwaitingApproval, actionID: actionID}, false, nil
	default:
		// A denied action executes nothing. Until GO-29's correction feedback, the run stops.
		return runEnd{status: contracts.RunStopped, reason: contractReason(decision.ReasonCode)}, false, nil
	}

	execution := loop.dependencies.Executor.Execute(ctx, run, actionID)
	switch execution.Status {
	case policy.ExecutionSucceeded:
	case policy.ExecutionPaused:
		return runEnd{status: contracts.RunPaused, reason: contractReason(execution.ReasonCode)}, false, nil
	case policy.ExecutionRefused:
		return refusalEnd(contractReason(execution.ReasonCode)), false, nil
	default:
		// A known adapter refusal; GO-29 turns it into bounded correction feedback.
		return runEnd{status: contracts.RunStopped, reason: contractReason(execution.ReasonCode)}, false, nil
	}

	inspection, err := loop.dependencies.Inspector.Inspect(ctx, agentRun, string(proposal.Tool), execution.Result)
	if err != nil || inspection.Outcome == InspectionPause {
		reason := inspection.Reason
		if err != nil || !reason.Valid() {
			reason = contracts.ReasonSecurityEvaluatorUnavailable
		}
		return runEnd{status: contracts.RunPaused, reason: reason}, false, nil
	}
	if !validInspection(inspection) {
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonSecurityEvaluatorUnavailable}, false, nil
	}
	call, err := callEntry(proposal)
	if err != nil {
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}, false, nil
	}
	if err = loop.dependencies.Contexts.AppendToolStep(ctx, run.OrganizationID, run.RunID, stepNumber, actionID, call, inspection); err != nil {
		return runEnd{}, false, err
	}
	return runEnd{}, true, nil
}

// endRun records the run's status change with its event. A change another writer already made
// (for example a cancellation) is not an error: the run has ended either way.
func (loop *Loop) endRun(ctx context.Context, run policy.RunIdentity, end runEnd) error {
	// The end is written even when the claim's context ended: the decision is already made.
	writeContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := loop.transition(writeContext, run, end)
	if errors.Is(err, repository.ErrInvalidTransition) {
		loop.dependencies.Logger.Info("run already changed by another writer", "run_id", run.RunID, "wanted_status", string(end.status))
		return nil
	}
	return err
}

// transition applies one guarded run status change and its event in one transaction.
func (loop *Loop) transition(ctx context.Context, run policy.RunIdentity, end runEnd) error {
	runID := run.RunID
	event := repository.NewEvent{OrganizationID: run.OrganizationID, RunID: &runID, EventType: eventFor(end.status)}
	var reason *contracts.ReasonCode
	if end.status.NeedsReason() {
		reasonCode := end.reason
		if !reasonCode.Valid() {
			reasonCode = contracts.ReasonDecisionUnavailable
		}
		reason = &reasonCode
		event.ReasonCode = &reasonCode
	}
	if end.actionID != "" {
		actionID := end.actionID
		event.ActionID = &actionID
		decision := contracts.DecisionApprovalRequired
		event.Decision = &decision
	}
	return loop.dependencies.Runs.InTransaction(ctx, func(tx repository.Tx) error {
		_, err := tx.TransitionRun(ctx, repository.RunTransition{
			OrganizationID: run.OrganizationID, RunID: run.RunID, To: end.status, Reason: reason, Event: event,
		})
		return err
	})
}

// eventFor names the safe event of a run status change.
func eventFor(status contracts.RunStatus) contracts.EventType {
	switch status {
	case contracts.RunRunning:
		return contracts.EventRunStarted
	case contracts.RunAwaitingApproval:
		return contracts.EventApprovalRequested
	case contracts.RunPaused:
		return contracts.EventRunPaused
	case contracts.RunCompleted:
		return contracts.EventRunCompleted
	case contracts.RunFailed:
		return contracts.EventRunFailed
	default:
		return contracts.EventRunStopped
	}
}

// stepErrorEnd maps a failed model step to the run's end. Nothing here continues the run.
func stepErrorEnd(err error) runEnd {
	switch {
	case errors.Is(err, budget.ErrExhausted), errors.Is(err, budget.ErrPaused), errors.Is(err, model.ErrOverspend):
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonAllowanceExhausted}
	case errors.Is(err, model.ErrUsageUnknown), errors.Is(err, model.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		// Alignment decision 7: unresolved usage pauses the run.
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown}
	case errors.Is(err, ErrModelNotAllowed):
		return runEnd{status: contracts.RunStopped, reason: contracts.ReasonModelNotAllowed}
	default:
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}
	}
}

// refusalEnd maps an executor refusal (a fresh check failed before dispatch).
func refusalEnd(reason contracts.ReasonCode) runEnd {
	switch reason {
	case contracts.ReasonRunCancelled:
		return runEnd{status: contracts.RunStopped, reason: reason}
	case contracts.ReasonAllowanceExhausted:
		return runEnd{status: contracts.RunPaused, reason: reason}
	default:
		return runEnd{status: contracts.RunFailed, reason: reason}
	}
}

// contractReason converts a policy reason code, falling back to decision_unavailable for any value
// outside the X-13 vocabulary.
func contractReason(reason policy.ReasonCode) contracts.ReasonCode {
	converted := contracts.ReasonCode(reason)
	if !converted.Valid() {
		return contracts.ReasonDecisionUnavailable
	}
	return converted
}

// validInspection accepts only a released outcome with JSON content.
func validInspection(inspection Inspection) bool {
	switch inspection.Outcome {
	case InspectionPass, InspectionRedacted, InspectionBlocked:
		return len(inspection.Content) > 0 && json.Valid(inspection.Content)
	default:
		return false
	}
}

// callEntry is the stored form of an executed call: the tool and its canonical arguments.
func callEntry(proposal contracts.ActionProposal) (json.RawMessage, error) {
	arguments, err := policy.DecodeArguments(policy.ToolName(proposal.Tool), proposal.Arguments)
	if err != nil {
		return nil, err
	}
	canonical, err := policy.CanonicalArguments(arguments)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}{Tool: string(proposal.Tool), Arguments: canonical})
}

// buildTaskContext is the fixed task message from the passport's authorized parameters (opaque
// references only), followed by each stored step: the call and its inspected result.
func buildTaskContext(passport contracts.Passport, entries []ContextEntry) []model.Message {
	templates := make([]string, 0, len(passport.Scope.ReportTemplates))
	for _, template := range passport.Scope.ReportTemplates {
		templates = append(templates, string(template))
	}
	task := fmt.Sprintf("Task %s: reconcile the invoices %s of vendor %s and look for repeated external invoice references. "+
		"Permitted report templates: %s. Permitted recipient references: %s.",
		passport.TaskVersion, strings.Join(passport.Scope.InvoiceIDs, ", "), strings.Join(passport.Scope.VendorIDs, ", "),
		strings.Join(templates, ", "), strings.Join(passport.Scope.RecipientReferences, ", "))
	messages := []model.Message{{Role: "user", Content: task}}
	for _, entry := range entries {
		switch entry.Kind {
		case entryAssistantCall:
			var call struct {
				Tool      string          `json:"tool"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if json.Unmarshal(entry.Content, &call) != nil {
				continue
			}
			messages = append(messages, model.Message{Role: "assistant", ToolCalls: []model.ToolCall{
				{Function: model.FunctionCall{Name: call.Tool, Arguments: compactJSON(call.Arguments)}},
			}})
		case entryToolResult:
			messages = append(messages, model.Message{Role: "tool", Content: string(compactJSON(entry.Content))})
		}
	}
	return messages
}

// compactJSON removes the whitespace jsonb adds when it renders stored content; jsonb's key order
// is deterministic, so the same stored step always yields the same request bytes.
func compactJSON(raw json.RawMessage) json.RawMessage {
	var compacted bytes.Buffer
	if json.Compact(&compacted, raw) != nil {
		return raw
	}
	return compacted.Bytes()
}

// newUUID returns a random UUID v4 for an action identifier or idempotency key.
func newUUID() (string, error) {
	var identifier [16]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return "", err
	}
	identifier[6] = (identifier[6] & 0x0f) | 0x40
	identifier[8] = (identifier[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", identifier[0:4], identifier[4:6], identifier[6:8], identifier[8:10], identifier[10:16]), nil
}

var _ worker.Handler = (*Loop)(nil)
