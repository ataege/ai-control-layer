import { createServer, type IncomingHttpHeaders, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { Logger } from "@nestjs/common";
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import type { AppConfigService } from "../config/app-config.service.js";
import { GatewayClientService } from "./gateway-client.service.js";

const SERVICE_TOKEN = "test-service-token-0123456789abcdef";
const GATEWAY_TIMEOUT_MS = 150;
const COMMAND_TIMEOUT_MS = 150;

type StubBehaviour = "healthy" | "unauthorized" | "server-error" | "bad-request" | "not-ready" | "slow" | "garbage" | "redirect";

function createClient(gatewayUrl: string): GatewayClientService {
  const config: Pick<AppConfigService, "gatewayUrl" | "gatewayServiceToken" | "gatewayTimeoutMs" | "commandTimeoutMs"> =
    { gatewayUrl, gatewayServiceToken: SERVICE_TOKEN, gatewayTimeoutMs: GATEWAY_TIMEOUT_MS, commandTimeoutMs: COMMAND_TIMEOUT_MS };
  return new GatewayClientService(config as AppConfigService);
}

describe("GatewayClientService", () => {
  let stubServer: Server;
  let client: GatewayClientService;
  let stubBehaviour: StubBehaviour;
  let receivedHeaders: IncomingHttpHeaders;
  let receivedPath: string | undefined;
  const loggedWarnings = vi.spyOn(Logger.prototype, "warn").mockImplementation(() => undefined);

  beforeAll(async () => {
    // Stand-in for the Go gateway.
    stubServer = createServer((incomingRequest, serverResponse) => {
      receivedHeaders = incomingRequest.headers;
      receivedPath = incomingRequest.url;
      const sendJson = (statusCode: number, body: unknown): void => {
        serverResponse.writeHead(statusCode, { "content-type": "application/json" });
        serverResponse.end(JSON.stringify(body));
      };
      switch (stubBehaviour) {
        case "healthy":
          sendJson(200, { status: "ok", service: "gateway" });
          break;
        case "unauthorized":
          sendJson(401, { error: { code: "unauthorized", message: "upstream-secret-detail" } });
          break;
        case "server-error":
          sendJson(500, { error: { code: "internal_error", message: "upstream-secret-detail" } });
          break;
        case "bad-request":
          sendJson(400, { error: { code: "invalid_input", message: "upstream-secret-detail" } });
          break;
        case "not-ready":
          sendJson(503, { status: "unavailable", service: "gateway" });
          break;
        case "garbage":
          serverResponse.writeHead(200, { "content-type": "text/plain" }).end("not json");
          break;
        case "redirect":
          serverResponse.writeHead(302, { location: "http://example.com" }).end();
          break;
        case "slow":
          // Never answers within the client deadline.
          setTimeout(() => sendJson(200, { status: "ok", service: "gateway" }), 2_000).unref();
          break;
      }
    });
    await new Promise<void>((resolve) => stubServer.listen(0, "127.0.0.1", resolve));
    const { port } = stubServer.address() as AddressInfo;
    client = createClient(`http://127.0.0.1:${port}`);
  });

  afterAll(async () => {
    stubServer.closeAllConnections();
    await new Promise((resolve) => stubServer.close(resolve));
    loggedWarnings.mockRestore();
  });

  beforeEach(() => {
    receivedHeaders = {};
    receivedPath = undefined;
    loggedWarnings.mockClear();
  });

  it("pings with the bearer token and the propagated request id", async () => {
    stubBehaviour = "healthy";

    const check = await client.ping("req-ping-1");

    expect(check).toMatchObject({ status: "up", upstreamStatus: 200 });
    expect(check.reason).toBeUndefined();
    expect(check.latencyMs).toBeGreaterThanOrEqual(0);
    expect(receivedPath).toBe("/internal/ping");
    expect(receivedHeaders.authorization).toBe(`Bearer ${SERVICE_TOKEN}`);
    expect(receivedHeaders["x-request-id"]).toBe("req-ping-1");
  });

  it("probes readiness without sending the token", async () => {
    stubBehaviour = "healthy";

    const check = await client.checkReadiness("req-ready-1");

    expect(check).toMatchObject({ status: "up", upstreamStatus: 200 });
    expect(receivedPath).toBe("/health/ready");
    expect(receivedHeaders.authorization).toBeUndefined();
    expect(receivedHeaders["x-request-id"]).toBe("req-ready-1");
  });

  it("maps 401 to unauthorized and never exposes the token or the upstream body", async () => {
    stubBehaviour = "unauthorized";

    const check = await client.ping("req-ping-2");

    expect(check).toMatchObject({ status: "down", upstreamStatus: 401, reason: "unauthorized" });
    const exposedText = JSON.stringify([check, loggedWarnings.mock.calls]);
    expect(exposedText).not.toContain(SERVICE_TOKEN);
    expect(exposedText).not.toContain("upstream-secret-detail");
  });

  it("maps a 5xx answer to unexpected_response", async () => {
    stubBehaviour = "server-error";

    await expect(client.ping("req-ping-3")).resolves.toMatchObject({
      status: "down",
      upstreamStatus: 500,
      reason: "unexpected_response",
    });
  });

  it("maps a 200 answer with an unexpected body to unexpected_response", async () => {
    stubBehaviour = "garbage";

    await expect(client.ping("req-ping-4")).resolves.toMatchObject({
      status: "down",
      upstreamStatus: 200,
      reason: "unexpected_response",
    });
  });

  it("maps a deadline overrun to timeout", async () => {
    stubBehaviour = "slow";

    const check = await client.ping("req-ping-5");

    expect(check).toMatchObject({ status: "down", reason: "timeout" });
    expect(check.upstreamStatus).toBeUndefined();
    expect(check.latencyMs).toBeGreaterThanOrEqual(GATEWAY_TIMEOUT_MS - 20);
    expect(check.latencyMs).toBeLessThan(1_500);
  });

  it("maps a refused connection to unreachable", async () => {
    // Port 1 on loopback has no listener.
    const unreachableClient = createClient("http://127.0.0.1:1");

    const check = await unreachableClient.ping("req-ping-6");

    expect(check).toMatchObject({ status: "down", reason: "unreachable" });
    expect(check.upstreamStatus).toBeUndefined();
  });

  it("maps a 503 readiness answer to not_ready", async () => {
    stubBehaviour = "not-ready";

    await expect(client.checkReadiness("req-ready-2")).resolves.toMatchObject({
      status: "down",
      upstreamStatus: 503,
      reason: "not_ready",
    });
  });

  describe("postCommand", () => {
    const { z } = require("zod");
    const testSchema = z.object({ result: z.string() });

    it("posts a command and parses a successful response", async () => {
      stubBehaviour = "healthy";
      const payload = { input: "test" };
      const outcome = await client.postCommand("/internal/runs", "req-cmd-1", payload, z.object({ status: z.string(), service: z.string() }));
      
      expect(outcome).toEqual({ success: true, data: { status: "ok", service: "gateway" } });
      expect(receivedPath).toBe("/internal/runs");
      expect(receivedHeaders.authorization).toBe(`Bearer ${SERVICE_TOKEN}`);
      expect(receivedHeaders["x-request-id"]).toBe("req-cmd-1");
    });

    it("handles an unauthorized response and does not expose the token", async () => {
      stubBehaviour = "unauthorized";
      const outcome = await client.postCommand("/internal/runs", "req-cmd-2", {}, testSchema);
      expect(outcome).toEqual({ success: false, reason: "unauthorized" });
      const exposedText = JSON.stringify([outcome, loggedWarnings.mock.calls]);
      expect(exposedText).not.toContain(SERVICE_TOKEN);
      expect(exposedText).not.toContain("upstream-secret-detail");
    });

    it("handles a bad request response with a code", async () => {
      stubBehaviour = "bad-request";
      const outcome = await client.postCommand("/internal/runs", "req-cmd-3", {}, testSchema);
      expect(outcome).toEqual({ success: false, reason: "bad_request", code: "invalid_input" });
    });

    it("handles a server error response with a code", async () => {
      stubBehaviour = "server-error";
      const outcome = await client.postCommand("/internal/runs", "req-cmd-4", {}, testSchema);
      expect(outcome).toEqual({ success: false, reason: "server_error", code: "internal_error" });
    });

    it("handles a body that is not JSON", async () => {
      stubBehaviour = "garbage";
      const outcome = await client.postCommand("/internal/runs", "req-cmd-5", {}, testSchema);
      expect(outcome).toEqual({ success: false, reason: "invalid_response" });
    });

    it("does not follow redirects", async () => {
      stubBehaviour = "redirect";
      const outcome = await client.postCommand("/internal/runs", "req-cmd-6", {}, testSchema);
      // Fetch with redirect: "manual" returns 302 directly. It matches unexpected_status if we don't handle it
      expect(outcome).toEqual({ success: false, reason: "unexpected_status" });
    });

    it("handles a timeout", async () => {
      stubBehaviour = "slow";
      const outcome = await client.postCommand("/internal/runs", "req-cmd-7", {}, testSchema);
      expect(outcome).toEqual({ success: false, reason: "timeout" });
    });
  });
});
