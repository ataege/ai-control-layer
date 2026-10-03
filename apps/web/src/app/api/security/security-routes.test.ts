import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GET as getExport } from "@/app/api/security/export/route";
import { GET as getSummary } from "@/app/api/security/summary/route";

interface StubUpstream {
  baseUrl: string;
  received: { url: string; cookie: string | undefined }[];
  close: () => Promise<void>;
}

// A real HTTP server on an ephemeral port, standing in for the API.
async function startStub(
  handler: (request: IncomingMessage, response: ServerResponse) => void,
): Promise<StubUpstream> {
  const received: StubUpstream["received"] = [];
  const server: Server = createServer((request, response) => {
    received.push({ url: request.url ?? "", cookie: request.headers.cookie });
    handler(request, response);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as AddressInfo;
  return {
    baseUrl: `http://127.0.0.1:${port}`,
    received,
    close: () =>
      new Promise<void>((resolve) => {
        server.closeAllConnections();
        server.close(() => resolve());
      }),
  };
}

function webRequest(path: string): Request {
  return new Request(`http://localhost:3000${path}`, {
    headers: { cookie: "session=abc; other=x" },
  });
}

let upstream: StubUpstream | undefined;

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

afterEach(async () => {
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  await upstream?.close();
  upstream = undefined;
});

describe("GET /api/security/summary", () => {
  it("forwards to the API's summary with the session cookie only", async () => {
    upstream = await startStub((_request, response) => {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ organizationId: "org" }));
    });
    vi.stubEnv("API_UPSTREAM_URL", upstream.baseUrl);

    const response = await getSummary(webRequest("/api/security/summary"));

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ organizationId: "org" });
    expect(upstream.received).toEqual([{ url: "/api/security/summary", cookie: "session=abc" }]);
  });

  it("passes a refusal through unchanged", async () => {
    upstream = await startStub((_request, response) => {
      response.writeHead(401, { "content-type": "application/json" });
      response.end(JSON.stringify({ error: { code: "unauthorized" } }));
    });
    vi.stubEnv("API_UPSTREAM_URL", upstream.baseUrl);
    expect((await getSummary(webRequest("/api/security/summary"))).status).toBe(401);
  });
});

describe("GET /api/security/export", () => {
  it("forwards the whole query and returns the JSON page as served", async () => {
    const page = { events: [], nextCursor: "v1.0.0.0" };
    upstream = await startStub((_request, response) => {
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify(page));
    });
    vi.stubEnv("API_UPSTREAM_URL", upstream.baseUrl);

    const response = await getExport(
      webRequest("/api/security/export?kind=events&format=json&after=v1.0.0.0&limit=5"),
    );

    expect(await response.json()).toEqual(page);
    expect(upstream.received[0]?.url).toBe(
      "/api/security/export?kind=events&format=json&after=v1.0.0.0&limit=5",
    );
  });

  it("streams a CSV body instead of rejecting it as non-JSON", async () => {
    const csv = '"eventId","eventType"\r\n"1","run.queued"\r\n';
    upstream = await startStub((_request, response) => {
      response.writeHead(200, { "content-type": "text/csv; charset=utf-8" });
      response.end(csv);
    });
    vi.stubEnv("API_UPSTREAM_URL", upstream.baseUrl);

    const response = await getExport(
      webRequest("/api/security/export?kind=events&format=csv&limit=5"),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("text/csv; charset=utf-8");
    expect(await response.text()).toBe(csv);
  });

  it("passes the reviewer-role refusal (403) through for JSON and CSV", async () => {
    upstream = await startStub((_request, response) => {
      response.writeHead(403, { "content-type": "application/json" });
      response.end(JSON.stringify({ error: { code: "forbidden" } }));
    });
    vi.stubEnv("API_UPSTREAM_URL", upstream.baseUrl);

    expect(
      (await getExport(webRequest("/api/security/export?kind=events&format=json&limit=1"))).status,
    ).toBe(403);
    expect(
      (await getExport(webRequest("/api/security/export?kind=events&format=csv&limit=1"))).status,
    ).toBe(403);
  });

  it("answers 502 when the API sends a non-JSON body for a JSON request", async () => {
    upstream = await startStub((_request, response) => {
      response.writeHead(200, { "content-type": "text/html" });
      response.end("<html>");
    });
    vi.stubEnv("API_UPSTREAM_URL", upstream.baseUrl);
    expect(
      (await getExport(webRequest("/api/security/export?kind=events&format=json&limit=1"))).status,
    ).toBe(502);
  });
});
