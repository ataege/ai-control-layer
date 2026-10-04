import { jwtVerify } from "jose";
import { createServer, type IncomingHttpHeaders, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { Logger } from "@nestjs/common";
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import type { AppConfigService } from "../config/app-config.service.js";
import { GatewayClientService } from "./gateway-client.service.js";

const SERVICE_TOKEN = "test-service-token-0123456789abcdef";
// Wide enough that a loaded machine does not turn a normal reply into a timeout; the stub that
// must time out answers after 2 s, well past it.
const GATEWAY_TIMEOUT_MS = 1_000;
const COMMAND_TIMEOUT_MS = 1_000;

type StubBehaviour =
  | "not-found"
  | "healthy"
  | "unauthorized"
  | "server-error"
  | "bad-request"
  | "not-ready"
  | "worker-not-ready"
  | "slow"
  | "garbage"
  | "redirect";

function createClient(gatewayUrl: string): GatewayClientService {
  const config: Pick<
    AppConfigService,
    | "gatewayUrl"
    | "gatewayServiceToken"
    | "operatorContextSigningKey"
    | "gatewayTimeoutMs"
    | "commandTimeoutMs"
  > = {
    gatewayUrl,
    gatewayServiceToken: SERVICE_TOKEN,
    operatorContextSigningKey: "test-signing-key-0123456789abcdef",
    gatewayTimeoutMs: GATEWAY_TIMEOUT_MS,
    commandTimeoutMs: COMMAND_TIMEOUT_MS,
  };
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
        case "not-found":
          sendJson(404, { error: { code: "not_found" } });
          break;
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
        case "worker-not-ready":
          // GO-09/GO-72: the database is up, but the worker loop or the active catalog is not.
          sendJson(503, {
            status: "unavailable",
            service: "gateway",
            checks: { database: { status: "up" } },
          });
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
    expect(receivedHeaders.authorization).toBe("Bearer " + SERVICE_TOKEN);
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
    expect(check.latencyMs).toBeLessThan(1_900);
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

  it("maps a not-ready worker or catalog with the database up to not_ready (API-15)", async () => {
    stubBehaviour = "worker-not-ready";

    await expect(client.checkReadiness("req-ready-3")).resolves.toMatchObject({
      status: "down",
      upstreamStatus: 503,
      reason: "not_ready",
    });
  });

  describe("postCommand", () => {
    const testSchema = z.object({ result: z.string() });
    // An explicit test operator context: the client itself never invents one.
    const testOperatorContext = {
      userId: "c28e2545-2de6-41b9-9be6-d21ae01e4901",
      organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
      roles: ["operator"],
    };

    it("posts a command and parses a successful response", async () => {
      stubBehaviour = "healthy";
      const payload = { input: "test" };
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-1",
        payload,
        z.object({ status: z.string(), service: z.string() }),
        testOperatorContext,
      );

      expect(outcome).toEqual({ success: true, data: { status: "ok", service: "gateway" } });
      expect(receivedPath).toBe("/internal/runs");
      expect(receivedHeaders.authorization).toBe("Bearer " + SERVICE_TOKEN);
      expect(receivedHeaders["x-operator-context"]).toMatch(/^eyJ/);
      const token = receivedHeaders["x-operator-context"] as string;
      const secret = new TextEncoder().encode("test-signing-key-0123456789abcdef");
      const { payload: jwtPayload } = await jwtVerify(token, secret, { issuer: "gateway-client" });
      expect(jwtPayload).toBeDefined();
      expect(receivedHeaders["x-request-id"]).toBe("req-cmd-1");
    });

    it("handles an unauthorized response and does not expose the token", async () => {
      stubBehaviour = "unauthorized";
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-2",
        {},
        testSchema,
        testOperatorContext,
      );
      expect(outcome).toEqual({ success: false, reason: "unauthorized", statusCode: 401 });
      const exposedText = JSON.stringify([outcome, loggedWarnings.mock.calls]);
      expect(exposedText).not.toContain(SERVICE_TOKEN);
      expect(exposedText).not.toContain("upstream-secret-detail");
    });

    it("handles a bad request response with a code", async () => {
      stubBehaviour = "bad-request";
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-3",
        {},
        testSchema,
        testOperatorContext,
      );
      // The 4xx outcome keeps the gateway's message (API-11); the caller decides whether it may
      // be shown: only the start-run route does, for an X-13 admission reason code.
      expect(outcome).toEqual({
        success: false,
        reason: "bad_request",
        code: "invalid_input",
        statusCode: 400,
        message: "upstream-secret-detail",
      });
    });

    it("handles a server error response with a code", async () => {
      stubBehaviour = "server-error";
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-4",
        {},
        testSchema,
        testOperatorContext,
      );
      expect(outcome).toEqual({
        success: false,
        reason: "server_error",
        code: "internal_error",
        statusCode: 500,
      });
    });

    it("handles a body that is not JSON", async () => {
      stubBehaviour = "garbage";
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-5",
        {},
        testSchema,
        testOperatorContext,
      );
      expect(outcome).toEqual({ success: false, reason: "invalid_response" });
    });

    it("does not follow redirects", async () => {
      stubBehaviour = "redirect";
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-6",
        {},
        testSchema,
        testOperatorContext,
      );
      // Fetch with redirect: "manual" returns 302 directly. It matches unexpected_status if we don't handle it
      expect(outcome).toEqual({ success: false, reason: "unexpected_status", statusCode: 302 });
    });

    it("handles a timeout", async () => {
      stubBehaviour = "slow";
      const outcome = await client.postCommand(
        "/internal/runs",
        "req-cmd-7",
        {},
        testSchema,
        testOperatorContext,
      );
      expect(outcome).toEqual({ success: false, reason: "timeout" });
    });
  });
  describe("getRead", () => {
    const context = {
      userId: "c28e2545-2de6-41b9-9be6-d21ae01e4901",
      organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
      roles: ["operator"],
    };
    const schema = z.strictObject({ status: z.string(), service: z.string() });

    it("signs only the supplied verified context and preserves query parameters", async () => {
      stubBehaviour = "healthy";
      await expect(
        client.getRead("/internal/runs/run/events?after=3&limit=10", "read-1", schema, context),
      ).resolves.toEqual({ success: true, data: { status: "ok", service: "gateway" } });
      expect(receivedPath).toBe("/internal/runs/run/events?after=3&limit=10");
      expect(receivedHeaders.authorization).toBe("Bearer " + SERVICE_TOKEN);
      expect(receivedHeaders["x-request-id"]).toBe("read-1");
      const { payload } = await jwtVerify(
        receivedHeaders["x-operator-context"] as string,
        new TextEncoder().encode("test-signing-key-0123456789abcdef"),
        { issuer: "gateway-client", audience: "gateway", algorithms: ["HS256"] },
      );
      expect(payload.ctx).toEqual(context);
    });

    it.each([
      ["not-found", "bad_request", 404],
      ["unauthorized", "unauthorized", 401],
      ["server-error", "server_error", 500],
      ["not-ready", "server_error", 503],
      ["redirect", "unexpected_status", 302],
    ] as const)(
      "fails closed on %s and retains HTTP status",
      async (behaviour, reason, statusCode) => {
        stubBehaviour = behaviour;
        const result = await client.getRead("/internal/runs/run", "read-2", schema, context);
        expect(result).toMatchObject({ success: false, reason, statusCode });
        expect(JSON.stringify([result, loggedWarnings.mock.calls])).not.toContain(SERVICE_TOKEN);
        expect(JSON.stringify(result)).not.toContain("upstream-secret-detail");
      },
    );

    it("fails on timeout and refused connections", async () => {
      stubBehaviour = "slow";
      await expect(
        client.getRead("/internal/runs/run", "read-3", schema, context),
      ).resolves.toEqual({ success: false, reason: "timeout" });
      await expect(
        createClient("http://127.0.0.1:1").getRead("/internal/runs/run", "read-4", schema, context),
      ).resolves.toEqual({ success: false, reason: "unreachable" });
    });

    it("refuses missing or malformed context before dispatching either method", async () => {
      for (const invalid of [
        undefined,
        { ...context, organizationId: "" },
        { ...context, roles: ["operator", "operator"] },
      ]) {
        await expect(
          client.getRead("/internal/runs/run", "read-5", schema, invalid as typeof context),
        ).resolves.toEqual({ success: false, reason: "unauthorized" });
        await expect(
          client.postCommand("/internal/runs", "read-5", {}, schema, invalid as typeof context),
        ).resolves.toEqual({ success: false, reason: "unauthorized" });
      }
      expect(receivedPath).toBeUndefined();
    });

    it("rejects non-JSON and unknown response fields", async () => {
      stubBehaviour = "garbage";
      await expect(
        client.getRead("/internal/runs/run", "read-6", schema, context),
      ).resolves.toEqual({ success: false, reason: "invalid_response" });
      stubBehaviour = "healthy";
      await expect(
        client.getRead(
          "/internal/runs/run",
          "read-7",
          z.strictObject({ status: z.string() }),
          context,
        ),
      ).resolves.toEqual({ success: false, reason: "invalid_response" });
    });
  });
});
