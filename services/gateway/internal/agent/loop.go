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
	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/model"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/runresult"
	"starter/services/gateway/internal/security"
	"starter/services/gateway/internal/worker"
)

// maximumStepsPerClaim bounds one claim of a job; the passport's agent call limit is the real
// step limit. A run that is still going afterwards is requeued and claimed again.
const maximumStepsPerClaim = 64

// concurrencyRetryDelay is how long a job waits when its run holds all its model call slots.
const concurrencyRetryDelay = time.Second

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

// StepCounter counts the run's reserved agent model calls (budget.PostgresStore, the ledger).
type StepCounter interface {
	CountAgentCalls(ctx context.Context, organizationID, runID string) (int, error)
}

// CorrectionCounter counts the run's denials from its durable events (policy.CorrectionCounter).
type CorrectionCounter interface {
	CorrectionsUsed(ctx context.Context, run policy.RunIdentity) (int, error)
	// Feedback builds the fixed correction feedback for a denial from the stored run state (GO-29).
	Feedback(ctx context.Context, run policy.RunIdentity, decision policy.Decision, scope policy.PassportScope) (policy.DenialFeedback, error)
}

// CatalogSource returns the active control catalog snapshot (catalog.Loader.Active), read before
// every model request; it fails closed.
type CatalogSource interface {
	Active(ctx context.Context) (catalog.Snapshot, error)
}

// PoolCatalog reads the active snapshot through the shared loader on the gateway pool.
type PoolCatalog struct {
	Loader *catalog.Loader
	Pool   catalog.Querier
}

// Active returns the active snapshot or catalog.ErrUnavailable.
func (source PoolCatalog) Active(ctx context.Context) (catalog.Snapshot, error) {
	if source.Loader == nil || source.Pool == nil {
		return catalog.Snapshot{}, catalog.ErrUnavailable
	}
	return source.Loader.Active(ctx, source.Pool)
}

// FinalResultValidator checks a final answer (GO-26): the canonical result reference to persist, a
// rejection reason, or an error when the check could not run.
type FinalResultValidator interface {
	Validate(ctx context.Context, organizationID, runID, answer string) (string, contracts.ReasonCode, error)
}

// PoolFinalResults validates final answers with runresult.Validate on the gateway pool.
type PoolFinalResults struct{ Pool runresult.Querier }

// Validate checks the answer names only reports this run created.
func (results PoolFinalResults) Validate(ctx context.Context, organizationID, runID, answer string) (string, contracts.ReasonCode, error) {
	return runresult.Validate(ctx, results.Pool, organizationID, runID, answer)
}

// finalAnswerRejectedMessage is the fixed feedback after a rejected final answer: it restates the
// exact required format (lead's decision after live run 8b59f19f).
const finalAnswerRejectedMessage = "The final answer was not accepted. " + runresult.FinalAnswerInstruction +
	" Send no other text and no code fence."

// ContinuationReader finds the decided action a continuation resumes (policy.Approvals).
type ContinuationReader interface {
	DecidedActionFor(ctx context.Context, organizationID, runID string) (policy.DecidedAction, bool, error)
}

// correctionsExhaustedMessage is the fixed operator message when the correction limit stops a run.
const correctionsExhaustedMessage = "The task used up its corrections after repeated denials, so the run is stopped."

// rejectedActionMessage is the fixed feedback after a reviewer rejected the waited-for action.
const rejectedActionMessage = "The reviewer rejected this action; it was not run."

// LoopDependencies are the components the loop drives. Every one is required.
type LoopDependencies struct {
	Runs          *repository.Repository
	Stepper       ModelStepper
	Gate          ActionGate
	Executor      ActionExecutor
	Inspector     ResultInspector
	Catalog       CatalogSource
	Scopes        policy.ScopeReader
	Corrections   CorrectionCounter
	Steps         StepCounter
	Contexts      *ContextStore
	Telemetry     *Telemetry
	Recovery      *Recovery
	Results       FinalResultValidator
	Continuations ContinuationReader
	Logger        *slog.Logger
}

// Loop is the bounded agent loop: the handler of contracts.JobKindAgentStep jobs (GO-11).
type Loop struct {
	dependencies LoopDependencies
	now          func() time.Time
}

