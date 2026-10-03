// Browser client for the exact-action review (WEB-14) and run cancellation (WEB-15). Every call
// goes to this app's same-origin routes, which forward to the API with the session cookie.
// NestJS and Go decide; this client only sends the displayed identifier and the decision.

import type {
  ApprovalDecision,
  ApprovalResponse,
  ReviewView,
  RunState,
} from "@workspace/contracts";

import { fetchJson, postJson, type FetchJsonError, type FetchJsonOptions } from "../fetch-json";

const RECORD_ID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** Whether a value is a record id the API accepts (a lowercase uuid). */
export function isRecordId(value: string): boolean {
  return RECORD_ID_PATTERN.test(value);
}

/** A failed request, as the interface presents it: never an approval. */
export interface ClientFailure {
  /** The fixed kind the page renders; "unconfirmed" means the outcome is unknown. */
  kind:
    | "expired"
    | "changed"
    | "run_stopped"
    | "already_decided"
    | "not_reviewer"
    | "not_found"
    | "signed_out"
    | "unconfirmed"
    | "unavailable"
    | "rejected_request";
  /** The HTTP status when a response arrived. */
  status?: number;
  message: string;
}

export type ClientResult<Data> = { ok: true; data: Data } | { ok: false; failure: ClientFailure };

const FAILURE_MESSAGES: Record<ClientFailure["kind"], string> = {
  expired: "The approval window has expired. Nothing was sent; the action needs a new review.",
  changed:
    "The action changed after it was frozen for review. Nothing was sent; the action needs a new review.",
  run_stopped: "The run is stopped or cancelled. Nothing was sent.",
  already_decided: "This action has already been decided. Reload to see its current state.",
  not_reviewer: "You are not a reviewer of this organization, so you cannot review this action.",
  not_found: "No action awaits review here.",
  signed_out: "Your session has ended. Sign in again to continue.",
  unconfirmed:
    "The request did not complete in time. Its outcome is unconfirmed; reload to see the current state.",
  unavailable: "The service could not complete the request. Nothing was recorded; try again.",
  rejected_request: "The request was not accepted.",
};

/**
 * Maps a failed request to the kind the page shows. Unknown failures are never a success. For a
 * command (approve, reject, cancel) a lost connection is unconfirmed: the server may have acted.
 */
export function describeFailure(error: FetchJsonError, { command = false } = {}): ClientFailure {
  const failure = (kind: ClientFailure["kind"], status?: number): ClientFailure => ({
    kind,
    status,
    message: FAILURE_MESSAGES[kind],
  });
  if (error.kind === "timeout") return failure("unconfirmed");
  if (error.kind !== "http") return failure(command ? "unconfirmed" : "unavailable");

  const code = (error.body as { error?: { code?: unknown } } | undefined)?.error?.code;
  switch (code) {
    case "approval_expired":
      return failure("expired", error.status);
    case "action_changed":
      return failure("changed", error.status);
    case "run_cancelled":
      return failure("run_stopped", error.status);
    case "outcome_unconfirmed":
    case "upstream_timeout":
      return failure("unconfirmed", error.status);
  }
  switch (error.status) {
    case 401:
      return failure("signed_out", 401);
    case 403:
      return failure("not_reviewer", 403);
    case 404:
      return failure("not_found", 404);
    case 409:
      return failure("already_decided", 409);
    case 504:
      return failure("unconfirmed", 504);
    case 400:
      return failure("rejected_request", 400);
    default:
      return failure("unavailable", error.status);
  }
}

/** Reads the frozen review material of one action; reviewers only. */
export async function getReview(
  actionId: string,
  options?: FetchJsonOptions,
): Promise<ClientResult<ReviewView>> {
  if (!isRecordId(actionId)) {
    return { ok: false, failure: { kind: "not_found", message: FAILURE_MESSAGES.not_found } };
  }
  const result = await fetchJson<ReviewView>(`/api/actions/${actionId}/review`, options);
  if (!result.ok) return { ok: false, failure: describeFailure(result.error) };
  if (result.data?.action_id !== actionId) {
    // A review of another action is never shown as this one.
    return { ok: false, failure: { kind: "unavailable", message: FAILURE_MESSAGES.unavailable } };
  }
  return { ok: true, data: result.data };
}

/** Sends exactly {"decision": ...} for the displayed action; nothing else leaves the browser. */
export async function decideApproval(
  actionId: string,
  decision: ApprovalDecision["decision"],
  options?: FetchJsonOptions,
): Promise<ClientResult<ApprovalResponse>> {
  if (!isRecordId(actionId)) {
    return { ok: false, failure: { kind: "not_found", message: FAILURE_MESSAGES.not_found } };
  }
  const body: ApprovalDecision = { decision };
  const result = await postJson<ApprovalResponse>(
    `/api/actions/${actionId}/approval`,
    body,
    options,
  );
  if (!result.ok) return { ok: false, failure: describeFailure(result.error, { command: true }) };
  if (result.data?.actionId !== actionId || result.data.decision !== decision) {
    // An answer about another action or decision is unconfirmed, never shown as recorded.
    return { ok: false, failure: { kind: "unconfirmed", message: FAILURE_MESSAGES.unconfirmed } };
  }
  return { ok: true, data: result.data };
}

/** Reads the run's current state, so a decided or finished review is not offered again. */
export async function getRunState(
  runId: string,
  options?: FetchJsonOptions,
): Promise<ClientResult<RunState>> {
  if (!isRecordId(runId)) {
    return { ok: false, failure: { kind: "not_found", message: "No run was found." } };
  }
  const result = await fetchJson<RunState>(`/api/runs/${runId}`, options);
  if (!result.ok) return { ok: false, failure: describeFailure(result.error) };
  if (result.data?.runId !== runId) {
    return { ok: false, failure: { kind: "unavailable", message: FAILURE_MESSAGES.unavailable } };
  }
  return { ok: true, data: result.data };
}

/** Requests cancellation of a run; the answer is the run state the server recorded. */
export async function cancelRun(
  runId: string,
  options?: FetchJsonOptions,
): Promise<ClientResult<RunState>> {
  if (!isRecordId(runId)) {
    return { ok: false, failure: { kind: "not_found", message: "No run was found." } };
  }
  const result = await postJson<RunState>(`/api/runs/${runId}/cancel`, {}, options);
  if (!result.ok) {
    const failure = describeFailure(result.error, { command: true });
    // A cancel needs no reviewer role; a 404 here is an unknown run, not a missing review.
    if (failure.kind === "not_found") failure.message = "No run was found.";
    return { ok: false, failure };
  }
  if (result.data?.runId !== runId) {
    return { ok: false, failure: { kind: "unconfirmed", message: FAILURE_MESSAGES.unconfirmed } };
  }
  return { ok: true, data: result.data };
}
