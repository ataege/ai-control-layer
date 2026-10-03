import type { ModuleMetadata } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { Test } from "@nestjs/testing";
import { configureApp } from "../app.setup.js";
import { DataSource } from "typeorm";
import { APP_GUARD } from "@nestjs/core";
import { DefaultDenyGuard } from "../auth/default-deny.guard.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";

/** Boots a Nest app from the given module metadata with the same HTTP wiring as main.ts. */
export async function createTestApp(metadata: ModuleMetadata): Promise<NestExpressApplication> {
  const moduleRef = await Test.createTestingModule({
    ...metadata,
    imports: [...(metadata.imports || [])],
    providers: [
      { provide: DataSource, useValue: { isInitialized: true } },
      { provide: APP_GUARD, useClass: DefaultDenyGuard },
      { provide: AUTH_PROVIDER, useValue: { authenticate: async () => ({ subjectId: "test-user" }) } },
      ...(metadata.providers || []),
    ],
  }).compile();
  const app = moduleRef.createNestApplication<NestExpressApplication>({ logger: false });
  configureApp(app, ["http://localhost:3000"], "test-cookie-secret");
  await app.init();
  return app;
}
