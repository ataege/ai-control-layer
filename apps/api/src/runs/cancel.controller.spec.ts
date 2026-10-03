import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/run-state.running.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { RunsController } from "./runs.controller.js";

describe("Cancellation facade", () => {
  let app: NestExpressApplication;
  let responseBody: unknown;
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
  const state = { ...fixture, cancelRequestedAt: "2026-10-04T00:00:00Z" };
  const postCommand = vi.fn((_path: string, _id: string, _body: unknown, schema: z.ZodTypeAny) => {
    if (failure) return failure;
    const parsed = schema.safeParse(responseBody);
    return parsed.success
      ? { success: true, data: parsed.data }
      : { success: false, reason: "invalid_response" };
  });
  beforeAll(async () => {
    app = await createTestApp(
      {
        controllers: [RunsController],
        providers: [
          { provide: GatewayClientService, useValue: { postCommand } },
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
    responseBody = state;
    failure = undefined;
    postCommand.mockClear();
  });
  it("forwards cancellation with verified context and returns Go's state unchanged", async () => {
    const result = await request(app.getHttpServer())
      .post(`/api/runs/${state.runId}/cancel`)
      .set("Cookie", "session=test")
      .set("x-request-id", "cancel-test")
      .send({})
      .expect(200);
    expect(result.body).toEqual(state);
    expect(postCommand).toHaveBeenCalledWith(
      `/internal/runs/${state.runId}/cancel`,
      "cancel-test",
      {},
      expect.anything(),
      context,
    );
  });
  it("refuses missing authentication before dispatch", async () => {
    await request(app.getHttpServer()).post(`/api/runs/${state.runId}/cancel`).send({}).expect(401);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it("rejects supplied identity and malformed ids before dispatch", async () => {
    await request(app.getHttpServer())
      .post(`/api/runs/${state.runId}/cancel`)
      .set("Cookie", "session=test")
      .send({ organizationId: "forged" })
      .expect(400);
    await request(app.getHttpServer())
      .post("/api/runs/invalid/cancel")
      .set("Cookie", "session=test")
      .send({})
      .expect(400);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 409, 503])("preserves Go status %s", async (statusCode) => {
    failure = { success: false, reason: "bad_request", statusCode };
    await request(app.getHttpServer())
      .post(`/api/runs/${state.runId}/cancel`)
      .set("Cookie", "session=test")
      .send({})
      .expect(statusCode);
  });
  it("reports a timeout as an unconfirmed command with 504", async () => {
    failure = { success: false, reason: "timeout" };
    const result = await request(app.getHttpServer())
      .post(`/api/runs/${state.runId}/cancel`)
      .set("Cookie", "session=test")
      .send({})
      .expect(504);
    expect(result.body).toMatchObject({ error: { code: "outcome_unconfirmed" } });
  });
  it.each(["unreachable", "invalid_response"])(
    "refuses %s without claiming cancellation",
    async (reason) => {
      failure = { success: false, reason };
      await request(app.getHttpServer())
        .post(`/api/runs/${state.runId}/cancel`)
        .set("Cookie", "session=test")
        .send({})
        .expect(503);
    },
  );
  it("rejects a different run in Go's response", async () => {
    responseBody = { ...state, runId: context.userId };
    await request(app.getHttpServer())
      .post(`/api/runs/${state.runId}/cancel`)
      .set("Cookie", "session=test")
      .send({})
      .expect(503);
  });
});
