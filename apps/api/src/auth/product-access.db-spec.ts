import "reflect-metadata";
import { getRepositoryToken } from "@nestjs/typeorm";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { DataSource, type QueryRunner } from "typeorm";
import { randomUUID } from "node:crypto";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import type { OperatorContext } from "@workspace/contracts";
import state from "@workspace/contracts/fixtures/run-state.completed.json" with { type: "json" };
import usage from "@workspace/contracts/fixtures/run-usage.ledger.json" with { type: "json" };
import options from "@workspace/contracts/fixtures/task-form-options.atlas.json" with { type: "json" };
import events from "@workspace/contracts/fixtures/run-events-page.export-denied.json" with { type: "json" };
import report from "@workspace/contracts/fixtures/report-view.vendor.json" with { type: "json" };
import review from "@workspace/contracts/fixtures/review-view.queue-report.json" with { type: "json" };
import approval from "@workspace/contracts/fixtures/approval-response.approve.json" with { type: "json" };
import evaluation from "@workspace/contracts/fixtures/control-evaluation-response.scope-deny.json" with { type: "json" };
import { parseEnvironment, databaseEnvironmentSchema } from "../config/environment.js";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { RunsController } from "../runs/runs.controller.js";
import { ControlEvaluationController } from "../runs/control-evaluation.controller.js";
import { ActionsController } from "../actions/actions.controller.js";
import { SecurityController } from "../security/security.controller.js";
import { User } from "../identity/entities/user.entity.js";
import { Organization } from "../identity/entities/organization.entity.js";
import { PasswordHash } from "../identity/entities/password-hash.entity.js";
import { Session } from "../identity/entities/session.entity.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { AuthController } from "./auth.controller.js";
import { AUTH_PROVIDER } from "./auth.types.js";
import { CookieAuthProvider } from "./cookie-auth.provider.js";
import { hashPassword } from "./password.util.js";

const database = new DataSource({
  ...buildTypeOrmOptions(parseEnvironment(databaseEnvironmentSchema, process.env)),
  migrations: [],
});
const ownerOrganization = events.events[0]!.organizationId;
const password = process.env.DEMO_OPERATOR_PASSWORD!;
let transaction: QueryRunner;
let app: NestExpressApplication;
let ownUser: User;
let foreignUser: User;
let cookies: string[];
// Labelled Go read/command fixture: these tests prove real database/session membership at the
// Nest boundary and Go's status propagation, not Go execution or semantic detection quality.
const fixtureOutcome = (path: string, schema: z.ZodTypeAny, operator: OperatorContext) => {
  if (operator.organizationId !== ownerOrganization)
    return { success: false, reason: "bad_request", statusCode: 404 };
  const fixture = path.endsWith("/usage")
    ? usage
    : path.endsWith("/events")
      ? events
      : path.includes("/reports/")
        ? report
        : path.endsWith("/review")
          ? review
          : path.endsWith("/approval")
            ? approval
            : path.endsWith("/evaluate")
              ? evaluation
              : state;
  const parsed = schema.safeParse(fixture);
  return parsed.success
    ? { success: true, data: parsed.data }
    : { success: false, reason: "invalid_response" };
};
const getRead = vi.fn(
  (path: string, _id: string, schema: z.ZodTypeAny, operator: OperatorContext) =>
    fixtureOutcome(path, schema, operator),
);
const postCommand = vi.fn(
  (path: string, _id: string, _body: unknown, schema: z.ZodTypeAny, operator: OperatorContext) =>
    fixtureOutcome(path, schema, operator),
);
const routes = [
  { method: "get", path: `/api/runs/${state.runId}`, fixture: state },
  { method: "get", path: `/api/runs/${usage.runId}/usage`, fixture: usage },
  { method: "get", path: `/api/runs/${events.events[0]!.runId}/events`, fixture: events },
  { method: "get", path: `/api/runs/${report.runId}/reports/${report.reportId}`, fixture: report },
  { method: "get", path: `/api/actions/${review.action_id}/review`, fixture: review },
  {
    method: "post",
    path: `/api/actions/${approval.actionId}/approval`,
    body: { decision: "approve" },
    fixture: approval,
  },
  { method: "post", path: `/api/runs/${state.runId}/cancel`, body: {}, fixture: state },
  {
    method: "post",
    path: "/api/control/evaluate",
    body: {
      runId: evaluation.runId,
      kind: "action_proposal",
      text: null,
      tool: "read_invoice",
      arguments: { invoice_id: "invoice_OUTSIDE" },
    },
    fixture: evaluation,
  },
] as const;

beforeAll(async () => {
  await database.initialize();
});
afterAll(async () => {
  await database.destroy();
});
beforeEach(async () => {
  transaction = database.createQueryRunner();
  await transaction.startTransaction();
  const manager = transaction.manager;
  const secondOrganization = await manager
    .getRepository(Organization)
    .save({ name: "Second organization (test fixture)" });
  const users = manager.getRepository(User);
  ownUser = await users.save({
    email: `${randomUUID()}@example.test`,
    name: "Own operator (test fixture)",
  });
  foreignUser = await users.save({
    email: `${randomUUID()}@example.test`,
    name: "Foreign operator (test fixture)",
  });
  const hash = await hashPassword(password);
  for (const [user, organizationId] of [
    [ownUser, ownerOrganization],
    [foreignUser, secondOrganization.id],
  ] as const) {
    await manager.getRepository(PasswordHash).save({ userId: user.id, hash });
    await manager
      .getRepository(Membership)
      .save({ userId: user.id, organizationId, roles: ["operator", "reviewer"] });
  }
  app = await createTestApp(
    {
      controllers: [
        AuthController,
        RunsController,
        ActionsController,
        SecurityController,
        ControlEvaluationController,
      ],
      providers: [
        { provide: DataSource, useValue: database },
        { provide: AUTH_PROVIDER, useClass: CookieAuthProvider },
        { provide: GatewayClientService, useValue: { getRead, postCommand } },
        ...[User, PasswordHash, Session, Membership].map((entity) => ({
          provide: getRepositoryToken(entity),
          useValue: manager.getRepository(entity),
        })),
      ],
    },
    { mockAuth: false },
  );
  cookies = [];
  for (const user of [ownUser, foreignUser]) {
    const response = await request(app.getHttpServer())
      .post("/api/auth/sign-in")
      .send({ email: user.email, password })
      .expect(200);
    cookies.push((response.headers["set-cookie"] as unknown as string[])[0]!.split(";")[0]!);
  }
  getRead.mockClear();
  postCommand.mockClear();
});
afterEach(async () => {
  await app.close();
  await transaction.rollbackTransaction();
  await transaction.release();
});

