import { Controller, Get, UnauthorizedException } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import request from "supertest";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { createTestApp } from "../testing/create-test-app.js";
import { Public } from "./public.decorator.js";
import { AUTH_PROVIDER } from "./auth.types.js";
import { DataSource } from "typeorm";

@Controller("guard-test")
class GuardTestController {
  @Get("protected")
  protectedRoute() {
    return { ok: true };
  }

  @Public()
  @Get("public")
  publicRoute() {
    return { ok: true };
  }
}

describe("DefaultDenyGuard", () => {
  let app: NestExpressApplication;
  let authProviderMock: any;
  let dataSourceMock: any;

  beforeAll(async () => {
    app = await createTestApp({ controllers: [GuardTestController] });
    authProviderMock = app.get(AUTH_PROVIDER);
    dataSourceMock = app.get(DataSource);
  });

  afterAll(async () => {
    await app.close();
  });

  it("denies access when session cookie is missing, returning 401", async () => {
    const response = await request(app.getHttpServer()).get("/api/guard-test/protected");
    expect(response.status).toBe(401);
    expect(response.body).toMatchObject({
      statusCode: 401,
      error: { code: "unauthorized" }
    });
  });

  it("accepts a correct credential and grants access", async () => {
    authProviderMock.authenticate = vi.fn().mockResolvedValue({ subjectId: "test" });
    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", ["session=valid-session-id"]);
      
    expect(response.status).toBe(200);
    expect(response.body).toEqual({ ok: true });
    expect(authProviderMock.authenticate).toHaveBeenCalledWith("valid-session-id");
  });

  it("rejects a wrong, expired or revoked credential with 401", async () => {
    authProviderMock.authenticate = vi.fn().mockRejectedValue(new UnauthorizedException("Invalid or expired session"));
    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", ["session=bad-session-id"]);
      
    expect(response.status).toBe(401);
    expect(response.body.error.code).toBe("unauthorized");
  });

  it("answers 503 while the database is down", async () => {
    dataSourceMock.isInitialized = false;
    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", ["session=valid-session-id"]);
      
    expect(response.status).toBe(503);
    expect(response.body.error.code).toBe("service_unavailable");

    // restore
    dataSourceMock.isInitialized = true;
  });

  it("allows access to a route with the @Public marker", async () => {
    const response = await request(app.getHttpServer()).get("/api/guard-test/public");
    expect(response.status).toBe(200);
    expect(response.body).toEqual({ ok: true });
  });

  it("returns 404 for an unknown route", async () => {
    const response = await request(app.getHttpServer()).get("/api/unknown");
    expect(response.status).toBe(404);
  });
});
