// Package contracts mirrors the Go-owned runtime wire contracts of packages/contracts:
// the start-run request (X-07), passport (X-08), action proposal and stored action (X-09),
// run state (X-11), safe event (X-12) and reason codes (X-13).
//
// Envelope fields are camelCase; tool arguments stay snake_case as in the report's appendix.
// Every field is always present on the wire; an optional value is JSON null, so nullable
// fields are pointers without omitempty. contracts_test.go checks these types against the
// shared schemas and fixtures.
package contracts

import (
	"encoding/json"
	"time"
)

// JobKindAgentStep is the runtime.jobs kind admission inserts and the worker claims. It is a
// Go-internal constant (a lead decision), shared here so admission and the worker cannot drift.
const JobKindAgentStep = "agent_step"

// ReasonCode is a stable decision reason (X-13).
type ReasonCode string

const (
	ReasonResourceOutOfScope           ReasonCode = "resource_out_of_scope"
	ReasonDestinationNotAllowed        ReasonCode = "destination_not_allowed"
	ReasonReportExportRestricted       ReasonCode = "report_export_restricted"
	ReasonReportLineageMissing         ReasonCode = "report_lineage_missing"
	ReasonSourcePolicyChanged          ReasonCode = "source_policy_changed"
	ReasonTemplateNotAllowed           ReasonCode = "template_not_allowed"
	ReasonApprovalRequired             ReasonCode = "approval_required"
	ReasonApprovalExpired              ReasonCode = "approval_expired"
	ReasonActionChanged                ReasonCode = "action_changed"
	ReasonResourceVersionChanged       ReasonCode = "resource_version_changed"
	ReasonAllowanceExhausted           ReasonCode = "allowance_exhausted"
	ReasonRunCancelled                 ReasonCode = "run_cancelled"
	ReasonOutcomeUnknown               ReasonCode = "outcome_unknown"
	ReasonSemanticInjectionDetected    ReasonCode = "semantic_injection_detected"
	ReasonSecurityEvaluatorUnavailable ReasonCode = "security_evaluator_unavailable"
	ReasonSecurityAllowanceExhausted   ReasonCode = "security_allowance_exhausted"
	ReasonContentRedacted              ReasonCode = "content_redacted"
	ReasonSignatureMatch               ReasonCode = "signature_match"
	ReasonPolicyReloadRejected         ReasonCode = "policy_reload_rejected"
	ReasonModelNotAllowed              ReasonCode = "model_not_allowed"
	ReasonMultipleActionsNotSupported  ReasonCode = "multiple_actions_not_supported"
	ReasonRunExpired                   ReasonCode = "run_expired"
	ReasonToolNotRegistered            ReasonCode = "tool_not_registered"
	ReasonInvalidArguments             ReasonCode = "invalid_arguments"
	ReasonToolNotAllowed               ReasonCode = "tool_not_allowed"
	ReasonDecisionUnavailable          ReasonCode = "decision_unavailable"
	ReasonContentBlocked               ReasonCode = "content_blocked"
	ReasonContentTooLarge              ReasonCode = "content_too_large"
)

// ReasonCodes lists every reason code in contract order.
var ReasonCodes = []ReasonCode{
	ReasonResourceOutOfScope, ReasonDestinationNotAllowed, ReasonReportExportRestricted,
	ReasonReportLineageMissing, ReasonSourcePolicyChanged, ReasonTemplateNotAllowed,
	ReasonApprovalRequired, ReasonApprovalExpired, ReasonActionChanged,
	ReasonResourceVersionChanged, ReasonAllowanceExhausted, ReasonRunCancelled,
	ReasonOutcomeUnknown, ReasonSemanticInjectionDetected, ReasonSecurityEvaluatorUnavailable,
	ReasonSecurityAllowanceExhausted, ReasonContentRedacted, ReasonSignatureMatch,
	ReasonPolicyReloadRejected, ReasonModelNotAllowed, ReasonMultipleActionsNotSupported,
	ReasonRunExpired, ReasonToolNotRegistered, ReasonInvalidArguments, ReasonToolNotAllowed,
	ReasonDecisionUnavailable, ReasonContentBlocked, ReasonContentTooLarge,
}

// Valid reports whether the code is part of the contract.
func (code ReasonCode) Valid() bool { return contains(ReasonCodes, code) }

// ToolName names one of the four registered tools.
type ToolName string

const (
	ToolReadInvoice  ToolName = "read_invoice"
	ToolReadVendor   ToolName = "read_vendor"
	ToolCreateReport ToolName = "create_report"
	ToolQueueReport  ToolName = "queue_report"
)

