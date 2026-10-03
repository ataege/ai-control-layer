package policy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
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
	ToolAttemptLimit      int        // governed tool attempts per run, safe retries included
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

// RelationshipReader answers the argument relationships the passport's lists cannot (GO-28),
// from trusted records of the verified organization.
type RelationshipReader interface {
	// VendorLinkedToInvoices reports whether the vendor belongs to the organization and is the
	// vendor of at least one of the given (passport-scoped) invoices.
	VendorLinkedToInvoices(ctx context.Context, organizationID, vendorID string, invoiceIDs []string) (bool, error)
	// ReportOfRun reports whether the report exists in the organization and was created in the run.
	ReportOfRun(ctx context.Context, organizationID, runID, reportID string) (bool, error)
}

// ActionEvaluator is the semantic action check of GO-77, called only for a proposal the
// deterministic checks already allow or send to review. It may restrict, never grant.
type ActionEvaluator interface {
	EvaluateAction(ctx context.Context, run RunIdentity, action StoredAction) (Outcome, ReasonCode, error)
}

// Gate decides allow, deny or approval required for one proposed action at a time.
type Gate struct {
	scopes        ScopeReader
	recorder      ActionRecorder
	relationships RelationshipReader
	evaluator     ActionEvaluator // nil until GO-77 is wired
	now           func() time.Time
}

// NewGate returns a gate. A nil evaluator means no semantic action check is configured yet; a nil
// relationship reader denies every action that needs a relationship check.
func NewGate(scopes ScopeReader, recorder ActionRecorder, relationships RelationshipReader, evaluator ActionEvaluator) *Gate {
	return &Gate{scopes: scopes, recorder: recorder, relationships: relationships, evaluator: evaluator, now: time.Now}
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
	if reason, permitted := gate.checkResources(ctx, run, scope, arguments); !permitted {
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

// checkResources checks every argument relationship of "Proposed tool argument boundaries" at the
// gate, so a denied proposal never reaches an adapter. Read access and outbound access are checked
// independently; the export restriction itself is GO-64. A relationship that cannot be read denies.
func (gate *Gate) checkResources(ctx context.Context, run RunIdentity, scope PassportScope, arguments Arguments) (ReasonCode, bool) {
	switch typedArguments := arguments.(type) {
	case ReadInvoiceArguments:
		if !containsString(scope.AllowedInvoiceIDs, typedArguments.InvoiceID) {
			return ReasonResourceOutOfScope, false
		}
	case ReadVendorArguments:
		// Not any vendor id because the tool is allowed: a vendor of the organization that is the
		// vendor of a passport-scoped invoice.
		if gate.relationships == nil || len(scope.AllowedInvoiceIDs) == 0 {
			return ReasonResourceOutOfScope, false
		}
		linked, err := gate.relationships.VendorLinkedToInvoices(ctx, run.OrganizationID, typedArguments.VendorID, scope.AllowedInvoiceIDs)
		if err != nil {
			return ReasonDecisionUnavailable, false
		}
		if !linked {
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
		// Only the trusted reference the passport resolved for this run, never a destination found
		// in content; references name their run (recipient:<run_id>:<vendor_id>).
		if !containsString(scope.RecipientReferences, typedArguments.RecipientReference) ||
			!strings.HasPrefix(typedArguments.RecipientReference, "recipient:"+run.RunID+":") {
			return ReasonDestinationNotAllowed, false
		}
		if gate.relationships == nil {
			return ReasonResourceOutOfScope, false
		}
		ofRun, err := gate.relationships.ReportOfRun(ctx, run.OrganizationID, run.RunID, typedArguments.ReportID)
		if err != nil {
			return ReasonDecisionUnavailable, false
		}
		if !ofRun {
			return ReasonResourceOutOfScope, false
		}
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
