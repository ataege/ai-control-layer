// Generic wire contracts shared by the web app, the API and the Go gateway.
// Keep this file in sync with schemas/*.schema.json and the Go DTOs in
// services/gateway/internal/health/dto.go. `pnpm test` checks the shared fixtures.

/** Header used to correlate one request across web, API and gateway. */
export const REQUEST_ID_HEADER = "x-request-id";

/** API-33: reviewer reloads the fixed repository policy; no uploads or identity fields. */
export type PolicyReloadRequest = Record<string, never>;
export type PolicyReloadResponse =
  | {
      status: "requested";
      requestedRevisionId: string;
      fileDigest: string;
      feedRevision: string | null;
    }
  | { status: "unchanged"; revisionId: string };

/** Sanitized NestJS policy validation failure; no configuration content or arbitrary fields. */
export interface PolicyReloadErrorResponse extends ErrorResponse {
  error: ErrorResponse["error"] & { issues?: { path: string; message: string }[] };
}

/** API-33: global catalog pointer read by an authenticated reviewer; bigint IDs are decimal strings. */
export interface PolicyStatusResponse {
  requestedRevisionId: string | null;
  validatedRevisionId: string | null;
  activeRevisionId: string | null;
  activeFeedRevisionId: string | null;
  lastError: {
    code: string;
    message: string;
    revisionId: string | null;
    stage: "gateway_validation" | "import_validation";
  } | null;
}

/** Services that expose health endpoints. */
export type ServiceName = "api" | "gateway";

/** Result of a single dependency check. */
export type CheckStatus = "up" | "down";

/** Process liveness. Never depends on PostgreSQL or another service. */
export interface LivenessResponse {
  status: "ok";
  service: ServiceName;
}

/** One dependency inside a readiness report. */
export interface DependencyCheck {
  status: CheckStatus;
  /** Short sanitized reason. Never contains hosts, credentials or stack traces. */
  message?: string;
}

/** Gateway readiness: GET /health/ready (200 when ready, 503 otherwise). */
export interface ReadinessResponse {
  status: "ok" | "unavailable";
  service: ServiceName;
  checks: {
    database: DependencyCheck;
  };
}

/** One indicator entry in the API readiness report. */
export interface ApiHealthIndicator {
  status: CheckStatus;
  message?: string;
}

/** API readiness: GET /api/health/ready, in the @nestjs/terminus report shape. */
export interface ApiReadinessResponse {
  status: "ok" | "error" | "shutting_down";
  info?: Record<string, ApiHealthIndicator>;
  error?: Record<string, ApiHealthIndicator>;
  details: Record<string, ApiHealthIndicator>;
}

/** Gateway internal ping: proves authenticated service-to-service reachability only. */
export interface GatewayPingResponse {
  status: "ok";
  service: "gateway";
}

/** Why a diagnostic check is down. */
export type DiagnosticFailureReason =
  "timeout" | "unreachable" | "unauthorized" | "not_ready" | "unexpected_response";

/** One upstream check performed by the API against the gateway. */
export interface DiagnosticCheck {
  status: CheckStatus;
  /** Round-trip time measured by the API in milliseconds. */
  latencyMs: number;
  /** HTTP status returned by the gateway, when a response was received. */
  upstreamStatus?: number;
  /** Present only when status is "down". */
  reason?: DiagnosticFailureReason;
}

/**
 * API diagnostics: GET /api/diagnostics/gateway.
 * - ok: ping and readiness both passed (HTTP 200).
 * - degraded: gateway is reachable and authenticated but its database is not ready (HTTP 503).
 * - unavailable: the authenticated ping failed (HTTP 502, or 504 on timeout).
 */
export interface GatewayDiagnosticsResponse {
  status: "ok" | "degraded" | "unavailable";
  requestId: string;
  checks: {
    /** Authenticated GET /internal/ping. */
    reachability: DiagnosticCheck;
    /** GET /health/ready, which covers the gateway's PostgreSQL connection. */
    databaseReadiness: DiagnosticCheck;
  };
}

/** Error envelope returned by both backends for every non-health failure. */
export interface ErrorResponse {
  error: {
    /** Stable machine-readable code, for example "not_found" or "unauthorized". */
    code: string;
    /** Safe human-readable message. */
    message: string;
  };
  statusCode: number;
  requestId: string;
  /** ISO 8601 timestamp. */
  timestamp: string;
  path?: string;
}

