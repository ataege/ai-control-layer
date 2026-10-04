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
import { DatabaseInitializerService } from "./database-initializer.service.js";
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

/** The name of the one constraint of the given type on a table, read from pg_constraint. */
async function constraintName(
  table: string,
  type: "f" | "u",
  referencedTable?: string,
): Promise<string> {
  const rows = await dataSource.query<{ conname: string }[]>(
    `SELECT conname FROM pg_constraint
     WHERE conrelid = $1::regclass AND contype = $2
       AND ($3::text IS NULL OR confrelid = $3::regclass)`,
    [table, type, referencedTable ?? null],
  );
  if (rows.length !== 1) throw new Error(`expected one ${type} constraint on ${table}`);
  return rows[0]!.conname;
}

/** Asserts the statement fails and the constraint that fired is the expected one (by name). */
async function expectConstraintViolation(
  sql: string,
  parameters: unknown[],
  expectedConstraint: string,
  expectedCode: "23503" | "23505",
) {
  const failure = await dataSource.query(sql, parameters).then(
    () => null,
    (error: unknown) => error,
  );
  expect(failure, `expected ${expectedConstraint} to refuse the statement`).toBeInstanceOf(
    QueryFailedError,
  );
  const driverError = (failure as QueryFailedError).driverError as {
    constraint?: string;
    code?: string;
  };
  expect(driverError.constraint).toBe(expectedConstraint);
  expect(driverError.code).toBe(expectedCode);
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
  const freshOptions = () =>
    buildTypeOrmOptions({ ...databaseEnvironment, POSTGRES_DB: temporaryDatabaseName });

  it("the shared options factory turns every schema-changing switch off", () => {
    expect(freshOptions()).toMatchObject({
      synchronize: false,
      migrationsRun: false,
      dropSchema: false,
      installExtensions: false,
      uuidExtension: "pgcrypto",
    });
  });

  it("the API's database initializer connects to a fresh database and creates no table or extension", async () => {
    const connection = new DataSource(freshOptions());
    const initializer = new DatabaseInitializerService(connection);
    initializer.onApplicationBootstrap();
    try {
      // The initializer connects in the background, as at startup; wait for it.
      for (let attempt = 0; attempt < 100 && !connection.isInitialized; attempt += 1) {
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      expect(connection.isInitialized, "the initializer did not connect").toBe(true);
      expect(await listSchemaObjects(connection)).toEqual({
        tables: [],
        extensions: ["plpgsql"],
      });
    } finally {
      await initializer.onApplicationShutdown();
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

  async function createUser(): Promise<string> {
    return insertReturningId(
      `INSERT INTO app.users (email, name) VALUES ($1, 'Test User') RETURNING id`,
      [`user-${randomUUID()}@example.test`],
    );
  }
  async function createOrganization(): Promise<string> {
    return insertReturningId(`INSERT INTO app.organizations (name) VALUES ($1) RETURNING id`, [
      `org-${randomUUID()}`,
    ]);
  }
  const insertMembership = (userId: string, organizationId: string, role: string) =>
    dataSource.query(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, $3)`,
      [userId, organizationId, `{${role}}`],
    );

  it("refuse a second membership of the same user in the same organization", async () => {
    const organizationId = await createOrganization();
    const userId = await createUser();
    await insertMembership(userId, organizationId, "operator");
    await expectConstraintViolation(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{reviewer}')`,
      [userId, organizationId],
      await constraintName("app.memberships", "u"),
      "23505",
    );
  });

  it("accept the same user in a second organization: the constraint is per pair, not per user", async () => {
    const userId = await createUser();
    await insertMembership(userId, await createOrganization(), "operator");
    await insertMembership(userId, await createOrganization(), "operator");
    const countRows = await dataSource.query<{ count: string }[]>(
      `SELECT count(*) FROM app.memberships WHERE "userId" = $1`,
      [userId],
    );
    expect(countRows[0]?.count).toBe("2");
  });

  it("refuse a membership of a missing organization for an existing user", async () => {
    await expectConstraintViolation(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{}')`,
      [await createUser(), randomUUID()],
      await constraintName("app.memberships", "f", "app.organizations"),
      "23503",
    );
  });

  it("refuse a membership of a missing user in an existing organization", async () => {
    await expectConstraintViolation(
      `INSERT INTO app.memberships ("userId", "organizationId", roles) VALUES ($1, $2, '{}')`,
      [randomUUID(), await createOrganization()],
      await constraintName("app.memberships", "f", "app.users"),
      "23503",
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
      await expectConstraintViolation(
        `INSERT INTO app.${table} ${columns}`,
        [randomUUID()],
        constraint,
        "23503",
      );
    },
  );
});
