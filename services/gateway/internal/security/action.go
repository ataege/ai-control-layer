package security

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
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
	// The semantic check classifies untrusted text, so it sees only the free-text parts of the
	// arguments. Values that match their tool's strict format (identifiers, registered templates,
	// uuids, run-scoped references) carry no text for it to classify; the gate and the passport
	// check them deterministically. With nothing to classify, no model call is made or charged and
	// the evidence says why.
	freeText, err := freeTextArguments(input.Tool, input.CanonicalArguments)
	if err != nil {
		return decide(ActionPause, ReasonSecurityEvaluatorUnavailable, ErrAction)
	}
	if len(freeText) == 0 {
		assessment.Records = append(assessment.Records, ControlRecord{Boundary: BoundaryActionProposal, Field: FieldActionProposalText,
			ControlClass: ClassSemantic, ControlID: ControlSemanticInjection, Outcome: OutcomeNotApplicable,
			ReasonCode: ReasonNoFreeTextArguments, EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID})
		return decide(ActionNoObjection, "", nil)
	}
	if inspector.evaluator == nil {
		assessment.Records = append(assessment.Records, ControlRecord{Boundary: BoundaryActionProposal, Field: FieldActionProposalText,
			ControlClass: ClassSemantic, ControlID: ControlSemanticInjection, Outcome: OutcomeError,
			ReasonCode: ReasonSecurityEvaluatorUnavailable, EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID, Failure: FailureUnavailable})
		return decide(ActionPause, ReasonSecurityEvaluatorUnavailable, ErrEvaluatorUnavailable)
	}
	proposalText := "Free-text arguments of a proposed " + input.Tool + " call:\n" + strings.Join(freeText, "\n")
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

// fieldRule is the strict format of one argument field of a registered tool.
type fieldRule struct {
	pattern *regexp.Regexp
	list    bool // an array of values, each matching the pattern
}

// The formats are stricter than the gate's decoder, which accepts any bounded value without control
// characters for an identifier: here a value is "constrained" only when it has no room for prose.
var (
	identifierFormat = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	uuidFormat       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	templateFormat   = regexp.MustCompile(`^(internal_investigation_v1|vendor_reconciliation_v1)$`)
	recipientFormat  = regexp.MustCompile(`^recipient:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}:[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
)

// constrainedArguments lists, per registered tool, every argument field and its format (X-09). A
// tool or field missing here is treated as free text, so a new tool is checked until it is listed.
var constrainedArguments = map[string]map[string]fieldRule{
	"read_invoice":  {"invoice_id": {pattern: identifierFormat}},
	"read_vendor":   {"vendor_id": {pattern: identifierFormat}},
	"create_report": {"template": {pattern: templateFormat}, "source_invoice_ids": {pattern: identifierFormat, list: true}},
	"queue_report":  {"report_id": {pattern: uuidFormat}, "recipient_reference": {pattern: recipientFormat}},
}

// freeTextArguments returns the "path: value" lines of every argument that is not a constrained
// value of its tool, in document order. An unknown tool has no constrained field, so all of its
// keys and values are free text.
func freeTextArguments(tool string, canonicalArguments json.RawMessage) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonicalArguments))
	decoder.UseNumber()
	var root map[string]any
	if decoder.Decode(&root) != nil {
		return nil, ErrAction
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrAction
	}
	rules := constrainedArguments[tool]
	var lines []string
	for _, key := range slices.Sorted(mapsKeys(root)) {
		rule, known := rules[key]
		lines = append(lines, freeTextOf(key, root[key], rule, known)...)
	}
	return lines, nil
}

func mapsKeys(object map[string]any) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for key := range object {
			if !yield(key) {
				return
			}
		}
	}
}

// freeTextOf returns the free-text lines of one field. A known field whose value has the expected
// shape and format contributes none; anything else contributes its full text, key included.
func freeTextOf(path string, value any, rule fieldRule, known bool) []string {
	if known {
		switch typed := value.(type) {
		case string:
			if !rule.list && rule.pattern.MatchString(typed) {
				return nil
			}
		case []any:
			if rule.list {
				var lines []string
				for index, element := range typed {
					if text, isString := element.(string); isString && rule.pattern.MatchString(text) {
						continue
					}
					lines = append(lines, leafLines(fmt.Sprintf("%s[%d]", path, index), element)...)
				}
				return lines
			}
		}
	}
	return leafLines(path, value)
}

// leafLines flattens any value into "path: leaf" lines; every key becomes part of a path, so a key
// cannot carry text past the check.
func leafLines(path string, value any) []string {
	switch typed := value.(type) {
	case map[string]any:
		var lines []string
		for _, key := range slices.Sorted(mapsKeys(typed)) {
			lines = append(lines, leafLines(path+"."+key, typed[key])...)
		}
		if len(typed) == 0 {
			lines = append(lines, path+": {}")
		}
		return lines
	case []any:
		var lines []string
		for index, element := range typed {
			lines = append(lines, leafLines(fmt.Sprintf("%s[%d]", path, index), element)...)
		}
		if len(typed) == 0 {
			lines = append(lines, path+": []")
		}
		return lines
	case string:
		return []string{path + ": " + typed}
	default:
		encoded, _ := json.Marshal(typed)
		return []string{path + ": " + string(encoded)}
	}
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
