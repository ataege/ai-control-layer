// Turns raw responses from the three proxy routes into what the diagnostics page shows.
// Pure functions: a "healthy" state is only ever produced from a matching response body.

import type {
  ApiReadinessResponse,
  DiagnosticCheck,
  DiagnosticFailureReason,
  ErrorResponse,
  GatewayDiagnosticsResponse,
  LivenessResponse,
} from "@workspace/contracts";

import type { FetchJsonResult } from "@/lib/fetch-json";

export type ServiceCheckState = "healthy" | "degraded" | "unavailable" | "error";

export interface ServiceCheckFact {
  label: string;
  value: string;
}

export interface ServiceCheckOutcome {
  state: ServiceCheckState;
  summary: string;
  /** HTTP status of the response the browser received, when there was one. */
  httpStatus?: number;
  /** Row label for `httpStatus` when the response covers more than this one check. */
  httpStatusLabel?: string;
  requestId?: string;
  latencyMs?: number;
  /** Says who measured `latencyMs`. */
  latencyLabel?: string;
  facts: ServiceCheckFact[];
}

const FAILURE_REASON_TEXT: Record<DiagnosticFailureReason, string> = {
  timeout: "The request timed out.",
  unreachable: "The service could not be reached.",
  unauthorized: "The service rejected the service token.",
  not_ready: "The service reported that it is not ready.",
  unexpected_response: "The service returned an unexpected response.",
};

const BROWSER_LATENCY_LABEL = "Round trip (browser)";
const API_LATENCY_LABEL = "Latency (measured by API)";
// Both gateway cards share one diagnostics response, so its status is not the check's own.
const DIAGNOSTICS_STATUS_LABEL = "Diagnostics response status";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isLivenessResponse(value: unknown): value is LivenessResponse {
  return isRecord(value) && value.status === "ok" && typeof value.service === "string";
}

function isApiReadinessResponse(value: unknown): value is ApiReadinessResponse {
  return isRecord(value) && typeof value.status === "string" && isRecord(value.details);
}

function isDiagnosticCheck(value: unknown): value is DiagnosticCheck {
  return (
    isRecord(value) &&
    (value.status === "up" || value.status === "down") &&
    typeof value.latencyMs === "number"
  );
}

function isGatewayDiagnosticsResponse(value: unknown): value is GatewayDiagnosticsResponse {
  return (
    isRecord(value) &&
    (value.status === "ok" || value.status === "degraded" || value.status === "unavailable") &&
    isRecord(value.checks) &&
    isDiagnosticCheck(value.checks.reachability) &&
    isDiagnosticCheck(value.checks.databaseReadiness)
  );
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  return (
    isRecord(value) &&
    isRecord(value.error) &&
    typeof value.error.code === "string" &&
    typeof value.error.message === "string"
  );
}

/** The response body regardless of status: 503 bodies carry health reports too. */
function responseBody(result: FetchJsonResult<unknown>): unknown {
  if (result.ok) {
    return result.data;
  }
  return result.error.kind === "http" ? result.error.body : undefined;
}

function responseStatus(result: FetchJsonResult<unknown>): number | undefined {
  if (result.ok) {
    return result.status;
  }
  return result.error.kind === "http" || result.error.kind === "invalid_json"
    ? result.error.status
    : undefined;
}

/** Outcome for every result that is not a recognised health report. */
function describeFailure(result: FetchJsonResult<unknown>): ServiceCheckOutcome {
  const sharedFields = {
    httpStatus: responseStatus(result),
    requestId: result.requestId,
    latencyMs: result.durationMs,
    latencyLabel: BROWSER_LATENCY_LABEL,
  };

  if (result.ok) {
    return {
      ...sharedFields,
      state: "error",
      summary: "The response did not match the expected format.",
      facts: [],
    };
  }

  switch (result.error.kind) {
    case "network":
      return {
        ...sharedFields,
        state: "error",
        summary: "The request did not reach the web server.",
        facts: [],
      };
    case "timeout":
      return {
        ...sharedFields,
        state: "error",
        summary: `No response within ${result.error.timeoutMs} ms.`,
        facts: [],
      };
    case "aborted":
      return { ...sharedFields, state: "error", summary: "The request was cancelled.", facts: [] };
    case "invalid_json":
      return {
        ...sharedFields,
        state: "error",
        summary: "The response was not valid JSON.",
        facts: [],
      };
    case "http": {
      const errorBody = result.error.body;
      if (isErrorResponse(errorBody)) {
        // 502 and 504 from the proxy mean the service behind it is not answering.
        const isUpstreamFailure = result.error.status === 502 || result.error.status === 504;
        return {
          ...sharedFields,
          requestId: result.requestId ?? errorBody.requestId,
          state: isUpstreamFailure ? "unavailable" : "error",
          summary: errorBody.error.message,
          facts: [{ label: "Error code", value: errorBody.error.code }],
        };
      }
      return {
        ...sharedFields,
        state: "error",
        summary: `Unexpected response with HTTP status ${result.error.status}.`,
        facts: [],
      };
    }
  }
}

