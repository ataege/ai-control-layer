import type { RunState } from "@workspace/contracts";
import { describe, expect, it } from "vitest";

import { CANCELLATION_EXPLANATION, cancelledStateText, isFinishedRun } from "./cancel-run-text";

function runIn(status: RunState["status"]): RunState {
  return {
    runId: "073514cb-6817-4cac-8af7-516e72952704",
    passportId: "cf67a545-4d64-49c6-ae9d-4a945bae8a69",
    status,
    terminalReason: status === "stopped" ? "run_cancelled" : null,
    cancelRequestedAt: "2026-10-04T00:00:00Z",
    createdAt: "2026-10-04T00:00:00Z",
    updatedAt: "2026-10-04T00:00:00Z",
    resultReference: null,
  };
}

describe("cancellation wording", () => {
  it("states that cancelling stops future work and reverses nothing", () => {
    expect(CANCELLATION_EXPLANATION).toMatch(/stops future work/);
    expect(CANCELLATION_EXPLANATION).toMatch(/does not reverse/);
    expect(CANCELLATION_EXPLANATION).toMatch(/stay recorded/);
  });

  it("reports the state the server recorded, never more", () => {
    expect(cancelledStateText(runIn("stopped"))).toBe("The run is stopped.");
    // A running run is stamped and stops at its next step; the page must not claim it stopped.
    expect(cancelledStateText(runIn("running"))).toMatch(/stops before its next step/);
    expect(cancelledStateText(runIn("completed"))).toMatch(/already ended/);
  });

  it("offers no cancel for a finished run", () => {
    expect(isFinishedRun("completed")).toBe(true);
    expect(isFinishedRun("stopped")).toBe(true);
    expect(isFinishedRun("awaiting_approval")).toBe(false);
  });
});
