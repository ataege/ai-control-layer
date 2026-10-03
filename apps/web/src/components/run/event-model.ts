import type { AssessmentRecord, SafeEvent, SafeEventType } from "@workspace/contracts";
import { reasonLabel } from "./labels";

// Pure view model of the sanitized X-12 events (WEB-09) and of the hybrid control decisions
// (WEB-32). It only reads the fields of the contract: the browser derives no classification, label or
// state of its own beyond how to display what the gateway stored.

export type EventKind =
  "state" | "attempt" | "effect" | "uncertain" | "review" | "control" | "guidance" | "unknown";

export type Tone = "allow" | "deny" | "review" | "redact" | "failed" | "uncertain" | "neutral";

const KNOWN_EVENTS: Record<SafeEventType, { kind: EventKind; title: string }> = {
  "admission.rejected": { kind: "state", title: "Task rejected before a run was created" },
  "run.queued": { kind: "state", title: "Run queued" },
  "run.started": { kind: "state", title: "Run started" },
  "run.paused": { kind: "state", title: "Run paused" },
  "run.completed": { kind: "state", title: "Run completed" },
  "run.failed": { kind: "state", title: "Run failed" },
  "run.stopped": { kind: "state", title: "Run stopped" },
  "run.cancel_requested": { kind: "state", title: "Cancellation requested" },
  "run.awaiting_approval": { kind: "state", title: "Waiting for a reviewer" },
  "run.resumed": { kind: "state", title: "Run resumed after review" },
  "model.completed": { kind: "state", title: "Model step completed" },
  "action.proposed": { kind: "attempt", title: "Action proposed" },
  "action.allowed": { kind: "attempt", title: "Action allowed to run" },
  "action.denied": { kind: "attempt", title: "Action denied" },
  "approval.requested": { kind: "review", title: "Approval requested" },
  "approval.decided": { kind: "review", title: "Reviewer decided" },
  "action.executing": { kind: "attempt", title: "Action running" },
  "action.succeeded": { kind: "effect", title: "Action completed" },
  "action.failed": { kind: "attempt", title: "Action failed" },
  "action.unknown": { kind: "uncertain", title: "Action outcome unknown" },
  "report.created": { kind: "effect", title: "Report stored" },
  "report.export_denied": { kind: "attempt", title: "Send attempt blocked" },
  "report.safe_template_offered": { kind: "guidance", title: "Permitted alternative offered" },
  "control.evaluated": { kind: "control", title: "Security controls evaluated" },
  "catalog.revision_rejected": { kind: "state", title: "Policy change rejected" },
};

const EFFECT_TEXT: Record<string, string> = {
  read: "Data was read and returned to the agent; nothing was changed.",
  report_created: "A report was stored as an internal record; nothing was sent.",
  outbox_message_queued: "A message was queued in the simulated outbox; nothing was delivered.",
};

const CLASSIFICATION_TEXT: Record<string, string> = {
  internal_only: "Internal only",
  vendor_shareable: "Vendor shareable",
};

const LINEAGE_TEXT: Record<string, string> = {
  passed: "source trail verified",
  failed: "source trail failed",
  missing: "source trail missing",
};

const TEMPLATE_TEXT: Record<string, string> = {
  internal_investigation_v1: "internal investigation report",
  vendor_reconciliation_v1: "vendor reconciliation report",
};

const REJECTION_TEXT: Record<string, string> = {
  not_json: "the answer was not JSON",
  extra_text: "the answer had text around the JSON",
  code_fence: "the answer was wrapped in a code fence the rules do not allow",
  wrong_status: 'the answer\'s status was not "completed"',
  wrong_fields: "the answer's fields, count or report ids were not as required",
  unknown_report: "the answer named a report this run did not create",
};

export type ControlFamily = "deterministic" | "signature" | "semantic" | "unidentified";
export type ControlResult =
  "passed" | "blocked" | "redacted" | "not_applicable" | "unavailable" | "unknown";

/** One control's decision, shown as a badge on the event it belongs to (WEB-32). */
export interface DecisionBadge {
  family: ControlFamily;
  label: string;
  result: ControlResult;
  /** Semantic only: whether the verdict came from the live model or from a labelled fixture. */
  source: "live" | "fixture" | null;
  text: string;
  /** What happened to the text: blocked text is withheld, never shown as consumed by the agent. */
  consequence: string | null;
  detail: string | null;
}

export interface EventFact {
  label: string;
  value: string;
}

