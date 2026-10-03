import { BadRequestException, Controller, Get, HttpException, Logger, Param } from "@nestjs/common";
import reasonContract from "@workspace/contracts/schemas/reason-code.schema.json" with { type: "json" };
import type { NestExpressApplication } from "@nestjs/platform-express";
import request from "supertest";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { createTestApp } from "../testing/create-test-app.js";
import { Public } from "../auth/public.decorator.js";

@Public()
@Controller("failing")
class FailingController {
  @Get("reason/:code")
  reason(@Param("code") code: string): never {
    throw new HttpException({ code, message: "The operation was refused" }, 409);
  }

  @Get("sensitive")
  sensitive(): never {
    throw Object.assign(new Error("Bearer test-service-secret; protected review payload"), {
      name: "test-signing-secret",
    });
  }
  @Get("unexpected")
  throwUnexpected(): never {
    throw new Error("connection string postgres://starter:hunter2@db-host:5432/starter");
  }

  @Get("rejected")
  throwBadRequest(): never {
    throw new BadRequestException("The request is not valid");
  }

  @Get("oversized")
  throwPayloadTooLarge(): never {
    // Shape of the http-errors object body-parser throws for an oversized body.
    throw Object.assign(new Error("request entity too large: limit 102400"), {
      name: "PayloadTooLargeError",
      status: 413,
      statusCode: 413,
    });
  }

  @Get("foreign-server-status")
  throwForeignServerStatus(): never {
    throw Object.assign(new Error("upstream detail"), { status: 503, statusCode: 503 });
  }
}

describe("AllExceptionsFilter", () => {
  let app: NestExpressApplication;
  const loggedErrors = vi.spyOn(Logger.prototype, "error").mockImplementation(() => undefined);

  beforeAll(async () => {
    app = await createTestApp({ controllers: [FailingController] });
  });

  afterAll(async () => {
    await app.close();
    loggedErrors.mockRestore();
  });

  it("hides unexpected error details in both responses and general logs", async () => {
    const response = await request(app.getHttpServer())
      .get("/api/failing/unexpected?debug=query-value")
      .set("x-request-id", "req-filter-1");

    expect(response.status).toBe(500);
    expect(response.headers["x-request-id"]).toBe("req-filter-1");
    expect(response.body).toEqual({
      error: { code: "internal_error", message: "Internal Server Error" },
      statusCode: 500,
      requestId: "req-filter-1",
      timestamp: expect.any(String) as string,
      path: "/api/failing/unexpected",
    });
    const serializedBody = JSON.stringify(response.body);
    expect(serializedBody).not.toContain("hunter2");
    expect(serializedBody).not.toContain("stack");
    expect(serializedBody).not.toContain("query-value");
    expect(loggedErrors).toHaveBeenCalledWith(
      "request failed",
      expect.objectContaining({ requestId: "req-filter-1", errorName: "Error" }),
    );
    expect(JSON.stringify(loggedErrors.mock.calls)).not.toContain("hunter2");
    expect(JSON.stringify(loggedErrors.mock.calls)).not.toContain("stack");
  });

  it.each(reasonContract.enum)("preserves the shared reason code %s", async (code) => {
    const response = await request(app.getHttpServer())
      .get(`/api/failing/reason/${code}`)
      .expect(409);
    expect(response.body).toMatchObject({ error: { code, message: "The operation was refused" } });
  });

  it("never exposes credentials, review content or an untrusted exception name", async () => {
    loggedErrors.mockClear();
    const response = await request(app.getHttpServer()).get("/api/failing/sensitive").expect(500);
    const text = JSON.stringify([response.body, loggedErrors.mock.calls]);
    for (const sensitive of [
      "test-service-secret",
      "test-signing-secret",
      "protected review payload",
    ]) {
      expect(text).not.toContain(sensitive);
    }
  });

  it("keeps the message of client errors", async () => {
    const response = await request(app.getHttpServer()).get("/api/failing/rejected");

    expect(response.status).toBe(400);
    expect(response.body).toMatchObject({
      error: { code: "bad_request", message: "The request is not valid" },
      statusCode: 400,
    });
  });

  it("honours the 4xx status of a non-Nest error without logging it as a server fault", async () => {
    loggedErrors.mockClear();

    const response = await request(app.getHttpServer()).get("/api/failing/oversized");

    expect(response.status).toBe(413);
    expect(response.body).toMatchObject({
      error: { code: "payload_too_large", message: "Payload Too Large" },
      statusCode: 413,
    });
    expect(loggedErrors).not.toHaveBeenCalled();
  });

  it("does not honour a 5xx status carried by a non-Nest error", async () => {
    const response = await request(app.getHttpServer()).get("/api/failing/foreign-server-status");

    expect(response.status).toBe(500);
    expect(response.body).toMatchObject({
      error: { code: "internal_error", message: "Internal Server Error" },
    });
  });

  it("answers unknown routes with not_found and replaces a malformed request id", async () => {
    const response = await request(app.getHttpServer())
      .get("/api/unknown-route")
      .set("x-request-id", "not a valid id!");

    expect(response.status).toBe(404);
    const body = response.body as { error: { code: string }; requestId: string };
    expect(body.error.code).toBe("not_found");
    expect(body.requestId).toMatch(/^[0-9a-f-]{36}$/);
    expect(response.headers["x-request-id"]).toBe(body.requestId);
  });
});
