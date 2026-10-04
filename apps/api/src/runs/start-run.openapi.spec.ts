import type { NestExpressApplication } from "@nestjs/platform-express";
import { DocumentBuilder, SwaggerModule, type OpenAPIObject } from "@nestjs/swagger";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { createTestApp } from "../testing/create-test-app.js";
import { RunsController } from "./runs.controller.js";

// API-11: POST /api/runs is documented with DTO classes that implement the X-07 contract types.
describe("OpenAPI document of POST /api/runs", () => {
  let app: NestExpressApplication;
  let document: OpenAPIObject;

  beforeAll(async () => {
    app = await createTestApp({
      controllers: [RunsController],
      providers: [{ provide: GatewayClientService, useValue: {} }],
    });
    document = SwaggerModule.createDocument(app, new DocumentBuilder().build());
  });

  afterAll(async () => {
    await app.close();
  });

  it("documents the request body with the X-07 fields and required properties", () => {
    const operation = document.paths["/api/runs"]?.post;
    const body = operation?.requestBody as
      { content: Record<string, { schema: { $ref: string } }> } | undefined;
    expect(body?.content["application/json"]?.schema.$ref).toBe(
      "#/components/schemas/StartRunRequestDto",
    );
    const schema = document.components?.schemas?.StartRunRequestDto as {
      required: string[];
      properties: Record<string, unknown>;
    };
    expect([...schema.required].sort()).toEqual(["destination", "invoiceIds", "template"]);
    expect(Object.keys(schema.properties).sort()).toEqual([
      "approvalRequirement",
      "destination",
      "invoiceIds",
      "limits",
      "template",
      "vendorId",
    ]);
  });

  it("documents the admitted answer and every failure status", () => {
    const responses = document.paths["/api/runs"]?.post?.responses ?? {};
    expect(Object.keys(responses).sort()).toEqual(["201", "400", "401", "503", "504"]);
    const created = responses["201"] as { content: Record<string, { schema: { $ref: string } }> };
    expect(created.content["application/json"]?.schema.$ref).toBe(
      "#/components/schemas/StartRunResponseDto",
    );
    const response = document.components?.schemas?.StartRunResponseDto as {
      required: string[];
    };
    expect([...response.required].sort()).toEqual(["passportId", "runId"]);
  });
});