export interface EventView {
  id: string;
  actionId: string | null;
  occurredAt: string;
  eventType: string;
  kind: EventKind;
  tone: Tone;
  title: string;
  reasonCode: string | null;
  reasonText: string | null;
  /** The gateway's own bounded sentence for the decision, when it recorded one. */
  safeMessage: string | null;
  /** The rule behind a decision: the reason's sentence plus the matched rule id and feed revision. */
  rule: string | null;
  /** What changed, for an effect; "No change was made" for an attempt that recorded effect none. */
  effectText: string | null;
  replayLabel: string | null;
  /** The bounded way forward the gateway offered, when it offered one. */
  correctionRoute: string | null;
  rejectionText: string | null;
  facts: EventFact[];
  badges: DecisionBadge[];
  unrecognized: boolean;
}

function toneOf(event: SafeEvent): Tone {
  if (event.eventType === "action.failed") {
    return "failed";
  }
  if (event.eventType === "action.unknown") {
    return "uncertain";
  }
  switch (event.decision) {
    case "allow":
    case "approved":
      return "allow";
    case "deny":
    case "rejected":
      return "deny";
    case "approval_required":
      return "review";
    case "redact":
      return "redact";
    default:
      return "neutral";
  }
}

function isDenied(event: SafeEvent): boolean {
  return event.decision === "deny" || event.decision === "rejected";
}

function shortId(id: string): string {
  return id.length > 12 ? `${id.slice(0, 8)}…` : id;
}

function ruleText(event: SafeEvent): string | null {
  const parts: string[] = [];
  const reason = reasonLabel(event.reasonCode);
  if (reason !== null && (isDenied(event) || event.decision === "redact")) {
    parts.push(reason);
  }
  const { matchedRule, feedRevision } = event.maskedSummary;
  if (matchedRule !== null) {
    parts.push(`rule ${matchedRule}`);
  }
  if (feedRevision !== null) {
    parts.push(`signature feed ${feedRevision}`);
  }
  return parts.length > 0 ? parts.join(" · ") : null;
}

function effectText(event: SafeEvent, kind: EventKind): string | null {
  const { effect } = event.maskedSummary;
  if (kind === "effect") {
    return (
      (effect !== null ? EFFECT_TEXT[effect] : undefined) ??
      "Completed; the kind of effect was not recorded."
    );
  }
  if (effect === "none") {
    return "No change was made.";
  }
  return null;
}

function facts(event: SafeEvent): EventFact[] {
  const { maskedSummary: summary } = event;
  const list: EventFact[] = [];
  if (summary.template !== null) {
    list.push({ label: "Report", value: TEMPLATE_TEXT[summary.template] ?? summary.template });
  }
  if (summary.classification !== null) {
    list.push({
      label: "Label (set by the gateway)",
      value: CLASSIFICATION_TEXT[summary.classification] ?? summary.classification,
    });
  }
  if (summary.lineageCheck !== null) {
    list.push({
      label: "Source trail",
      value: LINEAGE_TEXT[summary.lineageCheck] ?? summary.lineageCheck,
    });
  }
  if (summary.reportId !== null) {
    list.push({ label: "Report id", value: shortId(summary.reportId) });
  }
  if (summary.purpose !== null) {
    list.push({ label: "Model purpose", value: summary.purpose });
  }
  if (event.catalogRevisionId !== null) {
    list.push({ label: "Policy revision", value: String(event.catalogRevisionId) });
  }
  if (summary.actorId !== null) {
    list.push({ label: "Caused by", value: shortId(summary.actorId) });
  }
  return list;
}

/** The deterministic, signature or semantic control a reason code names, when it names one. */
function familyOfReason(
  reasonCode: string | null,
): { family: ControlFamily; result: ControlResult } | null {
  switch (reasonCode) {
    case "signature_match":
      return { family: "signature", result: "blocked" };
    case "semantic_injection_detected":
      return { family: "semantic", result: "blocked" };
    case "security_evaluator_unavailable":
    case "security_allowance_exhausted":
      return { family: "semantic", result: "unavailable" };
    case "content_blocked":
    case "content_too_large":
      return { family: "deterministic", result: "blocked" };
    case "content_redacted":
      // Masking can come from a content rule or from a semantic verdict; an assessment names it.
      return { family: "unidentified", result: "redacted" };
    default:
      return null;
  }
}

