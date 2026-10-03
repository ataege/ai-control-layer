import type { INestApplication } from "@nestjs/common";
import { DocumentBuilder, SwaggerModule } from "@nestjs/swagger";
import { GLOBAL_PREFIX } from "./app.setup.js";

/** Serves Swagger UI at /api/docs and the OpenAPI document at /api/docs-json. */
export function setupOpenApi(app: INestApplication): void {
  const documentConfig = new DocumentBuilder()
    .setTitle("Starter API")
    .setDescription("Health and diagnostics endpoints of the starter.")
    .setVersion("0.0.0")
    .build();
  const document = SwaggerModule.createDocument(app, documentConfig);
  SwaggerModule.setup(`${GLOBAL_PREFIX}/docs`, app, document, {
    jsonDocumentUrl: `${GLOBAL_PREFIX}/docs-json`,
  });
}
