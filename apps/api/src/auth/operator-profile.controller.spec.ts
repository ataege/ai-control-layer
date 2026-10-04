import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import request from "supertest";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createTestApp } from "../testing/create-test-app.js";
import { User } from "../identity/entities/user.entity.js";
import { PasswordHash } from "../identity/entities/password-hash.entity.js";
import { Session } from "../identity/entities/session.entity.js";
import { AuthController } from "./auth.controller.js";

let app: NestExpressApplication;
const findOne = vi.fn();
beforeEach(async () => {
  findOne.mockReset();
  app = await createTestApp({
    controllers: [AuthController],
    providers: [
      { provide: getRepositoryToken(User), useValue: { findOne } },
      { provide: getRepositoryToken(PasswordHash), useValue: {} },
      { provide: getRepositoryToken(Session), useValue: {} },
    ],
  });
});
afterEach(async () => app.close());

it("refuses an unauthenticated profile request without reading app users", async () => {
  await request(app.getHttpServer()).get("/api/auth/me").expect(401);
  expect(findOne).not.toHaveBeenCalled();
});
it("refuses a deleted user rather than fabricating a profile", async () => {
  findOne.mockResolvedValue(null);
  await request(app.getHttpServer())
    .get("/api/auth/me")
    .set("Cookie", "session=labelled-test-session")
    .expect(401);
  expect(findOne).toHaveBeenCalledWith({
    where: { id: "test-user" },
    select: { id: true, email: true, name: true },
  });
});
it("refuses an unavailable profile dependency without exposing its error", async () => {
  findOne.mockRejectedValue(new Error("database-password-fixture"));
  const response = await request(app.getHttpServer())
    .get("/api/auth/me")
    .set("Cookie", "session=labelled-test-session")
    .expect(503);
  expect(JSON.stringify(response.body)).not.toContain("database-password-fixture");
});
