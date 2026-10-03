import type { ReasonCode, RunState, RunStatus } from "@workspace/contracts";

// Business-language text for the stable X-13 reason codes. The record is typed on ReasonCode, so a
// code added to the contract stops the typecheck until it has a sentence here.
export const REASON_LABELS: Record<ReasonCode, string> = {
  resource_out_of_scope: "The record or vendor is outside this task's permitted scope",
  destination_not_allowed: "The recipient is not permitted for this task",
  report_export_restricted: "The report inherits an Internal only restriction and cannot be sent",
  report_lineage_missing: "The report has no verifiable source trail",
  source_policy_changed: "The source policy changed after the action was evaluated",
  template_not_allowed: "That report template is not permitted for this task",
  approval_required: "A reviewer's approval is required before this runs",
  approval_expired: "The approval expired before the action ran",
  action_changed: "The action changed after it was reviewed",
  resource_version_changed: "A source record changed after the action was reviewed",
  allowance_exhausted: "The task's allowance (model calls, tokens or time) is used up",
  run_cancelled: "An operator cancelled the run",
  outcome_unknown: "The outcome of a request is unknown; operator attention is needed",
  semantic_injection_detected: "The semantic check found hidden instructions in the text",
  security_evaluator_unavailable: "The security check could not run, so nothing was released",
  security_allowance_exhausted: "The security check's allowance is used up",
  content_redacted: "Sensitive content was masked before use",
  signature_match: "The text matches a known attack signature",
  policy_reload_rejected: "A policy change was rejected; the previous policy stays active",
  model_not_allowed: "The configured model is not permitted for this run",
  multiple_actions_not_supported: "The model asked for several actions at once; none ran",
  run_expired: "The run's time allowance expired",
  tool_not_registered: "That tool does not exist",
  invalid_arguments: "The request or answer was not in the required format",
  tool_not_allowed: "That tool is not permitted for this task",
  decision_unavailable: "The action could not be checked, so it did not run",
  content_blocked: "Content was blocked by a content rule",
  content_too_large: "Content was too large to be checked",
  limit_not_allowed: "The requested limit is above what the task may use",
  run_not_active: "The run is no longer active",
  approval_rejected: "A reviewer rejected the action",
};

/** The sentence for a reason code; a code this build does not know is shown as unrecognized. */
export function reasonLabel(code: string | null): string | null {
  if (code === null) {
    return null;
  }
  return Object.hasOwn(REASON_LABELS, code)
    ? REASON_LABELS[code as ReasonCode]
    : `Unrecognized reason (${code})`;
}

export type StateTone =
  "active" | "waiting" | "success" | "attention" | "stopped" | "failed" | "unknown";

/** How the run ended or is held, in the vocabulary of the operator's Journey 3. */
export type OutcomeKind =
  | "in_progress"
  | "waiting"
  | "succeeded"
  | "interrupted"
  | "uncertain"
  | "stopped"
  | "failed"
  | "unknown";

export interface RunStateView {
  /** The persisted status, as received; the browser never infers one. */
  status: string;
  title: string;
  tone: StateTone;
  outcomeKind: OutcomeKind;
  summary: string;
  next: string;
  /** The persisted terminal reason; present exactly when the run is paused, failed or stopped. */
  reasonCode: string | null;
  reasonText: string | null;
  /** The recorded explanation, when the run's last event carries one (never invented). */
  safeMessage: string | null;
  /** True when the persisted state breaks the contract (a missing reason, an unknown status). */
  contractProblem: string | null;
  reportIds: string[];
}

const STATUSES_WITH_REASON: ReadonlySet<string> = new Set(["paused", "failed", "stopped"]);

type StatusText = Pick<RunStateView, "title" | "tone" | "outcomeKind" | "summary" | "next">;

