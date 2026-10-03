import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/approval-response.approve.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { ActionsController } from "./actions.controller.js";

describe("Approval facade", () => {
  let app: NestExpressApplication;
  let responseBody: unknown;
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
  const state = fixture;
  let roles: string[];
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
        controllers: [ActionsController],
        providers: [
          { provide: GatewayClientService, useValue: { postCommand } },
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
    responseBody = state;
    failure = undefined;
    postCommand.mockClear();
  });
  it("forwards the exact decision with verified context and returns Go references unchanged", async () => {
    const result = await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .set("x-request-id", "cancel-test")
      .send({ decision: "approve" })
      .expect(200);
    expect(result.body).toEqual(state);
    expect(postCommand).toHaveBeenCalledWith(
      `/internal/actions/${state.actionId}/approval`,
      "cancel-test",
      { decision: "approve" },
      expect.anything(),
      { ...context, roles },
    );
  });
  it("refuses missing authentication before dispatch", async () => {
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .send({ decision: "approve" })
      .expect(401);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it("rejects supplied identity and malformed ids before dispatch", async () => {
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ organizationId: "forged" })
      .expect(400);
    await request(app.getHttpServer())
      .post("/api/actions/invalid/approval")
      .set("Cookie", "session=test")
      .send({ decision: "approve" })
      .expect(400);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 409, 503])("preserves Go status %s", async (statusCode) => {
    failure = { success: false, reason: "bad_request", statusCode };
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ decision: "approve" })
      .expect(statusCode);
  });
  it("reports a timeout as an unconfirmed command with 504", async () => {
    failure = { success: false, reason: "timeout" };
    const result = await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ decision: "approve" })
      .expect(504);
    expect(result.body).toMatchObject({ error: { code: "outcome_unconfirmed" } });
  });
  it.each(["unreachable", "invalid_response"])(
    "refuses %s without claiming an approval",
    async (reason) => {
      failure = { success: false, reason };
      await request(app.getHttpServer())
        .post(`/api/actions/${state.actionId}/approval`)
        .set("Cookie", "session=test")
        .send({ decision: "approve" })
        .expect(503);
    },
  );
  it("rejects a different action in Go's response", async () => {
    responseBody = { ...state, actionId: context.userId };
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ decision: "approve" })
      .expect(503);
  });
  it("refuses a non-reviewer before dispatch", async () => {
    roles = ["operator"];
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ decision: "approve" })
      .expect(403);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it.each([
    {},
    { decision: "allow" },
    { decision: "approve", payload: {} },
    { decision: "reject", reviewerId: "forged" },
  ])("refuses any body outside X-10: %j", async (body) => {
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send(body)
      .expect(400);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it("forwards reject unchanged rather than issuing a grant", async () => {
    responseBody = { ...state, decision: "reject" };
    const result = await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ decision: "reject" })
      .expect(200);
    expect(result.body).toEqual(responseBody);
    expect(postCommand.mock.calls[0]?.[2]).toEqual({ decision: "reject" });
  });
  it("refuses an approval response that changes the decision", async () => {
    responseBody = { ...state, decision: "reject" };
    await request(app.getHttpServer())
      .post(`/api/actions/${state.actionId}/approval`)
      .set("Cookie", "session=test")
      .send({ decision: "approve" })
      .expect(503);
  });
});
