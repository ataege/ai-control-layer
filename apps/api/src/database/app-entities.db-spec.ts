// Database-backed checks of the NestJS-owned `app` entities (API-04, API-05, X-24), run by
// `pnpm test:db api` only. Each run creates its own temporary database, applies every migration
// and drops the database afterwards, so the configured development database is never touched.
import "reflect-metadata";
import { randomUUID } from "node:crypto";
import { readdirSync } from "node:fs";
import { join } from "node:path";
import { DataSource, QueryFailedError, type MigrationInterface } from "typeorm";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "./typeorm-options.js";

const databaseEnvironment = parseEnvironment(databaseEnvironmentSchema, process.env);
const temporaryDatabaseName = `app_entities_test_${randomUUID().replaceAll("-", "")}`;

let administrationDataSource: DataSource;
let dataSource: DataSource;

type MigrationClass = new () => MigrationInterface;

/** Every migration class in the migrations folder, loaded the way the CLI would find them. */
async function loadAllMigrations(): Promise<MigrationClass[]> {
  const migrationsDirectory = join(import.meta.dirname, "migrations");
  const migrationFiles = readdirSync(migrationsDirectory)
    .filter((fileName) => fileName.endsWith(".ts"))
    .sort();
  const migrations: MigrationClass[] = [];
  for (const fileName of migrationFiles) {
    const migrationModule = (await import(`./migrations/${fileName}`)) as Record<string, unknown>;
    migrations.push(...(Object.values(migrationModule) as MigrationClass[]));
  }
  return migrations;
}

/** Table and extension names of the temporary database, to compare before and after startup. */
async function listSchemaObjects(connection: DataSource) {
  const tables = await connection.query<{ name: string }[]>(
    `SELECT schemaname || '.' || tablename AS name FROM pg_tables
     WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY 1`,
  );
  const extensions = await connection.query<{ name: string }[]>(
    `SELECT extname AS name FROM pg_extension ORDER BY 1`,
  );
  return { tables: tables.map((row) => row.name), extensions: extensions.map((row) => row.name) };
}

/** Runs an INSERT ... RETURNING id and returns that id. */
async function insertReturningId(sql: string, parameters: unknown[]): Promise<string> {
  const rows = await dataSource.query<{ id: string }[]>(sql, parameters);
  if (!rows[0]) throw new Error("the INSERT returned no row");
  return rows[0].id;
}

async function expectRejected(sql: string, parameters: unknown[], expectedMessage: string) {
  const failure = await dataSource.query(sql, parameters).then(
    () => null,
    (error: unknown) => error,
  );
  expect(failure, `expected a rejection mentioning ${expectedMessage}`).toBeInstanceOf(
    QueryFailedError,
  );
  expect((failure as QueryFailedError).message).toContain(expectedMessage);
}

beforeAll(async () => {
  administrationDataSource = new DataSource(buildTypeOrmOptions(databaseEnvironment));
  await administrationDataSource.initialize();
  await administrationDataSource.query(`CREATE DATABASE ${temporaryDatabaseName}`);
});

afterAll(async () => {
  if (dataSource?.isInitialized) await dataSource.destroy();
  if (administrationDataSource?.isInitialized) {
    await administrationDataSource.query(
      `DROP DATABASE IF EXISTS ${temporaryDatabaseName} WITH (FORCE)`,
    );
    await administrationDataSource.destroy();
  }
});

describe("API startup against a fresh database (API-04, API-05)", () => {
  it("initializing the data source creates no table and no extension", async () => {
    const options = buildTypeOrmOptions({
      ...databaseEnvironment,
      POSTGRES_DB: temporaryDatabaseName,
    });
    const connection = new DataSource(options);
    await connection.initialize();
    try {
      const before = await listSchemaObjects(connection);
      // Initialization is what the API does at startup; it must not change the schema.
      expect(before.tables).toEqual([]);
      expect(before.extensions).toEqual(["plpgsql"]);
    } finally {
      await connection.destroy();
    }

    const inspector = new DataSource(options);
    await inspector.initialize();
    try {
      expect(await listSchemaObjects(inspector)).toEqual({ tables: [], extensions: ["plpgsql"] });
    } finally {
      await inspector.destroy();
    }
  });
});

describe("app entities after the migrations", () => {
  beforeAll(async () => {
    dataSource = new DataSource({
      ...buildTypeOrmOptions({ ...databaseEnvironment, POSTGRES_DB: temporaryDatabaseName }),
      migrations: await loadAllMigrations(),
    });
    await dataSource.initialize();
    await dataSource.runMigrations();
  });

  it("agree with the migrated schema: the entity diff is empty", async () => {
    const pendingStatements = await dataSource.driver.createSchemaBuilder().log();
    expect(pendingStatements.upQueries.map((query) => query.query)).toEqual([]);
  });

  it("start with the same extensions as before: migrations add none", async () => {
    const { extensions } = await listSchemaObjects(dataSource);
    expect(extensions).toEqual(["plpgsql"]);
  });

  it("refuse a second membership of the same user in the same organization", async () => {
    const organizationId = await insertReturningId(
      `INSERT INTO app.organizations (name) VALUES ($1) RETURNING id`,
      [`org-${randomUUID()}`],
    );
    const userId = await insertReturningId(
      `INSERT INTO app.users (email, name) VALUES ($1, 'Test User') RETURNING id`,
      [`user-${randomUUID()}@example.test`],
    );
    await dataSource.query(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{operator}')`,
      [userId, organizationId],
    );
    await expectRejected(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{reviewer}')`,
      [userId, organizationId],
      "UQ_",
    );
  });

  it("refuse a membership of a user or organization that does not exist", async () => {
    await expectRejected(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{}')`,
      [randomUUID(), randomUUID()],
      "FK_",
    );
  });

  it.each([
    ["task_templates", "FK_task_org", `(organization_id, name) VALUES ($1, 'template')`],
    ["policy_versions", "FK_policy_org", `(organization_id, name, rules) VALUES ($1, 'v1', '{}')`],
    [
      "tool_definitions",
      "FK_tool_org",
      `(organization_id, name, schema) VALUES ($1, 'read_invoice', '{}')`,
    ],
  ])(
    "refuse a %s row of an organization that does not exist",
    async (table, constraint, columns) => {
      await expectRejected(`INSERT INTO app.${table} ${columns}`, [randomUUID()], constraint);
    },
  );
});
