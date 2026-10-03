import type { RunState } from "@workspace/contracts";

/** The limitation the confirmation states (WEB-15, Journey 3): cancelling is not a reversal. */
export const CANCELLATION_EXPLANATION =
  "Cancelling stops future work on this run: no further model request, tool call or report is started. It does not reverse anything already done: reports already created and messages already queued stay recorded.";

const FINISHED_STATUSES: ReadonlySet<RunState["status"]> = new Set([
  "completed",
  "failed",
  "stopped",
]);

/** Whether a run has ended, so there is nothing left to cancel. */
export function isFinishedRun(status: RunState["status"]): boolean {
  return FINISHED_STATUSES.has(status);
}

/** What the server recorded: stopped now, or stopping at the run's next step. */
export function cancelledStateText(run: RunState): string {
  if (run.status === "stopped") return "The run is stopped.";
  if (isFinishedRun(run.status)) return `The run had already ended (${run.status}).`;
  return "The run stops before its next step; a step already in progress finishes first.";
}
