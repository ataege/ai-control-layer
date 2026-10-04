// Client for the security posture dashboard (WEB-30) and the audit export (WEB-31). It talks only
// to the web's same-origin /api/security routes, never throws for a request failure, and gives
// every outcome back as a value. The summary is checked for its contract shape before it is shown:
// a response that does not fit is reported as such, never rendered or repaired.

import type {
  AssessmentPage,
  CatalogStatus,
  SecurityEventPage,
  SecuritySummary,
} from "@workspace/contracts";

import { fetchJson, type FetchJsonError } from "../fetch-json";

export const SECURITY_SUMMARY_URL = "/api/security/summary";
export const AUDIT_EXPORT_URL = "/api/security/export";
export const CATALOG_STATUS_URL = "/api/policies/catalog";

export const EXPORT_KINDS = ["events", "assessments"] as const;
export type ExportKind = (typeof EXPORT_KINDS)[number];
export const EXPORT_FORMATS = ["json", "csv"] as const;
export type ExportFormat = (typeof EXPORT_FORMATS)[number];

/** The API's page-size bounds for the export (1 to 500). */
export const EXPORT_LIMIT_MINIMUM = 1;
export const EXPORT_LIMIT_MAXIMUM = 500;
export const EXPORT_LIMIT_DEFAULT = 100;

const EXPORT_TIMEOUT_MS = 20_000;

/** Why a security request produced no data. Messages are fixed text; no upstream body is shown. */
export type SecurityFailure =
  | { kind: "unauthorized"; message: string }
  | { kind: "forbidden"; message: string }
  | { kind: "bad_request"; message: string }
  /** The route is not served (yet): a 404, or the web proxy refusing a path it does not forward. */
  | { kind: "not_available"; message: string }
  | { kind: "unavailable"; message: string; status?: number }
  | { kind: "network"; message: string }
  | { kind: "timeout"; message: string }
  | { kind: "invalid_response"; message: string };

const FAILURE_MESSAGES = {
  unauthorized: "Your session has ended. Sign in again to see the security posture.",
  forbidden: "The reviewer role is required for this view.",
  bad_request: "The request was not accepted. Check the values and try again.",
  not_available: "The API does not serve this information yet.",
  unavailable: "The security records are not available right now.",
  network: "The server could not be reached.",
  timeout: "The server did not answer in time.",
  invalid_response: "The server answered with data that does not match the security contract.",
} as const;

function failure(kind: SecurityFailure["kind"], status?: number): SecurityFailure {
  if (kind === "unavailable") return { kind, message: FAILURE_MESSAGES[kind], status };
  return { kind, message: FAILURE_MESSAGES[kind] } as SecurityFailure;
}

/** Maps an HTTP status to the failure the page shows. */
export function failureForStatus(status: number): SecurityFailure {
  if (status === 401) return failure("unauthorized");
  if (status === 403) return failure("forbidden");
  if (status === 400) return failure("bad_request");
  if (status === 404) return failure("not_available");
  return failure("unavailable", status);
}

function failureForFetchError(error: FetchJsonError): SecurityFailure {
  switch (error.kind) {
    case "http":
      return failureForStatus(error.status);
    case "network":
      return failure("network");
    case "timeout":
      return failure("timeout");
    case "invalid_json":
      return failure("invalid_response");
    case "aborted":
      return failure("network");
  }
}

// --- Summary --------------------------------------------------------------------------------

export type SummaryResult =
  | { ok: true; summary: SecuritySummary; requestId?: string; durationMs: number }
  | { ok: false; failure: SecurityFailure };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function everyRecord(
  value: unknown,
  check: (record: Record<string, unknown>) => boolean,
): value is Record<string, unknown>[] {
  return Array.isArray(value) && value.every((item) => isRecord(item) && check(item));
}

const isText = (value: unknown): value is string => typeof value === "string";
const isCount = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value) && value >= 0;
const isNullableText = (value: unknown): boolean => value === null || isText(value);

