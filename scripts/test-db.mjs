#!/usr/bin/env node
// Database-backed tests (`pnpm test:db [gateway|api] [--fresh]`): runs the tests that need a real
// PostgreSQL against a dedicated test database, never against the demo database named in .env.
// Both sides by default.
//
// Test database: `<POSTGRES_DB>_test` on the same server (POSTGRES_HOST, POSTGRES_PORT) with the
// same owner credentials (POSTGRES_USER, POSTGRES_PASSWORD). The tests commit rows, some of them
// immutable (runtime.audit_events), so they must never reach the database the judges see. The
// command refuses a name equal to POSTGRES_DB or longer than PostgreSQL's 63-byte identifier limit
// (a truncated name could collapse onto the demo database), and a host that is not loopback.
// Before the tests, and only inside this explicit command (nothing runs at startup):
// 1. `--fresh` drops the test database first (DROP DATABASE ... WITH (FORCE)).
// 2. It creates the test database when missing, connected to the `postgres` maintenance database.
// 3. `pnpm db:migration:run` and `pnpm db:seed` with POSTGRES_DB set to the test database (real
//    environment variables win over .env), so it has the current schema, the synthetic demo records
//    and an active control catalog, as the demo has. Both are idempotent, so reruns are safe.
// The demo database is never written; `pnpm reset:demo` remains the way to restore it.
//
// Conventions (README.md, "Testing and verification", `pnpm test:db`):
// - Go: a test that needs the database skips visibly when none is configured, so it shows as
//   skipped in `pnpm test`. This command runs `go test ./...` once without database settings to
//   find those tests, then again with the database, where they must pass.
// - API: test files are named `*.db-spec.ts`, which the API's own vitest config does not include.
// `pnpm test` and `pnpm verify` therefore stay free of a database.
// Both sides receive the POSTGRES_* settings, with POSTGRES_DB set to the test database, and
// TEST_DATABASE_REQUIRED=1; GATEWAY_TEST_DATABASE_URL is removed (the Go helper no longer reads it).
//
// Honest results: an unreachable database, a test database that cannot be prepared, a missing
// toolchain or a failing test is FAIL; a skipped test, or a side with no database-backed tests, is
// SKIPPED. Anything other than PASS on every selected side exits nonzero, so a skip is never
// reported as a pass.
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { commandExists, runCommand } from "./lib/commands.mjs";
import { databaseAddress, databaseIsReachable, hostIsLoopback } from "./lib/database-probe.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { countByStatus, printHeading, printResultTable } from "./lib/output.mjs";
import { fromRepositoryRoot, repositoryRoot } from "./lib/repo-root.mjs";
import {
  FRESH_FLAG,
  prepareTestDatabase,
  runWithLineOutput,
  testEnvironmentFor,
  withoutDatabaseSettings,
} from "./lib/test-database.mjs";

const GATEWAY_DIRECTORY = fromRepositoryRoot("services", "gateway");
const VITEST_CONFIG_PATH = fromRepositoryRoot("scripts", "vitest.db.config.mjs");
const SIDES = ["gateway", "api"];

/**
 * Runs `go test -count=1 -json ./...` in the gateway and returns the exit code and the final
 * outcome ("pass", "fail" or "skip") of every test, keyed by package and test name.
 * -count=1 disables the test cache: a cached pass must not hide a database that is down.
 */
async function runGoTests(environment, { echoOutput }) {
  const testOutcomes = new Map();
  const exitCode = await runWithLineOutput("go", ["test", "-count=1", "-json", "./..."], {
    env: environment,
    cwd: GATEWAY_DIRECTORY,
    onStdoutLine: (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        // Build errors and other non-JSON output are printed as they are.
        console.log(line);
        return;
      }
      if (echoOutput && event.Action === "output") process.stdout.write(event.Output);
      if (event.Test && ["pass", "fail", "skip"].includes(event.Action)) {
        testOutcomes.set(`${event.Package} ${event.Test}`, event.Action);
      }
    },
  });
  return { exitCode, testOutcomes };
}

