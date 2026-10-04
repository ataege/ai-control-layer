import type { RunEventsPage, RunState, RunUsage, SafeEvent } from "@workspace/contracts";
import runStateRunning from "@workspace/contracts/fixtures/run-state.running.json";
import runStateCompleted from "@workspace/contracts/fixtures/run-state.completed.json";
import usageFixture from "@workspace/contracts/fixtures/run-usage.ledger.json";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProductClient } from "@/lib/product-client";
import { createRunPoller } from "./run-poller";

// The poller's two guarantees: the cursor moves only past a page that was received, and a failed read
// keeps the last real events and reports the failure.

const ok = <Data>(data: Data) => ({ ok: true as const, status: 200, data, durationMs: 1 });
const failure = (error: unknown) => ({ ok: false as const, error, durationMs: 1 }) as never;

function safeEvent(eventId: string): SafeEvent {
  return {
    eventId,
    eventType: "run.started",
    occurredAt: "2026-10-04T10:00:00Z",
    actionId: null,
    maskedSummary: {},
  } as unknown as SafeEvent;
}

const page = (ids: string[], nextCursor: string): RunEventsPage =>
  ({ events: ids.map(safeEvent), nextCursor }) as RunEventsPage;

function stubReads(options: { runs: unknown[]; pages: unknown[]; usages?: unknown[] }) {
  const runs = [...options.runs];
  const pages = [...options.pages];
  const usages = [...(options.usages ?? [])];
  vi.spyOn(ProductClient, "getRun").mockImplementation(async () => runs.shift() as never);
  vi.spyOn(ProductClient, "getRunEvents").mockImplementation(async () => pages.shift() as never);
  vi.spyOn(ProductClient, "getUsage").mockImplementation(
    async () => (usages.shift() ?? ok(usageFixture as unknown as RunUsage)) as never,
  );
}

afterEach(() => vi.restoreAllMocks());

describe("the events cursor", () => {
  it("starts with no cursor and advances only to the one the received page returned", async () => {
    stubReads({
      runs: [ok(runStateRunning as RunState), ok(runStateRunning as RunState)],
      pages: [ok(page(["1", "2"], "c-2")), ok(page(["3"], "c-3"))],
    });
    const poller = createRunPoller("run_1");
    await poller.refresh();
    await poller.refresh();
    const calls = vi.mocked(ProductClient.getRunEvents).mock.calls.map((call) => call[1]);
    expect(calls).toEqual([undefined, "c-2"]);
  });

  it("does not advance the cursor when the read failed, and asks for the same events again", async () => {
    stubReads({
      runs: [
        ok(runStateRunning as RunState),
        ok(runStateRunning as RunState),
        ok(runStateRunning as RunState),
      ],
      pages: [
        ok(page(["1"], "c-1")),
        failure({ kind: "network", message: "offline" }),
        ok(page(["2"], "c-2")),
      ],
    });
    const poller = createRunPoller("run_1");
    await poller.refresh();
    await poller.refresh();
    await poller.refresh();
    const calls = vi.mocked(ProductClient.getRunEvents).mock.calls.map((call) => call[1]);
    // The failed second poll leaves the cursor at "c-1", so the third asks after "c-1" again.
    expect(calls).toEqual([undefined, "c-1", "c-1"]);
  });

  it("shows each event once even when a page repeats events already received", async () => {
    stubReads({
      runs: [ok(runStateRunning as RunState), ok(runStateRunning as RunState)],
      pages: [ok(page(["1", "2"], "c-2")), ok(page(["2", "3"], "c-3"))],
    });
    const poller = createRunPoller("run_1");
    await poller.refresh();
    const second = await poller.refresh();
    expect(second?.events.map((event) => event.eventId)).toEqual(["1", "2", "3"]);
  });
});

describe("a failed read", () => {
  it("keeps the events already received and reports the failure instead of a state", async () => {
    stubReads({
      runs: [ok(runStateRunning as RunState), ok(runStateRunning as RunState)],
      pages: [ok(page(["1", "2"], "c-2")), failure({ kind: "timeout", timeoutMs: 15000 })],
    });
    const poller = createRunPoller("run_1");
    await poller.refresh();
    const second = await poller.refresh();
    expect(second?.events.map((event) => event.eventId)).toEqual(["1", "2"]);
    expect(second?.error).toBeTruthy();
    expect(second?.keepPolling).toBe(true);
  });

  it("reports a failed run-state read with no run, keeps the events and keeps polling", async () => {
    stubReads({
      runs: [
        ok(runStateRunning as RunState),
        failure({ kind: "http", status: 503, body: undefined }),
      ],
      pages: [ok(page(["1"], "c-1"))],
    });
    const poller = createRunPoller("run_1");
    await poller.refresh();
    const second = await poller.refresh();
    expect(second?.run).toBeUndefined();
    expect(second?.events.map((event) => event.eventId)).toEqual(["1"]);
    expect(second?.error).toBeTruthy();
    expect(second?.keepPolling).toBe(true);
    // The events were not read after the state failed: nothing is shown from a half-failed refresh.
    expect(ProductClient.getRunEvents).toHaveBeenCalledTimes(1);
  });

  it("reports a failed usage read but still returns the run and events it did read", async () => {
    stubReads({
      runs: [ok(runStateRunning as RunState)],
      pages: [ok(page(["1"], "c-1"))],
      usages: [failure({ kind: "http", status: 500, body: undefined })],
    });
    const result = await createRunPoller("run_1").refresh();
    expect(result?.run).toBeDefined();
    expect(result?.usage).toBeUndefined();
    expect(result?.events).toHaveLength(1);
    expect(result?.error).toBeTruthy();
  });

  it("never turns the server's message into the shown failure", async () => {
    stubReads({
      runs: [ok(runStateRunning as RunState)],
      pages: [
        failure({
          kind: "http",
          status: 500,
          body: { error: { code: "internal_error", message: "db password secret" } },
        }),
      ],
    });
    const result = await createRunPoller("run_1").refresh();
    expect(result?.error).not.toContain("secret");
  });
});

describe("polling", () => {
  it("stops once the run is finished and continues while it is not", async () => {
    stubReads({
      runs: [ok(runStateRunning as RunState), ok(runStateCompleted as RunState)],
      pages: [ok(page(["1"], "c-1")), ok(page([], "c-1"))],
    });
    const poller = createRunPoller("run_1");
    expect((await poller.refresh())?.keepPolling).toBe(true);
    expect((await poller.refresh())?.keepPolling).toBe(false);
  });

  it("returns null, not a failure, when the page left and the read was cancelled", async () => {
    stubReads({ runs: [failure({ kind: "aborted" })], pages: [] });
    expect(await createRunPoller("run_1").refresh()).toBeNull();
    vi.restoreAllMocks();
    stubReads({
      runs: [ok(runStateRunning as RunState)],
      pages: [failure({ kind: "aborted" })],
    });
    expect(await createRunPoller("run_1").refresh()).toBeNull();
  });

  it("reads the further pages of a full page in the same refresh", async () => {
    const fullPage = Array.from({ length: 500 }, (_, index) => String(index + 1));
    stubReads({
      runs: [ok(runStateRunning as RunState)],
      pages: [ok(page(fullPage, "c-500")), ok(page(["501"], "c-501"))],
    });
    const result = await createRunPoller("run_1").refresh();
    expect(result?.events).toHaveLength(501);
    const cursors = vi.mocked(ProductClient.getRunEvents).mock.calls.map((call) => call[1]);
    expect(cursors).toEqual([undefined, "c-500"]);
  });
});
