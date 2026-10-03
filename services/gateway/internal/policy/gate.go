package policy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"starter/services/gateway/internal/contracts"
	"starter/services/gateway/internal/security"
)

// Outcome is one of the gate's three decisions. Anything that is not a clean allow or approval
// requirement is a deny: a transport or configuration error is never an allow (fail closed).
type Outcome string

const (
	OutcomeAllow            Outcome = "allow"
	OutcomeDeny             Outcome = "deny"
	OutcomeApprovalRequired Outcome = "approval_required"
)

// ReasonCode is a stable reason from the X-13 vocabulary (contracts).
type ReasonCode = contracts.ReasonCode

// The X-13 reason codes the gate and the executor emit.
const (
	ReasonResourceOutOfScope     = contracts.ReasonResourceOutOfScope
	ReasonDestinationNotAllowed  = contracts.ReasonDestinationNotAllowed
	ReasonReportExportRestricted = contracts.ReasonReportExportRestricted
	ReasonReportLineageMissing   = contracts.ReasonReportLineageMissing
	ReasonTemplateNotAllowed     = contracts.ReasonTemplateNotAllowed
	ReasonApprovalRequired       = contracts.ReasonApprovalRequired
	ReasonRunCancelled           = contracts.ReasonRunCancelled
	ReasonRunExpired             = contracts.ReasonRunExpired
	ReasonToolNotRegistered      = contracts.ReasonToolNotRegistered
	ReasonInvalidArguments       = contracts.ReasonInvalidArguments
	ReasonToolNotAllowed         = contracts.ReasonToolNotAllowed
	ReasonDecisionUnavailable    = contracts.ReasonDecisionUnavailable
	ReasonActionChanged          = contracts.ReasonActionChanged
	ReasonAllowanceExhausted     = contracts.ReasonAllowanceExhausted
	ReasonOutcomeUnknown         = contracts.ReasonOutcomeUnknown
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
	// ReplaySource is labelled_replay:<fixture id> for a labelled replay (GO-36) and empty for a
	// model proposal. It only labels the records; the gate's checks are the same.
	ReplaySource string
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
	// AdmissionCatalogRevisionID is the catalog revision the passport was admitted under.
	AdmissionCatalogRevisionID int64
	ExpiresAt                  time.Time
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
	ReplaySource            string
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
	// AlternativeTemplate names the permitted continuation after an export denial (GO-29 offers
	// it only when the passport permits it).
	AlternativeTemplate string
	// Review is the frozen review material of an approval request (GO-43).
	Review *FrozenReview
	// ControlRecords are the security controls' evidence for this decision (GO-77), written to
	// runtime.control_assessments with the decision.
	ControlRecords             []security.ControlRecord
	AdmissionCatalogRevisionID int64
	// ReplaySource labels every record of a replayed proposal.
	ReplaySource string
	// DeniedReport names the stored report whose export was denied (GO-64), so the denial event
	// carries its references. Nil for every other decision.
	DeniedReport *ReportRef
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
	// ReportExport decides, from the stored report and its stored lineage, whether the report of
	// this organization and run may be sent to the registered vendor recipient (GO-64).
	ReportExport(ctx context.Context, organizationID, runID, reportID string) (ExportVerdict, error)
}

// ExportVerdict is the provenance answer for queue_report. Found is false when no report with
// that id exists in this organization and run.
type ExportVerdict struct {
	Found               bool
	Allowed             bool
	ReasonCode          ReasonCode
	AlternativeTemplate string // a permitted continuation, never extra authority
	// Report names the stored report by references (id, template, stored classification), so a
	// denial event can point at it. Empty when Found is false.
	Report ReportRef
}

// ReportRef names a stored report by references only: never its content, title or sources.
type ReportRef struct {
	ID             string
	Template       string
	Classification string
}

// ActionEvaluator is the semantic action check of GO-77, called only for a proposal the
// deterministic checks already allow or send to review. It may restrict, never grant: its
// outcome is allow (no objection) or deny, and an error is a deny that the worker treats as a
// pause. Its control records are kept even when it fails.
type ActionEvaluator interface {
	EvaluateAction(ctx context.Context, run RunIdentity, action StoredAction) (ActionCheck, error)
}

// ActionCheck is the evaluator's answer and its evidence.
type ActionCheck struct {
	Outcome    Outcome
	ReasonCode ReasonCode
	Records    []security.ControlRecord
}

// Gate decides allow, deny or approval required for one proposed action at a time.
type Gate struct {
	scopes        ScopeReader
	recorder      ActionRecorder
	relationships RelationshipReader
	evaluator     ActionEvaluator // nil until GO-77 is wired
	freezer       ReviewFreezer   // nil: every approval request is denied
	now           func() time.Time
}