/** API process liveness from GET /api/health/live. */
export function describeApiLiveness(result: FetchJsonResult<unknown>): ServiceCheckOutcome {
  if (result.ok && isLivenessResponse(result.data) && result.data.service === "api") {
    return {
      state: "healthy",
      summary: "The API process is running.",
      httpStatus: result.status,
      requestId: result.requestId,
      latencyMs: result.durationMs,
      latencyLabel: BROWSER_LATENCY_LABEL,
      facts: [{ label: "Service", value: result.data.service }],
    };
  }
  return describeFailure(result);
}

/** API readiness (its PostgreSQL connection) from GET /api/health/ready. */
export function describeApiReadiness(result: FetchJsonResult<unknown>): ServiceCheckOutcome {
  const body = responseBody(result);
  if (!isApiReadinessResponse(body)) {
    return describeFailure(result);
  }

  const indicatorEntries = Object.entries(body.details);
  const facts = indicatorEntries.map(([indicatorName, indicator]) => ({
    label: `Dependency: ${indicatorName}`,
    value: indicator.message ? `${indicator.status} (${indicator.message})` : indicator.status,
  }));
  const allIndicatorsUp =
    indicatorEntries.length > 0 &&
    indicatorEntries.every(([, indicator]) => indicator.status === "up");
  const isReady = result.ok && body.status === "ok" && allIndicatorsUp;

  return {
    state: isReady ? "healthy" : "unavailable",
    summary: isReady
      ? "The API is ready and its database connection works."
      : body.status === "shutting_down"
        ? "The API is shutting down."
        : "The API is not ready: a dependency check failed.",
    httpStatus: responseStatus(result),
    requestId: result.requestId,
    latencyMs: result.durationMs,
    latencyLabel: BROWSER_LATENCY_LABEL,
    facts,
  };
}

function gatewayCheckFacts(check: DiagnosticCheck): ServiceCheckFact[] {
  const facts: ServiceCheckFact[] = [];
  if (check.upstreamStatus !== undefined) {
    facts.push({ label: "Gateway HTTP status", value: String(check.upstreamStatus) });
  }
  if (check.reason) {
    facts.push({ label: "Reason", value: check.reason });
  }
  return facts;
}

/** Authenticated gateway ping, reported by GET /api/diagnostics/gateway. */
export function describeGatewayReachability(result: FetchJsonResult<unknown>): ServiceCheckOutcome {
  const body = responseBody(result);
  if (!isGatewayDiagnosticsResponse(body)) {
    return describeFailure(result);
  }

  const reachabilityCheck = body.checks.reachability;
  const isReachable = reachabilityCheck.status === "up";
  return {
    state: isReachable ? "healthy" : "unavailable",
    summary: isReachable
      ? "The API reached the gateway with its service token."
      : FAILURE_REASON_TEXT[reachabilityCheck.reason ?? "unexpected_response"],
    httpStatus: responseStatus(result),
    httpStatusLabel: DIAGNOSTICS_STATUS_LABEL,
    requestId: result.requestId ?? body.requestId,
    latencyMs: reachabilityCheck.latencyMs,
    latencyLabel: API_LATENCY_LABEL,
    facts: gatewayCheckFacts(reachabilityCheck),
  };
}

/** Gateway database readiness, reported by GET /api/diagnostics/gateway. */
export function describeGatewayDatabase(result: FetchJsonResult<unknown>): ServiceCheckOutcome {
  const body = responseBody(result);
  if (!isGatewayDiagnosticsResponse(body)) {
    return describeFailure(result);
  }

  const readinessCheck = body.checks.databaseReadiness;
  const isReady = readinessCheck.status === "up";
  // Degraded: the gateway itself answers, only its database dependency is failing.
  const failureState = body.status === "degraded" ? "degraded" : "unavailable";
  return {
    state: isReady ? "healthy" : failureState,
    summary: isReady
      ? "The gateway is ready and its database connection works."
      : FAILURE_REASON_TEXT[readinessCheck.reason ?? "unexpected_response"],
    httpStatus: responseStatus(result),
    httpStatusLabel: DIAGNOSTICS_STATUS_LABEL,
    requestId: result.requestId ?? body.requestId,
    latencyMs: readinessCheck.latencyMs,
    latencyLabel: API_LATENCY_LABEL,
    facts: gatewayCheckFacts(readinessCheck),
  };
}