const STATUS_TEXT: Record<Exclude<RunStatus, "paused" | "stopped">, StatusText> = {
  queued: {
    title: "Queued",
    tone: "active",
    outcomeKind: "in_progress",
    summary: "The task was admitted and is waiting for a worker to start it.",
    next: "Nothing is needed; the run starts by itself.",
  },
  running: {
    title: "Running",
    tone: "active",
    outcomeKind: "in_progress",
    summary: "The agent is working inside the permissions it was granted at admission.",
    next: "Nothing is needed; the page shows each step as it is recorded.",
  },
  awaiting_approval: {
    title: "Waiting for approval",
    tone: "waiting",
    outcomeKind: "waiting",
    summary: "The agent proposed an action that needs a reviewer. Nothing has run for it yet.",
    next: "A reviewer approves or rejects the exact action; the run then continues.",
  },
  completed: {
    title: "Completed",
    tone: "success",
    outcomeKind: "succeeded",
    summary: "The task finished and its final answer named the reports this run created.",
    next: "Open the reports below; the interface shows their stored labels.",
  },
  failed: {
    title: "Failed",
    tone: "failed",
    outcomeKind: "failed",
    summary: "The task could not be completed. No action ran after the failure.",
    next: "Read the reason below; start a new task if it should be retried.",
  },
};

// A paused run keeps a permitted path forward; a stopped run was ended on purpose or by a limit.
function pausedText(reason: string | null): StatusText {
  if (reason === "outcome_unknown") {
    return {
      title: "Operator attention required",
      tone: "attention",
      outcomeKind: "uncertain",
      summary:
        "A request was sent but its outcome is not known. The allowance it reserved stays held, and the result is uncertain, not zero.",
      next: "An operator reviews the run before anything continues; nothing is repeated automatically.",
    };
  }
  if (reason === "allowance_exhausted") {
    return {
      title: "Paused at its allowance",
      tone: "attention",
      outcomeKind: "interrupted",
      summary:
        "The run used its allowance of model calls or tokens, so it is paused. This is a justified interruption: nothing was lost and no further request is sent for it.",
      next: "Start a new task with a larger allowance if the work should continue.",
    };
  }
  return {
    title: "Paused",
    tone: "attention",
    outcomeKind: "interrupted",
    summary: "The run is paused and no further work is dispatched for it.",
    next: "Read the reason below.",
  };
}

function stoppedText(reason: string | null): StatusText {
  const next = "Start a new task if the work should be redone.";
  if (reason === "run_cancelled") {
    return {
      title: "Stopped: cancelled",
      tone: "stopped",
      outcomeKind: "stopped",
      summary:
        "An operator cancelled the run. Work already recorded stays; nothing runs after the cancel.",
      next,
    };
  }
  if (reason === "run_expired") {
    return {
      title: "Stopped: expired",
      tone: "stopped",
      outcomeKind: "stopped",
      summary: "The run's time allowance passed, so it was stopped.",
      next,
    };
  }
  return {
    title: "Stopped",
    tone: "stopped",
    outcomeKind: "stopped",
    summary: "The run was stopped and nothing runs after the stop.",
    next,
  };
}

/**
 * The business view of one persisted run state (X-11). Every status of the contract has a view; an
 * unknown status is shown as unknown and never as success, and a paused, failed or stopped run
 * always shows its terminal reason (or says that none was recorded).
 */
export function describeRunState(run: RunState, safeMessage: string | null = null): RunStateView {
  const status: string = run.status;
  const reasonCode = run.terminalReason ?? null;
  const reportIds = run.resultReference?.reportIds ?? [];
  let text: StatusText;
  let contractProblem: string | null = null;
  if (status === "paused") {
    text = pausedText(reasonCode);
  } else if (status === "stopped") {
    text = stoppedText(reasonCode);
  } else if (Object.hasOwn(STATUS_TEXT, status)) {
    text = STATUS_TEXT[status as keyof typeof STATUS_TEXT];
  } else {
    text = {
      title: `Unknown state (${status})`,
      tone: "unknown",
      outcomeKind: "unknown",
      summary:
        "This interface does not recognize the run's state, so it makes no claim about the run.",
      next: "Treat the run as unverified and check it with an operator.",
    };
    contractProblem = `The run state "${status}" is not part of the contract this page was built for.`;
  }
  if (STATUSES_WITH_REASON.has(status) && reasonCode === null) {
    contractProblem = `The run is ${status} but no terminal reason was recorded.`;
  }
  return {
    status,
    ...text,
    reasonCode,
    reasonText: STATUSES_WITH_REASON.has(status)
      ? (reasonLabel(reasonCode) ?? "No reason was recorded")
      : null,
    safeMessage,
    contractProblem,
    reportIds,
  };
}
