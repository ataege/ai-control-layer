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

describe("Admission verified context", () => {
  let app: NestExpressApplication;
  let responseBody: unknown;
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
  const state = fixture;
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
  it("forwards admission with the verified context and request id", async () => {
    const result = await request(app.getHttpServer())
      .post("/api/runs")
      .set("Cookie", "session=test")
      .set("x-request-id", "cancel-test")
      .send(input)
      .expect(201);
    expect(result.body).toEqual(state);
    expect(postCommand).toHaveBeenCalledWith(
      "/internal/runs",
      "cancel-test",
      input,
      expect.anything(),
      context,
    );
  });
  it("refuses missing authentication before dispatch", async () => {
    await request(app.getHttpServer()).post("/api/runs").send(input).expect(401);
    expect(postCommand).not.toHaveBeenCalled();
  });

  it.each(["organizationId", "actorId", "roles"])(
    "rejects supplied %s before admission",
    async (key) => {
      await request(app.getHttpServer())
        .post("/api/runs")
        .set("Cookie", "session=test")
        .send({ ...input, [key]: "forged" })
        .expect(400);
      expect(postCommand).not.toHaveBeenCalled();
    },
  );
  it.each([401, 403, 404, 409, 503])("preserves Go status %s", async (statusCode) => {
    failure = { success: false, reason: "bad_request", statusCode };
    await request(app.getHttpServer())
      .post("/api/runs")
      .set("Cookie", "session=test")
      .send(input)
      .expect(statusCode);
  });
  it("reports a timeout as an unconfirmed command with 504", async () => {
    failure = { success: false, reason: "timeout" };
    const result = await request(app.getHttpServer())
      .post("/api/runs")
      .set("Cookie", "session=test")
      .send(input)
      .expect(504);
    expect(result.body).toMatchObject({ error: { code: "outcome_unconfirmed" } });
  });
  it.each(["unreachable", "invalid_response"])(
    "refuses %s without claiming admission",
    async (reason) => {
      failure = { success: false, reason };
      await request(app.getHttpServer())
        .post("/api/runs")
        .set("Cookie", "session=test")
        .send(input)
        .expect(503);
    },
  );

  it("rejects unknown fields and malformed references in admission responses", async () => {
    for (const body of [
      { ...state, rawPayload: "secret" },
      { ...state, runId: "malformed" },
    ]) {
      responseBody = body;
      await request(app.getHttpServer())
        .post("/api/runs")
        .set("Cookie", "session=test")
        .send(input)
        .expect(503);
    }
  });
});
