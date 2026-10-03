// Generic wire contracts shared by the web app, the API and the Go gateway.
// Keep this file in sync with schemas/*.schema.json and the Go DTOs in
// services/gateway/internal/health/dto.go. `pnpm test` checks the shared fixtures.

/** Header used to correlate one request across web, API and gateway. */
export const REQUEST_ID_HEADER = "x-request-id";

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
  invoices: { id: string; number: string; date: string; amount: number }[];
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
  | "limit_not_allowed";

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
