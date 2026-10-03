package security

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// ActionDecision is the security controls' answer for one action proposal. It is deliberately not
// "allow": no_objection only means these controls add no restriction, and the deterministic gate
// remains the authority (Figure 6).
type ActionDecision string

const (
	ActionNoObjection ActionDecision = "no_objection"
	ActionBlock       ActionDecision = "block"
	ActionPause       ActionDecision = "pause"
)

// ActionInput is one stored, canonicalized proposal that the deterministic gate already allows or
// sends to review. Only then may EvaluateAction run (scope and provenance first, Figure 6).
type ActionInput struct {
	RunID              string
	ActionID           string
	Tool               string
	CanonicalArguments json.RawMessage
}

// ActionAssessment is the evidence of the action check: one record per control that ran and the
// semantic call, if one was made. It holds no classifier input.
type ActionAssessment struct {
	Decision     ActionDecision
	ReasonCode   string
	Records      []ControlRecord
	SemanticCall *SemanticResult
}

var ErrAction = errors.New("action proposal cannot be inspected")

// Bounds that keep "Proposed tool call: <tool>\nArguments: <arguments>" within MaxFieldBytes.
const (
	maxActionToolBytes     = 64
	maxActionArgumentBytes = MaxFieldBytes - 128
)

// EvaluateAction runs the signature rules and then the semantic check on a proposal at the
// action_proposal boundary. The field limit holds whatever the settings. A semantic hit blocks in
// either mode, because an action cannot be partly redacted; any guard failure pauses.
func (inspector *Inspector) EvaluateAction(ctx context.Context, input ActionInput, settings Settings) (ActionAssessment, error) {
	assessment := ActionAssessment{}
	decide := func(decision ActionDecision, reason string, err error) (ActionAssessment, error) {
		assessment.Decision, assessment.ReasonCode = decision, reason
		return assessment, err
	}
	if err := settings.validate(); err != nil {
		return decide(ActionPause, ReasonSecurityEvaluatorUnavailable, err)
	}
	// The arguments plus the tool name and prefix must fit one field, so the semantic check sees
	// the whole proposal; a larger proposal is withheld, never truncated.
	if len(input.CanonicalArguments) > maxActionArgumentBytes {
		assessment.Records = append(assessment.Records, ControlRecord{Boundary: BoundaryActionProposal, Field: FieldActionProposalText,
			ControlClass: ClassDeterministic, ControlID: ControlFieldLimit, Outcome: OutcomeBlock,
			ReasonCode: ReasonContentTooLarge, EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID})
		return decide(ActionBlock, ReasonContentTooLarge, nil)
	}
	values, err := argumentStrings(input.CanonicalArguments)
	if err != nil || input.RunID == "" || input.Tool == "" || len(input.Tool) > maxActionToolBytes {
		return decide(ActionPause, ReasonSecurityEvaluatorUnavailable, ErrAction)
	}
	// Signatures see the decoded string values, so JSON escapes cannot hide a pattern.
	signatureText := strings.Join(append([]string{input.Tool}, values...), "\n")
	signature, err := MatchSignatures(signatureText, BoundaryActionProposal, FieldActionProposalText, settings)
	assessment.Records = append(assessment.Records, signature)
	if err != nil {
		return decide(ActionPause, ReasonSecurityEvaluatorUnavailable, err)
	}
	if signature.Outcome == OutcomeBlock {
		return decide(ActionBlock, signature.ReasonCode, nil)
	}
	if !settings.SemanticInjection.appliesAt(BoundaryActionProposal) {
		return decide(ActionNoObjection, "", nil)
	}
	if inspector.evaluator == nil {
		assessment.Records = append(assessment.Records, ControlRecord{Boundary: BoundaryActionProposal, Field: FieldActionProposalText,
			ControlClass: ClassSemantic, ControlID: ControlSemanticInjection, Outcome: OutcomeError,
			ReasonCode: ReasonSecurityEvaluatorUnavailable, EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID, Failure: FailureUnavailable})
		return decide(ActionPause, ReasonSecurityEvaluatorUnavailable, ErrEvaluatorUnavailable)
	}
	// The classifier sees the tool name and its canonical arguments, nothing else.
	proposalText := "Proposed tool call: " + input.Tool + "\nArguments: " + string(input.CanonicalArguments)
	semantic, err := inspector.evaluator.Evaluate(ctx, input.RunID, BoundaryActionProposal,
		Field{Name: FieldActionProposalText, Text: proposalText}, settings)
	assessment.Records = append(assessment.Records, semantic.Record)
	assessment.SemanticCall = &semantic
	switch {
	case err != nil || semantic.Paused():
		if err == nil {
			err = ErrEvaluatorUnavailable
		}
		reason := semantic.Record.ReasonCode
		if reason == "" {
			reason = ReasonSecurityEvaluatorUnavailable
		}
		return decide(ActionPause, reason, err)
	case semantic.Record.Outcome == OutcomeBlock || semantic.Record.Outcome == OutcomeRedact:
		reason := semantic.Record.ReasonCode
		return decide(ActionBlock, reason, nil)
	}
	return decide(ActionNoObjection, "", nil)
}

// argumentStrings returns every string of a canonical arguments object, keys included, in
// document order. The arguments must be one JSON object.
func argumentStrings(raw json.RawMessage) ([]string, error) {
	if !uniqueJSONKeys(raw) {
		return nil, ErrAction
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, ErrAction
	}
	var values []string
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if text, isString := token.(string); isString {
			values = append(values, text)
		}
	}
	return values, nil
}
