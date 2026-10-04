import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import active from "@workspace/contracts/fixtures/catalog-status.active.json" with { type: "json" };
import rejected from "@workspace/contracts/fixtures/catalog-status.rejected-request.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { createTestApp } from "../testing/create-test-app.js";
import { PolicyCatalogController } from "./policy-catalog.controller.js";

describe("Policy catalog facade", () => {
  let app: NestExpressApplication;
  let membership: { userId: string; organizationId: string; roles: string[] };
  let upstreamBody: unknown;
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
        controllers: [PolicyCatalogController],
        providers: [
          { provide: GatewayClientService, useValue: { getRead } },
          {
            provide: AUTH_PROVIDER,
            useValue: { authenticate: () => Promise.resolve({ subjectId: context.userId }) },
          },
          {
            provide: getRepositoryToken(Membership),
            useValue: { findOne: () => Promise.resolve(membership) },
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
    membership = structuredClone(context);
    upstreamBody = structuredClone(active);
    failure = undefined;
    getRead.mockClear();
  });

  const read = () => request(app.getHttpServer()).get("/api/policies/catalog");

  it("passes the exact shared shape, verified context and request id to the gateway", async () => {
    const response = await read()
      .set("Cookie", "session=test")
      .set("x-request-id", "catalog-read-test")
      .expect(200);
    expect(response.body).toEqual(active);
    expect(response.headers["cache-control"]).toBe("no-store");
    expect(getRead).toHaveBeenCalledWith(
      "/internal/catalog/active",
      "catalog-read-test",
      expect.anything(),
      context,
    );
  });
  it("returns a rejected activation with the last good revision unchanged", async () => {
    upstreamBody = structuredClone(rejected);
    const response = await read().set("Cookie", "session=test").expect(200);
    expect(response.body).toEqual(rejected);
  });
  it("rejects an unauthenticated read before the gateway", async () => {
    await read().expect(401);
    expect(getRead).not.toHaveBeenCalled();
  });
  it("refuses an operator without the reviewer role before the gateway", async () => {
    membership = { ...membership, roles: ["operator"] };
    await read().set("Cookie", "session=test").expect(403);
    expect(getRead).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 503])("keeps the gateway status %s", async (statusCode) => {
    failure = { success: false, reason: "bad_request", statusCode };
    await read().set("Cookie", "session=test").expect(statusCode);
  });
  it.each(["timeout", "unreachable", "invalid_response"])(
    "answers 503 on %s and never an empty catalog",
    async (reason) => {
      failure = { success: false, reason };
      const response = await read().set("Cookie", "session=test").expect(503);
      expect(response.body).not.toHaveProperty("controls");
    },
  );
  it.each(["unknown field", "wrong type"])(
    "refuses a gateway answer outside the contract: %s",
    async (scenario) => {
      upstreamBody =
        scenario === "unknown field"
          ? { ...active, policyText: "secret-policy-text" }
          : { ...active, activeRevisionId: "1" };
      const response = await read().set("Cookie", "session=test").expect(503);
      expect(JSON.stringify(response.body)).not.toContain("secret-policy-text");
    },
  );
});
