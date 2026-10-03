import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { afterEach, describe, expect, it, vi } from "vitest";
import { POST } from "./route";

describe("POST /api/control/evaluate", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("forwards the body and session cookie to the API and passes its status through", async () => {
    let seen: { url?: string; method?: string; cookie?: string; body: string } = { body: "" };
    const server = createServer((request, response) => {
      request.on("data", (chunk) => (seen.body += chunk));
      request.on("end", () => {
        seen = {
          ...seen,
          url: request.url,
          method: request.method,
          cookie: request.headers.cookie,
        };
        response.writeHead(404, { "content-type": "application/json" });
        response.end(JSON.stringify({ error: { code: "not_found" } }));
      });
    });
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    vi.stubEnv("API_UPSTREAM_URL", `http://127.0.0.1:${(server.address() as AddressInfo).port}`);

    const body = JSON.stringify({ runId: "r", kind: "model_input", text: "hi" });
    const response = await POST(
      new Request("http://web.test/api/control/evaluate", {
        method: "POST",
        headers: { "content-type": "application/json", cookie: "session=abc; other=1" },
        body,
      }),
    );
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));

    expect(response.status).toBe(404);
    expect(seen).toMatchObject({
      url: "/api/control/evaluate",
      method: "POST",
      cookie: "session=abc",
      body,
    });
  });
});
