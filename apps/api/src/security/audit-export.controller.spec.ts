import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import eventFixture from "@workspace/contracts/fixtures/security-event-page.judge.json" with { type: "json" };
import assessmentFixture from "@workspace/contracts/fixtures/assessment-page.two-records.json" with { type: "json" };
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { SecurityController } from "./security.controller.js";
import { auditCsv } from "./audit-export.js";

describe("Reviewer audit export", () => {
  let app: NestExpressApplication;
  let roles: string[];
  let body: unknown;
  let failure: { success: false; reason: string; statusCode?: number } | undefined;
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
        controllers: [SecurityController],
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
    body = structuredClone(eventFixture);
    failure = undefined;
    getRead.mockClear();
  });
  it.each(["events", "assessments"])(
    "relays the exact %s page and maps after to Go's cursor",
    async (kind) => {
      const fixture = kind === "events" ? eventFixture : assessmentFixture;
      body = structuredClone(fixture);
      const result = await request(app.getHttpServer())
        .get(`/api/security/export?kind=${kind}&format=json&after=v1.0.50.0&limit=2`)
        .set("Cookie", "session=test")
        .set("x-request-id", "export-test")
        .expect(200);
      expect(result.body).toEqual(fixture);
      expect(getRead).toHaveBeenCalledWith(
        `/internal/security/${kind}?cursor=v1.0.50.0&limit=2`,
        "export-test",
        expect.anything(),
        { ...context, roles },
      );
    },
  );
  it.each(["events", "assessments"])(
    "exports %s CSV with the returned cursor header",
    async (kind) => {
      const fixture = kind === "events" ? eventFixture : assessmentFixture;
      body = structuredClone(fixture);
      const result = await request(app.getHttpServer())
        .get(`/api/security/export?kind=${kind}&format=csv`)
        .set("Cookie", "session=test")
        .expect(200);
      expect(result.headers["content-type"]).toContain("text/csv");
      expect(result.headers["x-next-cursor"]).toBe(fixture.nextCursor);
      expect(result.headers["cache-control"]).toBe("no-store");
      expect(result.text).not.toContain("[object Object]");
      expect(result.text).toContain(kind === "events" ? '"eventId"' : '"assessmentId"');
    },
  );
  it("refuses a non-reviewer before contacting Go", async () => {
    roles = ["operator"];
    await request(app.getHttpServer())
      .get("/api/security/export?kind=events&roles=reviewer")
      .set("Cookie", "session=test")
      .expect(403);
    expect(getRead).not.toHaveBeenCalled();
  });
  it("refuses missing authentication before contacting Go", async () => {
    await request(app.getHttpServer()).get("/api/security/export?kind=events").expect(401);
    expect(getRead).not.toHaveBeenCalled();
  });
  it.each([
    "kind=unknown",
    "kind=events&limit=501",
    "kind=events&after=v1.50.20.0",
    "kind=events&after=v1.0.0.1",
    "kind=events&after=v1.0.18446744073709551616.0",
    "kind=events&organizationId=forged",
    "kind=events&format=raw",
    "kind=events&limit=1&limit=2",
  ])("rejects invalid query: %s", async (query) => {
    await request(app.getHttpServer())
      .get(`/api/security/export?${query}`)
      .set("Cookie", "session=test")
      .expect(400);
    expect(getRead).not.toHaveBeenCalled();
  });
  it.each([401, 403, 404, 503])("preserves Go failure %s", async (statusCode) => {
    failure = { success: false, reason: "server_error", statusCode };
    await request(app.getHttpServer())
      .get("/api/security/export?kind=events")
      .set("Cookie", "session=test")
      .expect(statusCode);
  });
  it.each(["unreachable", "timeout", "invalid_response"])("fails closed on %s", async (reason) => {
    failure = { success: false, reason };
    await request(app.getHttpServer())
      .get("/api/security/export?kind=events")
      .set("Cookie", "session=test")
      .expect(503);
  });
  it("refuses another organization's events and raw payload fields", async () => {
    body = {
      ...eventFixture,
      events: eventFixture.events.map((event) => ({ ...event, organizationId: context.userId })),
    };
    await request(app.getHttpServer())
      .get("/api/security/export?kind=events")
      .set("Cookie", "session=test")
      .expect(503);
    body = { ...eventFixture, rawNote: "protected-text" };
    const result = await request(app.getHttpServer())
      .get("/api/security/export?kind=events")
      .set("Cookie", "session=test")
      .expect(503);
    expect(JSON.stringify(result.body)).not.toContain("protected-text");
  });
  it("neutralizes an upstream formula cell in CSV", async () => {
    body = {
      ...assessmentFixture,
      records: assessmentFixture.records.map((record) => ({ ...record, matchedRuleId: "=1+1" })),
    };
    const result = await request(app.getHttpServer())
      .get("/api/security/export?kind=assessments&format=csv")
      .set("Cookie", "session=test")
      .expect(200);
    expect(result.text).toContain('"\'=1+1"');
  });
  it.each(["=1", "+1", "-1", "@SUM(1)", " \t=1", "\u0000=1"])(
    "neutralizes formula trigger %s",
    (text) => {
      expect(auditCsv([{ value: text }], ["value"])).toContain("\"'" + text + '"');
    },
  );
});
