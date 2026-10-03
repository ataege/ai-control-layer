package security

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
)

// MaxResultBytes bounds one minimized tool result; a larger result is withheld whole.
const MaxResultBytes = 16 << 10

// ResultOutcome is the overall outcome of a tool-result inspection.
type ResultOutcome string

const (
	// ResultPass: every value passed; the result returns unchanged.
	ResultPass ResultOutcome = "pass"
	// ResultRedacted: some values were masked; only the validated masked text returns.
	ResultRedacted ResultOutcome = "redacted"
	// ResultBlocked: some values were withheld behind a marker; the permitted values return.
	ResultBlocked ResultOutcome = "blocked"
	// ResultPaused: a guard failed; nothing returns and the run pauses.
	ResultPaused ResultOutcome = "paused"
)

// UntrustedPath marks a JSON path whose string is untrusted free text that also needs the semantic
// check, with the trusted source it came from. For read_invoice this is internal_note.text.
type UntrustedPath struct {
	Path   []string
	Name   FieldName
	Source SourceRef
}

// ToolResultInput is one minimized, authorized tool result (tools.MinimizeForModel).
type ToolResultInput struct {
	RunID      string
	Tool       string
	ResultJSON json.RawMessage
	// Source is the trusted source of the result's structured values.
	Source    SourceRef
	Untrusted []UntrustedPath
}

// ValueInspection is the evidence for one string value. It holds no inspected text.
type ValueInspection struct {
	Path    []string
	Field   FieldName
	Source  SourceRef
	Outcome Outcome
	Records []ControlRecord
}

// ToolResultInspection is what may enter the agent context. ResultJSON holds only permitted or
// validated masked values, with blocked values replaced by "[WITHHELD:<reason>]"; it is empty when
// the result is paused or withheld whole.
type ToolResultInspection struct {
	Outcome    ResultOutcome
	ReasonCode string
	ResultJSON json.RawMessage
	Values     []ValueInspection
	Records    []ControlRecord
	// SemanticCalls lists the metered security calls made, for usage and timing records.
	SemanticCalls []SemanticResult
}

var ErrToolResult = errors.New("tool result cannot be inspected")

// Inspector runs the tool-result pipeline of Figure 10. The worker calls it after the effect is
// recorded and before the result becomes agent context; there is no other path into the context.
type Inspector struct {
	evaluator *SemanticEvaluator
}

// NewInspector takes the semantic evaluator; nil is allowed only while the semantic guard is
// disabled for tool results, otherwise inspection pauses.
func NewInspector(evaluator *SemanticEvaluator) *Inspector {
	return &Inspector{evaluator: evaluator}
}

// InspectToolResult applies, to every string value: the field limit and secret rules, then the
// signature rules on the original text, then (only on untrusted paths) the semantic check on the
// redacted text. A blocked value is withheld behind a marker and the rest returns; any guard
// failure withholds the whole result and pauses.
func (inspector *Inspector) InspectToolResult(ctx context.Context, input ToolResultInput, settings Settings) (ToolResultInspection, error) {
	inspection := ToolResultInspection{}
	pause := func(reason string, err error) (ToolResultInspection, error) {
		inspection.Outcome, inspection.ReasonCode, inspection.ResultJSON = ResultPaused, reason, nil
		return inspection, err
	}
	if err := settings.validate(); err != nil {
		return pause("", err)
	}
	if len(input.ResultJSON) > MaxResultBytes {
		inspection.Outcome, inspection.ReasonCode = ResultBlocked, ReasonContentTooLarge
		inspection.Records = append(inspection.Records, ControlRecord{
			Boundary: BoundaryToolResult, ControlClass: ClassDeterministic, ControlID: ControlFieldLimit,
			Outcome: OutcomeBlock, ReasonCode: ReasonContentTooLarge, EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID,
		})
		return inspection, nil
	}
	var root any
	decoder := json.NewDecoder(bytes.NewReader(input.ResultJSON))
	decoder.UseNumber()
	if !uniqueJSONKeys(input.ResultJSON) || decoder.Decode(&root) != nil {
		return pause("", ErrToolResult)
	}
	if _, isObject := root.(map[string]any); !isObject || input.RunID == "" {
		return pause("", ErrToolResult)
	}
	// An untrusted path must hold a string when present, or its semantic check would be skipped.
	for _, untrusted := range input.Untrusted {
		if value, present := lookupPath(root, untrusted.Path); present {
			if _, isString := value.(string); !isString {
				return pause("", ErrToolResult)
			}
		}
	}

	changed := false
	var walk func(value any, path []string) (any, error)
	walk = func(value any, path []string) (any, error) {
		switch typed := value.(type) {
		case map[string]any:
			// Sorted keys keep the evidence order deterministic.
			keys := slices.Sorted(maps.Keys(typed))
			for _, key := range keys {
				replacement, err := walk(typed[key], append(slices.Clone(path), key))
				if err != nil {
					return nil, err
				}
				typed[key] = replacement
			}
			return typed, nil
		case []any:
			for index, child := range typed {
				replacement, err := walk(child, append(slices.Clone(path), strconv.Itoa(index)))
				if err != nil {
					return nil, err
				}
				typed[index] = replacement
			}
			return typed, nil
		case string:
			permitted, err := inspector.inspectValue(ctx, input, settings, path, typed, &inspection)
			if permitted != typed {
				changed = true
			}
			return permitted, err
		default:
			return value, nil
		}
	}
	walked, err := walk(root, nil)
	if err != nil {
		return pause(pausedReason(inspection.Records), err)
	}
	if !changed {
		inspection.ResultJSON = slices.Clone(input.ResultJSON)
	} else if inspection.ResultJSON, err = json.Marshal(walked); err != nil {
		return pause("", ErrToolResult)
	}
	inspection.Outcome = ResultPass
	for _, value := range inspection.Values {
		switch value.Outcome {
		case OutcomeBlock:
			inspection.Outcome, inspection.ReasonCode = ResultBlocked, firstReason(value.Records, OutcomeBlock)
		case OutcomeRedact:
			if inspection.Outcome == ResultPass {
				inspection.Outcome, inspection.ReasonCode = ResultRedacted, firstReason(value.Records, OutcomeRedact)
			}
		}
		if inspection.Outcome == ResultBlocked {
			break
		}
	}
	return inspection, nil
}

