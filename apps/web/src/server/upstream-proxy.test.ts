import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GET as getGatewayDiagnostics } from "@/app/api/diagnostics/gateway/route";
import { GET as getLiveness } from "@/app/api/health/live/route";
import { GET as getReadiness } from "@/app/api/health/ready/route";
import { proxyUpstreamGet, type UpstreamPath } from "@/server/upstream-proxy";

type StubHandler = (request: IncomingMessage, response: ServerResponse) => void;

interface StubUpstream {
  baseUrl: string;
  receivedRequests: IncomingMessage[];
  close: () => Promise<void>;
}

// Real HTTP server on an ephemeral port, standing in for the API.
async function startStubUpstream(handler: StubHandler): Promise<StubUpstream> {
  const receivedRequests: IncomingMessage[] = [];
  const server: Server = createServer((request, response) => {
    receivedRequests.push(request);
    handler(request, response);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as AddressInfo;

  return {
    baseUrl: `http://127.0.0.1:${port}`,
    receivedRequests,
    close: () =>
      new Promise<void>((resolve) => {
        server.closeAllConnections();
        server.close(() => resolve());
      }),
  };
}

function sendJson(
  response: ServerResponse,
  statusCode: number,
  body: unknown,
  headers: Record<string, string> = {},
): void {
  response.writeHead(statusCode, { "content-type": "application/json", ...headers });
  response.end(JSON.stringify(body));
}

function incomingRequest(path: string, headers: Record<string, string> = {}): Request {
  return new Request(`http://web.test${path}`, { headers });
}

describe("upstream proxy", () => {
  let stubUpstream: StubUpstream | undefined;

  beforeEach(() => {
    // The proxy logs failure codes; keep test output quiet.
    vi.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(async () => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
    await stubUpstream?.close();
    stubUpstream = undefined;
  });

  it("forwards each route to its fixed upstream path and passes the body through", async () => {
    stubUpstream = await startStubUpstream((request, response) => {
      sendJson(response, 200, { status: "ok", service: "api", seenPath: request.url });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const routes = [
      { handler: getLiveness, path: "/api/health/live" },
      { handler: getReadiness, path: "/api/health/ready" },
      { handler: getGatewayDiagnostics, path: "/api/diagnostics/gateway" },
    ];
    for (const route of routes) {
      // The caller's query string must not reach the upstream.
      const response = await route.handler(incomingRequest(`${route.path}?target=/other`));

      expect(response.status).toBe(200);
      expect(response.headers.get("cache-control")).toBe("no-store");
      expect(await response.json()).toEqual({ status: "ok", service: "api", seenPath: route.path });
    }
    expect(stubUpstream.receivedRequests.map((request) => request.method)).toEqual([
      "GET",
      "GET",
      "GET",
    ]);
  });

  it("passes a non-2xx upstream status and body through unchanged", async () => {
    const readinessReport = {
      status: "error",
      info: {},
      error: { database: { status: "down", message: "database unreachable" } },
      details: { database: { status: "down", message: "database unreachable" } },
    };
    stubUpstream = await startStubUpstream((_request, response) => {
      sendJson(response, 503, readinessReport);
    });
    vi.stubEnv("API_UPSTREAM_URL", `${stubUpstream.baseUrl}/`);

    const response = await getReadiness(incomingRequest("/api/health/ready"));

    expect(response.status).toBe(503);
    expect(await response.json()).toEqual(readinessReport);
  });

  it("forwards a valid inbound request ID and returns the upstream one", async () => {
    stubUpstream = await startStubUpstream((request, response) => {
      const forwardedRequestId = String(request.headers["x-request-id"]);
      sendJson(response, 200, { status: "ok" }, { "x-request-id": forwardedRequestId });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await getLiveness(
      incomingRequest("/api/health/live", { "x-request-id": "client-request.42" }),
    );

    expect(stubUpstream.receivedRequests[0]?.headers["x-request-id"]).toBe("client-request.42");
    expect(response.headers.get("x-request-id")).toBe("client-request.42");
  });

  it("replaces a missing or malformed request ID and forwards the session cookie but not authorization", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      sendJson(response, 200, { status: "ok" });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await getLiveness(
      incomingRequest("/api/health/live", {
        "x-request-id": "bad id with spaces",
        authorization: "Bearer caller-token",
        cookie: "other=123; session=caller; foo=bar",
      }),
    );

    const upstreamHeaders = stubUpstream.receivedRequests[0]?.headers;
    const generatedRequestId = upstreamHeaders?.["x-request-id"];
    expect(generatedRequestId).toMatch(/^[A-Za-z0-9._-]{1,64}$/);
    expect(generatedRequestId).not.toBe("bad id with spaces");
    expect(response.headers.get("x-request-id")).toBe(generatedRequestId);
    expect(upstreamHeaders?.authorization).toBeUndefined();
    expect(upstreamHeaders?.cookie).toBe("session=caller");
  });

  it("returns a 502 envelope when the upstream is down", async () => {
    // Start and stop a server to get a port that is certainly closed.
    const closedUpstream = await startStubUpstream(() => undefined);
    await closedUpstream.close();
    vi.stubEnv("API_UPSTREAM_URL", closedUpstream.baseUrl);

    const response = await getLiveness(
      incomingRequest("/api/health/live", { "x-request-id": "request-502" }),
    );
    const body = await response.json();

    expect(response.status).toBe(502);
    expect(body).toMatchObject({
      error: { code: "upstream_unreachable" },
      statusCode: 502,
      requestId: "request-502",
      path: "/api/health/live",
    });
    expect(Number.isNaN(Date.parse(body.timestamp))).toBe(false);
    expect(JSON.stringify(body)).not.toContain(closedUpstream.baseUrl);
  });

  it("returns a 504 envelope when the upstream does not answer in time", async () => {
    // Accepts the request and never responds.
    stubUpstream = await startStubUpstream(() => undefined);
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await proxyUpstreamGet(
      incomingRequest("/api/diagnostics/gateway"),
      "/api/diagnostics/gateway",
      { timeoutMs: 50 },
    );

    expect(response.status).toBe(504);
    expect(await response.json()).toMatchObject({
      error: { code: "upstream_timeout" },
      statusCode: 504,
    });
  });

  it("returns a 502 envelope when the upstream body is not JSON", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      response.writeHead(200, { "content-type": "text/html" });
      response.end("<html>not the API</html>");
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await getLiveness(incomingRequest("/api/health/live"));

    expect(response.status).toBe(502);
    expect(await response.json()).toMatchObject({ error: { code: "upstream_invalid_response" } });
  });

  it.each([
    { label: "missing", configuredValue: undefined },
    { label: "malformed", configuredValue: "not a url" },
    { label: "non-http", configuredValue: "file:///etc/hosts" },
    { label: "with credentials", configuredValue: "http://user:secret-value@127.0.0.1:1" },
  ])(
    "returns a 500 envelope without leaking a $label config value",
    async ({ configuredValue }) => {
      vi.stubEnv("API_UPSTREAM_URL", configuredValue);

      const response = await getLiveness(incomingRequest("/api/health/live"));
      const bodyText = await response.text();

      expect(response.status).toBe(500);
      expect(JSON.parse(bodyText)).toMatchObject({
        error: { code: "configuration_error" },
        statusCode: 500,
      });
      expect(response.headers.get("x-request-id")).toMatch(/^[A-Za-z0-9._-]{1,64}$/);
      expect(bodyText).not.toContain("API_UPSTREAM_URL");
      if (configuredValue) {
        expect(bodyText).not.toContain(configuredValue);
        expect(JSON.stringify(vi.mocked(console.error).mock.calls)).not.toContain(configuredValue);
      }
    },
  );

  it("refuses a path that is not on the allowlist", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      sendJson(response, 200, { status: "ok" });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await proxyUpstreamGet(
      incomingRequest("/api/docs"),
      "/api/docs" as UpstreamPath,
    );

    expect(response.status).toBe(500);
    expect(stubUpstream.receivedRequests).toHaveLength(0);
  });
});
