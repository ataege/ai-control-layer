package policy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
)

// Outcome is one of the gate's three decisions. Anything that is not a clean allow or approval
// requirement is a deny: a transport or configuration error is never an allow (fail closed).
type Outcome string

const (
	OutcomeAllow            Outcome = "allow"
	OutcomeDeny             Outcome = "deny"
	OutcomeApprovalRequired Outcome = "approval_required"
)

// ReasonCode is a stable reason from the X-13 vocabulary.
type ReasonCode string

// Reason codes from the report's proposed vocabulary.
const (
	ReasonResourceOutOfScope     ReasonCode = "resource_out_of_scope"
	ReasonDestinationNotAllowed  ReasonCode = "destination_not_allowed"
	ReasonReportExportRestricted ReasonCode = "report_export_restricted"
	ReasonReportLineageMissing   ReasonCode = "report_lineage_missing"
	ReasonTemplateNotAllowed     ReasonCode = "template_not_allowed"
	ReasonApprovalRequired       ReasonCode = "approval_required"
	ReasonRunCancelled           ReasonCode = "run_cancelled"
)

// Provisional reason codes, proposed for X-13 because the report's vocabulary has none for these
// cases; renamed here if X-13 freezes other names.
const (
	ReasonToolNotRegistered   ReasonCode = "tool_not_registered"
	ReasonInvalidArguments    ReasonCode = "invalid_arguments"
	ReasonToolNotAllowed      ReasonCode = "tool_not_allowed"
	ReasonDecisionUnavailable ReasonCode = "decision_unavailable"
)

// RunIdentity is the verified context of the run the proposal belongs to. It comes from the
// worker's claimed job and the stored run, never from model output.
type RunIdentity struct {
	OrganizationID string
	RunID          string
}

// Proposal is one action the model proposed in one step, before any check.
type Proposal struct {
	ActionID       string // assigned by the worker (stable action identity)
	StepNumber     int    // one action per model step; unique per run
	IdempotencyKey string // kept by every safe retry of the same action (GO-02)
	Tool           string
	RawArguments   json.RawMessage
}

// PassportScope is the part of the immutable passport the gate checks, as loaded for a verified
// run. It mirrors the passport contract (X-08) as far as the gate needs it.
type PassportScope struct {
	OrganizationID        string
	RunID                 string
	PassportID            string
	AllowedTools          []ToolName
	AllowedInvoiceIDs     []string
	AllowedTemplates      []string
	RecipientReferences   []string
	ApprovalRequiredTools []ToolName // the passport's approval rule, for example every queue_report
	ExpiresAt             time.Time
}

// StoredAction is the immutable record of a proposal, written before any evaluation.
type StoredAction struct {
	OrganizationID          string
	RunID                   string
	ActionID                string
	StepNumber              int
	IdempotencyKey          string
	Tool                    ToolName
	CanonicalArguments      json.RawMessage
	CanonicalizationVersion int
	ActionDigest            [sha256.Size]byte
	EvaluatedRevisionID     int64
}

// Decision is the gate's answer for one proposal. The zero value is not a valid decision; every
// path through Evaluate sets an outcome and, for deny and approval, a reason.
type Decision struct {
	Outcome             Outcome
	ReasonCode          ReasonCode
	ActionID            string
	ActionStored        bool
	ActionDigest        [sha256.Size]byte
	EvaluatedRevisionID int64
}

// ScopeReader loads the passport scope and the active catalog revision for a verified run.
type ScopeReader interface {
	LoadScope(ctx context.Context, run RunIdentity) (PassportScope, error)
	ActiveCatalogRevision(ctx context.Context) (int64, error)
}

// ActionRecorder persists the action and the decision. StoreAction commits before the decision is
// recorded, and RecordDecision commits (as a safe event) before any tool effect.
type ActionRecorder interface {
	StoreAction(ctx context.Context, action StoredAction) error
	RecordDecision(ctx context.Context, run RunIdentity, decision Decision) error
}

// ActionEvaluator is the semantic action check of GO-77, called only for a proposal the
// deterministic checks already allow or send to review. It may restrict, never grant.
type ActionEvaluator interface {
	EvaluateAction(ctx context.Context, run RunIdentity, action StoredAction) (Outcome, ReasonCode, error)
}

// Gate decides allow, deny or approval required for one proposed action at a time.
type Gate struct {
	scopes    ScopeReader
	recorder  ActionRecorder
	evaluator ActionEvaluator // nil until GO-77 is wired
	now       func() time.Time
}

// NewGate returns a gate. A nil evaluator means no semantic action check is configured yet.
func NewGate(scopes ScopeReader, recorder ActionRecorder, evaluator ActionEvaluator) *Gate {
	return &Gate{scopes: scopes, recorder: recorder, evaluator: evaluator, now: time.Now}
}

// Evaluate checks one proposal in the order of Figure 6: registered tool and strict arguments,
// verified scope, tools and resources, the approval rule, then the semantic check. It always
// returns a decision; every failure is a deny with a reason code.
func (gate *Gate) Evaluate(ctx context.Context, run RunIdentity, proposal Proposal) Decision {
	decision := gate.decide(ctx, run, proposal)
	if err := gate.recorder.RecordDecision(ctx, run, decision); err != nil {
		// Without a recorded decision nothing may run.
		decision.Outcome = OutcomeDeny
		decision.ReasonCode = ReasonDecisionUnavailable
	}
	return decision
}

