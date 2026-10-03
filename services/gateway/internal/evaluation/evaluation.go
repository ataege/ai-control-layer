// Package evaluation is the control evaluation adapter (GO-82, X-91): one interaction of an
// admitted run, evaluated at one boundary through the same controls the agent path uses. It never
// grants anything: evaluated actions are decisions only, so nothing is stored as an action or
// executed, and the call never dispatches the agent model. The evidence (an X-12
// control.evaluated event and the control records) commits in one transaction.
package evaluation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"starter/services/gateway/internal/catalog"
	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/policy"
	"starter/services/gateway/internal/repository"
	"starter/services/gateway/internal/security"
)

var (
	// ErrInvalid means the request does not fit X-91.
	ErrInvalid = errors.New("invalid control evaluation request")
	// ErrNotFound means the run does not exist in the operator's organization.
	ErrNotFound = errors.New("run not found")
	// ErrUnavailable means no decision could be made or its evidence could not be stored.
	ErrUnavailable = errors.New("control evaluation unavailable")
)

// judgeSource names the untrusted text an evaluation inspects; it is not a stored record.
const judgeSource = "judge_input"

// SemanticChecker is the metered semantic evaluator; *security.SemanticEvaluator implements it.
type SemanticChecker interface {
	Evaluate(ctx context.Context, runID string, boundary security.Boundary, field security.Field, settings security.Settings) (security.SemanticResult, error)
}

// ToolResultInspector is the tool-result pipeline; *security.Inspector implements it.
type ToolResultInspector interface {
	InspectToolResult(ctx context.Context, input security.ToolResultInput, settings security.Settings) (security.ToolResultInspection, error)
}

// ActionGate decides one proposal; *policy.Gate implements it.
type ActionGate interface {
	Evaluate(ctx context.Context, run policy.RunIdentity, proposal policy.Proposal) policy.Decision
}

// GateFactory builds the agent path's gate (same scopes, relationships and security evaluator)
// with the given recorder and freezer, so an evaluation can inject ones that store nothing.
type GateFactory func(recorder policy.ActionRecorder, freezer policy.ReviewFreezer) ActionGate

// Dependencies are the shared components of the agent path the evaluation reuses. Nil semantic,
// inspector or gate components make the boundaries that need them fail closed.
type Dependencies struct {
	Repository *repository.Repository
	Catalog    *catalog.Loader
	// Database reads the active catalog snapshot; the gateway pool in production.
	Database  catalog.Querier
	Semantic  SemanticChecker
	Inspector ToolResultInspector
	Gates     GateFactory
}

// Evaluator runs evaluations.
type Evaluator struct {
	dependencies Dependencies
	now          func() time.Time
}

// New returns an evaluator over the agent path's components.
func New(dependencies Dependencies) *Evaluator {
	return &Evaluator{dependencies: dependencies, now: time.Now}
}

// outcome is one boundary's result before it is recorded.
type outcome struct {
	decision            contracts.EvaluationDecision
	reason              contracts.ReasonCode
	records             []security.ControlRecord
	redactedText        *string
	alternativeTemplate *contracts.ReportTemplate
}

func deny(reason contracts.ReasonCode, records []security.ControlRecord) outcome {
	return outcome{decision: contracts.EvaluationDeny, reason: reason, records: records}
}