const testsWithOutcome = (testOutcomes, outcome) =>
  [...testOutcomes].filter(([, testOutcome]) => testOutcome === outcome).map(([name]) => name);

async function runGatewayTests(environment) {
  if (!commandExists("go", { env: environment, versionArguments: ["version"] })) {
    return { status: "FAIL", detail: "Go toolchain not found on PATH" };
  }

  // 1. Without database settings, a test that needs the database skips; those skips name
  //    the database tests. Nothing is echoed: this run only identifies them.
  console.log("Identifying the database tests: go test without database settings ...");
  const identification = await runGoTests(withoutDatabaseSettings(environment), {
    echoOutput: false,
  });
  if (identification.exitCode !== 0) {
    return {
      status: "FAIL",
      detail: `go test without a database failed (exit code ${identification.exitCode}); run pnpm --filter gateway run test`,
    };
  }
  const databaseTests = testsWithOutcome(identification.testOutcomes, "skip");
  if (databaseTests.length === 0) {
    return { status: "SKIPPED", detail: "no Go test needs the database (none skips without one)" };
  }

  // 2. With the database: every test must pass and none may skip.
  console.log(`Running go test with the database (${databaseTests.length} database tests) ...`);
  const databaseRun = await runGoTests(environment, { echoOutput: true });
  const passedCount = testsWithOutcome(databaseRun.testOutcomes, "pass").length;
  const failedCount = testsWithOutcome(databaseRun.testOutcomes, "fail").length;
  const skippedTests = testsWithOutcome(databaseRun.testOutcomes, "skip");
  const countsText = `${passedCount} passed, ${failedCount} failed, ${skippedTests.length} skipped; ${databaseTests.length} need the database`;
  const databaseTestsNotPassed = databaseTests.filter(
    (testName) => databaseRun.testOutcomes.get(testName) !== "pass",
  );

  if (databaseRun.exitCode !== 0 || failedCount > 0) {
    return { status: "FAIL", detail: `go test exit code ${databaseRun.exitCode}; ${countsText}` };
  }
  if (skippedTests.length > 0) {
    return { status: "SKIPPED", detail: `${countsText}; skipped: ${skippedTests.join(", ")}` };
  }
  if (databaseTestsNotPassed.length > 0) {
    return { status: "FAIL", detail: `did not pass: ${databaseTestsNotPassed.join(", ")}` };
  }
  return { status: "PASS", detail: countsText };
}

async function runApiTests(environment) {
  // The API imports built workspace packages (for example @workspace/contracts), as `pnpm test`
  // gets them through turbo's ^build; build them first so a clean checkout works.
  const dependencyBuildExitCode = await runCommand(
    "pnpm",
    ["exec", "turbo", "run", "build", "--filter=api^...", "--output-logs=errors-only"],
    { cwd: repositoryRoot },
  );
  if (dependencyBuildExitCode !== 0) {
    return {
      status: "FAIL",
      detail: `building the API's workspace dependencies failed (exit code ${dependencyBuildExitCode})`,
    };
  }

  const reportDirectory = mkdtempSync(join(tmpdir(), "test-db-"));
  const reportPath = join(reportDirectory, "vitest-report.json");
  try {
    const exitCode = await runWithLineOutput(
      "pnpm",
      [
        "--filter",
        "api",
        "exec",
        "vitest",
        "run",
        "--config",
        VITEST_CONFIG_PATH,
        "--reporter=default",
        "--reporter=json",
        `--outputFile.json=${reportPath}`,
        // Exit 0 on an empty match; the report below turns it into SKIPPED, never PASS.
        "--passWithNoTests",
      ],
      { env: environment, cwd: repositoryRoot, onStdoutLine: (line) => console.log(line) },
    );

    let report = null;
    try {
      report = JSON.parse(readFileSync(reportPath, "utf8"));
    } catch {
      // No report: vitest did not start.
    }
    if (!report)
      return { status: "FAIL", detail: `vitest wrote no report (exit code ${exitCode})` };
    // No file matched: a missing side, not a pass. A file that failed to load has no tests
    // either, but appears in testResults and is a failure.
    if (report.testResults.length === 0) {
      return { status: "SKIPPED", detail: "no *.db-spec.ts test files in apps/api/src" };
    }
    if (report.numTotalTests === 0) {
      return { status: "FAIL", detail: `test files found but no test ran (exit code ${exitCode})` };
    }

    const skippedCount = report.numPendingTests + report.numTodoTests;
    const countsText = `${report.numPassedTests} passed, ${report.numFailedTests} failed, ${skippedCount} skipped`;
    if (exitCode !== 0 || report.numFailedTests > 0) {
      return { status: "FAIL", detail: `vitest exit code ${exitCode}; ${countsText}` };
    }
    if (skippedCount > 0) return { status: "SKIPPED", detail: countsText };
    return { status: "PASS", detail: countsText };
  } finally {
    rmSync(reportDirectory, { recursive: true, force: true });
  }
}