/** The structural check that stands between the network and the dashboard. */
export function isSecuritySummary(data: unknown): data is SecuritySummary {
  if (!isRecord(data)) return false;
  return (
    isText(data.organizationId) &&
    isText(data.generatedAt) &&
    isCount(data.judgeSecurityCalls) &&
    everyRecord(data.runs, (run) => isText(run.status) && isCount(run.count)) &&
    everyRecord(
      data.decisions,
      (row) =>
        isText(row.eventType) &&
        isNullableText(row.decision) &&
        isNullableText(row.reasonCode) &&
        isNullableText(row.inputSource) &&
        isCount(row.count),
    ) &&
    everyRecord(
      data.assessments,
      (row) =>
        isText(row.controlClass) &&
        isText(row.controlId) &&
        isText(row.outcome) &&
        isNullableText(row.verdictSource) &&
        isNullableText(row.inputSource) &&
        isCount(row.count),
    ) &&
    everyRecord(
      data.modelUsage,
      (row) =>
        isText(row.purpose) &&
        isCount(row.dispatched) &&
        isCount(row.completed) &&
        isCount(row.failed) &&
        isCount(row.usageUnknown) &&
        isCount(row.inFlight) &&
        isCount(row.settledTokens) &&
        isCount(row.heldTokens) &&
        isCount(row.usageUnknownReservations),
    ) &&
    everyRecord(
      data.timings,
      (row) =>
        isText(row.phase) &&
        isCount(row.count) &&
        isCount(row.failed) &&
        isCount(row.medianMicroseconds) &&
        isCount(row.p95Microseconds) &&
        isCount(row.maxMicroseconds),
    )
  );
}

export async function getSecuritySummary(
  options: { signal?: AbortSignal; fetchImplementation?: typeof fetch } = {},
): Promise<SummaryResult> {
  const result = await fetchJson<unknown>(SECURITY_SUMMARY_URL, options);
  if (!result.ok) return { ok: false, failure: failureForFetchError(result.error) };
  if (!isSecuritySummary(result.data)) return { ok: false, failure: failure("invalid_response") };
  return {
    ok: true,
    summary: result.data,
    requestId: result.requestId,
    durationMs: result.durationMs,
  };
}

// --- Active catalog status (WEB-29) -----------------------------------------------------------

export type CatalogStatusResult =
  | { ok: true; status: CatalogStatus; requestId?: string; durationMs: number }
  | { ok: false; failure: SecurityFailure };

const CONTROL_IDS = ["secret_pattern", "semantic_injection", "signature_match"];
const BOUNDARIES = ["model_input", "tool_result", "action_proposal"];
const isNullableCount = (value: unknown): boolean => value === null || isCount(value);

/** The structural check that stands between the network and the active-controls panel. */
export function isCatalogStatus(data: unknown): data is CatalogStatus {
  if (!isRecord(data)) return false;
  const lastError = data.lastError;
  return (
    isNullableCount(data.activeRevisionId) &&
    isNullableCount(data.requestedRevisionId) &&
    isNullableCount(data.validatedRevisionId) &&
    isNullableText(data.policyDigest) &&
    isNullableCount(data.feedRevisionId) &&
    isNullableText(data.feedRevision) &&
    isNullableText(data.feedDigest) &&
    isNullableCount(data.feedRuleCount) &&
    (lastError === null ||
      (isRecord(lastError) &&
        isText(lastError.code) &&
        isText(lastError.message) &&
        isCount(lastError.revisionId) &&
        isText(lastError.stage))) &&
    everyRecord(
      data.controls,
      (control) =>
        isText(control.controlId) &&
        CONTROL_IDS.includes(control.controlId) &&
        (control.controlClass === "deterministic" || control.controlClass === "semantic") &&
        typeof control.enabled === "boolean" &&
        (control.mode === null || control.mode === "block" || control.mode === "redact") &&
        (control.threshold === null || typeof control.threshold === "number") &&
        Array.isArray(control.boundaries) &&
        control.boundaries.every((boundary) => BOUNDARIES.includes(boundary as string)),
    ) &&
    Array.isArray(data.disabledRules) &&
    data.disabledRules.every(isText)
  );
}