// ToolNames lists the registered tools.
var ToolNames = []ToolName{ToolReadInvoice, ToolReadVendor, ToolCreateReport, ToolQueueReport}

// Valid reports whether the tool is registered.
func (tool ToolName) Valid() bool { return contains(ToolNames, tool) }

// ReportTemplate names one of the two fixed report templates.
type ReportTemplate string

const (
	TemplateInternalInvestigation ReportTemplate = "internal_investigation_v1"
	TemplateVendorReconciliation  ReportTemplate = "vendor_reconciliation_v1"
)

// ReportTemplates lists the fixed templates.
var ReportTemplates = []ReportTemplate{TemplateInternalInvestigation, TemplateVendorReconciliation}

// Valid reports whether the template is one of the fixed templates.
func (template ReportTemplate) Valid() bool { return contains(ReportTemplates, template) }

// StartRunRequest is X-07, landed by the web + API implementer and accepted unchanged.
// Its optional fields may be absent, unlike the Go-owned contracts below.
type StartRunRequest struct {
	Template            string          `json:"template"`
	VendorID            *string         `json:"vendorId,omitempty"`
	InvoiceIDs          []string        `json:"invoiceIds"`
	Destination         string          `json:"destination"`
	ApprovalRequirement *string         `json:"approvalRequirement,omitempty"`
	Limits              *StartRunLimits `json:"limits,omitempty"`
}

