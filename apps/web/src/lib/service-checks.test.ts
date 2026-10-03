import { describe, expect, it } from "vitest";

import type { FetchJsonResult } from "@/lib/fetch-json";
import {
  describeApiLiveness,
  describeApiReadiness,
  describeGatewayDatabase,
  describeGatewayReachability,
} from "@/lib/service-checks";

function successResult(data: unknown): FetchJsonResult<unknown> {
  return { ok: true, status: 200, data, durationMs: 12, requestId: "request-1" };
}

function httpErrorResult(status: number, body: unknown): FetchJsonResult<unknown> {
  return { ok: false, error: { kind: "http", status, body }, durationMs: 12 };
}

describe("service check interpretation", () => {
  it("never reports healthy for a 200 response with an unexpected body", () => {
    const unexpectedResult = successResult({ hello: "world" });

    expect(describeApiLiveness(unexpectedResult).state).toBe("error");
    expect(describeApiReadiness(unexpectedResult).state).toBe("error");
    expect(describeGatewayReachability(unexpectedResult).state).toBe("error");
    expect(describeGatewayDatabase(unexpectedResult).state).toBe("error");
  });

  it("reads the readiness report from a 503 body", () => {
    const outcome = describeApiReadiness(
      httpErrorResult(503, {
        status: "error",
        info: {},
        error: { database: { status: "down", message: "database unreachable" } },
        details: { database: { status: "down", message: "database unreachable" } },
      }),
    );

    expect(outcome).toMatchObject({ state: "unavailable", httpStatus: 503 });
    expect(outcome.facts).toEqual([
      { label: "Dependency: database", value: "down (database unreachable)" },
    ]);
  });

  it("splits a degraded gateway report into reachable and database not ready", () => {
    const degradedResult = httpErrorResult(503, {
      status: "degraded",
      requestId: "request-2",
      checks: {
        reachability: { status: "up", latencyMs: 4, upstreamStatus: 200 },
        databaseReadiness: {
          status: "down",
          latencyMs: 9,
          upstreamStatus: 503,
          reason: "not_ready",
        },
      },
    });

    expect(describeGatewayReachability(degradedResult)).toMatchObject({
      state: "healthy",
      latencyMs: 4,
      requestId: "request-2",
    });
    expect(describeGatewayDatabase(degradedResult)).toMatchObject({
      state: "degraded",
      latencyMs: 9,
      httpStatus: 503,
    });
  });

  it("maps proxy failures to unavailable and configuration problems to error", () => {
    const proxyError = (statusCode: number, code: string) =>
      httpErrorResult(statusCode, {
        error: { code, message: "message" },
        statusCode,
        requestId: "request-3",
        timestamp: "2026-01-01T00:00:00.000Z",
      });

    expect(describeApiLiveness(proxyError(502, "upstream_unreachable"))).toMatchObject({
      state: "unavailable",
      requestId: "request-3",
    });
    expect(describeApiLiveness(proxyError(504, "upstream_timeout")).state).toBe("unavailable");
    expect(describeApiLiveness(proxyError(500, "configuration_error")).state).toBe("error");
  });

  it("reports browser-side failures as errors", () => {
    const networkFailure: FetchJsonResult<unknown> = {
      ok: false,
      error: { kind: "network", message: "fetch failed" },
      durationMs: 3,
    };

    expect(describeApiLiveness(networkFailure)).toMatchObject({ state: "error" });
    expect(describeApiLiveness(networkFailure).httpStatus).toBeUndefined();
  });
});
