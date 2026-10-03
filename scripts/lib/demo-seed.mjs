// Loads the synthetic demo records (fixtures/demo-records.json) into the draft `demo` tables.
// Shared by `pnpm db:seed` and `pnpm reset:demo`. DRAFT: the tables are the unapproved SH-17
// migration. Never run at startup.
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import { fromRepositoryRoot } from "./repo-root.mjs";

const DEMO_RECORDS_PATH = fromRepositoryRoot("fixtures", "demo-records.json");
// The API's own `pg` client, so these scripts add no dependency and use the API's version.
const requireFromApi = createRequire(fromRepositoryRoot("apps", "api", "package.json"));

/** A seeded row that already exists with other values; reseeding never overwrites it. */
export class SeedDriftError extends Error {}

export function loadDemoRecords() {
  return JSON.parse(readFileSync(DEMO_RECORDS_PATH, "utf8"));
}

/** Opens a connection to the database configured in the environment. */
export async function connectToDatabase(environment) {
  const { Client } = requireFromApi("pg");
  const client = new Client({
    host: environment.POSTGRES_HOST || "localhost",
    port: Number(environment.POSTGRES_PORT || 5432),
    user: environment.POSTGRES_USER,
    password: environment.POSTGRES_PASSWORD,
    database: environment.POSTGRES_DB,
    connectionTimeoutMillis: 5_000,
  });
  await client.connect();
  return client;
}

/** The base tables of the given schemas, as quoted "schema"."table" names. */
export async function listTables(client, schemaNames) {
  const result = await client.query(
    `SELECT format('%I.%I', table_schema, table_name) AS qualified_name
       FROM information_schema.tables
      WHERE table_schema = ANY($1) AND table_type = 'BASE TABLE'
      ORDER BY table_schema, table_name`,
    [schemaNames],
  );
  return result.rows.map((row) => row.qualified_name);
}

/** Row count per table, for the before and after report. */
export async function countRows(client, qualifiedTableNames) {
  const counts = {};
  for (const tableName of qualifiedTableNames) {
    const result = await client.query(`SELECT count(*)::int AS row_count FROM ${tableName}`);
    counts[tableName] = result.rows[0].row_count;
  }
  return counts;
}

/** Fails with a clear message when the demo tables were not migrated. */
export async function requireDemoTables(client) {
  const result = await client.query(
    `SELECT to_regclass('demo.vendors') IS NOT NULL AND to_regclass('demo.invoices') IS NOT NULL AS present`,
  );
  if (!result.rows[0].present) {
    throw new Error("the demo tables do not exist; run `pnpm db:migration:run` first");
  }
}

const VENDOR_COLUMNS = ["id", "organization_id", "version", "name"];
const INVOICE_COLUMNS = [
  "id",
  "organization_id",
  "vendor_id",
  "version",
  "external_reference",
  "currency",
  "total_minor_units",
  "issued_on",
  "due_on",
  "internal_note",
];
// Read back as text where pg would otherwise return another type (dates, bigint, char).
const COLUMN_READ_EXPRESSIONS = {
  organization_id: "organization_id::text",
  total_minor_units: "total_minor_units::text",
  issued_on: "issued_on::text",
  due_on: "due_on::text",
  currency: "currency::text",
};
const asComparableText = (value) => (value === null ? null : String(value));

/**
 * Inserts the missing rows of one table and checks that every fixture row now exists with the
 * fixture's values. Returns { inserted, alreadyPresent }; throws SeedDriftError on a difference.
 */
async function seedTable(client, tableName, columns, records) {
  const placeholders = columns.map((_, index) => `$${index + 1}`).join(", ");
  let inserted = 0;
  for (const record of records) {
    const result = await client.query(
      `INSERT INTO ${tableName} (${columns.join(", ")}) VALUES (${placeholders})
       ON CONFLICT (id) DO NOTHING`,
      columns.map((column) => record[column]),
    );
    inserted += result.rowCount;
  }

  const readColumns = columns.map(
    (column) => `${COLUMN_READ_EXPRESSIONS[column] ?? column} AS ${column}`,
  );
  const stored = await client.query(
    `SELECT ${readColumns.join(", ")} FROM ${tableName} WHERE id = ANY($1)`,
    [records.map((record) => record.id)],
  );
  const storedById = new Map(stored.rows.map((row) => [row.id, row]));
  const drifted = records.filter((record) => {
    const row = storedById.get(record.id);
    return (
      !row ||
      columns.some((column) => asComparableText(row[column]) !== asComparableText(record[column]))
    );
  });
  if (drifted.length > 0) {
    throw new SeedDriftError(
      `${tableName}: ${drifted.map((record) => record.id).join(", ")} already exist with other values; nothing was changed (run pnpm reset:demo to restore the fixtures)`,
    );
  }
  return { inserted, alreadyPresent: records.length - inserted };
}

/** Seeds vendors, then invoices. The caller owns the transaction. */
export async function seedDemoRecords(client, records = loadDemoRecords()) {
  return {
    vendors: await seedTable(client, "demo.vendors", VENDOR_COLUMNS, records.vendors),
    invoices: await seedTable(client, "demo.invoices", INVOICE_COLUMNS, records.invoices),
  };
}
