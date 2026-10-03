import { Controller, Get, Req } from "@nestjs/common";
import { Test, TestingModule } from "@nestjs/testing";
import request from "supertest";
import { configureApp } from "./app.setup.js";
import { describe, expect, it } from "vitest";
import type { NestExpressApplication } from "@nestjs/platform-express";
import type { Request } from "express";
import { Public } from "./auth/public.decorator.js";
import { DefaultDenyGuard } from "./auth/default-deny.guard.js";
import { APP_GUARD } from "@nestjs/core";
import { AUTH_PROVIDER } from "./auth/auth.types.js";
import { DataSource } from "typeorm";
import { getRepositoryToken } from "@nestjs/typeorm";
import { Membership } from "./identity/entities/membership.entity.js";

@Controller("test")
class SetupTestController {
  @Public()
  @Get("ip-host")
  getIpAndHost(@Req() req: Request) {
    return {
      ip: req.ip,
      host: req.hostname,
    };
  }
}

describe("App Setup (Decision 3: Forwarder)", () => {
  it("ignores forged x-forwarded-for and x-forwarded-host headers", async () => {
    const moduleRef: TestingModule = await Test.createTestingModule({
      controllers: [SetupTestController],
      providers: [
        { provide: APP_GUARD, useClass: DefaultDenyGuard },
        { provide: AUTH_PROVIDER, useValue: { authenticate: async () => ({ subjectId: "test" }) } },
        { provide: DataSource, useValue: { isInitialized: true } },
        { provide: getRepositoryToken(Membership), useValue: { findOne: async () => ({ userId: "test", organizationId: "test-org", roles: [] }) } },
      ],
    }).compile();

    const app = moduleRef.createNestApplication<NestExpressApplication>({ logger: false });
    configureApp(app, []);
    await app.init();

    const response = await request(app.getHttpServer())
      .get("/api/test/ip-host")
      .set("x-forwarded-for", "203.0.113.195")
      .set("x-forwarded-host", "hacker.com");

    expect(response.status).toBe(200);
    // Since trust proxy is false, IP should be the loopback address of the test client (e.g. ::ffff:127.0.0.1 or ::1)
    expect(response.body.ip).not.toBe("203.0.113.195");
    expect(response.body.host).not.toBe("hacker.com");

    await app.close();
  });
});
