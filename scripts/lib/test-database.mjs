// Shared preparation of the dedicated test database, used by `pnpm test:db` and
// `pnpm verify:controls`. The test database is `<POSTGRES_DB>_test` on the same loopback server;
// the demo database named in .env is never written. Nothing here runs at application startup.
import { spawn } from "node:child_process";

import { runCommand } from "./commands.mjs";
import { connectToDatabase } from "./demo-seed.mjs";
import { printHeading } from "./output.mjs";
import { repositoryRoot } from "./repo-root.mjs";

// Set for every test run against the test database: a database test can treat a missing database
// as a failure.
export const DATABASE_REQUIRED_VARIABLE = "TEST_DATABASE_REQUIRED";
// Legacy connection URLs (GO-06 once read GATEWAY_TEST_DATABASE_URL first; internal/testdb now
// reads only POSTGRES_*). Removed from every run, so none can bypass the identification run or
// point a test at another database.
export const DATABASE_URL_VARIABLES = ["GATEWAY_TEST_DATABASE_URL"];
export const FRESH_FLAG = "--fresh";
const TEST_DATABASE_SUFFIX = "_test";
// The server's maintenance database, used only to create and drop the test database.
const MAINTENANCE_DATABASE = "postgres";
// PostgreSQL silently truncates longer identifiers (NAMEDATALEN - 1).
const POSTGRES_IDENTIFIER_MAX_BYTES = 63;
const DUPLICATE_DATABASE_ERROR_CODE = "42P04";

/** Runs a command, echoing its stdout line by line through `onStdoutLine`; resolves the exit code. */
export function runWithLineOutput(command, commandArguments, { env, cwd, onStdoutLine }) {
  return new Promise((resolveExitCode) => {
    const child = spawn(command, commandArguments, {
      env,
      cwd,
      stdio: ["ignore", "pipe", "inherit"],
    });
    let pendingText = "";
    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk) => {
      pendingText += chunk;
      const completeLines = pendingText.split("\n");
      pendingText = completeLines.pop();
      for (const line of completeLines) onStdoutLine(line);
    });
    child.on("error", (spawnError) => {
      console.error(`Cannot run "${command}": ${spawnError.message}`);
      resolveExitCode(127);
    });
    child.on("close", (exitCode) => {
      if (pendingText) onStdoutLine(pendingText);
      resolveExitCode(exitCode ?? 1);
    });
  });
}

/** The environment without any database setting, as a plain `pnpm test` without .env sees it. */
export function withoutDatabaseSettings(environment) {
  return Object.fromEntries(
    Object.entries(environment).filter(
      ([name]) =>
        !name.startsWith("POSTGRES_") &&
        name !== DATABASE_REQUIRED_VARIABLE &&
        !DATABASE_URL_VARIABLES.includes(name),
    ),
  );
}

/**
 * The test database name for a demo database name, or a problem that makes the command refuse.
 * The name is derived, never configured, so it cannot be set to the demo database itself.
 */
export function testDatabaseNameFor(demoDatabaseName) {
  if (!demoDatabaseName) return { problem: "POSTGRES_DB is not set" };
  const testDatabaseName = `${demoDatabaseName}${TEST_DATABASE_SUFFIX}`;
  if (testDatabaseName === demoDatabaseName) {
    return { problem: "the test database name equals POSTGRES_DB" };
  }
  if (Buffer.byteLength(testDatabaseName, "utf8") > POSTGRES_IDENTIFIER_MAX_BYTES) {
    return {
      problem: `"${testDatabaseName}" exceeds PostgreSQL's ${POSTGRES_IDENTIFIER_MAX_BYTES}-byte identifier limit and could be truncated onto another database; shorten POSTGRES_DB`,
    };
  }
  return { testDatabaseName };
}

/** A double-quoted SQL identifier; embedded double quotes are doubled. */
const quoteIdentifier = (identifier) => `"${identifier.replaceAll('"', '""')}"`;