/**
 * X-14: resolved operator context sent to Go with every governed command, as claim `ctx` of the
 * X-Operator-Context JWT (HS256, issuer gateway-client, audience gateway, short expiry, jti).
 * userId and organizationId are the uuid ids of app.users and app.organizations.
 */
export interface OperatorContext {
  userId: string;
  organizationId: string;
  roles: string[];
}

/** Form Options for the task setup UI (API-12). */
export interface TaskFormOptions {
  templates: { id: string; name: string }[];
  vendors: { id: string; name: string }[];
  invoices: {
    id: string;
    number: string;
    date: string;
    /** Total in minor units of `currency`. */
    amount: number;
    /** ISO 4217 alphabetic code, for example "EUR". */
    currency: string;
    vendorId: string;
  }[];
  destinations: { id: string; name: string }[];
  approvalRequirements: { id: string; description: string }[];
  limits: { maxModelCalls: number; maxTimeoutSeconds: number };
}

/** Start Run Request (X-07). */
export interface StartRunRequest {
  template: string;
  vendorId?: string;
  invoiceIds: string[];
  destination: string;
  approvalRequirement?: string;
  limits?: {
    modelCalls?: number;
    timeoutSeconds?: number;
  };
}

/** Start Run Response. */
export interface StartRunResponse {
  runId: string;
  passportId: string;
}

/** Run View (X-11). */
export interface RunView {
  id: string;
  status: "pending" | "running" | "paused" | "failed" | "completed";
  usage: {
    modelCalls: number;
    cost: number;
  };
  terminalReason?: string;
  passport: {
    id: string;
    template: string;
  };
}

/** Sanitized Event (X-12). */
export interface SanitizedEvent {
  id: string;
  type: string;
  timestamp: string;
  details: Record<string, unknown>;
}

// Go-owned runtime contracts (X-08, X-09, X-11, X-12, X-13). Envelope fields are camelCase;
// tool arguments stay snake_case as in the report's appendix. Every field is present; an
// optional value is null. Mirrored in services/gateway/internal/contracts.

/** X-13: stable reason codes shared by the gate, admission, events and the interface. */
export type ReasonCode =
  | "resource_out_of_scope"
  | "destination_not_allowed"
  | "report_export_restricted"
  | "report_lineage_missing"
  | "source_policy_changed"
  | "template_not_allowed"
  | "approval_required"
  | "approval_expired"
  | "action_changed"
  | "resource_version_changed"
  | "allowance_exhausted"
  | "run_cancelled"
  | "outcome_unknown"
  | "semantic_injection_detected"
  | "security_evaluator_unavailable"
  | "security_allowance_exhausted"
  | "content_redacted"
  | "signature_match"
  | "policy_reload_rejected"
  | "model_not_allowed"
  | "multiple_actions_not_supported"
  | "run_expired"
  | "tool_not_registered"
  | "invalid_arguments"
  | "tool_not_allowed"
  | "decision_unavailable"
  | "content_blocked"
  | "content_too_large"
  | "limit_not_allowed"
  | "run_not_active"
  | "approval_rejected";

/** The four registered tools. */
export type ToolName = "read_invoice" | "read_vendor" | "create_report" | "queue_report";

/** The two fixed report templates. */
export type ReportTemplate = "internal_investigation_v1" | "vendor_reconciliation_v1";

/** X-08: the immutable grant of one run. Identity comes from verified context only. */
export interface Passport {
  passportId: string;
  runId: string;
  organizationId: string;
  actorId: string;
  taskVersion: string;
  admissionCatalogRevisionId: number;
  /** RFC 3339, UTC. */
  issuedAt: string;
  expiresAt: string;
  scope: {
    tools: ToolName[];
    invoiceIds: string[];
    vendorIds: string[];
    reportTemplates: ReportTemplate[];
    projectionRules: string[];
    /** Trusted directory references, format recipient:<runId>:<vendorId>; never an address. */
    recipientReferences: string[];
    allowedModels: string[];
    /** The "Internal note allowed for investigation" field rule. */
    internalNoteReadable: boolean;
    approvalRequiredTools: ToolName[];
  };
  limits: {
    callsTotal: number;
    callsAgent: number;
    callsSecurity: number;
    tokensTotal: number;
    /** Null: no purpose sub-limit beyond the shared token total. */
    tokensAgent: number | null;
    tokensSecurity: number | null;
    requestTimeoutSeconds: number;
    localMaxConcurrency: number;
    toolAttempts: number;
    corrections: number;
    runExpiryMinutes: number;
  };
}

