import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import fixture from "@workspace/contracts/fixtures/run-events-page.export-denied.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { RunsController } from "./runs.controller.js";

describe("Run events facade", () => {
  let app: NestExpressApplication;
  let upstreamBody: unknown;
  let upstreamStatus: number;
  const runId = fixture.events[0]!.runId;
  const context = {
    userId: "c28e2545-2de6-41b9-9be6-d21ae01e4901",
    organizationId: fixture.events[0]!.organizationId,
    roles: ["operator", "reviewer"],
  };
  const getRead = vi.fn((_path: string, _requestId: string, schema: z.ZodTypeAny) => {
    if (upstreamStatus !== 200) {
      return { success: false, reason: "bad_request", statusCode: upstreamStatus };
    }
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
    upstreamStatus = 200;
    getRead.mockClear();
  });

  it("relays the exact sanitized contract with verified context and cursor", async () => {
    const response = await request(app.getHttpServer())
      .get(`/api/runs/${runId}/events?after=40&limit=2`)
      .set("Cookie", "session=test-session")
      .set("x-request-id", "events-test")
      .expect(200);
    expect(response.body).toEqual(fixture);
    expect(getRead).toHaveBeenCalledWith(
      `/internal/runs/${runId}/events?after=40&limit=2`,
      "events-test",
      expect.anything(),
      context,
    );
  });

  it("preserves the cursor on an empty page", async () => {
    upstreamBody = { events: [], nextCursor: "42" };
    await request(app.getHttpServer())
      .get(`/api/runs/${runId}/events?after=42`)
      .set("Cookie", "session=test-session")
      .expect(200);
  });

  it("refuses requests without a session before reading Go", async () => {
    await request(app.getHttpServer()).get(`/api/runs/${runId}/events`).expect(401);
    expect(getRead).not.toHaveBeenCalled();
  });

  it.each([
    "after=abc",
    "after=1.5",
    "after=1e3",
    "after=",
    "after=-1",
    "after=9223372036854775808",
    "after=1&after=2",
    "limit=0",
    "limit=501",
    "organizationId=attacker",
  ])("refuses invalid pagination or supplied identity: %s", async (query) => {
    await request(app.getHttpServer())
      .get(`/api/runs/${runId}/events?${query}`)
      .set("Cookie", "session=test-session")
      .expect(400);
    expect(getRead).not.toHaveBeenCalled();
  });

  it("refuses malformed run references", async () => {
    await request(app.getHttpServer())
      .get("/api/runs/invalid/events")
      .set("Cookie", "session=test-session")
      .expect(400);
    expect(getRead).not.toHaveBeenCalled();
  });

  it.each([404, 401, 403, 503])(
    "preserves Go's %s without an existence leak",
    async (statusCode) => {
      upstreamStatus = statusCode;
      await request(app.getHttpServer())
        .get(`/api/runs/${runId}/events`)
        .set("Cookie", "session=test-session")
        .expect(statusCode);
    },
  );

  it.each([
    "foreign organization",
    "foreign run",
    "duplicate",
    "reversed",
    "unknown field",
    "unknown summary",
    "bad cursor",
    "missing run",
  ])("refuses an unsafe upstream page: %s", async (scenario) => {
    const page = structuredClone(fixture);
    switch (scenario) {
      case "foreign organization":
        page.events[0]!.organizationId = context.userId;
        break;
      case "foreign run":
        page.events[0]!.runId = context.userId;
        break;
      case "duplicate":
        page.events[1]!.eventId = "41";
        break;
      case "reversed":
        page.events.reverse();
        break;
      case "unknown field":
        upstreamBody = { ...page, rawPayload: "secret" };
        break;
      case "unknown summary":
        Object.assign(page.events[0]!.maskedSummary, { rawNote: "secret" });
        break;
      case "bad cursor":
        page.nextCursor = "43";
        break;
      case "missing run":
        Object.assign(page.events[0]!, { runId: null });
        break;
    }
    if (scenario !== "unknown field") upstreamBody = page;
    const response = await request(app.getHttpServer())
      .get(`/api/runs/${runId}/events`)
      .set("Cookie", "session=test-session")
      .expect(503);
    expect(JSON.stringify(response.body)).not.toContain("secret");
  });
});
