import type { ModuleMetadata } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { Test } from "@nestjs/testing";
import { configureApp } from "../app.setup.js";

/** Boots a Nest app from the given module metadata with the same HTTP wiring as main.ts. */
export async function createTestApp(metadata: ModuleMetadata): Promise<NestExpressApplication> {
  const moduleRef = await Test.createTestingModule(metadata).compile();
  const app = moduleRef.createNestApplication<NestExpressApplication>({ logger: false });
  configureApp(app, ["http://localhost:3000"]);
  await app.init();
  return app;
}
