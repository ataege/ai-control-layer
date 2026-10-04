import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/task-form-options.atlas.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import {
  GatewayClientService,
  type CommandOutcome,
} from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { RunsController } from "./runs.controller.js";

let app: NestExpressApplication;
let body: unknown;
let failure: CommandOutcome<unknown> | undefined;
const membership = vi.fn();
const getRead = vi.fn((_path: string, _id: string, schema: z.ZodTypeAny) => {
  if (failure) return failure;
  const parsed = schema.safeParse(body);
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
        { provide: getRepositoryToken(Membership), useValue: { findOne: membership } },
      ],
    },
    { mockAuth: false },
  );
});
afterAll(async () => app.close());
beforeEach(() => {
  body = structuredClone(fixture);
  failure = undefined;
  membership.mockResolvedValue(context);
  getRead.mockClear();
});
it("relays exact shared options using verified membership, ignoring browser identity", async () => {
  const response = await request(app.getHttpServer())
    .get("/api/runs/options?organizationId=forged&roles=reviewer")
    .set("Cookie", "session=test")
    .set("X-Operator-Context", "forged")
    .set("x-request-id", "task-options-test")
    .expect(200);
  expect(response.body).toEqual(fixture);
  expect(getRead).toHaveBeenCalledWith(
    "/internal/task-options",
    "task-options-test",
    expect.anything(),
    context,
  );
});
it("requires no reviewer role", async () => {
  membership.mockResolvedValue({ ...context, roles: ["operator"] });
  await request(app.getHttpServer())
    .get("/api/runs/options")
    .set("Cookie", "session=test")
    .expect(200);
});
it("refuses a missing session before Go", async () => {
  await request(app.getHttpServer()).get("/api/runs/options").expect(401);
  expect(getRead).not.toHaveBeenCalled();
});
it("refuses missing membership before Go", async () => {
  membership.mockResolvedValue(null);
  await request(app.getHttpServer())
    .get("/api/runs/options")
    .set("Cookie", "session=test")
    .expect(401);
  expect(getRead).not.toHaveBeenCalled();
});
it.each([401, 403, 404, 503])("preserves upstream status %s", async (statusCode) => {
  failure = { success: false, reason: "server_error", statusCode };
  const response = await request(app.getHttpServer())
    .get("/api/runs/options")
    .set("Cookie", "session=test")
    .expect(statusCode);
  expect(response.body).not.toHaveProperty("templates");
});
it.each(["timeout", "unreachable", "invalid_response"] as const)(
  "fails closed on %s",
  async (reason) => {
    failure = { success: false, reason };
    await request(app.getHttpServer())
      .get("/api/runs/options")
      .set("Cookie", "session=test")
      .expect(503);
  },
);
it.each(["extra field", "missing limits", "invalid invoice"])(
  "rejects malformed Go options: %s",
  async (scenario) => {
    const invalid = structuredClone(fixture) as Record<string, unknown>;
    if (scenario === "extra field") invalid.internalNote = "protected-fixture-value";
    if (scenario === "missing limits") delete invalid.limits;
    if (scenario === "invalid invoice")
      invalid.invoices = [{ ...fixture.invoices[0], amount: 1.5 }];
    body = invalid;
    const response = await request(app.getHttpServer())
      .get("/api/runs/options")
      .set("Cookie", "session=test")
      .expect(503);
    expect(JSON.stringify(response.body)).not.toContain("protected-fixture-value");
  },
);
