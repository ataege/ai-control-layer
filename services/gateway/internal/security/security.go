// Package security holds the gateway's hybrid security controls: deterministic content rules,
// signature matching and the semantic evaluator. A control here can only withhold, mask or pause;
// it never grants a tool, record, recipient or export (AGENTS.md guardrail 8).
//
// The package reads neither PostgreSQL nor policy.yaml. The caller passes the active catalog's
// settings and trusted source metadata; every result records the evaluated catalog revision.
package security

import (
	"errors"
	"math"
	"slices"
	"time"
)

// Boundary names the place of inspection, using the boundary names of config/policy.yaml.
type Boundary string

const (
	BoundaryModelInput     Boundary = "model_input"
	BoundaryToolResult     Boundary = "tool_result"
	BoundaryActionProposal Boundary = "action_proposal"
)

// FieldName names a designated text field. Only designated fields are inspected or redacted.
type FieldName string

const (
	FieldToolResultText FieldName = "tool_result_text"
	FieldInternalNote   FieldName = "internal_note"
	FieldModelInputText FieldName = "model_input_text"
	// FieldToolResultValue is any other string value of a minimized tool result, for example a
	// vendor display name: deterministic rules only, never a semantic call.
	FieldToolResultValue FieldName = "tool_result_value"
)

// designatedFields lists which fields each boundary may carry.
var designatedFields = map[Boundary][]FieldName{
	BoundaryToolResult: {FieldToolResultText, FieldInternalNote, FieldToolResultValue},
	BoundaryModelInput: {FieldModelInputText},
}

// SourceRef is trusted provenance resolved by Go from source records. This package copies it
// through unchanged and never sets it, so masking a field cannot relabel its source.
type SourceRef struct {
	SourceID       string
	Version        int64
	Classification string
}

// Field is one designated text field and the trusted source it came from.
type Field struct {
	Name   FieldName
	Text   string
	Source SourceRef
}

// Mode is a guard's configured response.
type Mode string

const (
	ModeBlock  Mode = "block"
	ModeRedact Mode = "redact"
)

// Outcome is a control's result, using the outcome names of runtime.control_assessments.
type Outcome string

const (
	OutcomePass          Outcome = "pass"
	OutcomeRedact        Outcome = "redact"
	OutcomeBlock         Outcome = "block"
	OutcomeError         Outcome = "error"
	OutcomeNotApplicable Outcome = "not_applicable"
)

// ControlClass separates deterministic controls from the semantic evaluator.
type ControlClass string

const (
	ClassDeterministic ControlClass = "deterministic"
	ClassSemantic      ControlClass = "semantic"
)

// Control IDs: the three registered guards of policy.yaml, plus the fixed field limit.
const (
	ControlSecretPattern     = "secret_pattern"
	ControlSignatureMatch    = "signature_match"
	ControlSemanticInjection = "semantic_injection"
	ControlFieldLimit        = "field_limit"
)

// Reason codes from the proposed vocabulary (docs/product/README.md). ReasonContentBlocked and
// ReasonContentTooLarge were approved by the lead's delegate on 2026-10-03 and wait for X-13.
const (
	ReasonContentRedacted              = "content_redacted"
	ReasonContentBlocked               = "content_blocked"
	ReasonContentTooLarge              = "content_too_large"
	ReasonSignatureMatch               = "signature_match"
	ReasonSemanticInjectionDetected    = "semantic_injection_detected"
	ReasonSecurityEvaluatorUnavailable = "security_evaluator_unavailable"
	ReasonSecurityAllowanceExhausted   = "security_allowance_exhausted"
)

// MaxFieldBytes bounds one inspected field. An oversized field is withheld whole, never truncated,
// so no uninspected remainder can reach the agent context. The bound also keeps one security call's
// byte-based token reservation small against the shared run total.
const MaxFieldBytes = 4096

var (
	ErrSettings = errors.New("invalid security settings")
	ErrField    = errors.New("field is not designated for this boundary")
	ErrSpan     = errors.New("invalid match span")
)

// GuardSettings is one registered guard as configured in the active catalog.
type GuardSettings struct {
	Enabled    bool
	Mode       Mode
	Boundaries []Boundary
}

// appliesAt reports whether the guard is enabled for the boundary.
func (guard GuardSettings) appliesAt(boundary Boundary) bool {
	return guard.Enabled && slices.Contains(guard.Boundaries, boundary)
}

// SemanticSettings is the semantic_injection guard: a score at or above Threshold fires it.
type SemanticSettings struct {
	GuardSettings
	Threshold float64
}

// Settings is the part of the active catalog revision the security controls read.
type Settings struct {
	EvaluatedCatalogRevisionID int64
	SecretPattern              GuardSettings
	SemanticInjection          SemanticSettings
	// SignatureMatch uses no Mode: each feed rule carries its own response.
	SignatureMatch GuardSettings
	DisabledRules  []string
	// Feed is the validated, digest-pinned feed bound to this revision; nil only when
	// SignatureMatch is disabled.
	Feed *Feed
}

// validate rejects settings the controls cannot enforce; a bad catalog is never an allow.
func (settings Settings) validate() error {
	if settings.EvaluatedCatalogRevisionID <= 0 {
		return ErrSettings
	}
	for _, guard := range []GuardSettings{settings.SecretPattern, settings.SemanticInjection.GuardSettings} {
		if guard.Enabled && guard.Mode != ModeBlock && guard.Mode != ModeRedact {
			return ErrSettings
		}
	}
	threshold := settings.SemanticInjection.Threshold
	if settings.SemanticInjection.Enabled && (math.IsNaN(threshold) || threshold < 0 || threshold > 1) {
		return ErrSettings
	}
	// An enabled signature guard never runs as an empty rule set.
	if settings.SignatureMatch.Enabled && settings.Feed == nil {
		return ErrSettings
	}
	return nil
}

// ControlRecord is the safe evidence of one control decision. It never holds inspected text.
type ControlRecord struct {
	Boundary                   Boundary
	Field                      FieldName
	ControlClass               ControlClass
	ControlID                  string
	Outcome                    Outcome
	ReasonCode                 string
	MatchedRuleID              string
	FeedRevision               string
	FeedDigest                 string
	EvaluatedCatalogRevisionID int64
	Duration                   time.Duration
	// Semantic records only: the validated verdict, whether it came from the live model or a
	// labelled fixture, the metered security call and the failure kind when the guard failed.
	Verdict             *Verdict
	VerdictSource       VerdictSource
	SecurityModelCallID string
	Failure             string
}
