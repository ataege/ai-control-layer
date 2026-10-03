import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/control-evaluation-response.signature-deny.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { ControlEvaluationController } from "./control-evaluation.controller.js";
import inputFixture from "@workspace/contracts/fixtures/control-evaluation-request.model-input.json" with { type: "json" };

describe("Judge evaluation facade", () => {
  let app: NestExpressApplication;
  let responseBody: unknown;
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
  const state = fixture;
  const input = { ...inputFixture, runId: fixture.runId };
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
        controllers: [ControlEvaluationController],
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
  it("forwards judge input unchanged and returns the recorded deny decision", async () => {
    const result = await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .set("x-request-id", "cancel-test")
      .send(input)
      .expect(200);
    expect(result.body).toEqual(state);
    expect(postCommand).toHaveBeenCalledWith(
      "/internal/control/evaluate",
      "cancel-test",
      input,
      expect.anything(),
      context,
    );
  });
  it("refuses missing authentication before dispatch", async () => {
    await request(app.getHttpServer()).post("/api/control/evaluate").send(input).expect(401);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it("rejects supplied identity before dispatch", async () => {
    await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .send({ organizationId: "forged" })
      .expect(400);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 409, 503])("preserves Go status %s", async (statusCode) => {
    failure = { success: false, reason: "bad_request", statusCode };
    await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .send(input)
      .expect(statusCode);
  });
  it("reports a timeout as an unconfirmed command with 504", async () => {
    failure = { success: false, reason: "timeout" };
    const result = await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .send(input)
      .expect(504);
    expect(result.body).toMatchObject({ error: { code: "outcome_unconfirmed" } });
  });
  it.each(["unreachable", "invalid_response"])(
    "refuses %s without claiming an evaluation",
    async (reason) => {
      failure = { success: false, reason };
      await request(app.getHttpServer())
        .post("/api/control/evaluate")
        .set("Cookie", "session=test")
        .send(input)
        .expect(503);
    },
  );
  it("rejects a different run in Go's response", async () => {
    responseBody = { ...state, runId: context.userId };
    await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .send(input)
      .expect(503);
  });
  it.each([
    { ...input, text: null },
    { ...input, tool: "read_invoice" },
    { ...input, kind: "tool_result", tool: null },
    { ...input, kind: "action_proposal", text: null, tool: "read_invoice", arguments: null },
    { ...input, actorId: "forged" },
    { ...input, inputSource: "agent" },
  ])("refuses invalid X-91 input: %j", async (body) => {
    await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .send(body)
      .expect(400);
    expect(postCommand).not.toHaveBeenCalled();
  });
  it.each([
    { ...input, kind: "tool_result", tool: "read_invoice" },
    {
      ...input,
      kind: "action_proposal",
      text: null,
      tool: "read_invoice",
      arguments: { invoice_id: "invoice_A01" },
    },
  ])("forwards each supported boundary without granting an action", async (body) => {
    const result = await request(app.getHttpServer())
      .post("/api/control/evaluate")
      .set("Cookie", "session=test")
      .send(body)
      .expect(200);
    expect(result.body).toMatchObject({ actionId: null, decision: "deny" });
    expect(postCommand.mock.calls[0]?.[2]).toEqual(body);
  });
});
