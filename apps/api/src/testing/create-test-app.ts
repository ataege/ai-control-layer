import type { ModuleMetadata } from "@nestjs/common";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { Test } from "@nestjs/testing";
import { configureApp } from "../app.setup.js";
import { DataSource } from "typeorm";
import { APP_GUARD } from "@nestjs/core";
import { DefaultDenyGuard } from "../auth/default-deny.guard.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { getRepositoryToken } from "@nestjs/typeorm";
import { Membership } from "../identity/entities/membership.entity.js";

/** Boots a Nest app from the given module metadata with the same HTTP wiring as main.ts. */
export async function createTestApp(
  metadata: ModuleMetadata,
  options: { mockAuth?: boolean } = { mockAuth: true }
): Promise<NestExpressApplication> {
  const providers = [
    { provide: DataSource, useValue: { isInitialized: true } },
    { provide: APP_GUARD, useClass: DefaultDenyGuard },
    ...(metadata.providers || []),
  ];

  if (options.mockAuth) {
    providers.push(
      { provide: AUTH_PROVIDER, useValue: { authenticate: async () => ({ subjectId: "test-user" }) } },
      { provide: getRepositoryToken(Membership), useValue: { findOne: async () => ({ userId: "test-user", organizationId: "test-org", roles: [] }) } }
    );
  }

  const moduleRef = await Test.createTestingModule({
    ...metadata,
    imports: [...(metadata.imports || [])],
    providers,
  }).compile();
  
  const app = moduleRef.createNestApplication<NestExpressApplication>({ logger: false });
  configureApp(app, ["http://localhost:3000"]);
  await app.init();
  return app;
}
