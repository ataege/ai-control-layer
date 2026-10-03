import { fetchJson, postJson, FetchJsonResult, FetchJsonOptions } from "./fetch-json";
import type {
  RunEventsPage,
  RunState,
  RunUsage,
  StartRunRequest,
  StartRunResponse,
  TaskFormOptions,
} from "@workspace/contracts";

export const REASON_CODE_MESSAGES: Record<string, string> = {
  resource_out_of_scope: "The requested resource is outside the authorized scope.",
  destination_not_allowed: "The destination is not allowed by policy.",
  report_export_restricted: "The report export is restricted due to its classification.",
  report_lineage_missing: "Required report lineage metadata is missing.",
  source_policy_changed: "The underlying policy for this source has changed.",
  template_not_allowed: "The requested template is not permitted for this operation.",
  approval_required: "This action requires explicit approval.",
  approval_expired: "The approval for this action has expired.",
  action_changed: "The proposed action was modified after approval.",
  resource_version_changed: "The resource was updated concurrently.",
  allowance_exhausted: "The operation budget or allowance was exhausted.",
  run_cancelled: "The run was cancelled by an operator.",
  outcome_unknown: "The outcome of the operation is unknown. It remains unconfirmed.",
  semantic_injection_detected: "Semantic security analysis detected an injection attempt.",
  security_evaluator_unavailable: "The semantic security evaluator is currently unavailable.",
  security_allowance_exhausted: "The allowance for semantic security evaluation was exhausted.",
  content_redacted: "Sensitive content was redacted.",
  signature_match: "The content matched a known attack signature.",
  policy_reload_rejected: "The provided policy reload was rejected by validation.",
  model_not_allowed: "The requested model is not allowed.",
  upstream_unreachable: "The upstream API could not be reached.",
  upstream_timeout: "The upstream API did not respond in time. The outcome remains unconfirmed.",
  invalid_json: "The response was not valid JSON.",
  configuration_error: "A server configuration error occurred.",
};

import { type FetchJsonError } from "./fetch-json";

export function getSafeMessage(error: FetchJsonError | string): string {
  if (typeof error === "string") {
    return REASON_CODE_MESSAGES[error] || "An unknown error occurred.";
  }
  if (error.kind === "http") {
    const code = (error.body as { error?: { code?: string } })?.error?.code;
    if (code === "unauthorized") return "Invalid credentials.";
    if (code) return REASON_CODE_MESSAGES[code] || "An unknown error occurred.";
    if (error.status === 401) return "Invalid credentials.";
  }
  if (error.kind === "network") return "The server could not be reached.";
  if (error.kind === "timeout") return "The request timed out.";
  return REASON_CODE_MESSAGES[error.kind] || "An unknown error occurred.";
}

export function getErrorCode(error: FetchJsonError | string): string | undefined {
  if (typeof error === "string") return error;
  if (error.kind === "http") {
    const code = (error.body as { error?: { code?: string } })?.error?.code;
    return code || (error.status === 401 ? "unauthorized" : undefined);
  }
  return error.kind;
}

// Type Guards for frozen contracts
function isStartRunResponse(data: unknown): data is StartRunResponse {
  return (
    typeof data === "object" &&
    data !== null &&
    "runId" in data &&
    typeof (data as Record<string, unknown>).runId === "string" &&
    "passportId" in data &&
    typeof (data as Record<string, unknown>).passportId === "string"
  );
}