// NewLoop validates its dependencies.
func NewLoop(dependencies LoopDependencies) (*Loop, error) {
	if dependencies.Runs == nil || dependencies.Stepper == nil || dependencies.Gate == nil || dependencies.Executor == nil ||
		dependencies.Inspector == nil || dependencies.Catalog == nil || dependencies.Scopes == nil || dependencies.Corrections == nil ||
		dependencies.Steps == nil || dependencies.Contexts == nil || dependencies.Telemetry == nil || dependencies.Recovery == nil ||
		dependencies.Results == nil || dependencies.Continuations == nil || dependencies.Logger == nil {
		return nil, ErrInvalid
	}
	return &Loop{dependencies: dependencies, now: time.Now}, nil
}

// runEnd is a decided end of this claim: a status change to record, or nothing (zero value).
type runEnd struct {
	status   contracts.RunStatus
	reason   contracts.ReasonCode
	actionID string
	// message is the specific safe operator message of the actual failure (GO-58); empty uses
	// the reason's X-13 message.
	message string
	// purpose names the metered model purpose whose call ended the run, if one did.
	purpose string
	// resultReference is the validated final result stored with a completion (GO-26).
	resultReference *string
	// resumed marks the return from a review wait to running (run.resumed).
	resumed bool
}

// Handle runs model steps for the job's run until the run ends, waits, or the claim's step bound
// is reached. Before every model request it rereads the run and passport and stops a run that is
// cancelled, expired or out of agent steps. An error leaves the job for lease expiry.
func (loop *Loop) Handle(ctx context.Context, job worker.Job) (worker.Outcome, error) {
	runIdentity := policy.RunIdentity{OrganizationID: job.OrganizationID, RunID: job.RunID}
	// GO-49: a running run may carry what a former claim left behind; reconcile it first.
	state, err := loop.dependencies.Runs.RunState(ctx, runIdentity.OrganizationID, runIdentity.RunID)
	if err != nil {
		return worker.Outcome{}, err
	}
	if state.Status == contracts.RunRunning {
		recoveryContext, cancelRecovery := context.WithTimeout(ctx, recoveryTimeout)
		end, recoverErr := loop.recoverClaim(recoveryContext, runIdentity)
		cancelRecovery()
		if recoverErr != nil {
			return worker.Outcome{}, recoverErr
		}
		if end.status != "" {
			if err = loop.endRun(ctx, runIdentity, end); err != nil {
				return worker.Outcome{}, err
			}
			return worker.Completed(), nil
		}
	}
	for range maximumStepsPerClaim {
		if err := ctx.Err(); err != nil {
			return worker.Outcome{}, err
		}
		stepStarted := time.Now()
		end, proceed, err := loop.step(ctx, runIdentity)
		loop.recordSpans(ctx, runIdentity, Span{Phase: PhaseTotal, Duration: time.Since(stepStarted), Failed: err != nil})
		if errors.Is(err, budget.ErrConcurrencyLimit) {
			// The run holds all its call slots (a call with unresolved usage): try again shortly.
			return worker.Requeue(concurrencyRetryDelay), nil
		}
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
	lookupStarted := time.Now()
	state, err := runs.RunState(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	passport, err := runs.Passport(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	switch state.Status {
	case contracts.RunQueued:
		// The first claim starts the run. Another writer (a cancellation) may win the race.
		if err = loop.transition(ctx, run, runEnd{status: contracts.RunRunning}); errors.Is(err, repository.ErrInvalidTransition) {
			return runEnd{}, true, nil
		} else if errors.Is(err, repository.ErrCancelRequested) {
			return cancelledEnd, false, nil
		} else if err != nil {
			return runEnd{}, false, err
		}
	case contracts.RunRunning:
	case contracts.RunAwaitingApproval:
		// GO-40: a continuation resumes the run once the waited-for action has been decided; until
		// then the run keeps waiting and no worker holds it.
		decided, found, err := loop.dependencies.Continuations.DecidedActionFor(ctx, run.OrganizationID, run.RunID)
		if err != nil {
			return runEnd{}, false, err
		}
		if !found {
			return runEnd{}, false, nil
		}
		// A cancelled or expired run stops from the wait without a run.resumed event.
		if end, stopped := loop.stopBeforeWork(state, passport); stopped {
			return end, false, nil
		}
		if err = loop.transition(ctx, run, runEnd{status: contracts.RunRunning, actionID: decided.ActionID, resumed: true}); errors.Is(err, repository.ErrInvalidTransition) {
			return runEnd{}, true, nil
		} else if errors.Is(err, repository.ErrCancelRequested) {
			return cancelledEnd, false, nil
		} else if err != nil {
			return runEnd{}, false, err
		}
		// GO-80: the review wait lasted from the awaiting transition (the run's last update, since
		// nothing else changes a waiting run) to this resume.
		loop.recordSpans(ctx, run, Span{Phase: PhaseApprovalWait, Duration: max(loop.now().Sub(state.UpdatedAt), 0), ActionID: decided.ActionID})
	default:
		// Terminal or paused: this job has nothing to do.
		return runEnd{}, false, nil
	}

	if end, stopped := loop.stopBeforeWork(state, passport); stopped {
		return end, false, nil
	}
	// GO-72: the active catalog narrows the immutable passport before every model request. A
	// missing or invalid catalog dispatches nothing and leaves the job for a later claim.
	snapshot, err := loop.dependencies.Catalog.Active(ctx)
	if err != nil {
		return runEnd{}, false, err
	}
	effective := catalog.EffectiveFor(passport, snapshot)
	// GO-40: a decided, waited-for action is handled before any new model request: the approved
	// stored action executes (never a new proposal), a rejected or expired one is a counted denial.
	if handled, end, proceed, err := loop.continueDecidedAction(ctx, run, passport, effective, snapshot.Security); handled {
		return end, proceed, err
	}
	stepsTaken, err := loop.dependencies.Steps.CountAgentCalls(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	if int64(stepsTaken) >= effective.CallsAgent {
		// Alignment decision 7: an exhausted allowance pauses the run.
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonAllowanceExhausted}, false, nil
	}
	stepNumber := stepsTaken + 1
	loop.recordSpans(ctx, run, Span{Phase: PhasePolicyLookup, Duration: time.Since(lookupStarted)})

	entries, err := loop.dependencies.Contexts.List(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	agentRun := Run{OrganizationID: run.OrganizationID, RunID: run.RunID, AllowedModels: effective.AllowedModels}
	// A cancel that landed during the checks above dispatches no model request.
	if end, stopped, err := loop.cancelledSince(ctx, run); err != nil || stopped {
		return end, false, err
	}
	result, err := loop.dependencies.Stepper.Step(ctx, agentRun, buildTaskContext(passport, entries))
	if result.CallID != "" {
		loop.recordSpans(ctx, run, Span{Phase: PhaseProvider, Duration: result.Duration, Failed: err != nil, ModelCallID: result.CallID})
	}
	if err != nil {
		if ctx.Err() != nil {
			return runEnd{}, false, ctx.Err()
		}
		if errors.Is(err, budget.ErrConcurrencyLimit) {
			return runEnd{}, false, err
		}
		return stepErrorEnd(err), false, nil
	}
	// A cancel that landed during the model request: its result is not acted on, a final answer
	// does not complete the run.
	if end, stopped, err := loop.cancelledSince(ctx, run); err != nil || stopped {
		return end, false, err
	}

	switch result.Kind {
	case StepRejected:
		// GO-01: the whole response is denied and recorded as a denial event; no subset runs.
		// It counts as a correction, like any other denial.
		if err = loop.recordRejection(ctx, run, result.RejectReason, "", ""); err != nil {
			return runEnd{}, false, err
		}
		rejection := policy.Decision{Outcome: policy.OutcomeDeny, ReasonCode: policy.ReasonCode(result.RejectReason)}
		return loop.correct(ctx, run, effective, stepNumber, nil, rejection, "")
	case StepFinal:
		// GO-26: only a final answer naming reports this run created completes the run, and the
		// validated reference is stored with the completion.
		reference, reason, err := loop.dependencies.Results.Validate(ctx, run.OrganizationID, run.RunID, result.FinalAnswer)
		if err != nil {
			return runEnd{status: contracts.RunPaused, reason: contracts.ReasonDecisionUnavailable}, false, nil
		}
		if reason != "" {
			// A rejected final answer is a denial like any other: recorded and counted as a correction,
			// with its fixed cause kind (never the text) so a failure can be diagnosed.
			if err = loop.recordRejection(ctx, run, reason, "", finalAnswerCause(result.FinalAnswer, reason)); err != nil {
				return runEnd{}, false, err
			}
			rejection := policy.Decision{Outcome: policy.OutcomeDeny, ReasonCode: policy.ReasonCode(reason)}
			return loop.correct(ctx, run, effective, stepNumber, nil, rejection, finalAnswerRejectedMessage)
		}
		return runEnd{status: contracts.RunCompleted, resultReference: &reference}, false, nil
	case StepAction:
		return loop.act(ctx, run, agentRun, passport, effective, snapshot.Security, stepNumber, result.Proposal)
	default:
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}, false, nil
	}
}

// act sends one proposal through the gate, executes an allowed one, inspects its result and
// stores the step's context.
func (loop *Loop) act(ctx context.Context, run policy.RunIdentity, agentRun Run, passport contracts.Passport, effective catalog.Effective,
	settings security.Settings, stepNumber int, proposal contracts.ActionProposal) (runEnd, bool, error) {
	actionID, err := newUUID()
	if err != nil {
		return runEnd{}, false, err
	}
	idempotencyKey, err := newUUID()
	if err != nil {
		return runEnd{}, false, err
	}
	gateStarted := time.Now()
	decision := loop.dependencies.Gate.Evaluate(ctx, run, policy.Proposal{
		ActionID: actionID, StepNumber: stepNumber, IdempotencyKey: idempotencyKey,
		Tool: string(proposal.Tool), RawArguments: proposal.Arguments,
	})
	gateSpan := Span{Phase: PhaseDeterministic, Duration: time.Since(gateStarted)}
	if decision.ActionStored {
		gateSpan.ActionID = actionID
	}
	loop.recordSpans(ctx, run, gateSpan)
	switch decision.Outcome {
	case policy.OutcomeAllow:
	case policy.OutcomeApprovalRequired:
		// Figure 7: persist the wait; the worker releases the job. GO-40 resumes the stored action.
		return runEnd{status: contracts.RunAwaitingApproval, actionID: actionID}, false, nil
	default:
		// A denied action executes nothing (GO-29: bounded feedback while corrections remain).
		return loop.correct(ctx, run, effective, stepNumber, proposedCall(proposal), decision, "")
	}

	return loop.executeAllowed(ctx, run, agentRun, passport, effective, settings, stepNumber, actionID, proposal)
}

// executeAllowed runs an allowed or approved stored action through the executor, inspects its
// result and stores the step's context.
func (loop *Loop) executeAllowed(ctx context.Context, run policy.RunIdentity, agentRun Run, passport contracts.Passport,
	effective catalog.Effective, settings security.Settings, stepNumber int, actionID string, proposal contracts.ActionProposal) (runEnd, bool, error) {
	executionStarted := time.Now()
	execution := loop.dependencies.Executor.Execute(ctx, run, actionID)
	loop.recordSpans(ctx, run, Span{Phase: PhaseCommit, Duration: time.Since(executionStarted),
		Failed: execution.Status != policy.ExecutionSucceeded, ActionID: actionID})
	switch execution.Status {
	case policy.ExecutionSucceeded:
	case policy.ExecutionPaused:
		return runEnd{status: contracts.RunPaused, reason: contractReason(execution.ReasonCode)}, false, nil
	case policy.ExecutionRefused:
		reason := contractReason(execution.ReasonCode)
		if !correctableRefusal(reason) {
			return refusalEnd(reason), false, nil
		}
		// GO-45/GO-29: an action whose fresh check failed (an expired grant, a changed record, action
		// or source policy) executes nothing and is a counted denial with bounded feedback.
		if err := loop.recordRejection(ctx, run, reason, actionID, ""); err != nil {
			return runEnd{}, false, err
		}
		denial := policy.Decision{Outcome: policy.OutcomeDeny, ReasonCode: policy.ReasonCode(reason), ActionID: actionID}
		return loop.correct(ctx, run, effective, stepNumber, proposedCall(proposal), denial, "")
	default:
		// A known adapter refusal; GO-29 turns it into bounded correction feedback.
		return runEnd{status: contracts.RunStopped, reason: contractReason(execution.ReasonCode)}, false, nil
	}

	inspection, err := loop.dependencies.Inspector.Inspect(ctx, agentRun, string(proposal.Tool), execution.Result, settings)
	evaluationID, idErr := newUUID()
	if idErr != nil {
		return runEnd{}, false, idErr
	}
	if err != nil || inspection.Outcome == InspectionPause {
		// Nothing is released; the decisions and timing are still recorded as evidence.
		if recordErr := loop.dependencies.Telemetry.RecordInspection(ctx, run.OrganizationID, run.RunID, actionID,
			passport.AdmissionCatalogRevisionID, evaluationID, inspection.Evidence); recordErr != nil {
			loop.dependencies.Logger.Warn("inspection evidence not recorded", "run_id", run.RunID, "error", recordErr.Error())
		}
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
	if err = loop.dependencies.Contexts.AppendToolStep(ctx, run.OrganizationID, run.RunID, stepNumber, actionID, call, inspection,
		passport.AdmissionCatalogRevisionID, evaluationID); err != nil {
		return runEnd{}, false, err
	}
	return runEnd{}, true, nil
}

// correct applies the passport's correction limit after a denial (GO-29). The count comes from
// the run's durable denial events, so it survives a restart. While corrections remain, the
// denied call (if any) and the fixed denial feedback join the context and the loop continues;
// a permitted alternative in the feedback grants nothing, the next proposal is checked again.
func (loop *Loop) correct(ctx context.Context, run policy.RunIdentity, effective catalog.Effective, stepNumber int,
	call json.RawMessage, decision policy.Decision, message string) (runEnd, bool, error) {
	denialReason := contractReason(decision.ReasonCode)
	used, err := loop.dependencies.Corrections.CorrectionsUsed(ctx, run)
	if err != nil {
		return runEnd{status: contracts.RunStopped, reason: denialReason}, false, nil
	}
	verdict := policy.CheckCorrections(used, int(effective.Corrections))
	if !verdict.Continue {
		return runEnd{status: contracts.RunStopped, reason: contractReason(verdict.StopReason), message: correctionsExhaustedMessage}, false, nil
	}
	scope, err := loop.dependencies.Scopes.LoadScope(ctx, run)
	if err != nil {
		return runEnd{status: contracts.RunStopped, reason: denialReason}, false, nil
	}
	denialFeedback, err := loop.dependencies.Corrections.Feedback(ctx, run, decision, scope)
	if err != nil {
		return runEnd{status: contracts.RunStopped, reason: denialReason}, false, nil
	}
	if message != "" {
		denialFeedback = policy.DenialFeedback{ReasonCode: decision.ReasonCode, SafeMessage: message}
	}
	feedback, err := json.Marshal(denialFeedback)
	if err != nil {
		return runEnd{status: contracts.RunStopped, reason: denialReason}, false, nil
	}
	if err = loop.dependencies.Contexts.AppendCorrection(ctx, run.OrganizationID, run.RunID, stepNumber, call, feedback, denialReason); err != nil {
		return runEnd{}, false, err
	}
	return runEnd{}, true, nil
}

// recordRejection writes a denial event the correction counter counts: a whole rejected response or
// final answer (no action), or a waited-for action a reviewer rejected or that expired.
func (loop *Loop) recordRejection(ctx context.Context, run policy.RunIdentity, reason contracts.ReasonCode, actionID, rejectionCause string) error {
	runID := run.RunID
	decision := contracts.DecisionDeny
	if !reason.Valid() {
		reason = contracts.ReasonDecisionUnavailable
	}
	event := repository.NewEvent{OrganizationID: run.OrganizationID, RunID: &runID, EventType: contracts.EventActionDenied,
		Decision: &decision, ReasonCode: &reason}
	if actionID != "" {
		event.ActionID = &actionID
	}
	if rejectionCause != "" {
		event.MaskedSummary.RejectionCause = &rejectionCause
	}
	return loop.dependencies.Runs.InTransaction(ctx, func(tx repository.Tx) error {
		_, err := tx.AppendEvent(ctx, event)
		return err
	})
}

// continueDecidedAction handles the waited-for action of a resumed run once (GO-40). It reports
// whether it handled one; a step already in the context was handled before.
func (loop *Loop) continueDecidedAction(ctx context.Context, run policy.RunIdentity, passport contracts.Passport, effective catalog.Effective,
	settings security.Settings) (bool, runEnd, bool, error) {
	decided, found, err := loop.dependencies.Continuations.DecidedActionFor(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return true, runEnd{}, false, err
	}
	if !found {
		return false, runEnd{}, false, nil
	}
	handled, err := loop.dependencies.Contexts.HasStep(ctx, run.OrganizationID, run.RunID, decided.StepNumber)
	if err != nil {
		return true, runEnd{}, false, err
	}
	if handled {
		return false, runEnd{}, false, nil
	}
	proposal := contracts.ActionProposal{Tool: contracts.ToolName(decided.Tool), Arguments: decided.CanonicalArguments}
	if decided.Status == contracts.ActionApproved && decided.GrantOpen {
		agentRun := Run{OrganizationID: run.OrganizationID, RunID: run.RunID, AllowedModels: effective.AllowedModels}
		end, proceed, err := loop.executeAllowed(ctx, run, agentRun, passport, effective, settings, decided.StepNumber, decided.ActionID, proposal)
		return true, end, proceed, err
	}
	reason, message := contracts.ReasonApprovalExpired, ""
	if decided.Status == contracts.ActionRejected {
		reason, message = contracts.ReasonApprovalRejected, rejectedActionMessage
	}
	if err = loop.recordRejection(ctx, run, reason, decided.ActionID, ""); err != nil {
		return true, runEnd{}, false, err
	}
	denial := policy.Decision{Outcome: policy.OutcomeDeny, ReasonCode: policy.ReasonCode(reason), ActionID: decided.ActionID}
	end, proceed, err := loop.correct(ctx, run, effective, decided.StepNumber, proposedCall(proposal), denial, message)
	return true, end, proceed, err
}

// proposedCall is the stored form of a denied call as the model sent it: the tool and its
// arguments when they are a small JSON object, otherwise an empty object. It is model output shown
// back to the model, never authority.
func proposedCall(proposal contracts.ActionProposal) json.RawMessage {
	arguments := json.RawMessage(`{}`)
	trimmed := bytes.TrimSpace(proposal.Arguments)
	if len(trimmed) > 0 && len(trimmed) <= 4096 && trimmed[0] == '{' && json.Valid(trimmed) {
		arguments = trimmed
	}
	tool := string(proposal.Tool)
	if len(tool) == 0 || len(tool) > 64 {
		tool = "unknown_tool"
	}
	encoded, err := json.Marshal(struct {
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}{Tool: tool, Arguments: arguments})
	if err != nil {
		return nil
	}
	return encoded
}

// recordSpans writes timing best effort: telemetry is evidence, never a reason to stop or retry a
// run, so a failed write is logged and the step goes on.
func (loop *Loop) recordSpans(ctx context.Context, run policy.RunIdentity, spans ...Span) {
	writeContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := loop.dependencies.Telemetry.RecordSpans(writeContext, run.OrganizationID, run.RunID, spans); err != nil {
		loop.dependencies.Logger.Warn("timing not recorded", "run_id", run.RunID, "error", err.Error())
	}
}

// endRun records the run's status change with its event. A change another writer already made
// (for example a cancellation) is not an error: the run has ended either way.
func (loop *Loop) endRun(ctx context.Context, run policy.RunIdentity, end runEnd) error {
	// The end is written even when the claim's context ended: the decision is already made.
	writeContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := loop.transition(writeContext, run, end)
	if errors.Is(err, repository.ErrCancelRequested) {
		// A cancel landed while the step ran: the run stops instead of pausing, waiting or completing.
		err = loop.transition(writeContext, run, cancelledEnd)
	}
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
	if end.resumed {
		event.EventType = contracts.EventRunResumed
	}
	var reason *contracts.ReasonCode
	if end.status.NeedsReason() {
		reasonCode := end.reason
		if !reasonCode.Valid() {
			reasonCode = contracts.ReasonDecisionUnavailable
		}
		reason = &reasonCode
		event.ReasonCode = &reasonCode
		// Every stop, pause and failure says what happened in fixed, safe text (GO-58).
		message := end.message
		if message == "" {
			message = reasonCode.SafeMessage()
		}
		event.MaskedSummary.SafeMessage = &message
	}
	if end.purpose != "" {
		purpose := end.purpose
		event.MaskedSummary.Purpose = &purpose
	}
	if end.actionID != "" {
		// The run's own status event names the action it waits for; approval.requested is the gate's.
		actionID := end.actionID
		event.ActionID = &actionID
	}
	return loop.dependencies.Runs.InTransaction(ctx, func(tx repository.Tx) error {
		_, err := tx.TransitionRun(ctx, repository.RunTransition{
			OrganizationID: run.OrganizationID, RunID: run.RunID, To: end.status, Reason: reason, Event: event,
			ResultReference: end.resultReference,
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
		return contracts.EventRunAwaitingApproval
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

// Safe operator messages of the model failures, so a run-end event records which failure it was
// (GO-58: "On provider failure, show the actual failure state").
const (
	messageModelUnreachable  = "The local model could not be reached; whether the request used tokens is unknown, so its allowance stays held and the run is paused."
	messageModelBadResponse  = "The local model's response could not be read; whether it used tokens is unknown, so its allowance stays held and the run is paused."
	messageModelUsageUnknown = "The local model call failed or returned no usage counts; whether it used tokens is unknown, so its allowance stays held and the run is paused."
	messageModelTimeout      = "The local model did not answer in time; whether it used tokens is unknown, so its allowance stays held and the run is paused."
	messageModelUnusable     = "The local model answered with neither one action nor a final answer, so the run failed."
	messageModelNotRecorded  = "The model call could not be recorded, so the run failed."
	messageModelCallFailed   = "The model call failed before its answer could be used, so the run failed."
)

// stepErrorEnd maps a failed model step to the run's end. Nothing here continues the run.
func stepErrorEnd(err error) runEnd {
	agentPurpose := string(model.AgentPurpose)
	switch {
	case errors.Is(err, budget.ErrExhausted), errors.Is(err, budget.ErrPaused), errors.Is(err, model.ErrOverspend):
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonAllowanceExhausted, purpose: agentPurpose}
	case errors.Is(err, model.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		// Alignment decision 7: unresolved usage pauses the run.
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown, message: messageModelTimeout, purpose: agentPurpose}
	case errors.Is(err, model.ErrTransport):
		// The accounted call joins the cause to ErrUsageUnknown and keeps the reservation.
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown, message: messageModelUnreachable, purpose: agentPurpose}
	case errors.Is(err, model.ErrResponse):
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown, message: messageModelBadResponse, purpose: agentPurpose}
	case errors.Is(err, model.ErrUsageUnknown):
		return runEnd{status: contracts.RunPaused, reason: contracts.ReasonOutcomeUnknown, message: messageModelUsageUnknown, purpose: agentPurpose}
	case errors.Is(err, ErrModelNotAllowed):
		return runEnd{status: contracts.RunStopped, reason: contracts.ReasonModelNotAllowed}
	case errors.Is(err, ErrUnusableResponse):
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable, message: messageModelUnusable, purpose: agentPurpose}
	case errors.Is(err, ErrRecording):
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable, message: messageModelNotRecorded, purpose: agentPurpose}
	case errors.Is(err, ErrModelCallFailed):
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable, message: messageModelCallFailed, purpose: agentPurpose}
	default:
		return runEnd{status: contracts.RunFailed, reason: contracts.ReasonDecisionUnavailable}
	}
}

// finalAnswerCause is the fixed rejection cause of a final answer: runresult.Cause for a format
// failure, unknown_report when the answer named a report this run did not create.
func finalAnswerCause(answer string, reason contracts.ReasonCode) string {
	if cause := runresult.Cause(answer); cause != "" {
		return cause
	}
	if reason == contracts.ReasonResourceOutOfScope {
		return runresult.CauseUnknownReport
	}
	return ""
}

// correctableRefusal reports whether an executor refusal is a policy outcome about the action
// itself, which the model can correct, rather than a run-level stop or an unavailable dependency.
func correctableRefusal(reason contracts.ReasonCode) bool {
	switch reason {
	case contracts.ReasonApprovalExpired, contracts.ReasonApprovalRequired, contracts.ReasonActionChanged,
		contracts.ReasonResourceVersionChanged, contracts.ReasonSourcePolicyChanged, contracts.ReasonResourceOutOfScope,
		contracts.ReasonDestinationNotAllowed, contracts.ReasonReportExportRestricted, contracts.ReasonTemplateNotAllowed,
		contracts.ReasonToolNotAllowed:
		return true
	default:
		return false
	}
}

// cancelledEnd stops a run whose cancel was requested.
var cancelledEnd = runEnd{status: contracts.RunStopped, reason: contracts.ReasonRunCancelled}

// stopBeforeWork stops a run whose cancel was requested or whose passport expired.
func (loop *Loop) stopBeforeWork(state contracts.RunState, passport contracts.Passport) (runEnd, bool) {
	if state.CancelRequestedAt != nil {
		return cancelledEnd, true
	}
	if !loop.now().Before(passport.ExpiresAt) {
		return runEnd{status: contracts.RunStopped, reason: contracts.ReasonRunExpired}, true
	}
	return runEnd{}, false
}

// cancelledSince re-reads the run: a cancel stamped since the step began stops it, and a run another
// writer already moved out of running ends this claim without a change.
func (loop *Loop) cancelledSince(ctx context.Context, run policy.RunIdentity) (runEnd, bool, error) {
	state, err := loop.dependencies.Runs.RunState(ctx, run.OrganizationID, run.RunID)
	if err != nil {
		return runEnd{}, false, err
	}
	if state.CancelRequestedAt != nil {
		return cancelledEnd, true, nil
	}
	if state.Status != contracts.RunRunning {
		return runEnd{}, true, nil
	}
	return runEnd{}, false, nil
}

// refusalEnd maps an executor refusal (a fresh check failed before dispatch).
func refusalEnd(reason contracts.ReasonCode) runEnd {
	switch reason {
	case contracts.ReasonRunCancelled, contracts.ReasonRunExpired:
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
	// The business task as the report's storyboard describes it (beats 3 to 8): an internal
	// investigation first, then what the vendor needs. It names no control and no report to send.
	task := fmt.Sprintf("Task %s for vendor %s. The finance team suspects a duplicate charge on the invoices %s. "+
		"First investigate internally: read each invoice, including any authorized internal note, find repeated external "+
		"invoice references, and record your findings in an internal investigation report. "+
		"Then send the vendor (recipient reference %s) what they need to reconcile the duplicate on their side. "+
		"Use each recipient_reference exactly as read_vendor returns it, character for character. "+
		"Registered report templates: %s.",
		passport.TaskVersion, strings.Join(passport.Scope.VendorIDs, ", "), strings.Join(passport.Scope.InvoiceIDs, ", "),
		strings.Join(passport.Scope.RecipientReferences, ", "), strings.Join(templates, ", "))
	messages := []model.Message{{Role: "user", Content: task}}
	previousCallStep := 0
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
			previousCallStep = entry.StepNumber
		case entryToolResult:
			messages = append(messages, model.Message{Role: "tool", Content: string(compactJSON(entry.Content))})
		case entryCorrection:
			// The answer to the denied call of the same step, or, after a rejected response that
			// had no single call, a message to the model.
			role := "user"
			if previousCallStep == entry.StepNumber {
				role = "tool"
			}
			messages = append(messages, model.Message{Role: role, Content: string(compactJSON(entry.Content))})
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
