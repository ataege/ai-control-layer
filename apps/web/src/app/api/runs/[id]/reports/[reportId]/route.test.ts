import { beforeEach, describe, expect, it, vi } from "vitest";

const proxyUpstream = vi.hoisted(() => vi.fn());
vi.mock("@/server/upstream-proxy", () => ({ proxyUpstream }));

import { GET } from "./route";

describe("GET /api/runs/{id}/reports/{reportId}", () => {
  beforeEach(() => {
    proxyUpstream.mockReset();
    proxyUpstream.mockResolvedValue(new Response("{}"));
  });

  it("forwards to the API's stored report route with both segments encoded", async () => {
    const request = new Request("http://localhost/api/runs/a/reports/b?injected=1");
    await GET(request, { params: Promise.resolve({ id: "run/../1", reportId: "rep ort" }) });
    expect(proxyUpstream).toHaveBeenCalledWith(request, "/api/runs/run%2F..%2F1/reports/rep%20ort");
  });
});
