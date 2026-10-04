import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/start-run-response.created.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { RunsController } from "./runs.controller.js";

// POST /api/runs/judge starts a judge run: a passport and a run with no agent. It takes the same
// strict X-07 body and returns the same X-07 response as POST /api/runs, with the same guard and
// error mapping; only the gateway route differs.
describe("Judge run admission", () => {
  let app: NestExpressApplication;
  let responseBody: unknown;
  let failure:
    | { success: false; reason: string; statusCode?: number; code?: string; message?: string }
    | undefined;
  const input = {
    template: "reconcile_atlas_v1",
    invoiceIds: ["invoice_A01"],
    destination: "vendor_Atlas",
  };
  const postCommand = vi.fn((_path: string, _id: string, _body: unknown, schema: z.ZodTypeAny) => {
    if (failure) return failure;
    const parsed = schema.safeParse(responseBody);
    return parsed.success
      ? { success: true, data: parsed.data }
      : { success: false, reason: "invalid_response" };
  });
  const post = (body: unknown = input) =>
    request(app.getHttpServer())
      .post("/api/runs/judge")
      .set("Cookie", "session=test")
      .send(body as object);

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
    responseBody = fixture;
    failure = undefined;
    postCommand.mockClear();
  });

  it("forwards the body to the gateway's judge admission with the verified context", async () => {
    const result = await post().set("x-request-id", "judge-test").expect(201);
    expect(result.body).toEqual(fixture);
    expect(postCommand).toHaveBeenCalledTimes(1);
    expect(postCommand).toHaveBeenCalledWith(
      "/internal/judge-runs",
      "judge-test",
      input,
      expect.anything(),
      context,
      { keepErrorMessage: true },
    );
  });

  it("never reaches the agent run route", async () => {
    await post().expect(201);
    expect(postCommand.mock.calls.map((call) => call[0])).not.toContain("/internal/runs");
  });

  it("refuses missing authentication before dispatch", async () => {
    await request(app.getHttpServer()).post("/api/runs/judge").send(input).expect(401);
    expect(postCommand).not.toHaveBeenCalled();
  });

  it.each(["organizationId", "actorId", "roles", "inputSource", "agent"])(
    "rejects a supplied %s before admission (the body is strict)",
    async (key) => {
      await post({ ...input, [key]: "forged" }).expect(400);
      expect(postCommand).not.toHaveBeenCalled();
    },
  );

  it("rejects a wrong value type and a missing field before any upstream call", async () => {
    for (const body of [
      { ...input, template: 7 },
      { ...input, invoiceIds: "invoice_A01" },
      { ...input, limits: { modelCalls: -1 } },
      { template: input.template, destination: input.destination },
    ]) {
      await post(body).expect(400);
    }
    expect(postCommand).not.toHaveBeenCalled();
  });

  it.each([401, 403, 404, 409, 503])("preserves Go status %s", async (statusCode) => {
    failure = { success: false, reason: "bad_request", statusCode };
    await post().expect(statusCode);
  });

  it("keeps an admission rejection's reason code and explanation", async () => {
    failure = {
      success: false,
      reason: "bad_request",
      statusCode: 400,
      code: "resource_out_of_scope",
      message: "an invoice in invoiceIds is not available to this organization",
    };
    const result = await post().expect(400);
    expect(result.body).toMatchObject({
      statusCode: 400,
      error: {
        code: "resource_out_of_scope",
        message: "an invoice in invoiceIds is not available to this organization",
      },
    });
  });

  it.each([
    ["an unknown code", "not_a_reason_code", "Driver said: host db.internal"],
    ["control characters", "resource_out_of_scope", "line one\nline two"],
    ["an explanation over 300 characters", "resource_out_of_scope", "x".repeat(301)],
    ["a valid X-13 code that is not an admission rejection", "approval_required", "review needed"],
  ])("keeps the generic text for %s", async (_name, code, message) => {
    failure = { success: false, reason: "bad_request", statusCode: 400, code, message };
    const result = await post().expect(400);
    const body = result.body as { error: { code: string; message: string } };
    expect(body.error.code).toBe(code === "not_a_reason_code" ? "bad_request" : code);
    expect(body.error.message).toBe("Gateway request failed");
  });

  it("reports a timeout as an unconfirmed command with 504", async () => {
    failure = { success: false, reason: "timeout" };
    const result = await post().expect(504);
    expect(result.body).toMatchObject({ error: { code: "outcome_unconfirmed" } });
  });

  it.each(["unreachable", "invalid_response"])(
    "refuses %s without claiming admission",
    async (reason) => {
      failure = { success: false, reason };
      await post().expect(503);
    },
  );

  it("rejects unknown fields and malformed references in the admission response", async () => {
    for (const body of [
      { ...fixture, rawPayload: "secret" },
      { ...fixture, runId: "malformed" },
    ]) {
      responseBody = body;
      await post().expect(503);
    }
  });
});
