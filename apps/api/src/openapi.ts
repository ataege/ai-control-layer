import type { INestApplication } from "@nestjs/common";
import { DocumentBuilder, SwaggerModule } from "@nestjs/swagger";
import { GLOBAL_PREFIX } from "./app.setup.js";

/**
 * Serves Swagger UI at /api/docs and the OpenAPI document at /api/docs-json. Both are mounted on the
 * HTTP adapter, outside the global DefaultDenyGuard: a deliberate development-only exposure (smoke
 * asserts docs-json 200). The document describes routes, never data; product routes stay guarded.
 */
export function setupOpenApi(app: INestApplication): void {
  const documentConfig = new DocumentBuilder()
    .setTitle("Task Passport API")
    .setDescription("Internal API for Task Passport operators and automated tests.")
    .setVersion("1.0")
    .addCookieAuth("session")
    .build();
  const document = SwaggerModule.createDocument(app, documentConfig);
  SwaggerModule.setup(`${GLOBAL_PREFIX}/docs`, app, document, {
    jsonDocumentUrl: `${GLOBAL_PREFIX}/docs-json`,
  });
}
