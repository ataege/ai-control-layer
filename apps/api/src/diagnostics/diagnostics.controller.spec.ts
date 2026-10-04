import type { NestExpressApplication } from "@nestjs/platform-express";
import type { DiagnosticCheck } from "@workspace/contracts";
import request from "supertest";
import { afterEach, describe, expect, it } from "vitest";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { createTestApp } from "../testing/create-test-app.js";
import { DiagnosticsController } from "./diagnostics.controller.js";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import type { AppConfigService } from "../config/app-config.service.js";

const checkUp: DiagnosticCheck = { status: "up", latencyMs: 3, upstreamStatus: 200 };

describe("DiagnosticsController", () => {
  let app: NestExpressApplication | undefined;

  async function requestDiagnostics(reachability: DiagnosticCheck, readiness: DiagnosticCheck) {
    const gatewayClientStub: Pick<GatewayClientService, "ping" | "checkReadiness"> = {
      ping: () => Promise.resolve(reachability),
      checkReadiness: () => Promise.resolve(readiness),
    };
    app = await createTestApp({
      controllers: [DiagnosticsController],
      providers: [{ provide: GatewayClientService, useValue: gatewayClientStub }],
    });
    return request(app.getHttpServer())
      .get("/api/diagnostics/gateway")
      .set("x-request-id", "req-diagnostics-1");
  }

  afterEach(async () => {
    await app?.close();
    app = undefined;
  });

  it("returns 200 ok with the request id when both checks pass", async () => {
    const response = await requestDiagnostics(checkUp, checkUp);

    expect(response.status).toBe(200);
    expect(response.body).toEqual({
      status: "ok",
      requestId: "req-diagnostics-1",
      checks: { reachability: checkUp, databaseReadiness: checkUp },
    });
  });

  it("returns 503 degraded when the gateway is reachable but not ready", async () => {
    const notReady: DiagnosticCheck = {
      status: "down",
      latencyMs: 5,
      upstreamStatus: 503,
      reason: "not_ready",
    };

    const response = await requestDiagnostics(checkUp, notReady);

    expect(response.status).toBe(503);
    expect(response.body).toMatchObject({
      status: "degraded",
      checks: { databaseReadiness: notReady },
    });
  });

  it("preserves worker/catalog unavailability even when the upstream database check is up", async () => {
    // HTTP fixture of the decision 11 readiness response; no live worker is simulated.
    const upstream = createServer((req, res) => {
      res.setHeader("content-type", "application/json");
      if (req.url === "/internal/ping") {
        res.end(JSON.stringify({ status: "ok", service: "gateway" }));
      } else {
        res.statusCode = 503;
        res.end(
          JSON.stringify({
            status: "unavailable",
            service: "gateway",
            checks: { database: { status: "up", latencyMs: 1 } },
          }),
        );
      }
    });
    await new Promise<void>((resolve) => upstream.listen(0, "127.0.0.1", resolve));
    try {
      const { port } = upstream.address() as AddressInfo;
      const gateway = new GatewayClientService({
        gatewayUrl: `http://127.0.0.1:${port}`,
        gatewayServiceToken: "test-service-token",
        operatorContextSigningKey: "test-signing-key",
        gatewayTimeoutMs: 3000,
      } as AppConfigService);
      app = await createTestApp({
        controllers: [DiagnosticsController],
        providers: [{ provide: GatewayClientService, useValue: gateway }],
      });
      const response = await request(app.getHttpServer())
        .get("/api/diagnostics/gateway")
        .expect(503);
      expect(response.body).toMatchObject({
        status: "degraded",
        checks: {
          reachability: { status: "up", upstreamStatus: 200 },
          databaseReadiness: { status: "down", upstreamStatus: 503, reason: "not_ready" },
        },
      });
    } finally {
      upstream.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        upstream.close((error) => (error ? reject(error) : resolve())),
      );
    }
  });

  it("returns 504 unavailable when the ping timed out", async () => {
    const timedOut: DiagnosticCheck = { status: "down", latencyMs: 3000, reason: "timeout" };

    const response = await requestDiagnostics(timedOut, timedOut);

    expect(response.status).toBe(504);
    expect(response.body).toMatchObject({ status: "unavailable", requestId: "req-diagnostics-1" });
  });

  it.each([
    { reason: "unreachable" as const, upstreamStatus: undefined },
    { reason: "unauthorized" as const, upstreamStatus: 401 },
    { reason: "unexpected_response" as const, upstreamStatus: 500 },
  ])("returns 502 unavailable when the ping is $reason", async ({ reason, upstreamStatus }) => {
    const pingDown: DiagnosticCheck = {
      status: "down",
      latencyMs: 2,
      reason,
      ...(upstreamStatus === undefined ? {} : { upstreamStatus }),
    };

    // Readiness being up must not hide a failed authenticated ping.
    const response = await requestDiagnostics(pingDown, checkUp);

    expect(response.status).toBe(502);
    expect(response.body).toMatchObject({
      status: "unavailable",
      checks: { reachability: pingDown },
    });
  });
});