/**
 * Reads the active catalog status. A 404, and the web proxy's own refusal of a path it does not
 * forward yet (500 configuration_error), both mean the route is not served: "not available", not an
 * outage. A 503 means the active revision cannot be enforced: unavailable, never shown as empty.
 */
export async function getCatalogStatus(
  options: { signal?: AbortSignal; fetchImplementation?: typeof fetch } = {},
): Promise<CatalogStatusResult> {
  const result = await fetchJson<unknown>(CATALOG_STATUS_URL, options);
  if (!result.ok) {
    if (result.error.kind === "http" && result.error.status === 500) {
      const body = result.error.body as { error?: { code?: string } } | undefined;
      if (body?.error?.code === "configuration_error") {
        return { ok: false, failure: failure("not_available") };
      }
    }
    return { ok: false, failure: failureForFetchError(result.error) };
  }
  if (!isCatalogStatus(result.data)) return { ok: false, failure: failure("invalid_response") };
  return {
    ok: true,
    status: result.data,
    requestId: result.requestId,
    durationMs: result.durationMs,
  };
}

// --- Audit export ---------------------------------------------------------------------------

export interface ExportParameters {
  kind: ExportKind;
  format: ExportFormat;
  /** The cursor of the previous page ("v1.<low>.<high>.<afterId>"), or empty for the first page. */
  after: string;
  /** The page size as typed; validated here. */
  limit: string;
}

/** The opaque window cursor the gateway issues; the API is the authority on its meaning. */
export const EXPORT_CURSOR_PATTERN = /^v1\.[0-9]{1,20}\.[0-9]{1,20}\.[0-9]{1,19}$/;

/** What is wrong with each typed field; a field is absent when its value is fine. */
export interface ExportFieldProblems {
  limit?: string;
  after?: string;
}

export type ExportQuery =
  | { ok: true; query: string }
  | { ok: false; field: "limit" | "after"; message: string; problems: ExportFieldProblems };

/** Checks every typed field on its own, so one wrong value never hides another. */
export function exportFieldProblems(parameters: ExportParameters): ExportFieldProblems {
  const problems: ExportFieldProblems = {};
  const limitText = parameters.limit.trim();
  const limit = /^[0-9]{1,3}$/.test(limitText) ? Number(limitText) : Number.NaN;
  if (!Number.isInteger(limit) || limit < EXPORT_LIMIT_MINIMUM || limit > EXPORT_LIMIT_MAXIMUM) {
    problems.limit = `Use a whole number from ${EXPORT_LIMIT_MINIMUM} to ${EXPORT_LIMIT_MAXIMUM}.`;
  }
  const after = parameters.after.trim();
  if (after !== "" && !EXPORT_CURSOR_PATTERN.test(after)) {
    problems.after =
      "Paste the cursor of the previous page, for example v1.0.0.0, or leave it empty.";
  }
  return problems;
}

/** Builds the query string the API accepts, or says which fields are wrong. */
export function buildExportQuery(parameters: ExportParameters): ExportQuery {
  const problems = exportFieldProblems(parameters);
  const field = problems.limit ? "limit" : problems.after ? "after" : null;
  if (field) return { ok: false, field, message: problems[field] ?? "", problems };
  const query = new URLSearchParams({ kind: parameters.kind, format: parameters.format });
  const after = parameters.after.trim();
  if (after !== "") query.set("after", after);
  query.set("limit", String(Number(parameters.limit.trim())));
  return { ok: true, query: query.toString() };
}

/** A file name for a downloaded page, without characters a file system rejects. */
export function exportFileName(parameters: ExportParameters): string {
  const position =
    parameters.after.trim() === "" ? "from-start" : `after-${parameters.after.trim()}`;
  return `security-${parameters.kind}-${position.replaceAll(".", "-")}.${parameters.format}`;
}

