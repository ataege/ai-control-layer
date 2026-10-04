import { Controller, Get, Post } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import request from "supertest";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { Public } from "../auth/public.decorator.js";
import { createTestApp } from "../testing/create-test-app.js";

// Decision 3 (lead, 2026-10-04): browsers reach the API only through the web app's same-origin
// forwarder. The API therefore grants no credentialed cross-origin access and no command method
// to any origin; another origin gets no CORS headers at all (API-07).
@Controller("cors-test")
class CorsTestController {
  @Public()
  @Get("read")
  read() {
    return { ok: true };
  }

  @Post("command")
  command() {
    return { ok: true };
  }
}

describe("CORS on the forwarder path (API-07)", () => {
  let app: NestExpressApplication;

  beforeAll(async () => {
    // createTestApp allows exactly http://localhost:3000, like the default CORS_ALLOWED_ORIGINS.
    app = await createTestApp({ controllers: [CorsTestController] });
  });

  afterAll(async () => {
    await app.close();
  });

  it("gives a preflight from another origin no CORS headers", async () => {
    const response = await request(app.getHttpServer())
      .options("/api/cors-test/command")
      .set("Origin", "https://attacker.example")
      .set("Access-Control-Request-Method", "POST")
      .set("Access-Control-Request-Headers", "content-type");
    expect(response.headers["access-control-allow-origin"]).toBeUndefined();
    expect(response.headers["access-control-allow-credentials"]).toBeUndefined();
  });

  it("allows the configured origin only reads, never credentials or commands", async () => {
    const preflight = await request(app.getHttpServer())
      .options("/api/cors-test/command")
      .set("Origin", "http://localhost:3000")
      .set("Access-Control-Request-Method", "POST");
    expect(preflight.headers["access-control-allow-origin"]).toBe("http://localhost:3000");
    expect(preflight.headers["access-control-allow-methods"]).not.toMatch(/POST/);
    expect(preflight.headers["access-control-allow-credentials"]).toBeUndefined();

    const read = await request(app.getHttpServer())
      .get("/api/cors-test/read")
      .set("Origin", "http://localhost:3000");
    expect(read.status).toBe(200);
    expect(read.headers["access-control-allow-credentials"]).toBeUndefined();
  });

  it("refuses an unauthenticated command whatever its origin claims", async () => {
    const response = await request(app.getHttpServer())
      .post("/api/cors-test/command")
      .set("Origin", "http://localhost:3000")
      .set("x-forwarded-host", "localhost:3000");
    // createTestApp's auth double accepts any credential, but without a session cookie the
    // guard refuses before the handler runs.
    expect(response.status).toBe(401);
  });
});