const SIDE_RUNNERS = { gateway: runGatewayTests, api: runApiTests };

const commandArguments = process.argv.slice(2);
const fresh = commandArguments.includes(FRESH_FLAG);
const requestedSides = commandArguments.filter((argument) => argument !== FRESH_FLAG);
const unknownSides = requestedSides.filter((side) => !SIDES.includes(side));
if (unknownSides.length > 0) {
  console.error(
    `Usage: pnpm test:db [${SIDES.join("|")}] [${FRESH_FLAG}] (unknown: ${unknownSides.join(", ")})`,
  );
  process.exit(2);
}
const selectedSides = requestedSides.length > 0 ? requestedSides : SIDES;

const { fileFound, environment: rootEnvironment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}

const demoDatabaseName = rootEnvironment.POSTGRES_DB;
// Every child process (migrations, seed, both test sides) sees only the test database. The URL
// variables are removed, so a stale value can never point a test at the demo database.
const {
  testEnvironment,
  testDatabaseName,
  problem: testDatabaseNameProblem,
} = testEnvironmentFor(rootEnvironment);
if (testDatabaseNameProblem) {
  console.error(`Refusing to run the database tests: ${testDatabaseNameProblem}.`);
  process.exit(1);
}

const database = databaseAddress(testEnvironment);
// Names and address only, never the password.
console.log(
  `Test database: ${testDatabaseName} on ${database.label} (the demo database ${demoDatabaseName} is not used)`,
);
const results = [];
const notRun = (reason) =>
  selectedSides.map((side) => ({ name: side, status: "SKIPPED", detail: `not run: ${reason}` }));

if (!(await hostIsLoopback(database.host))) {
  // --fresh drops a database and the seed writes one: only on this machine.
  results.push({
    name: "database",
    status: "FAIL",
    detail: `refusing ${database.label}: POSTGRES_HOST must resolve only to a loopback address (localhost, 127.0.0.1, ::1)`,
  });
  results.push(...notRun("database host is not loopback"));
} else if (!(await databaseIsReachable(database))) {
  results.push({
    name: "database",
    status: "FAIL",
    detail: `cannot connect to ${database.label}; start PostgreSQL (pnpm infra:up) or fix POSTGRES_HOST/POSTGRES_PORT`,
  });
  results.push(...notRun("database unreachable"));
} else {
  results.push({ name: "database", status: "PASS", detail: `${database.label} reachable` });
  const preparation = await prepareTestDatabase(testEnvironment, testDatabaseName, { fresh });
  results.push({ name: "test database", ...preparation });
  if (preparation.status === "PASS") {
    for (const side of selectedSides) {
      printHeading(`test:db ${side} (${testDatabaseName})`);
      results.push({ name: side, ...(await SIDE_RUNNERS[side](testEnvironment)) });
    }
  } else {
    results.push(...notRun("test database not prepared"));
  }
}

printHeading("Database test summary");
printResultTable("CHECK", results);
const counts = countByStatus(results);
console.log(`\n${counts.PASS} passed, ${counts.FAIL} failed, ${counts.SKIPPED} skipped`);
process.exit(counts.FAIL === 0 && counts.SKIPPED === 0 ? 0 : 1);