export type ExportAuthorization = "authorized" | "forbidden" | "unauthorized" | "unavailable";

/**
 * Asks the real export route whether this viewer may export: one record of the events page, which
 * is discarded. The web has no role information of its own (no /api/auth/me upstream), so the
 * API's answer is the only source: 403 hides the control, anything but 200 or 403 does not
 * authorize it.
 */
export async function probeExportAuthorization(
  options: { signal?: AbortSignal; fetchImplementation?: typeof fetch } = {},
): Promise<ExportAuthorization> {
  const query = buildExportQuery({ kind: "events", format: "json", after: "", limit: "1" });
  if (!query.ok) return "unavailable";
  const result = await fetchJson<unknown>(`${AUDIT_EXPORT_URL}?${query.query}`, options);
  if (result.ok) return "authorized";
  if (result.error.kind === "http") {
    if (result.error.status === 403) return "forbidden";
    if (result.error.status === 401) return "unauthorized";
  }
  return "unavailable";
}

export interface ExportPage {
  /** The body exactly as the server sent it; this is what a download saves. */
  body: string;
  contentType: string;
  requestId?: string;
  durationMs: number;
  /** Set for a JSON page: how many records it holds and where the next page starts. */
  recordCount?: number;
  nextCursor?: string;
}

export type ExportResult = { ok: true; page: ExportPage } | { ok: false; failure: SecurityFailure };

/** Fetches one export page; a JSON page is checked to be a page of the kind that was asked for. */
export async function fetchExportPage(
  parameters: ExportParameters,
  options: { signal?: AbortSignal; fetchImplementation?: typeof fetch } = {},
): Promise<ExportResult> {
  const built = buildExportQuery(parameters);
  if (!built.ok) return { ok: false, failure: failure("bad_request") };
  const fetchImplementation = options.fetchImplementation ?? fetch;
  const timeout = AbortSignal.timeout(EXPORT_TIMEOUT_MS);
  const signal = options.signal ? AbortSignal.any([options.signal, timeout]) : timeout;
  const startedAt = performance.now();
  try {
    const response = await fetchImplementation(`${AUDIT_EXPORT_URL}?${built.query}`, {
      method: "GET",
      cache: "no-store",
      headers: { accept: parameters.format === "csv" ? "text/csv" : "application/json" },
      signal,
    });
    const body = await response.text();
    const durationMs = Math.round(performance.now() - startedAt);
    if (!response.ok) return { ok: false, failure: failureForStatus(response.status) };
    const contentType = response.headers.get("content-type") ?? "";
    const requestId = response.headers.get("x-request-id") ?? undefined;
    if (parameters.format === "csv") {
      if (!contentType.toLowerCase().startsWith("text/csv")) {
        return { ok: false, failure: failure("invalid_response") };
      }
      return { ok: true, page: { body, contentType, requestId, durationMs } };
    }
    const summary = summarizeJsonPage(body, parameters.kind);
    if (!summary) return { ok: false, failure: failure("invalid_response") };
    return { ok: true, page: { body, contentType, requestId, durationMs, ...summary } };
  } catch {
    if (options.signal?.aborted) return { ok: false, failure: failure("network") };
    return { ok: false, failure: failure(timeout.aborted ? "timeout" : "network") };
  }
}

/** Reads the record count and next cursor of a JSON page, or null when it is not such a page. */
export function summarizeJsonPage(
  body: string,
  kind: ExportKind,
): { recordCount: number; nextCursor: string } | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(body);
  } catch {
    return null;
  }
  if (!isRecord(parsed) || !isText(parsed.nextCursor)) return null;
  const records = kind === "events" ? parsed.events : parsed.records;
  if (!Array.isArray(records)) return null;
  const page = parsed as unknown as AssessmentPage | SecurityEventPage;
  return { recordCount: records.length, nextCursor: page.nextCursor };
}