/**
 * Creates the test database when missing (after dropping it with `fresh`), connected to the
 * maintenance database as the owner. Resolves { ok, created } or { ok: false, detail }.
 */
async function ensureTestDatabase(environment, testDatabaseName, { fresh }) {
  let maintenanceClient;
  try {
    maintenanceClient = await connectToDatabase({
      ...environment,
      POSTGRES_DB: MAINTENANCE_DATABASE,
    });
  } catch (connectionError) {
    return {
      ok: false,
      detail: `cannot connect to the ${MAINTENANCE_DATABASE} maintenance database as POSTGRES_USER: ${connectionError.message}`,
    };
  }
  try {
    const quotedName = quoteIdentifier(testDatabaseName);
    if (fresh) {
      // FORCE ends leftover sessions, for example of an interrupted earlier run.
      await maintenanceClient.query(`DROP DATABASE IF EXISTS ${quotedName} WITH (FORCE)`);
      console.log(`Dropped the test database ${testDatabaseName} (${FRESH_FLAG}).`);
    }
    const existing = await maintenanceClient.query("SELECT 1 FROM pg_database WHERE datname = $1", [
      testDatabaseName,
    ]);
    if (existing.rowCount > 0) {
      console.log(`The test database ${testDatabaseName} exists.`);
      return { ok: true, created: false };
    }
    try {
      await maintenanceClient.query(`CREATE DATABASE ${quotedName}`);
    } catch (createError) {
      // Created concurrently by another run: it exists, which is all this step needs.
      if (createError.code !== DUPLICATE_DATABASE_ERROR_CODE) throw createError;
    }
    console.log(`Created the test database ${testDatabaseName}.`);
    return { ok: true, created: true };
  } catch (queryError) {
    return {
      ok: false,
      detail: `preparing the test database ${testDatabaseName} failed: ${queryError.message}`,
    };
  } finally {
    await maintenanceClient.end();
  }
}

/**
 * Brings the test database to the current schema and the demo baseline: create (or recreate),
 * then `pnpm db:migration:run` and `pnpm db:seed` (demo records and control catalog), both with
 * POSTGRES_DB set to the test database. Resolves { status, detail } for the summary.
 */
export async function prepareTestDatabase(testEnvironment, testDatabaseName, { fresh }) {
  printHeading(`test:db prepare ${testDatabaseName}`);
  const ensured = await ensureTestDatabase(testEnvironment, testDatabaseName, { fresh });
  if (!ensured.ok) return { status: "FAIL", detail: ensured.detail };

  for (const scriptName of ["db:migration:run", "db:seed"]) {
    console.log(`Running pnpm ${scriptName} against ${testDatabaseName} ...`);
    const exitCode = await runCommand("pnpm", ["run", scriptName], {
      env: testEnvironment,
      cwd: repositoryRoot,
    });
    if (exitCode !== 0) {
      return {
        status: "FAIL",
        detail: `pnpm ${scriptName} on ${testDatabaseName} failed (exit code ${exitCode})`,
      };
    }
  }
  const preparation = fresh ? "recreated" : ensured.created ? "created" : "reused";
  return { status: "PASS", detail: `${testDatabaseName} ${preparation}, migrated and seeded` };
}

/**
 * The environment every child process of a test run sees: POSTGRES_DB set to the test database,
 * TEST_DATABASE_REQUIRED=1 and no legacy URL variable. Returns { testEnvironment, testDatabaseName }
 * or { problem } when the name is unsafe.
 */
export function testEnvironmentFor(rootEnvironment) {
  const { testDatabaseName, problem } = testDatabaseNameFor(rootEnvironment.POSTGRES_DB);
  if (problem) return { problem };
  const testEnvironment = {
    ...rootEnvironment,
    POSTGRES_DB: testDatabaseName,
    [DATABASE_REQUIRED_VARIABLE]: "1",
  };
  for (const urlVariable of DATABASE_URL_VARIABLES) delete testEnvironment[urlVariable];
  return { testEnvironment, testDatabaseName };
}
