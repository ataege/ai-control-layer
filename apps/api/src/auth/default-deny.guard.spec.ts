import { Controller, Get, UnauthorizedException } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import request from "supertest";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { createTestApp } from "../testing/create-test-app.js";
import { setupOpenApi } from "../openapi.js";
import { configureApp } from "../app.setup.js";
import { DefaultDenyGuard } from "./default-deny.guard.js";
import { Test } from "@nestjs/testing";
import { APP_GUARD } from "@nestjs/core";
import { Public } from "./public.decorator.js";
import { AUTH_PROVIDER, type AuthenticatedPrincipal } from "./auth.types.js";
import { DataSource } from "typeorm";
import { getRepositoryToken } from "@nestjs/typeorm";
import { Membership } from "../identity/entities/membership.entity.js";

// Counts how often the protected handler ran, to prove a refused request never reaches it.
const protectedHandlerCalls = { count: 0 };

@Controller("guard-test")
class GuardTestController {
  @Get("protected")
  protectedRoute() {
    protectedHandlerCalls.count += 1;
    return { ok: true };
  }

  @Public()
  @Get("public")
  publicRoute() {
    return { ok: true };
  }
}

// The test doubles that createTestApp registers, replaced per test.
interface AuthProviderDouble {
  authenticate: (credential: string) => Promise<AuthenticatedPrincipal>;
}
interface DataSourceDouble {
  isInitialized: boolean;
}
interface MembershipRepositoryDouble {
  findOne: (options: unknown) => Promise<unknown>;
}
interface ErrorBody {
  error: { code: string; message: string };
}

describe("DefaultDenyGuard", () => {
  let app: NestExpressApplication;
  let authProviderMock: AuthProviderDouble;
  let dataSourceMock: DataSourceDouble;
  let membershipRepoMock: MembershipRepositoryDouble;

  beforeAll(async () => {
    app = await createTestApp({ controllers: [GuardTestController] });
    authProviderMock = app.get<unknown, AuthProviderDouble>(AUTH_PROVIDER);
    dataSourceMock = app.get<unknown, DataSourceDouble>(DataSource);
    membershipRepoMock = app.get<unknown, MembershipRepositoryDouble>(
      getRepositoryToken(Membership),
    );
  });

  afterAll(async () => {
    await app.close();
  });

  it("denies access when session cookie is missing, returning 401", async () => {
    const response = await request(app.getHttpServer()).get("/api/guard-test/protected");
    expect(response.status).toBe(401);
    expect(response.body).toMatchObject({
      statusCode: 401,
      error: { code: "unauthorized" },
    });
  });

  it("accepts a correct credential and grants access", async () => {
    authProviderMock.authenticate = vi.fn().mockResolvedValue({ subjectId: "test-user" });
    membershipRepoMock.findOne = vi
      .fn()
      .mockResolvedValue({ userId: "test-user", organizationId: "test-org", roles: [] });

    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", ["session=valid-session-id"]);

    expect(response.status).toBe(200);
    expect(response.body).toEqual({ ok: true });
    expect(authProviderMock.authenticate).toHaveBeenCalledWith("valid-session-id");
    expect(membershipRepoMock.findOne).toHaveBeenCalledWith({
      where: { userId: "test-user" },
      order: { createdAt: "ASC" },
    });
  });

  it("denies access if the user has no membership, and an organization identifier in body/query/path is ignored", async () => {
    authProviderMock.authenticate = vi
      .fn()
      .mockResolvedValue({ subjectId: "test-user-no-membership" });
    membershipRepoMock.findOne = vi.fn().mockResolvedValue(null); // no membership

    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected?organizationId=test-org")
      .send({ organizationId: "test-org" })
      .set("Cookie", ["session=valid-session-id"]);

    expect(response.status).toBe(401);
    expect((response.body as ErrorBody).error.message).toBe("User has no organization membership");
  });

  it("rejects a wrong, expired or revoked credential with 401", async () => {
    authProviderMock.authenticate = vi
      .fn()
      .mockRejectedValue(new UnauthorizedException("Invalid or expired session"));
    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", ["session=bad-session-id"]);

    expect(response.status).toBe(401);
    expect((response.body as ErrorBody).error.code).toBe("unauthorized");
  });

  it("answers 503 while the database is down", async () => {
    dataSourceMock.isInitialized = false;
    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", ["session=valid-session-id"]);

    expect(response.status).toBe(503);
    expect((response.body as ErrorBody).error.code).toBe("service_unavailable");

    // restore
    dataSourceMock.isInitialized = true;
  });

  it("answers 503 when the session lookup dependency fails", async () => {
    authProviderMock.authenticate = vi
      .fn()
      .mockRejectedValue(new Error("database connection secret"));
    const response = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", "session=valid-session-id")
      .expect(503);
    expect(JSON.stringify(response.body)).not.toContain("database connection secret");
  });

  it("allows access to a route with the @Public marker", async () => {
    const response = await request(app.getHttpServer()).get("/api/guard-test/public");
    expect(response.status).toBe(200);
    expect(response.body).toEqual({ ok: true });
  });

  it("never runs a protected handler for a refused request (API-03)", async () => {
    const callsBefore = protectedHandlerCalls.count;
    const missing = await request(app.getHttpServer()).get("/api/guard-test/protected");
    authProviderMock.authenticate = vi.fn().mockRejectedValue(new UnauthorizedException());
    const invalid = await request(app.getHttpServer())
      .get("/api/guard-test/protected")
      .set("Cookie", "session=revoked-session-id");
    expect([missing.status, invalid.status]).toEqual([401, 401]);
    expect(missing.body).toMatchObject({ statusCode: 401, error: { code: "unauthorized" } });
    expect(protectedHandlerCalls.count).toBe(callsBefore);
  });

  it("still serves the OpenAPI document behind the guard (API-03)", async () => {
    // Built like main.ts: the document is mounted before init, with the same global guard.
    const moduleRef = await Test.createTestingModule({
      controllers: [GuardTestController],
      providers: [
        { provide: DataSource, useValue: { isInitialized: true } },
        { provide: APP_GUARD, useClass: DefaultDenyGuard },
        { provide: AUTH_PROVIDER, useValue: { authenticate: vi.fn() } },
        { provide: getRepositoryToken(Membership), useValue: { findOne: vi.fn() } },
      ],
    }).compile();
    const documentedApp = moduleRef.createNestApplication<NestExpressApplication>({
      logger: false,
    });
    configureApp(documentedApp, ["http://localhost:3000"]);
    setupOpenApi(documentedApp);
    await documentedApp.init();
    try {
      const response = await request(documentedApp.getHttpServer()).get("/api/docs-json");
      expect(response.status).toBe(200);
      expect((response.body as { openapi?: string }).openapi).toMatch(/^3\./);
      // The guard still protects the product route of the same app.
      const protectedResponse = await request(documentedApp.getHttpServer()).get(
        "/api/guard-test/protected",
      );
      expect(protectedResponse.status).toBe(401);
    } finally {
      await documentedApp.close();
    }
  });

  it("returns 404 for an unknown route", async () => {
    const response = await request(app.getHttpServer()).get("/api/unknown");
    expect(response.status).toBe(404);
  });
});