/** X-09: one proposed tool action with its typed, snake_case arguments. */
export type ActionProposal =
  | { tool: "read_invoice"; arguments: { invoice_id: string } }
  | { tool: "read_vendor"; arguments: { vendor_id: string } }
  | {
      tool: "create_report";
      arguments: { template: ReportTemplate; source_invoice_ids: string[] };
    }
  | { tool: "queue_report"; arguments: { report_id: string; recipient_reference: string } };

/** X-09: lifecycle of a stored action. */
export type ActionStatus =
  | "proposed"
  | "allowed"
  | "denied"
  | "awaiting_approval"
  | "approved"
  | "rejected"
  | "expired"
  | "executing"
  | "succeeded"
  | "failed"
  | "unknown";

/** X-09: the immutable stored action, recorded before any policy check. */
export interface StoredAction {
  actionId: string;
  runId: string;
  stepNumber: number;
  proposal: ActionProposal;
  canonicalizationVersion: 1;
  /** Lowercase hex SHA-256 of the canonical action (GO-04). */
  actionDigest: string;
  idempotencyKey: string;
  /** The active catalog revision when the action was stored. */
  evaluatedCatalogRevisionId: number;
  status: ActionStatus;
  expiresAt: string | null;
  createdAt: string;
  /** labelled_replay:<fixture id> for a labelled replay (GO-36); null for a model proposal. */
  replaySource: string | null;
}

/** X-11: run status. */
export type RunStatus =
  "queued" | "running" | "awaiting_approval" | "paused" | "completed" | "failed" | "stopped";

/** X-11: state of one run. terminalReason is set exactly when paused, failed or stopped. */
export interface RunState {
  runId: string;
  passportId: string;
  status: RunStatus;
  terminalReason: ReasonCode | null;
  cancelRequestedAt: string | null;
  createdAt: string;
  updatedAt: string;
  /** GO-26: the validated final result of a completed run; render the named stored reports. */
  resultReference: { reportIds: string[] } | null;
}

/** X-12: event names. */
export type SafeEventType =
  | "admission.rejected"
  | "run.queued"
  | "run.started"
  | "run.paused"
  | "run.completed"
  | "run.failed"
  | "run.stopped"
  | "run.cancel_requested"
  | "run.awaiting_approval"
  | "run.resumed"
  | "model.completed"
  | "action.proposed"
  | "action.allowed"
  | "action.denied"
  | "approval.requested"
  | "approval.decided"
  | "action.executing"
  | "action.succeeded"
  | "action.failed"
  | "action.unknown"
  | "report.created"
  | "report.export_denied"
  | "report.safe_template_offered"
  | "control.evaluated"
  | "catalog.revision_rejected";

/** X-12: decision recorded with an event. */
export type SafeEventDecision =
  "allow" | "deny" | "approval_required" | "redact" | "approved" | "rejected";

/** X-12: a sanitized event. The summary is a closed set of masked metadata, never raw content. */
/** Fixed kinds of a rejected final answer (X-13 rejectionCause). */
export type RejectionCause =
  "not_json" | "extra_text" | "code_fence" | "wrong_status" | "wrong_fields" | "unknown_report";

export interface SafeEvent {
  /** Decimal string of the event cursor. */
  eventId: string;
  organizationId: string;
  runId: string | null;
  actionId: string | null;
  eventType: SafeEventType;
  decision: SafeEventDecision | null;
  reasonCode: ReasonCode | null;
  catalogRevisionId: number | null;
  maskedSummary: {
    purpose: "agent" | "security" | null;
    admissionCatalogRevisionId: number | null;
    matchedRule: string | null;
    feedRevision: string | null;
    reportId: string | null;
    template: ReportTemplate | null;
    classification: "internal_only" | "vendor_shareable" | null;
    lineageCheck: "passed" | "failed" | "missing" | null;
    effect: "none" | "read" | "report_created" | "outbox_message_queued" | null;
    replaySource: string | null;
    alternativeTemplate: ReportTemplate | null;
    safeMessage: string | null;
    /** The verified operator who caused the event when it is not the run's agent (an evaluation). */
    actorId: string | null;
    /** "judge" for a control evaluation of submitted input (GO-82); null for agent decisions. */
    inputSource: "judge" | null;
    /** The evaluation id of a control evaluation; its control assessments carry the same id. */
    evaluationId: string | null;
    /** Why a final answer was rejected (fixed kind, never its text); null on every other event. */
    rejectionCause: RejectionCause | null;
  };
  occurredAt: string;
}

