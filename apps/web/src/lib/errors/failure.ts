import type { ReasonCode } from "@workspace/contracts";
import type { FetchJsonError } from "../fetch-json";
import { isReasonCode, REASON_FAILURES, type FailureScope } from "./reason-failures";

// One truthful state for every way a request can fail (WEB-23). A failure is never shown as a success
// or as an empty page: each kind has its own title and words, says where it failed (your browser, the
// web server, the API, the gateway, the live model, your session), and says whether a command may
// still have taken effect. The words are fixed here, never taken from a server message.

export type FailureKind =
  | "offline"
  | "timeout"
  | "cancelled"
  | "invalid_response"
  | "session_expired"
  | "forbidden"
  | "not_found"
  | "conflict"
  | "bad_request"
  | "payload_too_large"
  | "configuration_error"
  | "upstream_unreachable"
  | "upstream_timeout"
  | "upstream_invalid_response"
  | "upstream_unavailable"
  | "outcome_unconfirmed"
  | "not_implemented"
  | "server_error"
  | "reason"
  | "unknown";

/** What the operator can do about it. A command that may have taken effect offers neither. */
export type FailureAction = "retry" | "sign_in" | "none";

export interface Failure {
  kind: FailureKind;
  scope: FailureScope;
  title: string;
  description: string;
  action: FailureAction;
  /** A command may have taken effect; check the run before submitting it again. */
  unconfirmed: boolean;
  /** The X-13 reason code the failure came from, if it came from one. */
  reasonCode: ReasonCode | null;
  statusCode: number | null;
}

export interface FailureWords {
  title: string;
  description: string;
}

export interface ClassifyOptions {
  /**
   * The request was a command (it changes something). When no answer arrives, it may still have
   * been carried out, so the failure is unconfirmed and offers no retry.
   */
  command?: boolean;
  /** Page-specific words for a kind, for example "Report not found". Scope and action stay. */
  overrides?: Partial<Record<FailureKind, FailureWords>>;
}

const UNCONFIRMED_NOTE =
  " The request may still have been carried out: check the run before submitting it again.";

interface Definition extends FailureWords {
  scope: FailureScope;
  action: FailureAction;
  /** Whether a command that fails this way is unconfirmed. */
  unconfirmedForCommand: boolean;
}

const DEFINITIONS: Record<Exclude<FailureKind, "reason">, Definition> = {
  offline: {
    scope: "browser",
    title: "You appear to be offline",
    description: "This browser could not reach the web server. Check the connection and try again.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  timeout: {
    scope: "browser",
    title: "No answer in time",
    description: "The web server did not answer within the time allowed.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  cancelled: {
    scope: "browser",
    title: "Request cancelled",
    description: "The request was cancelled before it finished.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  invalid_response: {
    scope: "web_server",
    title: "Unexpected response",
    description: "The response was not in the expected form, so nothing is shown from it.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  session_expired: {
    scope: "session",
    title: "Your session has ended",
    description: "Sign in again to continue. Nothing was changed by this request.",
    action: "sign_in",
    unconfirmedForCommand: false,
  },
  forbidden: {
    scope: "request",
    title: "Not allowed",
    description: "Your account is not allowed to do this.",
    action: "none",
    unconfirmedForCommand: false,
  },
  not_found: {
    scope: "request",
    title: "Not found",
    description: "The record does not exist, or it belongs to another organization.",
    action: "none",
    unconfirmedForCommand: false,
  },
  conflict: {
    scope: "request",
    title: "Changed in the meantime",
    description: "The record changed since you loaded it. Reload to see its current state.",
    action: "retry",
    unconfirmedForCommand: false,
  },
  bad_request: {
    scope: "request",
    title: "Request not valid",
    description: "The request was not valid, so nothing was done. Correct it and submit again.",
    action: "none",
    unconfirmedForCommand: false,
  },
  payload_too_large: {
    scope: "request",
    title: "Request too large",
    description: "The request is larger than the limit, so nothing was done.",
    action: "none",
    unconfirmedForCommand: false,
  },
  configuration_error: {
    scope: "web_server",
    title: "The web server is not set up",
    description:
      "The web server is not configured to reach the API. This is a setup problem, not something to retry.",
    action: "none",
    unconfirmedForCommand: false,
  },
  upstream_unreachable: {
    scope: "api",
    title: "The API could not be reached",
    description: "The web server could not connect to the API. Nothing was received from it.",
    action: "retry",
    unconfirmedForCommand: false,
  },
  upstream_timeout: {
    scope: "api",
    title: "The API did not answer in time",
    description: "The API did not answer in time, so the outcome is not known.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  upstream_invalid_response: {
    scope: "api",
    title: "The API answered unexpectedly",
    description:
      "The API sent something that is not a valid response, so nothing is shown from it.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  upstream_unavailable: {
    scope: "gateway",
    title: "The gateway is unavailable",
    description: "The API could not get an answer from the gateway.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  outcome_unconfirmed: {
    scope: "gateway",
    title: "Outcome unconfirmed",
    description:
      "The gateway did not answer in time, so it is not known whether the request took effect.",
    action: "none",
    unconfirmedForCommand: true,
  },
  not_implemented: {
    scope: "api",
    title: "Not available",
    description: "This feature is not available in this build.",
    action: "none",
    unconfirmedForCommand: false,
  },
  server_error: {
    scope: "api",
    title: "Something went wrong on the server",
    description: "The server could not complete the request. Details are in its log, not here.",
    action: "retry",
    unconfirmedForCommand: true,
  },
  unknown: {
    scope: "request",
    title: "Request failed",
    description: "The request failed in a way this page does not recognize.",
    action: "retry",
    unconfirmedForCommand: true,
  },
};