function isTaskFormOptions(data: unknown): data is TaskFormOptions {
  return typeof data === "object" && data !== null && "templates" in data && "vendors" in data;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

/**
 * The X-11 run state the API returns for GET /api/runs/:id. The status is only required to be a
 * string: an unknown status is shown as unknown by the page, never rejected here or shown as success.
 */
export function isRunState(data: unknown): data is RunState {
  if (!isRecord(data)) return false;
  const reference = data.resultReference;
  return (
    typeof data.runId === "string" &&
    typeof data.passportId === "string" &&
    typeof data.status === "string" &&
    (data.terminalReason === null || typeof data.terminalReason === "string") &&
    (reference === null ||
      (isRecord(reference) &&
        Array.isArray(reference.reportIds) &&
        reference.reportIds.every((reportId) => typeof reportId === "string")))
  );
}

function isLedgerTokens(value: unknown): boolean {
  return (
    isRecord(value) &&
    (value.limit === null || isNumber(value.limit)) &&
    isNumber(value.reserved) &&
    isNumber(value.used)
  );
}

function isPurposeUsage(value: unknown): boolean {
  return (
    isRecord(value) &&
    (value.purpose === "agent" || value.purpose === "security") &&
    [
      "dispatched",
      "completed",
      "failed",
      "usageUnknown",
      "inFlight",
      "settledTokens",
      "heldTokens",
      "usageUnknownReservations",
    ].every((field) => isNumber(value[field]))
  );
}

/** The X-29 usage the API returns for GET /api/runs/:id/usage. */
export function isRunUsage(data: unknown): data is RunUsage {
  if (!isRecord(data)) return false;
  const { ledger, toolAttempts } = data;
  return (
    typeof data.runId === "string" &&
    Array.isArray(data.modelCalls) &&
    data.modelCalls.every(isPurposeUsage) &&
    (ledger === null ||
      (isRecord(ledger) &&
        typeof ledger.paused === "boolean" &&
        isLedgerTokens(ledger.tokens) &&
        isLedgerTokens(ledger.agentTokens) &&
        isLedgerTokens(ledger.securityTokens) &&
        isRecord(ledger.calls) &&
        isNumber(ledger.calls.limit) &&
        isNumber(ledger.requestTimeoutMilliseconds) &&
        isNumber(ledger.maxConcurrentCalls) &&
        isNumber(ledger.callsInFlight))) &&
    isRecord(toolAttempts) &&
    ["total", "succeeded", "failed", "aborted", "open"].every((field) =>
      isNumber(toolAttempts[field]),
    )
  );
}

/** The X-30 event page the API returns for GET /api/runs/:id/events. */
export function isRunEventsPage(data: unknown): data is RunEventsPage {
  return (
    isRecord(data) &&
    typeof data.nextCursor === "string" &&
    Array.isArray(data.events) &&
    data.events.every(
      (event) =>
        isRecord(event) &&
        typeof event.eventId === "string" &&
        typeof event.eventType === "string" &&
        typeof event.occurredAt === "string" &&
        isRecord(event.maskedSummary),
    )
  );
}

// Ensure the result matches the guard or return an invalid_json error.
function enforceGuard<T>(
  result: FetchJsonResult<unknown>,
  guard: (data: unknown) => data is T,
): FetchJsonResult<T> {
  if (!result.ok) {
    return result as FetchJsonResult<T>;
  }

  if (result.data === undefined) {
    return {
      ok: false,
      error: { kind: "invalid_json", status: result.status },
      durationMs: result.durationMs,
      requestId: result.requestId,
    };
  }

  if (guard(result.data)) {
    return result as FetchJsonResult<T>;
  }

  return {
    ok: false,
    error: { kind: "invalid_json", status: result.status },
    durationMs: result.durationMs,
    requestId: result.requestId,
  };
}

export class ProductClient {
  static async startRun(request: StartRunRequest): Promise<FetchJsonResult<StartRunResponse>> {
    const result = await postJson("/api/runs", request);
    return enforceGuard(result, isStartRunResponse);
  }

  static async getOptions(): Promise<FetchJsonResult<TaskFormOptions>> {
    const result = await fetchJson("/api/runs/options");
    return enforceGuard(result, isTaskFormOptions);
  }

  /** The persisted run state (X-11), exactly as the API returns it. */
  static async getRun(id: string, options?: FetchJsonOptions): Promise<FetchJsonResult<RunState>> {
    const result = await fetchJson(`/api/runs/${encodeURIComponent(id)}`, options);
    return enforceGuard(result, isRunState);
  }

  /** The run's model usage and allowance ledger (X-29). */
  static async getUsage(
    id: string,
    options?: FetchJsonOptions,
  ): Promise<FetchJsonResult<RunUsage>> {
    const result = await fetchJson(`/api/runs/${encodeURIComponent(id)}/usage`, options);
    return enforceGuard(result, isRunUsage);
  }

  /** One page of sanitized events after the cursor (the API's query parameter is `after`). */
  static async getRunEvents(
    id: string,
    after?: string,
    options?: FetchJsonOptions,
  ): Promise<FetchJsonResult<RunEventsPage>> {
    const query = after ? `?after=${encodeURIComponent(after)}` : "";
    const result = await fetchJson(`/api/runs/${encodeURIComponent(id)}/events${query}`, options);
    return enforceGuard(result, isRunEventsPage);
  }

  static async signIn(credentials: {
    email: string;
    password: string;
  }): Promise<FetchJsonResult<{ message: string }>> {
    const result = await postJson("/api/auth/sign-in", credentials);
    return enforceGuard(
      result,
      (data): data is { message: string } =>
        typeof data === "object" && data !== null && "message" in data,
    );
  }

  static async getMe(): Promise<
    FetchJsonResult<{
      id: string;
      email: string;
      name: string;
      organizationId: string;
      roles: string[];
    }>
  > {
    const result = await fetchJson("/api/auth/me");
    return enforceGuard(
      result,
      (
        data,
      ): data is {
        id: string;
        email: string;
        name: string;
        organizationId: string;
        roles: string[];
      } => typeof data === "object" && data !== null && "id" in data && "name" in data,
    );
  }
}