it.each(routes)("uses real verified membership on $method $path", async (route) => {
  const client = request(app.getHttpServer());
  const response = await (
    route.method === "get" ? client.get(route.path) : client.post(route.path).send(route.body)
  )
    .set("Cookie", cookies[0]!)
    .expect(200);
  expect(response.body).toEqual(route.fixture);
  const calls = route.method === "get" ? getRead.mock.calls : postCommand.mock.calls;
  expect(calls[0]?.at(-1)).toEqual({
    userId: ownUser.id,
    organizationId: ownerOrganization,
    roles: ["operator", "reviewer"],
  });
});
it.each(routes)(
  "preserves object denial for the second organization's $method $path",
  async (route) => {
    const client = request(app.getHttpServer());
    await (
      route.method === "get" ? client.get(route.path) : client.post(route.path).send(route.body)
    )
      .set("Cookie", cookies[1]!)
      .expect(404);
    const calls = route.method === "get" ? getRead.mock.calls : postCommand.mock.calls;
    const operator = calls[0]?.at(-1) as OperatorContext;
    expect(operator.userId).toBe(foreignUser.id);
    expect(operator.organizationId).not.toBe(ownerOrganization);
  },
);
it("uses current database roles rather than session age or browser claims", async () => {
  await transaction.manager
    .getRepository(Membership)
    .update({ userId: ownUser.id }, { roles: ["operator"] });
  await request(app.getHttpServer())
    .get(`/api/actions/${review.action_id}/review?roles=reviewer`)
    .set("Cookie", cookies[0]!)
    .expect(403);
  await request(app.getHttpServer())
    .post(`/api/actions/${approval.actionId}/approval`)
    .set("Cookie", cookies[0]!)
    .send({ decision: "approve" })
    .expect(403);
  await request(app.getHttpServer())
    .get("/api/security/export?kind=events&roles=reviewer")
    .set("Cookie", cookies[0]!)
    .expect(403);
  expect(getRead).not.toHaveBeenCalled();
  expect(postCommand).not.toHaveBeenCalled();
});
it.each(routes)("refuses a removed membership before $method $path reaches Go", async (route) => {
  await transaction.manager.getRepository(Membership).delete({ userId: ownUser.id });
  const client = request(app.getHttpServer());
  await (route.method === "get" ? client.get(route.path) : client.post(route.path).send(route.body))
    .set("Cookie", cookies[0]!)
    .expect(401);
  expect(getRead).not.toHaveBeenCalled();
  expect(postCommand).not.toHaveBeenCalled();
});
it("ignores browser organization and forwarded identity claims with a real stored session", async () => {
  const membership = await transaction.manager
    .getRepository(Membership)
    .findOneByOrFail({ userId: foreignUser.id });
  await request(app.getHttpServer())
    .get(
      `/api/runs/${state.runId}?organizationId=${membership.organizationId}&userId=${foreignUser.id}`,
    )
    .set("Cookie", cookies[0]!)
    .set("X-Operator-Context", "browser-forged-context")
    .set("X-Forwarded-User", foreignUser.id)
    .expect(200);
  expect(getRead.mock.calls[0]?.at(-1)).toEqual({
    userId: ownUser.id,
    organizationId: ownerOrganization,
    roles: ["operator", "reviewer"],
  });
});

it("scopes task options to each real session's organization through the labelled Go fixture", async () => {
  const foreignOptions = {
    ...options,
    vendors: [],
    invoices: [],
    destinations: [],
  };
  for (const [index, fixture] of [options, foreignOptions].entries()) {
    getRead.mockImplementationOnce((_path, _id, schema) => {
      const parsed = schema.safeParse(fixture);
      return parsed.success
        ? { success: true, data: parsed.data }
        : { success: false, reason: "invalid_response" };
    });
    const response = await request(app.getHttpServer())
      .get(`/api/runs/options?organizationId=${ownerOrganization}`)
      .set("Cookie", cookies[index]!)
      .expect(200);
    expect(response.body).toEqual(fixture);
    const operator = getRead.mock.calls.at(-1)?.at(-1) as OperatorContext;
    expect(operator.userId).toBe(index === 0 ? ownUser.id : foreignUser.id);
    expect(operator.organizationId === ownerOrganization).toBe(index === 0);
  }
});
it("refuses task options after current membership is removed", async () => {
  await transaction.manager.getRepository(Membership).delete({ userId: ownUser.id });
  await request(app.getHttpServer())
    .get("/api/runs/options")
    .set("Cookie", cookies[0]!)
    .expect(401);
  expect(getRead).not.toHaveBeenCalled();
});
