import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import passportFixture from "@workspace/contracts/fixtures/passport.atlas.json" with { type: "json" };
import stateFixture from "@workspace/contracts/fixtures/run-state.completed.json" with { type: "json" };
import usageFixture from "@workspace/contracts/fixtures/run-usage.ledger.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { RunsController } from "./runs.controller.js";

describe.each([
  ["state", "", stateFixture],
  ["passport", "/passport", passportFixture],
  ["usage", "/usage", usageFixture],
])("Run %s facade", (_name, suffix, fixture) => {
  let app: NestExpressApplication;
  let upstreamBody: unknown;
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
  const runId = fixture.runId;
  const getRead = vi.fn((_path: string, _id: string, schema: z.ZodTypeAny) => {
    if (failure) return failure;
    const parsed = schema.safeParse(upstreamBody);
    return parsed.success
      ? { success: true, data: parsed.data }
      : { success: false, reason: "invalid_response" };
  });
  beforeAll(async () => {
    app = await createTestApp(
      {
        controllers: [RunsController],
        providers: [
          { provide: GatewayClientService, useValue: { getRead } },
          {
            provide: AUTH_PROVIDER,
            useValue: { authenticate: () => Promise.resolve({ subjectId: context.userId }) },
          },
          {
            provide: getRepositoryToken(Membership),
            useValue: { findOne: () => Promise.resolve(context) },
          },
        ],
      },
      { mockAuth: false },
    );
  });
  afterAll(async () => {
    await app.close();
  });
  beforeEach(() => {
    upstreamBody = structuredClone(fixture);
    failure = undefined;
    getRead.mockClear();
  });

  it("passes the exact shared shape, verified context and request id", async () => {
    const response = await request(app.getHttpServer())
      .get(`/api/runs/${runId}${suffix}`)
      .set("Cookie", "session=test")
      .set("x-request-id", "run-read-test")
      .expect(200);
    expect(response.body).toEqual(fixture);
    expect(getRead).toHaveBeenCalledWith(
      `/internal/runs/${runId}${suffix}`,
      "run-read-test",
      expect.anything(),
      context,
    );
  });
  it("rejects unauthenticated reads before Go", async () => {
    await request(app.getHttpServer()).get(`/api/runs/${runId}${suffix}`).expect(401);
    expect(getRead).not.toHaveBeenCalled();
  });
  it("rejects malformed references before Go", async () => {
    await request(app.getHttpServer())
      .get(`/api/runs/invalid${suffix}`)
      .set("Cookie", "session=test")
      .expect(400);
    expect(getRead).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 503])(
    "preserves Go status %s including unknown or foreign objects",
    async (statusCode) => {
      failure = { success: false, reason: "bad_request", statusCode };
      await request(app.getHttpServer())
        .get(`/api/runs/${runId}${suffix}`)
        .set("Cookie", "session=test")
        .expect(statusCode);
    },
  );
  it.each(["timeout", "unreachable", "invalid_response"])(
    "returns 503 on %s without inventing a state",
    async (reason) => {
      failure = { success: false, reason };
      await request(app.getHttpServer())
        .get(`/api/runs/${runId}${suffix}`)
        .set("Cookie", "session=test")
        .expect(503);
    },
  );
  it.each(["unknown field", "different run"])(
    "refuses an unsafe response: %s",
    async (scenario) => {
      upstreamBody =
        scenario === "unknown field"
          ? { ...fixture, rawNote: "secret" }
          : { ...fixture, runId: context.userId };
      const response = await request(app.getHttpServer())
        .get(`/api/runs/${runId}${suffix}`)
        .set("Cookie", "session=test")
        .expect(503);
      expect(JSON.stringify(response.body)).not.toContain("secret");
    },
  );
});
