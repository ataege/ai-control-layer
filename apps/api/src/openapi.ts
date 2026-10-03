import type { INestApplication } from "@nestjs/common";
import { DocumentBuilder, SwaggerModule } from "@nestjs/swagger";
import { GLOBAL_PREFIX } from "./app.setup.js";

/** Serves Swagger UI at /api/docs and the OpenAPI document at /api/docs-json. */
export function setupOpenApi(app: INestApplication): void {
  const documentConfig = new DocumentBuilder()
    .setTitle("Task Passport API")
    .setDescription("Internal API for Task Passport operators and automated tests.")
    .setVersion("1.0")
    .addCookieAuth("sid")
    .build();
  const document = SwaggerModule.createDocument(app, documentConfig);
  SwaggerModule.setup(`${GLOBAL_PREFIX}/docs`, app, document, {
    jsonDocumentUrl: `${GLOBAL_PREFIX}/docs-json`,
  });
}