const FAMILY_LABEL: Record<ControlFamily, string> = {
  deterministic: "Deterministic",
  signature: "Signature",
  semantic: "Semantic",
  unidentified: "Content control",
};

const RESULT_TEXT: Record<ControlResult, string> = {
  passed: "passed",
  blocked: "blocked",
  redacted: "redacted",
  not_applicable: "not applicable",
  unavailable: "could not run",
  unknown: "unrecognized result",
};

function consequenceOf(result: ControlResult): string | null {
  switch (result) {
    case "blocked":
      return "Blocked text is withheld; it was never shown to the agent as read.";
    case "redacted":
      return "Masked before it could reach the agent.";
    case "unavailable":
      return "Nothing was released and the run is paused.";
    default:
      return null;
  }
}

function badgeText(
  label: string,
  result: ControlResult,
  source: "live" | "fixture" | null,
): string {
  const base = `${label}: ${RESULT_TEXT[result]}`;
  if (source === "fixture") {
    return `${base} (fixture verdict, not detection quality)`;
  }
  return source === "live" ? `${base} (live model)` : base;
}

/** The control that one stored assessment records, as a badge (WEB-32). */
export function badgeFromAssessment(record: AssessmentRecord): DecisionBadge {
  const family: ControlFamily =
    record.controlId === "signature_match"
      ? "signature"
      : record.controlClass === "semantic"
        ? "semantic"
        : "deterministic";
  const result: ControlResult =
    record.outcome === "pass"
      ? "passed"
      : record.outcome === "block"
        ? "blocked"
        : record.outcome === "redact"
          ? "redacted"
          : record.outcome === "not_applicable"
            ? "not_applicable"
            : record.outcome === "error"
              ? "unavailable"
              : "unknown";
  const label = FAMILY_LABEL[family];
  const source = family === "semantic" ? record.verdictSource : null;
  const detail = [
    record.matchedRuleId !== null ? `rule ${record.matchedRuleId}` : null,
    record.feedRevision !== null ? `signature feed ${record.feedRevision}` : null,
    `policy revision ${record.evaluatedCatalogRevisionId}`,
  ]
    .filter((part): part is string => part !== null)
    .join(" · ");
  return {
    family,
    label,
    result,
    source,
    text: badgeText(label, result, source),
    consequence: consequenceOf(result),
    detail,
  };
}

function badgeFromEvent(event: SafeEvent): DecisionBadge | null {
  const named = familyOfReason(event.reasonCode);
  if (named === null) {
    return null;
  }
  const label = FAMILY_LABEL[named.family];
  const detail = [
    event.maskedSummary.matchedRule !== null ? `rule ${event.maskedSummary.matchedRule}` : null,
    event.maskedSummary.feedRevision !== null
      ? `signature feed ${event.maskedSummary.feedRevision}`
      : null,
    event.catalogRevisionId !== null ? `policy revision ${event.catalogRevisionId}` : null,
  ]
    .filter((part): part is string => part !== null)
    .join(" · ");
  return {
    family: named.family,
    label,
    result: named.result,
    source: null,
    text: badgeText(label, named.result, null),
    consequence: consequenceOf(named.result),
    detail: detail === "" ? null : detail,
  };
}

/**
 * The control decisions to show on an event. Stored assessments that share the event's evaluation id
 * give the exact controls (and, for semantic ones, live or fixture); without them only a decision
 * whose reason code names one control is shown, never a guessed one.
 */
export function decisionBadges(
  event: SafeEvent,
  assessments: readonly AssessmentRecord[],
): DecisionBadge[] {
  const evaluationId = event.maskedSummary.evaluationId;
  if (evaluationId !== null) {
    const matching = assessments.filter((record) => record.evaluationId === evaluationId);
    if (matching.length > 0) {
      return matching.map(badgeFromAssessment);
    }
  }
  const fromEvent = badgeFromEvent(event);
  return fromEvent === null ? [] : [fromEvent];
}

/**
 * Describe one sanitized event. The kind comes from the event's type alone: a denied attempt is
 * never an effect even if its summary were inconsistent, and a type this build does not know is
 * shown as unknown instead of being dropped.
 */