/**
 * X-10: approve or reject one stored action (POST /internal/actions/{actionId}/approval). It carries
 * no replacement payload; the reviewer and organization come from the verified operator context.
 */
export interface ApprovalDecision {
  decision: "approve" | "reject";
}

/**
 * GO-44 (lane w3), the answer to an ApprovalDecision: the stored grant's references only. The grant,
 * the action's status, the approval.decided event and the continuation job committed together.
 */
export interface ApprovalResponse {
  approvalId: string;
  actionId: string;
  runId: string;
  decision: "approve" | "reject";
}

/** The exact destination a reviewer approves: the registered reporting address of the vendor. */
export interface ReviewedRecipient {
  reference: string;
  vendor_id: string;
  address: string;
}

/** One lineage source of the reviewed report, with the version it was rendered from. */
export interface ReviewedSource {
  kind: "invoice";
  id: string;
  version: number;
  classification: "internal_only" | "vendor_shareable";
  consumed_fields: string[];
}

/** The stored, server-rendered report exactly as it would be queued, with its source manifest. */
export interface ReviewedReport {
  id: string;
  version: number;
  template: ReportTemplate;
  template_version: number;
  projection_rule: string | null;
  projection_rule_version: number | null;
  classification: "internal_only" | "vendor_shareable";
  content_hash: string;
  content: string;
  sources: ReviewedSource[];
  source_manifest_digest: string;
}

/**
 * GO-43/GO-44 (lane w3), `GET /internal/actions/{actionId}/review`: the review material frozen before
 * review (Go's policy.ReviewPayload), for a reviewer of the organization. Keys are snake_case because
 * the stored SHA-256 digest of the frozen payload is computed over exactly this encoding. A
 * queue_report review always has the recipient and the report; other tools have neither.
 */
export interface ReviewView {
  canonicalization_version: number;
  action_id: string;
  run_id: string;
  passport_id: string;
  policy_revision_id: number;
  tool: ToolName;
  canonical_arguments: Record<string, unknown>;
  recipient: ReviewedRecipient | null;
  report: ReviewedReport | null;
  expires_at: string;
}

/** X-91: the boundary an evaluated interaction is checked at. */
export type ControlBoundary = "model_input" | "tool_result" | "action_proposal";

/**
 * X-91: POST /internal/control/evaluate. One interaction of an admitted run, evaluated through the
 * same gates as the agent path. Identity comes from the verified operator context, never from the
 * body; the caller cannot issue a grant; the call never dispatches the agent model.
 */
export interface ControlEvaluationRequest {
  runId: string;
  kind: ControlBoundary;
  /** Untrusted text for model_input and tool_result; null for action_proposal. */
  text: string | null;
  /** The tool a tool_result is attributed to, or the proposed tool; null for model_input. */
  tool: ToolName | null;
  /** The proposed tool's snake_case arguments (X-09) for action_proposal; otherwise null. */
  arguments: Record<string, unknown> | null;
}

/** X-91: one control that ran for an evaluation. */
export interface ControlEvaluationControl {
  boundary: ControlBoundary;
  controlClass: "deterministic" | "semantic";
  control: string;
  outcome: "pass" | "redact" | "block" | "error" | "not_applicable";
  reasonCode: string | null;
  ruleId: string | null;
  feedRevision: string | null;
}

/**
 * X-91: the decision for one evaluated interaction (HTTP 200 for every decision). Evaluated actions
 * are decisions only: nothing is stored as an action or executed, so actionId is always null.
 */