// inspectValue runs the controls on one string and returns the text that may continue: the
// original, the masked text or a withheld marker. An error means a guard failed.
func (inspector *Inspector) inspectValue(ctx context.Context, input ToolResultInput, settings Settings, path []string, text string, inspection *ToolResultInspection) (string, error) {
	field := Field{Name: FieldToolResultValue, Text: text, Source: input.Source}
	semanticPath := false
	for _, untrusted := range input.Untrusted {
		if slices.Equal(untrusted.Path, path) {
			field.Name, field.Source, semanticPath = untrusted.Name, untrusted.Source, true
		}
	}
	value := ValueInspection{Path: path, Field: field.Name, Source: field.Source, Outcome: OutcomePass}
	record := func(controlRecord ControlRecord) {
		value.Records = append(value.Records, controlRecord)
		inspection.Records = append(inspection.Records, controlRecord)
	}
	finish := func(outcome Outcome, permitted string, err error) (string, error) {
		value.Outcome = outcome
		inspection.Values = append(inspection.Values, value)
		return permitted, err
	}

	content, err := ApplyContentRules(field, BoundaryToolResult, settings)
	record(content.Record)
	if err != nil {
		return finish(OutcomeError, "", err)
	}
	if !content.Permitted() {
		return finish(OutcomeBlock, withheldMarker(content.Record.ReasonCode), nil)
	}
	signature, err := MatchSignatures(text, BoundaryToolResult, field.Name, settings)
	record(signature)
	if err != nil {
		return finish(OutcomeError, "", err)
	}
	if signature.Outcome == OutcomeBlock {
		return finish(OutcomeBlock, withheldMarker(signature.ReasonCode), nil)
	}
	outcome := OutcomePass
	if content.Record.Outcome == OutcomeRedact {
		outcome = OutcomeRedact
	}
	if !semanticPath || !settings.SemanticInjection.appliesAt(BoundaryToolResult) {
		return finish(outcome, content.Text, nil)
	}
	if inspector.evaluator == nil {
		record(ControlRecord{Boundary: BoundaryToolResult, Field: field.Name, ControlClass: ClassSemantic,
			ControlID: ControlSemanticInjection, Outcome: OutcomeError, ReasonCode: ReasonSecurityEvaluatorUnavailable,
			EvaluatedCatalogRevisionID: settings.EvaluatedCatalogRevisionID, Failure: FailureUnavailable})
		return finish(OutcomeError, "", ErrEvaluatorUnavailable)
	}
	// The classifier sees the redacted text, so secrets never reach it.
	semantic, err := inspector.evaluator.Evaluate(ctx, input.RunID, BoundaryToolResult,
		Field{Name: field.Name, Text: content.Text, Source: field.Source}, settings)
	record(semantic.Record)
	inspection.SemanticCalls = append(inspection.SemanticCalls, semantic)
	if err != nil || semantic.Paused() {
		if err == nil {
			err = ErrEvaluatorUnavailable
		}
		return finish(OutcomeError, "", err)
	}
	switch semantic.Record.Outcome {
	case OutcomeBlock:
		return finish(OutcomeBlock, withheldMarker(semantic.Record.ReasonCode), nil)
	case OutcomeRedact:
		return finish(OutcomeRedact, semantic.Text, nil)
	}
	return finish(outcome, content.Text, nil)
}

// withheldMarker is the fixed text that replaces a blocked value in the agent context.
func withheldMarker(reason string) string {
	return "[WITHHELD:" + reason + "]"
}

func firstReason(records []ControlRecord, outcome Outcome) string {
	for _, record := range records {
		if record.Outcome == outcome {
			return record.ReasonCode
		}
	}
	return ""
}

// pausedReason names the failed guard's reason for the run's pause.
func pausedReason(records []ControlRecord) string {
	for _, record := range records {
		if record.Outcome == OutcomeError && record.ReasonCode != "" {
			return record.ReasonCode
		}
	}
	return ReasonSecurityEvaluatorUnavailable
}

// lookupPath returns the value at an object path, if every step exists.
func lookupPath(root any, path []string) (any, bool) {
	current := root
	for _, step := range path {
		object, isObject := current.(map[string]any)
		if !isObject {
			return nil, false
		}
		if current, isObject = object[step]; !isObject {
			return nil, false
		}
	}
	return current, true
}