// The API's own error codes (apps/api common/all-exceptions.filter.ts) and the web proxy's, each
// mapped to a kind. Codes the API takes from a failed gateway call are gateway-side failures.
const KIND_BY_CODE: Record<string, Exclude<FailureKind, "reason">> = {
  unauthorized: "session_expired",
  forbidden: "forbidden",
  not_found: "not_found",
  conflict: "conflict",
  bad_request: "bad_request",
  invalid_input: "bad_request",
  payload_too_large: "payload_too_large",
  not_implemented: "not_implemented",
  internal_error: "server_error",
  configuration_error: "configuration_error",
  upstream_unreachable: "upstream_unreachable",
  upstream_timeout: "upstream_timeout",
  upstream_invalid_response: "upstream_invalid_response",
  upstream_unavailable: "upstream_unavailable",
  outcome_unconfirmed: "outcome_unconfirmed",
  timeout: "upstream_unavailable",
  network_error: "upstream_unavailable",
  server_error: "upstream_unavailable",
  unexpected_status: "upstream_unavailable",
  invalid_json: "upstream_unavailable",
};

const KIND_BY_STATUS: Record<number, Exclude<FailureKind, "reason">> = {
  400: "bad_request",
  401: "session_expired",
  403: "forbidden",
  404: "not_found",
  409: "conflict",
  413: "payload_too_large",
  500: "server_error",
  501: "not_implemented",
  502: "upstream_unreachable",
  503: "upstream_unavailable",
  504: "upstream_timeout",
};

function errorCodeOf(body: unknown): string | null {
  if (typeof body !== "object" || body === null) return null;
  const error = (body as { error?: unknown }).error;
  if (typeof error !== "object" || error === null) return null;
  const code = (error as { code?: unknown }).code;
  return typeof code === "string" ? code : null;
}

function build(
  kind: Exclude<FailureKind, "reason">,
  statusCode: number | null,
  options: ClassifyOptions,
): Failure {
  const definition = DEFINITIONS[kind];
  const words = options.overrides?.[kind] ?? definition;
  // The gateway's own "unconfirmed" answer is unconfirmed whatever the caller says it was.
  const unconfirmed =
    kind === "outcome_unconfirmed" ||
    (options.command === true && definition.unconfirmedForCommand);
  return {
    kind,
    scope: definition.scope,
    title: words.title,
    description: unconfirmed ? `${words.description}${UNCONFIRMED_NOTE}` : words.description,
    // Submitting a command again before checking could repeat it.
    action: unconfirmed ? "none" : definition.action,
    unconfirmed,
    reasonCode: null,
    statusCode,
  };
}

/** The state of an X-13 reason code, for example a run that stopped or a refused command. */
export function failureFromReason(reasonCode: ReasonCode): Failure {
  const reason = REASON_FAILURES[reasonCode];
  return {
    kind: "reason",
    scope: reason.scope,
    title: reason.title,
    description: reason.message,
    action: "none",
    unconfirmed: reasonCode === "outcome_unknown",
    reasonCode,
    statusCode: null,
  };
}

/**
 * Maps a failed same-origin request to its own state. A failure that never reached a response is a
 * browser failure (offline, timeout, cancelled); one with a response is read from the API's error
 * code (an X-13 reason code, a proxy code or an API code), then from the status.
 */
export function classifyFailure(error: FetchJsonError, options: ClassifyOptions = {}): Failure {
  switch (error.kind) {
    case "network":
      return build("offline", null, options);
    case "timeout":
      return build("timeout", null, options);
    case "aborted":
      return build("cancelled", null, options);
    case "invalid_json":
      return build("invalid_response", error.status, options);
    case "http": {
      const code = errorCodeOf(error.body);
      if (isReasonCode(code)) {
        const reasonFailure = failureFromReason(code);
        return { ...reasonFailure, statusCode: error.status };
      }
      const kind =
        (code !== null ? KIND_BY_CODE[code] : undefined) ??
        KIND_BY_STATUS[error.status] ??
        (error.status >= 500 ? "server_error" : "unknown");
      return build(kind, error.status, options);
    }
  }
}