// StartRunLimits are the limits an operator may request; admission caps them by policy.
type StartRunLimits struct {
	ModelCalls     *int64 `json:"modelCalls,omitempty"`
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`
}

// StartRunResponse answers an admitted start-run command.
type StartRunResponse struct {
	RunID      string `json:"runId"`
	PassportID string `json:"passportId"`
}

// Passport is X-08, the immutable grant of one run. Identity comes from verified context.
type Passport struct {
	PassportID                 string         `json:"passportId"`
	RunID                      string         `json:"runId"`
	OrganizationID             string         `json:"organizationId"`
	ActorID                    string         `json:"actorId"`
	TaskVersion                string         `json:"taskVersion"`
	AdmissionCatalogRevisionID int64          `json:"admissionCatalogRevisionId"`
	IssuedAt                   time.Time      `json:"issuedAt"`
	ExpiresAt                  time.Time      `json:"expiresAt"`
	Scope                      PassportScope  `json:"scope"`
	Limits                     PassportLimits `json:"limits"`
}

// PassportScope is the upper bound on tools, records, templates, destinations and models.
type PassportScope struct {
	Tools           []ToolName       `json:"tools"`
	InvoiceIDs      []string         `json:"invoiceIds"`
	VendorIDs       []string         `json:"vendorIds"`
	ReportTemplates []ReportTemplate `json:"reportTemplates"`
	ProjectionRules []string         `json:"projectionRules"`
	// RecipientReferences are trusted directory references, recipient:<runId>:<vendorId>.
	RecipientReferences []string `json:"recipientReferences"`
	AllowedModels       []string `json:"allowedModels"`
	// InternalNoteReadable is the "Internal note allowed for investigation" field rule.
	InternalNoteReadable  bool       `json:"internalNoteReadable"`
	ApprovalRequiredTools []ToolName `json:"approvalRequiredTools"`
}

// PassportLimits are the hard ceilings admitted for the run.
type PassportLimits struct {
	CallsTotal    int64 `json:"callsTotal"`
	CallsAgent    int64 `json:"callsAgent"`
	CallsSecurity int64 `json:"callsSecurity"`
	TokensTotal   int64 `json:"tokensTotal"`
	// Nil: no purpose sub-limit beyond the shared token total.
	TokensAgent           *int64 `json:"tokensAgent"`
	TokensSecurity        *int64 `json:"tokensSecurity"`
	RequestTimeoutSeconds int64  `json:"requestTimeoutSeconds"`
	LocalMaxConcurrency   int64  `json:"localMaxConcurrency"`
	ToolAttempts          int64  `json:"toolAttempts"`
	Corrections           int64  `json:"corrections"`
	RunExpiryMinutes      int64  `json:"runExpiryMinutes"`
}

// ActionProposal is X-09: one proposed tool action. Arguments stay raw here; the gate
// decodes them strictly into the typed arguments of Tool (the shapes below).
type ActionProposal struct {
	Tool      ToolName        `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

// ReadInvoiceArguments are the arguments of read_invoice.
type ReadInvoiceArguments struct {
	InvoiceID string `json:"invoice_id"`
}

// ReadVendorArguments are the arguments of read_vendor.
type ReadVendorArguments struct {
	VendorID string `json:"vendor_id"`
}

// CreateReportArguments are the arguments of create_report; source order is kept.
type CreateReportArguments struct {
	Template         ReportTemplate `json:"template"`
	SourceInvoiceIDs []string       `json:"source_invoice_ids"`
}

// QueueReportArguments are the arguments of queue_report.
type QueueReportArguments struct {
	ReportID           string `json:"report_id"`
	RecipientReference string `json:"recipient_reference"`
}

// ActionStatus is the lifecycle state of a stored action.
type ActionStatus string

const (
	ActionProposed         ActionStatus = "proposed"
	ActionAllowed          ActionStatus = "allowed"
	ActionDenied           ActionStatus = "denied"
	ActionAwaitingApproval ActionStatus = "awaiting_approval"
	ActionApproved         ActionStatus = "approved"
	ActionRejected         ActionStatus = "rejected"
	ActionExpired          ActionStatus = "expired"
	ActionExecuting        ActionStatus = "executing"
	ActionSucceeded        ActionStatus = "succeeded"
	ActionFailed           ActionStatus = "failed"
	ActionUnknown          ActionStatus = "unknown"
)

// ActionStatuses lists every action status.
var ActionStatuses = []ActionStatus{
	ActionProposed, ActionAllowed, ActionDenied, ActionAwaitingApproval, ActionApproved,
	ActionRejected, ActionExpired, ActionExecuting, ActionSucceeded, ActionFailed, ActionUnknown,
}

// Valid reports whether the status is part of the contract.
func (status ActionStatus) Valid() bool { return contains(ActionStatuses, status) }

// StoredAction is X-09: the immutable action recorded before any policy check.
type StoredAction struct {
	ActionID                string         `json:"actionId"`
	RunID                   string         `json:"runId"`
	StepNumber              int64          `json:"stepNumber"`
	Proposal                ActionProposal `json:"proposal"`
	CanonicalizationVersion int64          `json:"canonicalizationVersion"`
	// ActionDigest is the lowercase hex SHA-256 of the canonical action (GO-04).
	ActionDigest   string `json:"actionDigest"`
	IdempotencyKey string `json:"idempotencyKey"`
	// EvaluatedCatalogRevisionID is the active catalog revision when the action was stored.
	EvaluatedCatalogRevisionID int64        `json:"evaluatedCatalogRevisionId"`
	Status                     ActionStatus `json:"status"`
	ExpiresAt                  *time.Time   `json:"expiresAt"`
	CreatedAt                  time.Time    `json:"createdAt"`
}

// RunStatus is the state of a run (X-11).
type RunStatus string

const (
	RunQueued           RunStatus = "queued"
	RunRunning          RunStatus = "running"
	RunAwaitingApproval RunStatus = "awaiting_approval"
	RunPaused           RunStatus = "paused"
	RunCompleted        RunStatus = "completed"
	RunFailed           RunStatus = "failed"
	RunStopped          RunStatus = "stopped"
)

// RunStatuses lists every run status.
var RunStatuses = []RunStatus{
	RunQueued, RunRunning, RunAwaitingApproval, RunPaused, RunCompleted, RunFailed, RunStopped,
}

// Valid reports whether the status is part of the contract.
func (status RunStatus) Valid() bool { return contains(RunStatuses, status) }

// NeedsReason reports whether a run in this status must name its reason.
func (status RunStatus) NeedsReason() bool {
	return status == RunPaused || status == RunFailed || status == RunStopped
}

// RunState is X-11. TerminalReason is set exactly when the status needs a reason.
type RunState struct {
	RunID             string      `json:"runId"`
	PassportID        string      `json:"passportId"`
	Status            RunStatus   `json:"status"`
	TerminalReason    *ReasonCode `json:"terminalReason"`
	CancelRequestedAt *time.Time  `json:"cancelRequestedAt"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
}

// EventType names a safe event (X-12).
type EventType string

const (
	EventAdmissionRejected         EventType = "admission.rejected"
	EventRunQueued                 EventType = "run.queued"
	EventRunStarted                EventType = "run.started"
	EventRunPaused                 EventType = "run.paused"
	EventRunCompleted              EventType = "run.completed"
	EventRunFailed                 EventType = "run.failed"
	EventRunStopped                EventType = "run.stopped"
	EventModelCompleted            EventType = "model.completed"
	EventActionProposed            EventType = "action.proposed"
	EventActionAllowed             EventType = "action.allowed"
	EventActionDenied              EventType = "action.denied"
	EventApprovalRequested         EventType = "approval.requested"
	EventApprovalDecided           EventType = "approval.decided"
	EventActionExecuting           EventType = "action.executing"
	EventActionSucceeded           EventType = "action.succeeded"
	EventActionFailed              EventType = "action.failed"
	EventActionUnknown             EventType = "action.unknown"
	EventReportCreated             EventType = "report.created"
	EventReportExportDenied        EventType = "report.export_denied"
	EventReportSafeTemplateOffered EventType = "report.safe_template_offered"
	EventControlEvaluated          EventType = "control.evaluated"
	EventCatalogRevisionRejected   EventType = "catalog.revision_rejected"
)

// EventTypes lists every event type.
var EventTypes = []EventType{
	EventAdmissionRejected, EventRunQueued, EventRunStarted, EventRunPaused, EventRunCompleted,
	EventRunFailed, EventRunStopped, EventModelCompleted, EventActionProposed, EventActionAllowed,
	EventActionDenied, EventApprovalRequested, EventApprovalDecided, EventActionExecuting,
	EventActionSucceeded, EventActionFailed, EventActionUnknown, EventReportCreated,
	EventReportExportDenied, EventReportSafeTemplateOffered, EventControlEvaluated,
	EventCatalogRevisionRejected,
}

// Valid reports whether the event type is part of the contract.
func (eventType EventType) Valid() bool { return contains(EventTypes, eventType) }

// EventDecision is the decision recorded with an event.
type EventDecision string

const (
	DecisionAllow            EventDecision = "allow"
	DecisionDeny             EventDecision = "deny"
	DecisionApprovalRequired EventDecision = "approval_required"
	DecisionRedact           EventDecision = "redact"
	DecisionApproved         EventDecision = "approved"
	DecisionRejected         EventDecision = "rejected"
)

// EventDecisions lists every event decision.
var EventDecisions = []EventDecision{
	DecisionAllow, DecisionDeny, DecisionApprovalRequired, DecisionRedact, DecisionApproved,
	DecisionRejected,
}

// Valid reports whether the decision is part of the contract.
func (decision EventDecision) Valid() bool { return contains(EventDecisions, decision) }

// SafeEvent is X-12, one sanitized row of runtime.audit_events.
type SafeEvent struct {
	// EventID is the decimal string of the bigint cursor.
	EventID           string         `json:"eventId"`
	OrganizationID    string         `json:"organizationId"`
	RunID             *string        `json:"runId"`
	ActionID          *string        `json:"actionId"`
	EventType         EventType      `json:"eventType"`
	Decision          *EventDecision `json:"decision"`
	ReasonCode        *ReasonCode    `json:"reasonCode"`
	CatalogRevisionID *int64         `json:"catalogRevisionId"`
	MaskedSummary     MaskedSummary  `json:"maskedSummary"`
	OccurredAt        time.Time      `json:"occurredAt"`
}

// MaskedSummary is the closed set of masked metadata an event may carry. It never holds raw
// notes, secrets, model requests, classifier reasoning or review content.
type MaskedSummary struct {
	// Purpose is the metered model purpose: "agent" or "security".
	Purpose                    *string         `json:"purpose"`
	AdmissionCatalogRevisionID *int64          `json:"admissionCatalogRevisionId"`
	MatchedRule                *string         `json:"matchedRule"`
	FeedRevision               *string         `json:"feedRevision"`
	ReportID                   *string         `json:"reportId"`
	Template                   *ReportTemplate `json:"template"`
	// Classification is "internal_only" or "vendor_shareable".
	Classification *string `json:"classification"`
	// LineageCheck is "passed", "failed" or "missing".
	LineageCheck *string `json:"lineageCheck"`
	// Effect is "none", "read", "report_created" or "outbox_message_queued".
	Effect              *string         `json:"effect"`
	ReplaySource        *string         `json:"replaySource"`
	AlternativeTemplate *ReportTemplate `json:"alternativeTemplate"`
	SafeMessage         *string         `json:"safeMessage"`
}

func contains[Value comparable](values []Value, candidate Value) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// OperatorContext is X-14: the verified operator context NestJS signs into the
// X-Operator-Context JWT (claim ctx). Go uses it as the only identity source of a command and
// still authorizes every command against its organization and run.
type OperatorContext struct {
	UserID         string   `json:"userId"`
	OrganizationID string   `json:"organizationId"`
	Roles          []string `json:"roles"`
}
