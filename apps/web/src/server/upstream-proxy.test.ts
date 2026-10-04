import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import type { AddressInfo } from "node:net";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GET as getGatewayDiagnostics } from "@/app/api/diagnostics/gateway/route";
import { GET as getLiveness } from "@/app/api/health/live/route";
import { GET as getReadiness } from "@/app/api/health/ready/route";
import { GET as getRunEvents } from "@/app/api/runs/[id]/events/route";
import { GET as getRun } from "@/app/api/runs/[id]/route";
import { POST as postRun } from "@/app/api/runs/route";
import { proxyUpstream, proxyUpstreamGet, type UpstreamPath } from "@/server/upstream-proxy";

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
  // WEB-02: the browser reaches the API only through these same-origin handlers.
  it("never forwards spoofed forwarding, host, hop-by-hop or credential headers", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      sendJson(response, 200, { status: "ok" });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    await getRun(
      incomingRequest("/api/runs/run-1", {
        "x-forwarded-for": "203.0.113.9",
        "x-forwarded-host": "evil.example",
        "x-forwarded-proto": "https",
        forwarded: "for=203.0.113.9",
        "x-operator-context": "forged",
        "x-service-token": "forged",
        authorization: "Bearer forged",
        "proxy-authorization": "Basic forged",
        te: "trailers",
        upgrade: "websocket",
        "x-real-ip": "203.0.113.9",
        cookie: "session=abc; other=1",
      }),
      { params: Promise.resolve({ id: "run-1" }) },
    );

    const upstreamHeaders = stubUpstream.receivedRequests[0]?.headers ?? {};
    for (const forbidden of [
      "x-forwarded-for",
      "x-forwarded-host",
      "x-forwarded-proto",
      "forwarded",
      "x-operator-context",
      "x-service-token",
      "authorization",
      "proxy-authorization",
      "te",
      "upgrade",
      "x-real-ip",
    ]) {
      expect(upstreamHeaders[forbidden], forbidden).toBeUndefined();
    }
    // The host is the upstream's own, not the browser's, and only the session cookie travels.
    expect(upstreamHeaders.host).toBe(new URL(stubUpstream.baseUrl).host);
    expect(upstreamHeaders.cookie).toBe("session=abc");
  });

  it("refuses encoded traversal and paths outside the allowlist before any upstream call", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      sendJson(response, 200, { status: "ok" });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    for (const path of [
      "/api/runs/%2e%2e/%2e%2e/internal/runs",
      "/api/runs/../../internal/runs",
      "/internal/runs",
      "/api/runsx",
      "/apix/runs",
    ]) {
      const response = await proxyUpstream(incomingRequest("/api/runs"), path);
      expect(response.status, path).toBe(500);
    }
    expect(stubUpstream.receivedRequests).toHaveLength(0);
  });

  it("keeps an encoded slash inside one segment and never contacts a host named in the path", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      sendJson(response, 200, { status: "ok" });
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    for (const path of ["/api/runs/..%2f..%2finternal", "//evil.example/api/runs"]) {
      expect((await proxyUpstream(incomingRequest("/api/runs"), path)).status, path).toBe(200);
    }
    // Both reached the configured upstream only: one encoded segment, and no other host.
    expect(stubUpstream.receivedRequests.map((received) => received.url)).toEqual([
      "/api/runs/..%2f..%2finternal",
      "/api/runs",
    ]);
  });

  it("delivers every Set-Cookie header of the upstream", async () => {
    stubUpstream = await startStubUpstream((_request, response) => {
      response.writeHead(200, {
        "content-type": "application/json",
        "set-cookie": ["session=one; HttpOnly; SameSite=Lax", "theme=dark; Path=/"],
      });
      response.end("{}");
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await proxyUpstream(incomingRequest("/api/auth/sign-in"), "/api/auth/sign-in");

    expect(response.headers.getSetCookie()).toEqual([
      "session=one; HttpOnly; SameSite=Lax",
      "theme=dark; Path=/",
    ]);
  });

  it("reads the upstream configuration on every request", async () => {
    const first = await startStubUpstream((_request, response) =>
      sendJson(response, 200, { n: 1 }),
    );
    const second = await startStubUpstream((_request, response) =>
      sendJson(response, 200, { n: 2 }),
    );
    stubUpstream = second;
    try {
      vi.stubEnv("API_UPSTREAM_URL", first.baseUrl);
      expect(await (await getLiveness(incomingRequest("/api/health/live"))).json()).toEqual({
        n: 1,
      });
      vi.stubEnv("API_UPSTREAM_URL", second.baseUrl);
      expect(await (await getLiveness(incomingRequest("/api/health/live"))).json()).toEqual({
        n: 2,
      });
      vi.stubEnv("API_UPSTREAM_URL", "");
      expect((await getLiveness(incomingRequest("/api/health/live"))).status).toBe(500);
    } finally {
      await first.close();
    }
  });

  it("passes a command body and content type to the API and returns its 201 unchanged", async () => {
    let receivedBody = "";
    stubUpstream = await startStubUpstream((request, response) => {
      request.on("data", (chunk: Buffer) => (receivedBody += chunk.toString()));
      request.on("end", () => sendJson(response, 201, { runId: "r", passportId: "p" }));
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const response = await postRun(
      new Request("http://web.test/api/runs", {
        method: "POST",
        headers: { "content-type": "application/json", cookie: "session=abc" },
        body: JSON.stringify({ taskId: "reconcile_atlas_v1" }),
      }),
    );

    expect(response.status).toBe(201);
    expect(receivedBody).toBe('{"taskId":"reconcile_atlas_v1"}');
    expect(stubUpstream.receivedRequests[0]?.method).toBe("POST");
    expect(stubUpstream.receivedRequests[0]?.headers["content-type"]).toBe("application/json");
    expect(stubUpstream.receivedRequests[0]?.headers.cookie).toBe("session=abc");
  });

  it("carries the events cursor to the API and passes an empty 2xx body through", async () => {
    stubUpstream = await startStubUpstream((request, response) => {
      if (request.url?.startsWith("/api/runs/run-1/events")) {
        sendJson(response, 200, { events: [], nextCursor: "42" });
        return;
      }
      response.writeHead(204);
      response.end();
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    const events = await getRunEvents(incomingRequest("/api/runs/run-1/events?after=40&limit=2"), {
      params: Promise.resolve({ id: "run-1" }),
    });
    expect(await events.json()).toEqual({ events: [], nextCursor: "42" });
    expect(stubUpstream.receivedRequests[0]?.url).toBe("/api/runs/run-1/events?after=40&limit=2");

    const empty = await proxyUpstream(incomingRequest("/api/runs/run-1"), "/api/runs/run-1/cancel");
    expect(empty.status).toBe(204);
    expect(await empty.text()).toBe("");
  });

  it("does not buffer an unbuffered response and does not end it at the buffered deadline", async () => {
    let finishResponse: (() => void) | undefined;
    stubUpstream = await startStubUpstream((_request, response) => {
      response.writeHead(200, { "content-type": "application/json" });
      response.write('{"events":[');
      finishResponse = () => response.end("]}");
    });
    vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);

    // The response object exists while the upstream body is still open: nothing waited for it.
    const response = await proxyUpstream(
      incomingRequest("/api/runs/run-1/events"),
      "/api/runs/run-1/events",
      {
        buffer: false,
        timeoutMs: 20,
      },
    );
    await new Promise((resolve) => setTimeout(resolve, 60));
    finishResponse?.();
    expect(response.status).toBe(200);
    expect(await response.text()).toBe('{"events":[]}');
  });
  // CSRF guard on commands (WEB security review, finding 4).
  describe("commands from a browser page", () => {
    async function postFrom(headers: Record<string, string>, body: string | null = "{}") {
      stubUpstream = await startStubUpstream((_request, response) => {
        sendJson(response, 201, { runId: "r", passportId: "p" });
      });
      vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);
      return postRun(
        new Request("http://web.test/api/runs", {
          method: "POST",
          headers: body === null ? headers : { "content-type": "application/json", ...headers },
          body,
        }),
      );
    }

    it.each([
      ["a cross-site page", { "sec-fetch-site": "cross-site" }],
      ["a same-site page of another origin", { "sec-fetch-site": "same-site" }],
      ["another origin", { origin: "https://evil.example" }],
      ["a null origin", { origin: "null" }],
      ["an origin on the same host name but another port", { origin: "http://web.test:3999" }],
    ])("refuses %s with 403 before any upstream call", async (_name, headers) => {
      const response = await postFrom({ ...headers, cookie: "session=abc" });
      expect(response.status).toBe(403);
      const body = (await response.json()) as { error: { code: string } };
      expect(body.error.code).toBe("cross_site_request_refused");
      expect(stubUpstream?.receivedRequests).toHaveLength(0);
    });

    it.each([
      ["same-origin fetch metadata", { "sec-fetch-site": "same-origin" }],
      ["a direct navigation", { "sec-fetch-site": "none" }],
      ["the page's own origin", { origin: "http://web.test" }],
      ["no browser headers (a script or the judge client)", {}],
    ])("allows %s", async (_name, headers) => {
      const response = await postFrom(headers);
      expect(response.status).toBe(201);
      expect(stubUpstream?.receivedRequests).toHaveLength(1);
    });

    it("accepts the Host the browser addressed when the request URL carries an internal origin", async () => {
      stubUpstream = await startStubUpstream((_request, response) => {
        sendJson(response, 201, { runId: "r", passportId: "p" });
      });
      vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);
      const response = await postRun(
        new Request("http://internal-web:3000/api/runs", {
          method: "POST",
          headers: {
            "content-type": "application/json",
            origin: "https://panel.example",
            host: "panel.example",
          },
          body: "{}",
        }),
      );
      expect(response.status).toBe(201);
    });

    it.each([
      ["a form post", "application/x-www-form-urlencoded"],
      ["text/plain", "text/plain"],
      ["multipart", "multipart/form-data; boundary=x"],
      ["no content type", ""],
    ])("refuses a body sent as %s with 415", async (_name, contentType) => {
      stubUpstream = await startStubUpstream((_request, response) => sendJson(response, 201, {}));
      vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);
      const headers = new Headers();
      if (contentType !== "") headers.set("content-type", contentType);
      const response = await postRun(
        new Request("http://web.test/api/runs", { method: "POST", headers, body: "a=b" }),
      );
      expect(response.status).toBe(415);
      expect(stubUpstream.receivedRequests).toHaveLength(0);
    });

    it("lets a command without a body through, and a JSON type with a charset", async () => {
      stubUpstream = await startStubUpstream((_request, response) =>
        sendJson(response, 200, { ok: 1 }),
      );
      vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);
      const cancel = await proxyUpstream(
        new Request("http://web.test/api/runs/r/cancel", { method: "POST" }),
        "/api/runs/r/cancel",
      );
      expect(cancel.status).toBe(200);
      const charset = await postRun(
        new Request("http://web.test/api/runs", {
          method: "POST",
          headers: { "content-type": "Application/JSON; charset=utf-8" },
          body: "{}",
        }),
      );
      expect(charset.status).toBe(200);
    });

    it("treats a Content-Length 0 request with an empty stream as bodyless", async () => {
      stubUpstream = await startStubUpstream((_request, response) =>
        sendJson(response, 200, { ok: 1 }),
      );
      vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);
      // What a browser's bodyless POST looks like inside a Next.js route handler.
      const response = await proxyUpstream(
        new Request("http://web.test/api/auth/sign-out", {
          method: "POST",
          headers: { "content-length": "0" },
          body: new ReadableStream({ start: (controller) => controller.close() }),
          duplex: "half",
        } as RequestInit & { duplex: "half" }),
        "/api/auth/sign-out",
      );
      expect(response.status).toBe(200);
    });

    it("does not apply to reads", async () => {
      stubUpstream = await startStubUpstream((_request, response) =>
        sendJson(response, 200, { ok: 1 }),
      );
      vi.stubEnv("API_UPSTREAM_URL", stubUpstream.baseUrl);
      const response = await getRun(
        incomingRequest("/api/runs/run-1", {
          "sec-fetch-site": "cross-site",
          origin: "https://evil.example",
        }),
        { params: Promise.resolve({ id: "run-1" }) },
      );
      expect(response.status).toBe(200);
    });
  });
});
