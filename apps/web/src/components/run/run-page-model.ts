import type { RunState, SafeEvent } from "@workspace/contracts";

// Pure decisions of the run page: whether to keep polling, which action waits for a reviewer, and
// which reports the run created. All of it is read from persisted states and events; the page infers
// nothing of its own.

const TERMINAL_STATUSES: ReadonlySet<string> = new Set(["completed", "failed", "stopped"]);

/** A completed, failed or stopped run no longer changes; a paused run may still be reconciled. */
export function isTerminalStatus(status: string): boolean {
  return TERMINAL_STATUSES.has(status);
}

/**
 * The stored action a waiting run needs a reviewer for: the action of its latest
 * `run.awaiting_approval` (or `approval.requested`) event. Null unless the run is awaiting approval.
 */
export function awaitingActionId(run: RunState, events: readonly SafeEvent[]): string | null {
  if (run.status !== "awaiting_approval") {
    return null;
  }
  for (const event of [...events].reverse()) {
    if (
      (event.eventType === "run.awaiting_approval" || event.eventType === "approval.requested") &&
      event.actionId !== null
    ) {
      return event.actionId;
    }
  }
  return null;
}

export interface RunReport {
  reportId: string;
  template: string | null;
  classification: "internal_only" | "vendor_shareable" | null;
  /** True when the completed run's final answer named this report. */
  namedByFinalAnswer: boolean;
}

/**
 * The reports this run created (its `report.created` events, once each) with the label the gateway
 * stored. A report only named by the final answer is listed too, without a label.
 */
export function reportsOfRun(run: RunState, events: readonly SafeEvent[]): RunReport[] {
  const named = new Set(run.resultReference?.reportIds ?? []);
  const reports = new Map<string, RunReport>();
  for (const event of events) {
    const { reportId, template, classification } = event.maskedSummary;
    if (event.eventType === "report.created" && reportId !== null && !reports.has(reportId)) {
      reports.set(reportId, {
        reportId,
        template,
        classification,
        namedByFinalAnswer: named.has(reportId),
      });
    }
  }
  for (const reportId of named) {
    if (!reports.has(reportId)) {
      reports.set(reportId, {
        reportId,
        template: null,
        classification: null,
        namedByFinalAnswer: true,
      });
    }
  }
  return [...reports.values()];
}

/** The events a poll has not shown yet: ids are decimal strings in increasing order. */
export function newEventsAfter(
  shown: readonly SafeEvent[],
  page: readonly SafeEvent[],
): SafeEvent[] {
  const last = shown.length > 0 ? BigInt(shown[shown.length - 1]?.eventId ?? "0") : 0n;
  return page.filter((event) => BigInt(event.eventId) > last);
}