func (gate *Gate) decide(ctx context.Context, run RunIdentity, proposal Proposal) Decision {
	denied := func(reason ReasonCode) Decision {
		return Decision{Outcome: OutcomeDeny, ReasonCode: reason, ActionID: proposal.ActionID}
	}
	if run.OrganizationID == "" || run.RunID == "" || !uuidPattern.MatchString(proposal.ActionID) ||
		proposal.StepNumber <= 0 || proposal.IdempotencyKey == "" {
		return denied(ReasonInvalidArguments)
	}

	arguments, err := DecodeArguments(ToolName(proposal.Tool), proposal.RawArguments)
	if errors.Is(err, ErrUnknownTool) {
		return denied(ReasonToolNotRegistered)
	}
	if err != nil {
		// Malformed arguments cannot be canonicalized, so no action row exists for them; only the
		// denial is recorded.
		return denied(ReasonInvalidArguments)
	}

	scope, err := gate.scopes.LoadScope(ctx, run)
	if err != nil {
		return denied(ReasonDecisionUnavailable)
	}
	if scope.OrganizationID != run.OrganizationID || scope.RunID != run.RunID || scope.PassportID == "" {
		return denied(ReasonDecisionUnavailable)
	}
	revisionID, err := gate.scopes.ActiveCatalogRevision(ctx)
	if err != nil || revisionID <= 0 {
		return denied(ReasonDecisionUnavailable)
	}

	canonicalArguments, err := CanonicalArguments(arguments)
	if err != nil {
		return denied(ReasonInvalidArguments)
	}
	// The digest covers the fields known at proposal time; GO-43 freezes the review payload
	// (recipient, outbound content, versions, expiry) into the approval-bound digest.
	digest, err := CanonicalAction{
		ActionID:         proposal.ActionID,
		RunID:            run.RunID,
		Arguments:        arguments,
		PassportID:       scope.PassportID,
		PolicyRevisionID: revisionID,
	}.Digest()
	if err != nil {
		return denied(ReasonInvalidArguments)
	}
	storedAction := StoredAction{
		OrganizationID:          run.OrganizationID,
		RunID:                   run.RunID,
		ActionID:                proposal.ActionID,
		StepNumber:              proposal.StepNumber,
		IdempotencyKey:          proposal.IdempotencyKey,
		Tool:                    arguments.Tool(),
		CanonicalArguments:      canonicalArguments,
		CanonicalizationVersion: CanonicalizationVersion,
		ActionDigest:            digest,
		EvaluatedRevisionID:     revisionID,
	}
	if err := gate.recorder.StoreAction(ctx, storedAction); err != nil {
		return denied(ReasonDecisionUnavailable)
	}
	stored := func(outcome Outcome, reason ReasonCode) Decision {
		return Decision{
			Outcome: outcome, ReasonCode: reason, ActionID: proposal.ActionID,
			ActionStored: true, ActionDigest: digest, EvaluatedRevisionID: revisionID,
		}
	}

	if !gate.now().Before(scope.ExpiresAt) {
		return stored(OutcomeDeny, ReasonRunCancelled)
	}
	if !containsTool(scope.AllowedTools, arguments.Tool()) {
		return stored(OutcomeDeny, ReasonToolNotAllowed)
	}
	if reason, permitted := checkResources(scope, arguments); !permitted {
		return stored(OutcomeDeny, reason)
	}

	outcome, reason := OutcomeAllow, ReasonCode("")
	if containsTool(scope.ApprovalRequiredTools, arguments.Tool()) {
		outcome, reason = OutcomeApprovalRequired, ReasonApprovalRequired
	}
	return gate.restrictSemantically(ctx, run, storedAction, stored(outcome, reason))
}

// restrictSemantically applies the GO-77 check to a deterministic allow or approval requirement.
// Its verdict can only turn the decision into a deny; an error or unknown outcome also denies.
func (gate *Gate) restrictSemantically(ctx context.Context, run RunIdentity, action StoredAction, decision Decision) Decision {
	if gate.evaluator == nil {
		return decision
	}
	semanticOutcome, semanticReason, err := gate.evaluator.EvaluateAction(ctx, run, action)
	switch {
	case err != nil:
		decision.Outcome, decision.ReasonCode = OutcomeDeny, ReasonDecisionUnavailable
	case semanticOutcome == OutcomeDeny:
		decision.Outcome, decision.ReasonCode = OutcomeDeny, semanticReason
		if decision.ReasonCode == "" {
			decision.ReasonCode = ReasonDecisionUnavailable
		}
	case semanticOutcome == OutcomeAllow:
		// No change: a semantic allow never upgrades approval required to allow.
	default:
		decision.Outcome, decision.ReasonCode = OutcomeDeny, ReasonDecisionUnavailable
	}
	return decision
}

// checkResources checks the passport's resources for each tool. Deeper relationships (vendor to
// invoice, report to run, recipient directory) follow in GO-28, the export restriction in GO-64.
func checkResources(scope PassportScope, arguments Arguments) (ReasonCode, bool) {
	switch typedArguments := arguments.(type) {
	case ReadInvoiceArguments:
		if !containsString(scope.AllowedInvoiceIDs, typedArguments.InvoiceID) {
			return ReasonResourceOutOfScope, false
		}
	case CreateReportArguments:
		if !containsString(scope.AllowedTemplates, typedArguments.Template) {
			return ReasonTemplateNotAllowed, false
		}
		for _, invoiceID := range typedArguments.SourceInvoiceIDs {
			if !containsString(scope.AllowedInvoiceIDs, invoiceID) {
				return ReasonResourceOutOfScope, false
			}
		}
	case QueueReportArguments:
		if !containsString(scope.RecipientReferences, typedArguments.RecipientReference) {
			return ReasonDestinationNotAllowed, false
		}
	case ReadVendorArguments:
		// The vendor-to-passport-invoice relationship needs the demo records (GO-28).
	default:
		return ReasonToolNotRegistered, false
	}
	return "", true
}

func containsTool(tools []ToolName, tool ToolName) bool {
	for _, candidate := range tools {
		if candidate == tool {
			return true
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