export interface ControlEvaluationResponse {
  evaluationId: string;
  runId: string;
  actionId: null;
  decision: "allow" | "deny" | "redact" | "approval_required";
  reasonCode: ReasonCode | null;
  safeMessage: string;
  alternativeTemplate: ReportTemplate | null;
  controls: ControlEvaluationControl[];
  semantic: {
    source: "live" | "fixture";
    riskCategory: string;
    score: number;
    reasonCode: string;
  } | null;
  /** The server-redacted text, only when decision is redact; blocked text is never echoed. */
  content: { text: string } | null;
  catalog: { admissionRevisionId: number; activeRevisionId: number; feedRevisionId: number | null };
}

// Go-owned read contracts (lane w2), served by the gateway's private routes in
// services/gateway/internal/reads and internal/provenance. NestJS consumes them; the Go types are
// the source and decode these fixtures strictly.

/** X-29 draft (GO-24): model usage of one metered purpose, from recorded dispatches and the ledger. */
export interface PurposeUsage {
  purpose: "agent" | "security";
  /** Dispatch records committed before each call; split by outcome below. */
  dispatched: number;
  completed: number;
  failed: number;
  /** Usage could not be settled; the reservation stays held. Never shown as zero. */
  usageUnknown: number;
  /** Dispatched, no outcome recorded yet. */
  inFlight: number;
  /** Provider-reported tokens of settled calls. */
  settledTokens: number;
  /** Tokens reserved for calls in flight or with unknown usage. */
  heldTokens: number;
  usageUnknownReservations: number;
}

/** One token allowance of the ledger; limit is null for a purpose without its own sub-limit. */
export interface LedgerTokens {
  limit: number | null;
  reserved: number;
  used: number;
}

/** X-29 draft (GO-24): the run's model ledger, the authority for calls, tokens, time and slots. */
export interface ModelLedger {
  paused: boolean;
  /** The shared token total of both purposes. */
  tokens: LedgerTokens;
  agentTokens: LedgerTokens;
  securityTokens: LedgerTokens;
  /** Call limits and the calls counted at reservation (never refunded). */
  calls: {
    limit: number;
    agentLimit: number;
    securityLimit: number;
    agent: number;
    security: number;
  };
  requestTimeoutMilliseconds: number;
  maxConcurrentCalls: number;
  /** Held slots: calls in flight or with unknown usage. */
  callsInFlight: number;
}

/**
 * WEB-29, `GET /internal/catalog/active` (Go-owned read): the active control catalog as the gateway
 * enforces it. Never carries policy or feed text, a signature pattern or a secret.
 */
export interface CatalogStatus {
  /** Null until a revision has been activated. */
  activeRevisionId: number | null;
  requestedRevisionId: number | null;
  validatedRevisionId: number | null;
  /** SHA-256 hex of the active policy file as imported. */
  policyDigest: string | null;
  /** The feed fields are null when no signature feed is bound to the active revision. */
  feedRevisionId: number | null;
  feedRevision: string | null;
  feedDigest: string | null;
  feedRuleCount: number | null;
  /** The last rejected activation; the last good revision stays active. Null when none failed. */
  lastError: { code: string; message: string; revisionId: number; stage: string } | null;
  /** All three registered controls when a revision is active, each with its enabled flag. */
  controls: CatalogControl[];
  disabledRules: string[];
}

/** One registered control's setting in the active catalog revision. */
export interface CatalogControl {
  controlId: "secret_pattern" | "semantic_injection" | "signature_match";
  controlClass: "deterministic" | "semantic";
  enabled: boolean;
  /** Null for signature_match (each feed rule carries its own response) and when disabled. */
  mode: "block" | "redact" | null;
  /** Set on the semantic control only. */
  threshold: number | null;
  boundaries: ("model_input" | "tool_result" | "action_proposal")[];
}

/** X-29 draft (GO-24), `GET /internal/runs/{runId}/usage`: no estimate and no cost (local model). */
export interface RunUsage {
  runId: string;
  /** Always two entries, "agent" then "security". */
  modelCalls: PurposeUsage[];
  /** Null when the run has no ledger yet. */
  ledger: ModelLedger | null;
  /** Tool execution attempts by outcome; open attempts have no recorded outcome yet. */
  toolAttempts: { total: number; succeeded: number; failed: number; aborted: number; open: number };
}

/** X-30 (GO-24), `GET /internal/runs/{runId}/events?after=&limit=`: X-12 events in id order. */
export interface RunEventsPage {
  events: SafeEvent[];
  /** The id to pass as `after` next; equals the request's cursor when nothing new was committed. */
  nextCursor: string;
}

