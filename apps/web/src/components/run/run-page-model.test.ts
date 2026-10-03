import { describe, expect, it } from "vitest";
import type { RunState, SafeEvent } from "@workspace/contracts";
import completed from "@workspace/contracts/fixtures/run-state.completed.json";
import running from "@workspace/contracts/fixtures/run-state.running.json";
import exportDenied from "@workspace/contracts/fixtures/safe-event.export-denied.json";
import { awaitingActionId, isTerminalStatus, newEventsAfter, reportsOfRun } from "./run-page-model";

const asRun = (value: unknown, status?: string): RunState =>
  ({ ...(value as RunState), ...(status === undefined ? {} : { status }) }) as RunState;
const base = exportDenied as unknown as SafeEvent;

function event(
  eventId: string,
  eventType: string,
  actionId: string | null,
  summary: Partial<SafeEvent["maskedSummary"]> = {},
): SafeEvent {
  return {
    ...base,
    eventId,
    eventType: eventType as SafeEvent["eventType"],
    actionId,
    maskedSummary: {
      ...base.maskedSummary,
      reportId: null,
      template: null,
      classification: null,
      ...summary,
    },
  };
}

describe("isTerminalStatus", () => {
  it("stops polling for a completed, failed or stopped run only", () => {
    for (const status of ["completed", "failed", "stopped"])
      expect(isTerminalStatus(status)).toBe(true);
    for (const status of ["queued", "running", "awaiting_approval", "paused", "teleported"]) {
      expect(isTerminalStatus(status)).toBe(false);
    }
  });
});

describe("awaitingActionId", () => {
  const events = [
    event("1", "action.allowed", "a1"),
    event("2", "approval.requested", "a2"),
    event("3", "run.awaiting_approval", "a2"),
  ];

  it("names the waiting action of a run that awaits approval", () => {
    expect(awaitingActionId(asRun(running, "awaiting_approval"), events)).toBe("a2");
  });

  it("is null for a run in any other state, even with the same events", () => {
    for (const status of ["running", "completed", "paused", "stopped"]) {
      expect(awaitingActionId(asRun(running, status), events)).toBeNull();
    }
  });

  it("is null when no event names the action", () => {
    expect(
      awaitingActionId(asRun(running, "awaiting_approval"), [event("1", "run.started", null)]),
    ).toBeNull();
  });
});

describe("reportsOfRun", () => {
  it("lists each created report once with the label the gateway stored", () => {
    const reports = reportsOfRun(asRun(running), [
      event("1", "report.created", "a", {
        reportId: "r1",
        template: "internal_investigation_v1",
        classification: "internal_only",
      }),
      event("2", "report.created", "b", {
        reportId: "r2",
        template: "vendor_reconciliation_v1",
        classification: "vendor_shareable",
      }),
      event("3", "action.succeeded", "b", { reportId: "r2" }),
      event("4", "report.created", "b", {
        reportId: "r2",
        template: "vendor_reconciliation_v1",
        classification: "vendor_shareable",
      }),
    ]);
    expect(
      reports.map((report) => [report.reportId, report.classification, report.namedByFinalAnswer]),
    ).toEqual([
      ["r1", "internal_only", false],
      ["r2", "vendor_shareable", false],
    ]);
  });

  it("marks the reports the final answer named and lists a named one that no event shows", () => {
    const run = asRun({ ...(completed as object), resultReference: { reportIds: ["r2", "r9"] } });
    const reports = reportsOfRun(run, [
      event("2", "report.created", "b", {
        reportId: "r2",
        template: "vendor_reconciliation_v1",
        classification: "vendor_shareable",
      }),
    ]);
    expect(
      reports.map((report) => [report.reportId, report.namedByFinalAnswer, report.classification]),
    ).toEqual([
      ["r2", true, "vendor_shareable"],
      ["r9", true, null],
    ]);
  });
});

describe("newEventsAfter", () => {
  it("keeps only events after the last one shown, in order, even for large ids", () => {
    const shown = [event("9", "run.started", null)];
    const page = [
      event("9", "run.started", null),
      event("10", "run.paused", null),
      event("11", "run.stopped", null),
    ];
    expect(newEventsAfter(shown, page).map((item) => item.eventId)).toEqual(["10", "11"]);
    expect(newEventsAfter([], page)).toHaveLength(3);
    expect(
      newEventsAfter(
        [event("9223372036854775806", "x", null)],
        [event("9223372036854775807", "y", null)],
      ),
    ).toHaveLength(1);
  });
});
