import { fetchJson, postJson, FetchJsonResult } from "./fetch-json";
import type { 
  StartRunRequest, 
  StartRunResponse, 
  TaskFormOptions, 
  RunView, 
  SanitizedEvent 
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
    const code = (error.body as any)?.error?.code;
    if (code === "unauthorized") return "Invalid credentials.";
    if (code) return REASON_CODE_MESSAGES[code] || "An unknown error occurred.";
    if (error.status === 401) return "Invalid credentials.";
  }
  if (error.kind === "network") return "The server could not be reached.";
  if (error.kind === "timeout") return "The request timed out.";
  return REASON_CODE_MESSAGES[error.kind] || "An unknown error occurred.";
}

// Type Guards for frozen contracts
function isStartRunResponse(data: unknown): data is StartRunResponse {
  return typeof data === "object" && data !== null && 
    "runId" in data && typeof (data as Record<string, unknown>).runId === "string" &&
    "passportId" in data && typeof (data as Record<string, unknown>).passportId === "string";
}

function isTaskFormOptions(data: unknown): data is TaskFormOptions {
  return typeof data === "object" && data !== null && "templates" in data && "vendors" in data;
}

function isRunView(data: unknown): data is RunView {
  return typeof data === "object" && data !== null && "id" in data && "status" in data;
}

function isSanitizedEventsResponse(data: unknown): data is { events: SanitizedEvent[]; nextCursor?: string } {
  return typeof data === "object" && data !== null && "events" in data && Array.isArray((data as Record<string, unknown>).events);
}

// Ensure the result matches the guard or return an invalid_json error.
function enforceGuard<T>(
  result: FetchJsonResult<unknown>, 
  guard: (data: unknown) => data is T
): FetchJsonResult<T> {
  if (!result.ok) {
    return result as FetchJsonResult<T>;
  }
  
  if (result.data === undefined) {
    return { 
      ok: false, 
      error: { kind: "invalid_json", status: result.status },
      durationMs: result.durationMs,
      requestId: result.requestId
    };
  }

  if (guard(result.data)) {
    return result as FetchJsonResult<T>;
  }
  
  return { 
    ok: false, 
    error: { kind: "invalid_json", status: result.status },
    durationMs: result.durationMs,
    requestId: result.requestId
  };
}

export class ProductClient {
  static async startRun(request: StartRunRequest): Promise<FetchJsonResult<StartRunResponse>> {
    const result = await postJson("/api/runs", request);
    return enforceGuard(result, isStartRunResponse);
  }


  static async getRun(id: string): Promise<FetchJsonResult<RunView>> {
    const result = await fetchJson(`/api/runs/${encodeURIComponent(id)}`);
    return enforceGuard(result, isRunView);
  }

  static async getRunEvents(id: string, cursor?: string): Promise<FetchJsonResult<{ events: SanitizedEvent[]; nextCursor?: string }>> {
    const qs = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
    const result = await fetchJson(`/api/runs/${encodeURIComponent(id)}/events${qs}`);
    return enforceGuard(result, isSanitizedEventsResponse);
  }

  static async signIn(credentials: { email: string; password: string }): Promise<FetchJsonResult<{ message: string }>> {
    const result = await postJson("/api/auth/sign-in", credentials);
    return enforceGuard(result, (data): data is { message: string } => 
      typeof data === "object" && data !== null && "message" in data
    );
  }

  static async getMe(): Promise<FetchJsonResult<{ id: string; email: string; name: string; organizationId: string; roles: string[] }>> {
    const result = await fetchJson("/api/auth/me");
    return enforceGuard(result, (data): data is { id: string; email: string; name: string; organizationId: string; roles: string[] } => 
      typeof data === "object" && data !== null && "id" in data && "name" in data
    );
  }
}