export function describeEvent(
  event: SafeEvent,
  assessments: readonly AssessmentRecord[] = [],
): EventView {
  const known = Object.hasOwn(KNOWN_EVENTS, event.eventType) ? KNOWN_EVENTS[event.eventType] : null;
  let kind: EventKind = known?.kind ?? "unknown";
  if (kind === "effect" && isDenied(event)) {
    kind = "attempt";
  }
  const rejectionCause = event.maskedSummary.rejectionCause;
  const alternative = event.maskedSummary.alternativeTemplate;
  return {
    id: event.eventId,
    actionId: event.actionId,
    occurredAt: event.occurredAt,
    eventType: event.eventType,
    kind,
    tone: toneOf(event),
    title: known?.title ?? `Unknown event (${event.eventType})`,
    reasonCode: event.reasonCode,
    reasonText: reasonLabel(event.reasonCode),
    safeMessage: event.maskedSummary.safeMessage,
    rule: ruleText(event),
    effectText: effectText(event, kind),
    replayLabel:
      event.maskedSummary.replaySource !== null
        ? `Replay (${event.maskedSummary.replaySource}): a recorded proposal, not an action the model generated`
        : event.maskedSummary.inputSource === "judge"
          ? "Judge probe: submitted by an operator, not part of the agent's task"
          : null,
    correctionRoute:
      alternative !== null
        ? `Permitted alternative: a ${TEMPLATE_TEXT[alternative] ?? alternative}, rendered from approved fields`
        : null,
    rejectionText:
      rejectionCause !== null
        ? `Final answer rejected: ${REJECTION_TEXT[rejectionCause] ?? rejectionCause}`
        : null,
    facts: facts(event),
    badges: decisionBadges(event, assessments),
    unrecognized: known === null,
  };
}

export interface TimelineSummary {
  /** Distinct operations the agent attempted: stored actions plus proposals denied without one. */
  attemptedOperations: number;
  /** Proposals the gateway denied; each counts as a correction attempt against the run's limit. */
  deniedProposals: number;
  /** Completed effects, counted once per action and effect. */
  effects: { reads: number; reportsStored: number; messagesQueued: number };
  uncertain: number;
  replays: number;
  unrecognizedEvents: number;
}

/** Counts that keep attempted operations apart from completed effects. */
export function summarizeEvents(events: readonly SafeEvent[]): TimelineSummary {
  const actions = new Set<string>();
  let unattributed = 0;
  let deniedProposals = 0;
  let uncertain = 0;
  let replays = 0;
  let unrecognizedEvents = 0;
  const effectKeys = new Map<string, string>();
  for (const event of events) {
    const view = describeEvent(event);
    if (view.unrecognized) {
      unrecognizedEvents += 1;
    }
    if (view.replayLabel !== null && event.maskedSummary.replaySource !== null) {
      replays += 1;
    }
    if (
      view.kind === "attempt" ||
      view.kind === "effect" ||
      view.kind === "review" ||
      view.kind === "uncertain"
    ) {
      if (event.actionId !== null) {
        actions.add(event.actionId);
      } else if (event.eventType === "action.denied") {
        unattributed += 1;
      }
    }
    if (event.eventType === "action.denied" || event.eventType === "report.export_denied") {
      if (isDenied(event)) {
        deniedProposals += 1;
      }
    }
    if (view.kind === "uncertain") {
      uncertain += 1;
    }
    if (view.kind === "effect" && event.maskedSummary.effect !== null) {
      effectKeys.set(
        `${event.actionId ?? event.eventId}:${event.maskedSummary.effect}`,
        event.maskedSummary.effect,
      );
    }
  }
  const effects = { reads: 0, reportsStored: 0, messagesQueued: 0 };
  for (const effect of effectKeys.values()) {
    if (effect === "read") {
      effects.reads += 1;
    } else if (effect === "report_created") {
      effects.reportsStored += 1;
    } else if (effect === "outbox_message_queued") {
      effects.messagesQueued += 1;
    }
  }
  return {
    attemptedOperations: actions.size + unattributed,
    deniedProposals,
    effects,
    uncertain,
    replays,
    unrecognizedEvents,
  };
}

const TERMINAL_EVENTS: ReadonlySet<string> = new Set([
  "run.paused",
  "run.failed",
  "run.stopped",
  "run.completed",
]);

/**
 * The explanation the gateway recorded with the run's latest paused, failed, stopped or completed
 * event, or null when it recorded none. It is shown as stored and never composed here.
 */
export function terminalSafeMessage(events: readonly SafeEvent[]): string | null {
  for (const event of [...events].reverse()) {
    if (TERMINAL_EVENTS.has(event.eventType) && event.maskedSummary.safeMessage !== null) {
      return event.maskedSummary.safeMessage;
    }
  }
  return null;
}