/** GO-83: the decided semantic verdict schema, and only it. */
export interface VerdictSummary {
  risk_category: string;
  score: number;
  reason_code: string;
}

/** GO-83: one control assessment: stable codes and revisions, never inspected text. */
export interface AssessmentRecord {
  assessmentId: string;
  runId: string;
  evaluationId: string;
  actionId: string | null;
  securityModelCallId: string | null;
  boundary: string;
  controlClass: "deterministic" | "semantic";
  controlId: string;
  outcome: string;
  /** The control's own stable code: an X-13 code, or a control-level one such as no_free_text_arguments. */
  reasonCode: string | null;
  admissionCatalogRevisionId: number;
  evaluatedCatalogRevisionId: number;
  matchedRuleId: string | null;
  feedRevision: string | null;
  /** "live" or "fixture" on a semantic record that classified something; null on a deterministic one and on an unclassified (not_applicable) semantic one. */
  verdictSource: "live" | "fixture" | null;
  verdict: VerdictSummary | null;
  /** "judge" for a judge probe's evidence (GO-82), null for the run's own. */
  inputSource: "judge" | null;
  assessedAt: string;
}

/**
 * GO-83, `GET /internal/security/assessments?cursor=&limit=`: a window-cursor page. Every committed
 * record is returned exactly once; order is by id within a window, so sort by id for display.
 */
export interface AssessmentPage {
  records: AssessmentRecord[];
  /** Opaque "v1.<low>.<high>.<afterId>"; pass it back as `cursor`. */
  nextCursor: string;
}

/** GO-83, `GET /internal/security/events?cursor=&limit=`: the organization's X-12 events, windowed. */
export interface SecurityEventPage {
  events: SafeEvent[];
  nextCursor: string;
}

/** GO-83: one row of the security summary's event counts. */
export interface DecisionCount {
  eventType: SafeEventType;
  decision: SafeEventDecision | null;
  reasonCode: ReasonCode | null;
  inputSource: "judge" | null;
  /** The fixed kind of a rejected final answer (GO-26), null on every other event; never model text. */
  rejectionCause:
    | "not_json"
    | "extra_text"
    | "code_fence"
    | "wrong_status"
    | "wrong_fields"
    | "unknown_report"
    | null;
  count: number;
}

/** GO-83: one row of the security summary's assessment counts. */
export interface AssessmentCount {
  controlClass: "deterministic" | "semantic";
  controlId: string;
  outcome: string;
  verdictSource: "live" | "fixture" | null;
  inputSource: "judge" | null;
  count: number;
}

/** GO-83: observed spans of one measured phase, in microseconds. */
export interface PhaseTiming {
  phase: string;
  count: number;
  failed: number;
  medianMicroseconds: number;
  p95Microseconds: number;
  maxMicroseconds: number;
}

/** X-93 input (GO-83), `GET /internal/security/summary`: every count comes from stored records. */
export interface SecuritySummary {
  organizationId: string;
  generatedAt: string;
  runs: { status: RunStatus; count: number }[];
  decisions: DecisionCount[];
  assessments: AssessmentCount[];
  /** Every model call of the organization, judge probes' security calls included. */
  modelUsage: PurposeUsage[];
  /** The security calls behind judge probes, counted on their own. */
  judgeSecurityCalls: number;
  timings: PhaseTiming[];
}

/** X-64 (GO-37): one trusted source of a report: references and labels, never source values. */
export interface LineageSummary {
  sourceKind: "invoice";
  sourceId: string;
  sourceVersion: number;
  classification: "internal_only" | "vendor_shareable";
  consumedFields: string[];
}

/**
 * X-64 (GO-37), `GET /internal/runs/{runId}/reports/{reportId}`: the stored report with its stored
 * label; the interface reads the classification instead of computing one.
 */
export interface ReportView {
  reportId: string;
  runId: string;
  version: number;
  template: ReportTemplate;
  templateVersion: number;
  projectionRule: "vendor_invoice_fields_v1" | null;
  projectionRuleVersion: number | null;
  classification: "internal_only" | "vendor_shareable";
  destinationClass: "internal_reviewers" | "registered_vendor_recipient";
  title: string;
  contentHash: string;
  /** Null exactly when contentWithheld is true. */
  content: string | null;
  contentWithheld: boolean;
  lineage: LineageSummary[];
}
