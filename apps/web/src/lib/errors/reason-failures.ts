import type { ReasonCode } from "@workspace/contracts";

// The words for every X-13 reason code (WEB-23). Each message is the gateway's fixed safe text for
// that code (services/gateway/internal/contracts/reason_messages.go), so the interface never shows
// request values, record content or model text, and never names a run status: one code can end a run
// in different states, and the run's own status says which.

/** Where a failure happened, so the operator can tell a failing live model from a working gateway. */
export type FailureScope =
  "browser" | "web_server" | "api" | "gateway" | "live_model" | "session" | "request";

export interface ReasonFailure {
  title: string;
  message: string;
  /** The place the failure belongs to when it is not the request itself. */
  scope: FailureScope;
}

export const REASON_FAILURES: Record<ReasonCode, ReasonFailure> = {
  resource_out_of_scope: {
    title: "Outside the task's scope",
    message: "The requested record is outside this task's scope.",
    scope: "request",
  },
  destination_not_allowed: {
    title: "Destination not allowed",
    message: "The destination is not a registered recipient of this task.",
    scope: "request",
  },
  report_export_restricted: {
    title: "Export restricted",
    message: "This report inherits an Internal only restriction and cannot be sent to the vendor.",
    scope: "request",
  },
  report_lineage_missing: {
    title: "Source trail missing",
    message: "This report has no verifiable source trail and cannot be sent.",
    scope: "request",
  },
  source_policy_changed: {
    title: "Source policy changed",
    message: "A source policy changed after the check, so the action was not run.",
    scope: "request",
  },
  template_not_allowed: {
    title: "Template not allowed",
    message: "This report template is not permitted for this task.",
    scope: "request",
  },
  approval_required: {
    title: "Approval required",
    message: "This action waits for a reviewer's approval of the exact action.",
    scope: "request",
  },
  approval_expired: {
    title: "Approval expired",
    message: "The approval expired before the action ran; a new review is needed.",
    scope: "request",
  },
  action_changed: {
    title: "Action changed",
    message: "The action changed after it was checked and was not run.",
    scope: "request",
  },
  resource_version_changed: {
    title: "Source changed",
    message: "A source record changed after the check, so the action was not run.",
    scope: "request",
  },
  allowance_exhausted: {
    title: "Limit reached",
    message:
      "The run reached one of its limits (model calls, tokens, time or corrections), so no further work of that kind is dispatched.",
    scope: "gateway",
  },
  run_cancelled: {
    title: "Run cancelled",
    message: "The run was cancelled by an operator.",
    scope: "request",
  },
  outcome_unknown: {
    title: "Outcome unknown",
    message:
      "Whether the last request took effect is unknown, so it needs operator attention before the run can continue.",
    scope: "live_model",
  },
  semantic_injection_detected: {
    title: "Instruction-like content withheld",
    message: "The security check found instruction-like content and withheld it.",
    scope: "gateway",
  },
  security_evaluator_unavailable: {
    title: "Security check unavailable",
    message:
      "The security check could not run, so the content or action it guards did not continue.",
    scope: "live_model",
  },
  security_allowance_exhausted: {
    title: "Security check allowance used up",
    message:
      "The security check has no remaining allowance, so nothing further is checked or dispatched.",
    scope: "gateway",
  },
  content_redacted: {
    title: "Content masked",
    message: "Part of the content matched a protected pattern and was masked.",
    scope: "gateway",
  },
  signature_match: {
    title: "Known attack signature",
    message: "The content matched a known attack signature and was withheld.",
    scope: "gateway",
  },
  policy_reload_rejected: {
    title: "Policy rejected",
    message: "The new policy was rejected; the previous active policy stays in force.",
    scope: "gateway",
  },
  model_not_allowed: {
    title: "Model not allowed",
    message:
      "The configured model is not allowed by the active policy, so no model request is sent.",
    scope: "live_model",
  },
  multiple_actions_not_supported: {
    title: "Several actions proposed",
    message: "The model proposed several actions at once; only one action per step is allowed.",
    scope: "live_model",
  },
  run_expired: {
    title: "Task expired",
    message: "The task has expired.",
    scope: "request",
  },
  tool_not_registered: {
    title: "Tool not available",
    message: "That tool is not available.",
    scope: "request",
  },
  invalid_arguments: {
    title: "Invalid request",
    message: "The request or tool arguments are not valid.",
    scope: "request",
  },
  tool_not_allowed: {
    title: "Tool not allowed",
    message: "That tool is not permitted for this task.",
    scope: "request",
  },
  decision_unavailable: {
    title: "Check unavailable",
    message: "A required check could not be made, so nothing was run.",
    scope: "gateway",
  },
  content_blocked: {
    title: "Content withheld",
    message: "The content was withheld by a security control.",
    scope: "gateway",
  },
  content_too_large: {
    title: "Content too large",
    message: "The content exceeds the inspected size limit and was withheld.",
    scope: "gateway",
  },
  limit_not_allowed: {
    title: "Limit not allowed",
    message: "The requested limit exceeds what the active policy allows.",
    scope: "request",
  },
  run_not_active: {
    title: "Run not active",
    message: "The run is no longer active, so nothing was evaluated for it.",
    scope: "request",
  },
  approval_rejected: {
    title: "Rejected by the reviewer",
    message: "The reviewer rejected this action; it was not run.",
    scope: "request",
  },
};

/** True when the value is one of the 31 X-13 reason codes. */
export function isReasonCode(value: unknown): value is ReasonCode {
  return typeof value === "string" && Object.hasOwn(REASON_FAILURES, value);
}
