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

/** Resolved operator context sent to Go with every governed command. */
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
  status: 'pending' | 'running' | 'paused' | 'failed' | 'completed';
  usage: {
    modelCalls: number;
    cost: number;
  };
  terminalReason?: string;
  passport: {
    id: string;
    template: string;
    vendorId?: string;
    invoiceIds: string[];
    destination: string;
    approvalRequirement?: string;
    limits: {
      modelCalls: number;
      timeoutSeconds: number;
    };
    versions: {
      task: string;
      policy: string;
    };
    rules: string[];
    expiresAt: string;
  };
}

/** Sanitized Event (X-12). */
export interface SanitizedEvent {
  id: string;
  type: string;
  timestamp: string;
  details: Record<string, unknown>;
}
