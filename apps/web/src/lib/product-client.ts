import { classifyFailure } from "./errors/failure";
import { isReasonCode, REASON_FAILURES } from "./errors/reason-failures";
import {
  fetchJson,
  postJson,
  type FetchJsonError,
  type FetchJsonOptions,
  type FetchJsonResult,
} from "./fetch-json";
import type {
  RunEventsPage,
  RunState,
  RunUsage,
  StartRunRequest,
  StartRunResponse,
  TaskFormOptions,
} from "@workspace/contracts";

const UNKNOWN_MESSAGE = "The request failed in a way this page does not recognize.";

/**
 * The fixed operator message for a failure. An X-13 reason code (given directly or found in an error
 * body) gets the gateway's safe text for that code from the one typed mapping, WEB-23's
 * REASON_FAILURES; anything else gets the shared classification's words. A server message is never
 * shown.
 */
export function getSafeMessage(error: FetchJsonError | string): string {
  if (typeof error === "string") {
    return isReasonCode(error) ? REASON_FAILURES[error].message : UNKNOWN_MESSAGE;
  }
  return classifyFailure(error).description;
}

// Type Guards for frozen contracts
const isNonEmptyString = (value: unknown): value is string =>
  typeof value === "string" && value !== "";

/** True when the record has exactly these keys: the contracts are `additionalProperties: false`. */
function hasExactlyKeys(record: Record<string, unknown>, keys: string[]): boolean {
  const actual = Object.keys(record);
  return actual.length === keys.length && keys.every((key) => key in record);
}

const isArrayOf = (value: unknown, guard: (item: unknown) => boolean): boolean =>
  Array.isArray(value) && value.every(guard);

/** X-07: the start-run answer is exactly the run and passport ids Go issued. */
export function isStartRunResponse(data: unknown): data is StartRunResponse {
  return (
    isRecord(data) &&
    hasExactlyKeys(data, ["runId", "passportId"]) &&
    isNonEmptyString(data.runId) &&
    isNonEmptyString(data.passportId)
  );
}

const isIdAndName = (item: unknown): boolean =>
  isRecord(item) &&
  hasExactlyKeys(item, ["id", "name"]) &&
  isNonEmptyString(item.id) &&
  isNonEmptyString(item.name);

const isOfferedInvoice = (item: unknown): boolean =>
  isRecord(item) &&
  hasExactlyKeys(item, ["id", "number", "date", "amount", "vendorId", "currency"]) &&
  isNonEmptyString(item.id) &&
  typeof item.number === "string" &&
  typeof item.date === "string" &&
  /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/.test(item.date) &&
  Number.isInteger(item.amount) &&
  isNonEmptyString(item.vendorId) &&
  typeof item.currency === "string" &&
  /^[A-Z]{3}$/.test(item.currency);

const isApprovalRequirement = (item: unknown): boolean =>
  isRecord(item) &&
  hasExactlyKeys(item, ["id", "description"]) &&
  isNonEmptyString(item.id) &&
  isNonEmptyString(item.description);

const isPositiveInteger = (value: unknown): boolean =>
  Number.isInteger(value) && Number(value) >= 1;

/** The task form options (API-12): every list and both limits, in the contract's exact shape. */
export function isTaskFormOptions(data: unknown): data is TaskFormOptions {
  return (
    isRecord(data) &&
    hasExactlyKeys(data, [
      "templates",
      "vendors",
      "invoices",
      "destinations",
      "approvalRequirements",
      "limits",
    ]) &&
    isArrayOf(data.templates, isIdAndName) &&
    isArrayOf(data.vendors, isIdAndName) &&
    isArrayOf(data.invoices, isOfferedInvoice) &&
    isArrayOf(data.destinations, isIdAndName) &&
    isArrayOf(data.approvalRequirements, isApprovalRequirement) &&
    isRecord(data.limits) &&
    hasExactlyKeys(data.limits, ["maxModelCalls", "maxTimeoutSeconds"]) &&
    isPositiveInteger(data.limits.maxModelCalls) &&
    isPositiveInteger(data.limits.maxTimeoutSeconds)
  );
}

/** What `GET /api/auth/me` returns: the verified session's user, organization and roles. */
export interface OperatorProfile {
  id: string;
  email: string;
  name: string;
  organizationId: string;
  roles: string[];
  /** The server's own mark of the development identity, when it sends one. */
  developmentDemonstration?: boolean;
}

export function isOperatorProfile(data: unknown): data is OperatorProfile {
  if (!isRecord(data)) return false;
  const requiredKeys = ["id", "email", "name", "organizationId", "roles"];
  const allowedKeys = [...requiredKeys, "developmentDemonstration"];
  return (
    requiredKeys.every((key) => key in data) &&
    Object.keys(data).every((key) => allowedKeys.includes(key)) &&
    isNonEmptyString(data.id) &&
    isNonEmptyString(data.email) &&
    typeof data.name === "string" &&
    isNonEmptyString(data.organizationId) &&
    isArrayOf(data.roles, (role) => typeof role === "string") &&
    (data.developmentDemonstration === undefined ||
      typeof data.developmentDemonstration === "boolean")
  );
}

export function isSignInResponse(data: unknown): data is { message: string } {
  return isRecord(data) && hasExactlyKeys(data, ["message"]) && typeof data.message === "string";
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
    return enforceGuard(result, isSignInResponse);
  }

  /** Ends the session. A success may carry no body (200 or 204), so there is no guard to apply. */
  static async signOut(): Promise<FetchJsonResult<unknown>> {
    return postJson("/api/auth/sign-out", {});
  }

  static async getMe(): Promise<FetchJsonResult<OperatorProfile>> {
    const result = await fetchJson("/api/auth/me");
    return enforceGuard(result, isOperatorProfile);
  }
}
