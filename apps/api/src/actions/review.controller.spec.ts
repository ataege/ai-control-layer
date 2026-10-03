import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/review-view.queue-report.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { ActionsController } from "./actions.controller.js";

describe("Frozen action review facade", () => {
  let app: NestExpressApplication;
  let upstreamBody: unknown;
  let roles: string[];
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
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
        controllers: [ActionsController],
        providers: [
          { provide: GatewayClientService, useValue: { getRead } },
          {
            provide: AUTH_PROVIDER,
            useValue: { authenticate: () => Promise.resolve({ subjectId: context.userId }) },
          },
          {
            provide: getRepositoryToken(Membership),
            useValue: { findOne: () => Promise.resolve({ ...context, roles }) },
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
    roles = ["operator", "reviewer"];
    upstreamBody = structuredClone(fixture);
    failure = undefined;
    getRead.mockClear();
  });

  it("passes the exact shared shape, verified context and request id", async () => {
    const response = await request(app.getHttpServer())
      .get(`/api/actions/${fixture.action_id}/review`)
      .set("Cookie", "session=test")
      .set("x-request-id", "run-read-test")
      .expect(200);
    expect(response.body).toEqual(fixture);
    expect(getRead).toHaveBeenCalledWith(
      `/internal/actions/${fixture.action_id}/review`,
      "run-read-test",
      expect.anything(),
      { ...context, roles },
    );
  });
  it("rejects unauthenticated reads before Go", async () => {
    await request(app.getHttpServer()).get(`/api/actions/${fixture.action_id}/review`).expect(401);
    expect(getRead).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 503])(
    "preserves Go status %s including unknown or foreign objects",
    async (statusCode) => {
      failure = { success: false, reason: "bad_request", statusCode };
      await request(app.getHttpServer())
        .get(`/api/actions/${fixture.action_id}/review`)
        .set("Cookie", "session=test")
        .expect(statusCode);
    },
  );
  it.each(["timeout", "unreachable", "invalid_response"])(
    "returns 503 on %s without inventing a state",
    async (reason) => {
      failure = { success: false, reason };
      await request(app.getHttpServer())
        .get(`/api/actions/${fixture.action_id}/review`)
        .set("Cookie", "session=test")
        .expect(503);
    },
  );
  it.each(["unknown field", "different action"])(
    "refuses an unsafe response: %s",
    async (scenario) => {
      upstreamBody =
        scenario === "unknown field"
          ? { ...fixture, rawNote: "secret" }
          : { ...fixture, action_id: context.userId };
      const response = await request(app.getHttpServer())
        .get(`/api/actions/${fixture.action_id}/review`)
        .set("Cookie", "session=test")
        .expect(503);
      expect(JSON.stringify(response.body)).not.toContain("secret");
    },
  );
  it("refuses non-reviewers before Go and ignores browser role claims", async () => {
    roles = ["operator"];
    await request(app.getHttpServer())
      .get(`/api/actions/${fixture.action_id}/review?roles=reviewer`)
      .set("Cookie", "session=test")
      .expect(403);
    expect(getRead).not.toHaveBeenCalled();
  });
  it.each([
    "missing recipient",
    "missing report",
    "unknown canonical argument",
    "unknown source field",
  ])("refuses invalid frozen material: %s", async (scenario) => {
    switch (scenario) {
      case "missing recipient":
        upstreamBody = { ...fixture, recipient: null };
        break;
      case "missing report":
        upstreamBody = { ...fixture, report: null };
        break;
      case "unknown canonical argument":
        upstreamBody = {
          ...fixture,
          canonical_arguments: { ...fixture.canonical_arguments, actorId: "forged" },
        };
        break;
      case "unknown source field":
        upstreamBody = {
          ...fixture,
          report: {
            ...fixture.report,
            sources: fixture.report.sources.map((source) => ({
              ...source,
              rawNote: "protected-text",
            })),
          },
        };
        break;
    }
    const response = await request(app.getHttpServer())
      .get(`/api/actions/${fixture.action_id}/review`)
      .set("Cookie", "session=test")
      .expect(503);
    expect(JSON.stringify(response.body)).not.toContain("protected-text");
  });
});
