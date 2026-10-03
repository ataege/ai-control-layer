import { join } from "node:path";
import type { DataSourceOptions } from "typeorm";
import type { DatabaseEnvironment } from "../config/environment.js";

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
    // Register entity classes here once the first one exists.
    entities: [],
    migrations: [join(import.meta.dirname, "migrations", `*.${migrationFileExtension}`)],
    // Schema changes happen only through explicit migration commands.
    synchronize: false,
    migrationsRun: false,
    dropSchema: false,
    connectTimeoutMS: databaseEnvironment.DATABASE_TIMEOUT_MS,
  };
}
