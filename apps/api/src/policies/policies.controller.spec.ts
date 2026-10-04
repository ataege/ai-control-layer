import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import { ServiceUnavailableException } from "@nestjs/common";
import request from "supertest";
import { beforeAll, afterAll, beforeEach, expect, it, vi } from "vitest";
import { createTestApp } from "../testing/create-test-app.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { PoliciesController } from "./policies.controller.js";
import { PoliciesService } from "./policies.service.js";
import context from "@workspace/contracts/fixtures/operator-context.operator.json" with { type: "json" };

let app: NestExpressApplication;
const reload = vi.fn();
const pointer = vi.fn();
const membership = vi.fn();
const accepted = {
  accepted: true,
  unchanged: false,
  revisionId: "3",
  fileDigest: "a".repeat(64),
  activeRevisionId: "2",
  feedRevisionId: "1",
  feedDigest: "b".repeat(64),
};
const current = {
  requestedRevisionId: "3",
  validatedRevisionId: "2",
  activeRevisionId: "2",
  activeFeedRevisionId: "1",
  lastError: null,
};
beforeAll(async () => {
  app = await createTestApp(
    {
      controllers: [PoliciesController],
      providers: [
        { provide: PoliciesService, useValue: { reload, pointer } },
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
  reload.mockReset().mockResolvedValue({ outcome: accepted, feedRevision: "feed_v1" });
  pointer.mockReset().mockResolvedValue(current);
  membership.mockResolvedValue({ ...context, roles: ["operator", "reviewer"] });
});
it("requests a revision without claiming activation and uses only the verified actor", async () => {
  const res = await request(app.getHttpServer())
    .post("/api/policies/reload?userId=forged")
    .set("Cookie", "session=test")
    .send({})
    .expect(202);
  expect(res.body).toEqual({
    status: "requested",
    requestedRevisionId: "3",
    fileDigest: accepted.fileDigest,
    feedRevision: "feed_v1",
  });
  expect(res.headers["cache-control"]).toBe("no-store");
  expect(reload).toHaveBeenCalledWith(context.userId);
});
it("returns 200 unchanged without adding response fields", async () => {
  reload.mockResolvedValue({ outcome: { ...accepted, unchanged: true }, feedRevision: "feed_v1" });
  const res = await request(app.getHttpServer())
    .post("/api/policies/reload")
    .set("Cookie", "session=test")
    .send({})
    .expect(200);
  expect(res.body).toEqual({ status: "unchanged", revisionId: "3" });
});
it("reports absent feed as null", async () => {
  reload.mockResolvedValue({ outcome: { ...accepted, feedRevisionId: null }, feedRevision: null });
  const res = await request(app.getHttpServer())
    .post("/api/policies/reload")
    .set("Cookie", "session=test")
    .send({})
    .expect(202);
  expect(res.body).toMatchObject({ feedRevision: null });
});
it.each(["reload", "status"])("refuses unauthenticated %s before any operation", async (route) => {
  const client = request(app.getHttpServer());
  await (
    route === "reload"
      ? client.post(`/api/policies/${route}`).send({})
      : client.get(`/api/policies/${route}`)
  ).expect(401);
  expect(reload).not.toHaveBeenCalled();
  expect(pointer).not.toHaveBeenCalled();
});
it.each(["reload", "status"])("refuses non-reviewer %s before any operation", async (route) => {
  membership.mockResolvedValue({ ...context, roles: ["operator"] });
  const client = request(app.getHttpServer());
  await (
    route === "reload"
      ? client.post(`/api/policies/${route}`).send({})
      : client.get(`/api/policies/${route}`)
  )
    .set("Cookie", "session=test")
    .expect(403);
  expect(reload).not.toHaveBeenCalled();
  expect(pointer).not.toHaveBeenCalled();
});
it.each([{ userId: "forged" }, { path: "/tmp/other" }, { content: "policy" }, [], null])(
  "refuses nonempty or invalid reload body %j",
  async (body) => {
    await request(app.getHttpServer())
      .post("/api/policies/reload")
      .set("Cookie", "session=test")
      .set("content-type", "application/json")
      .send(JSON.stringify(body))
      .expect(400);
    expect(reload).not.toHaveBeenCalled();
  },
);
it("returns importer issues in the policy error envelope only", async () => {
  const issues = [{ path: "budgets", message: "limits are invalid" }];
  reload.mockResolvedValue({
    outcome: { accepted: false, issues, fileDigest: accepted.fileDigest, activeRevisionId: "2" },
    feedRevision: null,
  });
  const res = await request(app.getHttpServer())
    .post("/api/policies/reload")
    .set("Cookie", "session=test")
    .send({})
    .expect(400);
  expect(res.body).toMatchObject({
    error: {
      code: "policy_reload_rejected",
      message: "Policy validation failed",
      issues,
    },
  });
});
it("refuses a pending edit with the exact revision_pending code", async () => {
  reload.mockResolvedValue({
    outcome: {
      accepted: false,
      pendingRevisionId: "3",
      issues: [],
      fileDigest: accepted.fileDigest,
      activeRevisionId: "2",
    },
    feedRevision: null,
  });
  const res = await request(app.getHttpServer())
    .post("/api/policies/reload")
    .set("Cookie", "session=test")
    .send({})
    .expect(409);
  expect(res.body).toMatchObject({ error: { code: "revision_pending" } });
});
it("keeps pending, active and rejected states visible from stored pointer facts", async () => {
  const first = await request(app.getHttpServer())
    .get("/api/policies/status")
    .set("Cookie", "session=test")
    .expect(200);
  expect(first.body).toEqual(current);
  pointer.mockResolvedValue({ ...current, validatedRevisionId: "3", activeRevisionId: "3" });
  const active = await request(app.getHttpServer())
    .get("/api/policies/status")
    .set("Cookie", "session=test")
    .expect(200);
  expect(active.body).toMatchObject({ activeRevisionId: "3" });
  pointer.mockResolvedValue({
    ...current,
    lastError: {
      reason: "policy_reload_rejected",
      code: "catalog_invalid",
      message: "Catalog invalid",
      revision_id: 3,
      stage: "gateway_validation",
      source_text: "protected",
    },
  });
  const rejected = await request(app.getHttpServer())
    .get("/api/policies/status")
    .set("Cookie", "session=test")
    .expect(200);
  expect(rejected.body).toMatchObject({
    lastError: {
      code: "catalog_invalid",
      message: "Catalog invalid",
      revisionId: "3",
      stage: "gateway_validation",
    },
  });
  expect(rejected.body).toMatchObject({ activeRevisionId: "2" });
  expect(JSON.stringify(rejected.body)).not.toContain("protected");
});
it("maps importer rejection without inventing a candidate revision", async () => {
  pointer.mockResolvedValue({
    ...current,
    lastError: {
      reason: "policy_reload_rejected",
      issues: [{ path: "budgets", message: "invalid limits" }],
      source_text: "protected",
    },
  });
  const res = await request(app.getHttpServer())
    .get("/api/policies/status")
    .set("Cookie", "session=test")
    .expect(200);
  expect(res.body).toMatchObject({
    lastError: {
      code: "policy_reload_rejected",
      message: "budgets: invalid limits",
      revisionId: null,
      stage: "import_validation",
    },
  });
  expect(JSON.stringify(res.body)).not.toContain("protected");
});
it("refuses malformed stored errors and unavailable dependencies", async () => {
  pointer.mockResolvedValue({
    ...current,
    lastError: {
      stage: "gateway_validation",
      revision_id: 9007199254740992,
      code: "catalog_invalid",
      message: "invalid",
    },
  });
  await request(app.getHttpServer())
    .get("/api/policies/status")
    .set("Cookie", "session=test")
    .expect(503);
  reload.mockRejectedValue(new ServiceUnavailableException("dependency unavailable"));
  await request(app.getHttpServer())
    .post("/api/policies/reload")
    .set("Cookie", "session=test")
    .send({})
    .expect(503);
});
