import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/server/upstream-proxy", () => ({
  proxyUpstream: vi.fn(async () => new Response("{}")),
}));

import { proxyUpstream } from "@/server/upstream-proxy";
import { GET as readEvents } from "./[id]/events/route";
import { GET as readUsage } from "./[id]/usage/route";
import { POST as startJudgeRun } from "./judge/route";
import { POST as startRun } from "./route";

afterEach(() => vi.clearAllMocks());

const params = (id: string) => ({ params: Promise.resolve({ id }) });

describe("the run proxy routes", () => {
  it("starts a run without forwarding the browser's query string", async () => {
    await startRun(
      new Request("http://localhost/api/runs?admin=1&x=y", { method: "POST", body: "{}" }),
    );
    expect(proxyUpstream).toHaveBeenCalledTimes(1);
    expect(vi.mocked(proxyUpstream).mock.calls[0]?.[1]).toBe("/api/runs");
  });

  it("starts a judge run on its own path, without the browser's query string", async () => {
    await startJudgeRun(
      new Request("http://localhost/api/runs/judge?admin=1", { method: "POST", body: "{}" }),
    );
    expect(proxyUpstream).toHaveBeenCalledTimes(1);
    expect(vi.mocked(proxyUpstream).mock.calls[0]?.[1]).toBe("/api/runs/judge");
  });

  it("reads events as a buffered JSON page and keeps the paging query", async () => {
    await readEvents(
      new Request("http://localhost/api/runs/r1/events?after=41&limit=10"),
      params("r1"),
    );
    const call = vi.mocked(proxyUpstream).mock.calls[0];
    expect(call?.[1]).toBe("/api/runs/r1/events?after=41&limit=10");
    // No options: the default buffered, validated read with the upstream deadline.
    expect(call?.[2]).toBeUndefined();
  });

  it("reads the usage of an encoded run id", async () => {
    await readUsage(new Request("http://localhost/api/runs/a%20b/usage"), params("a b"));
    expect(vi.mocked(proxyUpstream).mock.calls[0]?.[1]).toBe("/api/runs/a%20b/usage");
  });
});
