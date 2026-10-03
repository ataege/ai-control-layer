import { describe, expect, it, vi } from "vitest";

const { proxyUpstream } = vi.hoisted(() => ({
  proxyUpstream: vi.fn(async () => new Response("{}", { status: 200 })),
}));
vi.mock("@/server/upstream-proxy", () => ({ proxyUpstream }));

import { GET } from "./route";

describe("GET /api/policies/status", () => {
  it("asks the shared proxy for the API's catalog status, with no query forwarded", async () => {
    const request = new Request("http://localhost:3000/api/policies/status?revision=9");
    const response = await GET(request);
    expect(response.status).toBe(200);
    expect(proxyUpstream).toHaveBeenCalledTimes(1);
    expect(proxyUpstream).toHaveBeenCalledWith(request, "/api/policies/status");
  });
});
