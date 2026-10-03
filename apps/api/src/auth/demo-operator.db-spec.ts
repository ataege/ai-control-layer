import "reflect-metadata";
import { Controller, Get, Req } from "@nestjs/common";
import { getRepositoryToken } from "@nestjs/typeorm";
import type { NestExpressApplication } from "@nestjs/platform-express";
import type { Request } from "express";
import { DataSource, type QueryRunner } from "typeorm";
import { afterAll, afterEach, beforeAll, beforeEach, expect, it } from "vitest";
import request from "supertest";
import { randomUUID } from "node:crypto";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { createTestApp } from "../testing/create-test-app.js";
import { User } from "../identity/entities/user.entity.js";
import { PasswordHash } from "../identity/entities/password-hash.entity.js";
import { Session } from "../identity/entities/session.entity.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { AuthController } from "./auth.controller.js";
import { AUTH_PROVIDER } from "./auth.types.js";
import { CookieAuthProvider } from "./cookie-auth.provider.js";
import { DEMO_OPERATOR_EMAIL, DEMO_OPERATOR_ID, seedDemoOperator } from "./demo-operator-seed.js";

@Controller("seed-test")
class ProtectedController {
  @Get()
  context(@Req() req: Request) {
    return req.operatorContext;
  }
}

const dataSource = new DataSource({
  ...buildTypeOrmOptions(parseEnvironment(databaseEnvironmentSchema, process.env)),
  migrations: [],
});
const organization = {
  id: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
  name: "Demo Organization (synthetic)",
};
const password = process.env.DEMO_OPERATOR_PASSWORD;
let queryRunner: QueryRunner;
let app: NestExpressApplication;
beforeAll(async () => {
  await dataSource.initialize();
});
afterAll(async () => {
  await dataSource.destroy();
});
beforeEach(async () => {
  queryRunner = dataSource.createQueryRunner();
  await queryRunner.startTransaction();
  app = await createTestApp(
    {
      controllers: [AuthController, ProtectedController],
      providers: [
        { provide: DataSource, useValue: dataSource },
        { provide: AUTH_PROVIDER, useClass: CookieAuthProvider },
        ...[User, PasswordHash, Session, Membership].map((entity) => ({
          provide: getRepositoryToken(entity),
          useValue: queryRunner.manager.getRepository(entity),
        })),
      ],
    },
    { mockAuth: false },
  );
});
afterEach(async () => {
  await app.close();
  await queryRunner.rollbackTransaction();
  await queryRunner.release();
});

it("seeds idempotently without changing stored password or membership, and signs in through the real session check", async () => {
  await seedDemoOperator(queryRunner.manager, organization, password);
  const before = await queryRunner.manager.query<{ hash: string }[]>(
    `SELECT hash FROM app.password_hashes WHERE "userId" = $1`,
    [DEMO_OPERATOR_ID],
  );
  await seedDemoOperator(queryRunner.manager, organization, password);
  expect(
    await queryRunner.manager.query(`SELECT hash FROM app.password_hashes WHERE "userId" = $1`, [
      DEMO_OPERATOR_ID,
    ]),
  ).toEqual(before);
  const signedIn = await request(app.getHttpServer())
    .post("/api/auth/sign-in")
    .send({ email: DEMO_OPERATOR_EMAIL, password })
    .expect(200);
  const cookies = signedIn.headers["set-cookie"] as unknown as string[];
  expect(cookies[0]).toContain("HttpOnly");
  expect(cookies[0]).toContain("SameSite=Lax");
  const cookie = cookies[0]!.split(";")[0]!;
  const context = await request(app.getHttpServer())
    .get("/api/seed-test")
    .set("Cookie", cookie)
    .expect(200);
  expect(context.body).toEqual({
    userId: DEMO_OPERATOR_ID,
    organizationId: organization.id,
    roles: ["operator", "reviewer"],
  });
  await request(app.getHttpServer())
    .post("/api/auth/sign-in")
    .send({ email: DEMO_OPERATOR_EMAIL, password: randomUUID() })
    .expect(401);
  await request(app.getHttpServer()).get("/api/seed-test").expect(401);
  await request(app.getHttpServer()).post("/api/auth/sign-out").set("Cookie", cookie).expect(200);
  await request(app.getHttpServer()).get("/api/seed-test").set("Cookie", cookie).expect(401);
});

it("refuses missing credentials and refuses drift without granting or overwriting authority", async () => {
  await expect(seedDemoOperator(queryRunner.manager, organization, undefined)).rejects.toThrow(
    "DEMO_OPERATOR_PASSWORD",
  );
  await expect(seedDemoOperator(queryRunner.manager, organization, randomUUID())).rejects.toThrow(
    "password differs",
  );
  await queryRunner.manager.query(
    `UPDATE app.memberships SET roles = ARRAY['operator'] WHERE "userId" = $1`,
    [DEMO_OPERATOR_ID],
  );
  await expect(seedDemoOperator(queryRunner.manager, organization, password)).rejects.toThrow(
    "membership differs",
  );
  const [membership] = await queryRunner.manager.query<{ roles: string[] }[]>(
    `SELECT roles FROM app.memberships WHERE "userId" = $1`,
    [DEMO_OPERATOR_ID],
  );
  expect(membership?.roles).toEqual(["operator"]);
});