// Evaluate checks one interaction for the operator's run and records the evidence.
func (evaluator *Evaluator) Evaluate(ctx context.Context, operator contracts.OperatorContext, request contracts.ControlEvaluationRequest) (contracts.ControlEvaluationResponse, error) {
	dependencies := evaluator.dependencies
	if ctx == nil || dependencies.Repository == nil || dependencies.Catalog == nil || dependencies.Database == nil {
		return contracts.ControlEvaluationResponse{}, ErrUnavailable
	}
	if err := validateRequest(request); err != nil {
		return contracts.ControlEvaluationResponse{}, err
	}
	passport, err := dependencies.Repository.Passport(ctx, operator.OrganizationID, request.RunID)
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrInvalid) {
		return contracts.ControlEvaluationResponse{}, ErrNotFound
	}
	if err != nil {
		return contracts.ControlEvaluationResponse{}, ErrUnavailable
	}
	runState, err := dependencies.Repository.RunState(ctx, operator.OrganizationID, request.RunID)
	if err != nil {
		return contracts.ControlEvaluationResponse{}, ErrUnavailable
	}
	snapshot, err := dependencies.Catalog.Active(ctx, dependencies.Database)
	if err != nil {
		return contracts.ControlEvaluationResponse{}, ErrUnavailable
	}

	// An inactive run spends nothing: it is denied before any metered model call, for every kind.
	var result outcome
	if reason, inactive := inactiveRunReason(runState, passport, evaluator.now()); inactive {
		result = deny(reason, nil)
	} else {
		result = evaluator.evaluateKind(ctx, operator.OrganizationID, request, snapshot.Security)
	}

	evaluationID := newUUID()
	response := buildResponse(evaluationID, request.RunID, passport.AdmissionCatalogRevisionID, snapshot, result)
	if err := evaluator.recordEvidence(ctx, operator, request.RunID, evaluationID,
		passport.AdmissionCatalogRevisionID, snapshot.RevisionID, result, response.SafeMessage); err != nil {
		// The judge's input must appear in evidence; without it there is no decision to report.
		return contracts.ControlEvaluationResponse{}, ErrUnavailable
	}
	return response, nil
}

// inactiveRunReason denies an evaluation of a cancelled, expired or finished run.
func inactiveRunReason(state contracts.RunState, passport contracts.Passport, now time.Time) (contracts.ReasonCode, bool) {
	switch {
	case state.CancelRequestedAt != nil || (state.TerminalReason != nil && *state.TerminalReason == contracts.ReasonRunCancelled):
		return contracts.ReasonRunCancelled, true
	case state.Status == contracts.RunCompleted || state.Status == contracts.RunFailed || state.Status == contracts.RunStopped:
		return contracts.ReasonRunNotActive, true
	case !now.Before(passport.ExpiresAt):
		return contracts.ReasonRunExpired, true
	}
	return "", false
}

// evaluateKind runs the boundary the request names.
func (evaluator *Evaluator) evaluateKind(ctx context.Context, organizationID string, request contracts.ControlEvaluationRequest, settings security.Settings) outcome {
	var result outcome
	switch request.Kind {
	case contracts.BoundaryModelInput:
		result = evaluator.modelInput(ctx, request.RunID, *request.Text, settings)
	case contracts.BoundaryToolResult:
		result = evaluator.toolResult(ctx, request.RunID, string(*request.Tool), *request.Text, settings)
	case contracts.BoundaryActionProposal:
		result = evaluator.actionProposal(ctx, organizationID, request)
	}
	return result
}

