import { afterEach, describe, expect, it, vi } from "vitest";
import runStateCompleted from "@workspace/contracts/fixtures/run-state.completed.json";
import runStatePaused from "@workspace/contracts/fixtures/run-state.paused-allowance.json";
import runUsageLedger from "@workspace/contracts/fixtures/run-usage.ledger.json";
import runUsageNoLedger from "@workspace/contracts/fixtures/run-usage.no-ledger.json";
import eventsPage from "@workspace/contracts/fixtures/run-events-page.export-denied.json";
import { fetchJson } from "./fetch-json";
import { ProductClient, isRunEventsPage, isRunState, isRunUsage } from "./product-client";

vi.mock("./fetch-json", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./fetch-json")>();
  return { ...actual, fetchJson: vi.fn(), postJson: vi.fn() };
});

function answer(data: unknown) {
  vi.mocked(fetchJson).mockResolvedValueOnce({ ok: true, status: 200, data, durationMs: 1 });
}

afterEach(() => vi.clearAllMocks());

describe("the run reads use the real contracts", () => {
  it("accepts the X-11 run state the API returns, and refuses the invented run view", () => {
    expect(isRunState(runStateCompleted)).toBe(true);
    expect(isRunState(runStatePaused)).toBe(true);
    // The shape that 49d0320 invented: an id and a passport object instead of the persisted state.
    expect(isRunState({ id: "r", status: "running", passport: { template: "t" } })).toBe(false);
    expect(isRunState({ ...runStateCompleted, terminalReason: 5 })).toBe(false);
    expect(isRunState({ ...runStateCompleted, resultReference: { reportIds: [1] } })).toBe(false);
  });

  it("keeps an unknown status as a string so the page can show it as unknown", () => {
    expect(isRunState({ ...runStateCompleted, status: "teleported" })).toBe(true);
  });

  it("accepts the usage fixtures, with and without a ledger, and refuses a broken one", () => {
    expect(isRunUsage(runUsageLedger)).toBe(true);
    expect(isRunUsage(runUsageNoLedger)).toBe(true);
    expect(isRunUsage({ ...runUsageLedger, ledger: { paused: false } })).toBe(false);
    expect(isRunUsage({ ...runUsageLedger, toolAttempts: null })).toBe(false);
    expect(isRunUsage({ runId: "r", modelCalls: [{ purpose: "other" }], ledger: null })).toBe(
      false,
    );
  });

  it("accepts the events page fixture and refuses an event without its summary", () => {
    expect(isRunEventsPage(eventsPage)).toBe(true);
    expect(isRunEventsPage({ events: [{ eventId: "1" }], nextCursor: "1" })).toBe(false);
    expect(isRunEventsPage({ events: [], nextCursor: 1 })).toBe(false);
  });
});

describe("ProductClient run reads", () => {
  it("reads the run state from /api/runs/:id", async () => {
    answer(runStateCompleted);
    const result = await ProductClient.getRun("r 1");
    expect(fetchJson).toHaveBeenCalledWith("/api/runs/r%201", undefined);
    expect(result.ok && result.data.status).toBe("completed");
  });

  it("refuses a body that is not a run state instead of passing it on", async () => {
    answer({ id: "r", status: "running", passport: {} });
    const result = await ProductClient.getRun("r");
    expect(result).toMatchObject({ ok: false, error: { kind: "invalid_json", status: 200 } });
  });

  it("reads the usage from /api/runs/:id/usage", async () => {
    answer(runUsageLedger);
    const result = await ProductClient.getUsage("r");
    expect(fetchJson).toHaveBeenCalledWith("/api/runs/r/usage", undefined);
    expect(result.ok).toBe(true);
  });

  it("pages events with the API's `after` parameter, never `cursor`", async () => {
    answer(eventsPage);
    await ProductClient.getRunEvents("r", "41");
    expect(fetchJson).toHaveBeenCalledWith("/api/runs/r/events?after=41", undefined);
    answer(eventsPage);
    await ProductClient.getRunEvents("r");
    expect(fetchJson).toHaveBeenLastCalledWith("/api/runs/r/events", undefined);
    for (const call of vi.mocked(fetchJson).mock.calls) {
      expect(String(call[0])).not.toContain("cursor");
    }
  });
});
