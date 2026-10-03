import { join } from "node:path";
import type { DataSourceOptions } from "typeorm";
import type { DatabaseEnvironment } from "../config/environment.js";
import { ControlCatalogPointer } from "../policies/control-catalog-pointer.entity.js";
import { ControlCatalogRevision } from "../policies/control-catalog-revision.entity.js";
import { SignatureFeedRevision } from "../policies/signature-feed-revision.entity.js";

// This file runs as .ts (TypeORM CLI loader, tests) or .js (compiled app); load only the matching copies.
const migrationFileExtension = import.meta.filename.endsWith(".ts") ? "ts" : "js";

/** Single source of TypeORM options for the Nest runtime and the TypeORM CLI. */
export function buildTypeOrmOptions(databaseEnvironment: DatabaseEnvironment): DataSourceOptions {
  return {
    type: "postgres",
    host: databaseEnvironment.POSTGRES_HOST,
    port: databaseEnvironment.POSTGRES_PORT,
    username: databaseEnvironment.POSTGRES_USER,
    password: databaseEnvironment.POSTGRES_PASSWORD,
    database: databaseEnvironment.POSTGRES_DB,
    // Every NestJS-owned `app` entity; runtime and demo tables have hand-written migrations only.
    entities: [ControlCatalogRevision, SignatureFeedRevision, ControlCatalogPointer],
    migrations: [join(import.meta.dirname, "migrations", `*.${migrationFileExtension}`)],
    // Schema changes happen only through explicit migration commands.
    synchronize: false,
    migrationsRun: false,
    dropSchema: false,
    connectTimeoutMS: databaseEnvironment.DATABASE_TIMEOUT_MS,
  };
}