// validateRequest enforces X-91's combinations: text for model_input and tool_result, a registered
// tool for tool_result and action_proposal, arguments only for action_proposal.
func validateRequest(request contracts.ControlEvaluationRequest) error {
	hasArguments := len(request.Arguments) > 0 && string(request.Arguments) != "null"
	validText := request.Text != nil && *request.Text != "" && len(*request.Text) <= security.MaxFieldBytes && utf8.ValidString(*request.Text)
	switch request.Kind {
	case contracts.BoundaryModelInput:
		if !validText || request.Tool != nil || hasArguments {
			return ErrInvalid
		}
	case contracts.BoundaryToolResult:
		if !validText || request.Tool == nil || !request.Tool.Valid() || hasArguments {
			return ErrInvalid
		}
	case contracts.BoundaryActionProposal:
		if request.Text != nil || request.Tool == nil || !request.Tool.Valid() || !hasArguments {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

// modelInput applies the content rules, the signatures and the semantic check to untrusted text
// at the model_input boundary. It never dispatches the agent model.
func (evaluator *Evaluator) modelInput(ctx context.Context, runID, text string, settings security.Settings) outcome {
	field := security.Field{Name: security.FieldModelInputText, Text: text, Source: security.SourceRef{SourceID: judgeSource}}
	content, err := security.ApplyContentRules(field, security.BoundaryModelInput, settings)
	records := []security.ControlRecord{content.Record}
	if err != nil || content.Record.Outcome == security.OutcomeError {
		return deny(contracts.ReasonDecisionUnavailable, records)
	}
	if !content.Permitted() {
		return deny(reasonOf(content.Record.ReasonCode, contracts.ReasonContentBlocked), records)
	}
	// Signatures match the original text, as on the tool-result path.
	signatures, err := security.MatchSignatures(text, security.BoundaryModelInput, security.FieldModelInputText, settings)
	records = append(records, signatures)
	if err != nil || signatures.Outcome == security.OutcomeError {
		return deny(contracts.ReasonDecisionUnavailable, records)
	}
	if signatures.Outcome == security.OutcomeBlock {
		return deny(reasonOf(signatures.ReasonCode, contracts.ReasonSignatureMatch), records)
	}
	permittedText := content.Text
	redacted := content.Record.Outcome == security.OutcomeRedact
	if semanticApplies(settings, security.BoundaryModelInput) {
		if evaluator.dependencies.Semantic == nil {
			return deny(contracts.ReasonSecurityEvaluatorUnavailable, records)
		}
		semantic, err := evaluator.dependencies.Semantic.Evaluate(ctx, runID, security.BoundaryModelInput,
			security.Field{Name: security.FieldModelInputText, Text: permittedText, Source: field.Source}, settings)
		records = append(records, semantic.Record)
		switch {
		case err != nil || semantic.Paused():
			return deny(reasonOf(semantic.Record.ReasonCode, contracts.ReasonSecurityEvaluatorUnavailable), records)
		case !semantic.Permitted():
			return deny(reasonOf(semantic.Record.ReasonCode, contracts.ReasonSemanticInjectionDetected), records)
		case semantic.Record.Outcome == security.OutcomeRedact:
			permittedText, redacted = semantic.Text, true
		}
	}
	if redacted {
		return outcome{decision: contracts.EvaluationRedact, reason: contracts.ReasonContentRedacted, records: records, redactedText: &permittedText}
	}
	return outcome{decision: contracts.EvaluationAllow, records: records}
}

// toolResult runs the agent path's tool-result pipeline over the text as one untrusted value.
func (evaluator *Evaluator) toolResult(ctx context.Context, runID, tool, text string, settings security.Settings) outcome {
	if evaluator.dependencies.Inspector == nil {
		return deny(contracts.ReasonSecurityEvaluatorUnavailable, nil)
	}
	resultJSON, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return deny(contracts.ReasonDecisionUnavailable, nil)
	}
	source := security.SourceRef{SourceID: judgeSource}
	inspection, err := evaluator.dependencies.Inspector.InspectToolResult(ctx, security.ToolResultInput{
		RunID: runID, Tool: tool, ResultJSON: resultJSON, Source: source,
		Untrusted: []security.UntrustedPath{{Path: []string{"text"}, Name: security.FieldToolResultText, Source: source}},
	}, settings)
	records := inspection.Records
	if err != nil {
		return deny(reasonOf(inspection.ReasonCode, contracts.ReasonSecurityEvaluatorUnavailable), records)
	}
	switch inspection.Outcome {
	case security.ResultPass:
		return outcome{decision: contracts.EvaluationAllow, records: records}
	case security.ResultRedacted:
		var redacted struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(inspection.ResultJSON, &redacted) != nil {
			return deny(contracts.ReasonDecisionUnavailable, records)
		}
		return outcome{decision: contracts.EvaluationRedact, reason: reasonOf(inspection.ReasonCode, contracts.ReasonContentRedacted),
			records: records, redactedText: &redacted.Text}
	case security.ResultBlocked:
		return deny(reasonOf(inspection.ReasonCode, contracts.ReasonContentBlocked), records)
	default:
		return deny(reasonOf(inspection.ReasonCode, contracts.ReasonSecurityEvaluatorUnavailable), records)
	}
}

// actionProposal runs the agent path's gate with a recorder and freezer that store nothing.
func (evaluator *Evaluator) actionProposal(ctx context.Context, organizationID string, request contracts.ControlEvaluationRequest) outcome {
	if evaluator.dependencies.Gates == nil {
		return deny(contracts.ReasonDecisionUnavailable, nil)
	}
	recorder := &decisionOnlyRecorder{}
	gate := evaluator.dependencies.Gates(recorder, decisionOnlyFreezer{})
	if gate == nil {
		return deny(contracts.ReasonDecisionUnavailable, nil)
	}
	decision := gate.Evaluate(ctx, policy.RunIdentity{OrganizationID: organizationID, RunID: request.RunID}, policy.Proposal{
		ActionID: newUUID(), StepNumber: 1, IdempotencyKey: newUUID(),
		Tool: string(*request.Tool), RawArguments: request.Arguments,
	})
	result := outcome{records: decision.ControlRecords, reason: decision.ReasonCode}
	switch decision.Outcome {
	case policy.OutcomeAllow:
		result.decision = contracts.EvaluationAllow
	case policy.OutcomeApprovalRequired:
		result.decision = contracts.EvaluationApprovalRequired
	default:
		result.decision = contracts.EvaluationDeny
		if result.reason == "" {
			result.reason = contracts.ReasonDecisionUnavailable
		}
	}
	if template := contracts.ReportTemplate(decision.AlternativeTemplate); template.Valid() {
		result.alternativeTemplate = &template
	}
	return result
}

// decisionOnlyRecorder lets the gate run without storing: StoreAction keeps nothing (the gate goes
// on as if the action were stored) and RecordDecision only captures the decision.
type decisionOnlyRecorder struct {
	decision *policy.Decision
}

func (recorder *decisionOnlyRecorder) StoreAction(context.Context, policy.StoredAction) error {
	return nil
}

func (recorder *decisionOnlyRecorder) RecordDecision(_ context.Context, _ policy.RunIdentity, decision policy.Decision) error {
	recorder.decision = &decision
	return nil
}

// decisionOnlyFreezer lets an approval-required decision stand without freezing any payload.
type decisionOnlyFreezer struct{}

func (decisionOnlyFreezer) Freeze(context.Context, policy.RunIdentity, policy.StoredAction, policy.PassportScope) (policy.FrozenReview, error) {
	return policy.FrozenReview{}, nil
}

// recordEvidence writes the control.evaluated event and the control records in one transaction.
func (evaluator *Evaluator) recordEvidence(ctx context.Context, operator contracts.OperatorContext, runID, evaluationID string,
	admissionRevisionID, activeRevisionID int64, result outcome, safeMessage string) error {
	organizationID := operator.OrganizationID
	var reason *contracts.ReasonCode
	if result.reason != "" {
		reasonCode := result.reason
		reason = &reasonCode
	}
	decision := eventDecisionOf(result.decision)
	summary := contracts.MaskedSummary{
		AdmissionCatalogRevisionID: &admissionRevisionID,
		Effect:                     pointer("none"),
		AlternativeTemplate:        result.alternativeTemplate,
		SafeMessage:                &safeMessage,
		// Judge evidence is labelled and attributed, so summaries count it apart from agent decisions.
		ActorID:      pointer(operator.UserID),
		InputSource:  pointer("judge"),
		EvaluationID: pointer(evaluationID),
	}
	for _, record := range result.records {
		if record.MatchedRuleID != "" && record.ControlID == security.ControlSignatureMatch {
			summary.MatchedRule, summary.FeedRevision = pointer(record.MatchedRuleID), optional(record.FeedRevision)
			break
		}
	}
	if slices.ContainsFunc(result.records, func(record security.ControlRecord) bool { return record.ControlClass == security.ClassSemantic }) {
		summary.Purpose = pointer("security")
	}
	return evaluator.dependencies.Repository.InTransaction(ctx, func(tx repository.Tx) error {
		if _, err := tx.AppendEvent(ctx, repository.NewEvent{
			OrganizationID: organizationID, RunID: &runID, EventType: contracts.EventControlEvaluated,
			Decision: &decision, ReasonCode: reason, CatalogRevisionID: &activeRevisionID, MaskedSummary: summary,
		}); err != nil {
			return err
		}
		return tx.InsertControlRecords(ctx, repository.ControlKeys{
			OrganizationID: organizationID, RunID: runID, EvaluationID: evaluationID,
			AdmissionCatalogRevisionID: admissionRevisionID, EvaluatedCatalogRevisionID: activeRevisionID,
		}, result.records)
	})
}

func eventDecisionOf(decision contracts.EvaluationDecision) contracts.EventDecision {
	switch decision {
	case contracts.EvaluationAllow:
		return contracts.DecisionAllow
	case contracts.EvaluationRedact:
		return contracts.DecisionRedact
	case contracts.EvaluationApprovalRequired:
		return contracts.DecisionApprovalRequired
	default:
		return contracts.DecisionDeny
	}
}

// buildResponse turns the outcome into X-91. Blocked text is never echoed; only a redaction
// returns its server-redacted text.
func buildResponse(evaluationID, runID string, admissionRevisionID int64, snapshot catalog.Snapshot, result outcome) contracts.ControlEvaluationResponse {
	response := contracts.ControlEvaluationResponse{
		EvaluationID: evaluationID, RunID: runID, Decision: result.decision,
		SafeMessage: safeMessageFor(result.decision, result.reason), AlternativeTemplate: result.alternativeTemplate,
		Controls: []contracts.ControlEvaluationControl{},
		Catalog: contracts.ControlEvaluationCatalog{AdmissionRevisionID: admissionRevisionID,
			ActiveRevisionID: snapshot.RevisionID, FeedRevisionID: snapshot.FeedRevisionID},
	}
	if result.reason != "" {
		reason := result.reason
		response.ReasonCode = &reason
	}
	if result.decision == contracts.EvaluationRedact && result.redactedText != nil {
		response.Content = &contracts.ControlEvaluationContent{Text: *result.redactedText}
	}
	for _, record := range result.records {
		response.Controls = append(response.Controls, contracts.ControlEvaluationControl{
			Boundary: contracts.ControlBoundary(record.Boundary), ControlClass: string(record.ControlClass),
			Control: record.ControlID, Outcome: string(record.Outcome), ReasonCode: optional(record.ReasonCode),
			RuleID: optional(record.MatchedRuleID), FeedRevision: optional(record.FeedRevision),
		})
		if record.ControlClass == security.ClassSemantic && record.Verdict != nil && response.Semantic == nil {
			response.Semantic = &contracts.ControlEvaluationSemantic{Source: string(record.VerdictSource),
				RiskCategory: record.Verdict.RiskCategory, Score: record.Verdict.Score, ReasonCode: record.Verdict.ReasonCode}
		}
	}
	return response
}

func safeMessageFor(decision contracts.EvaluationDecision, reason contracts.ReasonCode) string {
	// One message table for every reason code: contracts.ReasonCode.SafeMessage (GO-58).
	if reason != "" {
		return reason.SafeMessage()
	}
	switch decision {
	case contracts.EvaluationAllow:
		return "The controls let the interaction through; this grants no authority beyond the run's passport."
	case contracts.EvaluationApprovalRequired:
		return "The action is permitted only after exact review; the evaluation stored and executed nothing."
	default:
		return "The interaction was denied."
	}
}

// semanticApplies mirrors the catalog's semantic guard settings for one boundary.
func semanticApplies(settings security.Settings, boundary security.Boundary) bool {
	return settings.SemanticInjection.Enabled && slices.Contains(settings.SemanticInjection.Boundaries, boundary)
}

// reasonOf converts a control's reason string to X-13, falling back when it is not a known code.
func reasonOf(reason string, fallback contracts.ReasonCode) contracts.ReasonCode {
	if code := contracts.ReasonCode(reason); code.Valid() {
		return code
	}
	return fallback
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func pointer[Value any](value Value) *Value { return &value }

// newUUID returns a random version 4 UUID in the lowercase form PostgreSQL returns.
func newUUID() string {
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// JudgeGates builds the agent path's gate from its own scopes, relationships and security action
// evaluator (lane f3's production chain), with the evaluation's no-store recorder and freezer.
func JudgeGates(scopes policy.ScopeReader, relationships policy.RelationshipReader, actionEvaluator policy.ActionEvaluator) GateFactory {
	return func(recorder policy.ActionRecorder, freezer policy.ReviewFreezer) ActionGate {
		return policy.NewGate(scopes, recorder, relationships, actionEvaluator).WithReviewFreezer(freezer)
	}
}