// WithReviewFreezer sets the freezer for approval requests; without one, nothing can be reviewed,
// so an action that needs approval is denied.
func (gate *Gate) WithReviewFreezer(freezer ReviewFreezer) *Gate {
	gate.freezer = freezer
	return gate
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
		return Decision{Outcome: OutcomeDeny, ReasonCode: reason, ActionID: proposal.ActionID, ReplaySource: proposal.ReplaySource}
	}
	if run.OrganizationID == "" || run.RunID == "" || !uuidPattern.MatchString(proposal.ActionID) ||
		proposal.StepNumber <= 0 || proposal.IdempotencyKey == "" {
		return denied(ReasonInvalidArguments)
	}
	if proposal.ReplaySource != "" && !replaySourcePattern.MatchString(proposal.ReplaySource) {
		return Decision{Outcome: OutcomeDeny, ReasonCode: ReasonInvalidArguments, ActionID: proposal.ActionID}
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
		ReplaySource:            proposal.ReplaySource,
	}
	if err := gate.recorder.StoreAction(ctx, storedAction); err != nil {
		return denied(ReasonDecisionUnavailable)
	}
	stored := func(outcome Outcome, reason ReasonCode) Decision {
		return Decision{
			Outcome: outcome, ReasonCode: reason, ActionID: proposal.ActionID,
			ActionStored: true, ActionDigest: digest, EvaluatedRevisionID: revisionID,
			AdmissionCatalogRevisionID: scope.AdmissionCatalogRevisionID,
			ReplaySource:               proposal.ReplaySource,
		}
	}

	if !gate.now().Before(scope.ExpiresAt) {
		return stored(OutcomeDeny, ReasonRunExpired)
	}
	if !containsTool(scope.AllowedTools, arguments.Tool()) {
		return stored(OutcomeDeny, ReasonToolNotAllowed)
	}
	if reason, permitted := gate.checkResources(ctx, run, scope, arguments); !permitted {
		return stored(OutcomeDeny, reason)
	}
	// The export restriction is decided before the approval rule, so a forbidden export is a
	// denial and never an approval request (GO-64).
	if queueArguments, isQueue := arguments.(QueueReportArguments); isQueue {
		verdict, err := gate.relationships.ReportExport(ctx, run.OrganizationID, run.RunID, queueArguments.ReportID)
		switch {
		case err != nil:
			return stored(OutcomeDeny, ReasonDecisionUnavailable)
		case !verdict.Found:
			return stored(OutcomeDeny, ReasonResourceOutOfScope)
		case !verdict.Allowed:
			denial := stored(OutcomeDeny, verdict.ReasonCode)
			if denial.ReasonCode == "" {
				denial.ReasonCode = ReasonReportExportRestricted
			}
			denial.AlternativeTemplate = verdict.AlternativeTemplate
			denial.DeniedReport = &verdict.Report
			return denial
		}
	}

	outcome, reason := OutcomeAllow, ReasonCode("")
	if containsTool(scope.ApprovalRequiredTools, arguments.Tool()) {
		outcome, reason = OutcomeApprovalRequired, ReasonApprovalRequired
	}
	decision := gate.restrictSemantically(ctx, run, storedAction, stored(outcome, reason))
	if decision.Outcome == OutcomeApprovalRequired {
		// Nothing frozen means nothing to review: deny rather than request an unbound approval.
		if gate.freezer == nil {
			return stored(OutcomeDeny, ReasonDecisionUnavailable)
		}
		frozen, err := gate.freezer.Freeze(ctx, run, storedAction, scope)
		if err != nil {
			return stored(OutcomeDeny, ReasonDecisionUnavailable)
		}
		decision.Review = &frozen
	}
	return decision
}

// restrictSemantically applies the GO-77 check to a deterministic allow or approval requirement.
// Its verdict can only turn the decision into a deny; an error or unknown outcome also denies.
func (gate *Gate) restrictSemantically(ctx context.Context, run RunIdentity, action StoredAction, decision Decision) Decision {
	if gate.evaluator == nil {
		return decision
	}
	check, err := gate.evaluator.EvaluateAction(ctx, run, action)
	decision.ControlRecords = check.Records
	switch {
	case err != nil:
		// A guard failure pauses: the deny carries the evaluator's reason when it gives one.
		decision.Outcome, decision.ReasonCode = OutcomeDeny, check.ReasonCode
		if decision.ReasonCode == "" {
			decision.ReasonCode = ReasonDecisionUnavailable
		}
	case check.Outcome == OutcomeDeny:
		decision.Outcome, decision.ReasonCode = OutcomeDeny, check.ReasonCode
		if decision.ReasonCode == "" {
			decision.ReasonCode = ReasonDecisionUnavailable
		}
	case check.Outcome == OutcomeAllow:
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
