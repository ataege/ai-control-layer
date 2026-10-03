#!/usr/bin/env node
// Database-backed tests (`pnpm test:db [gateway|api]`): runs the tests that need a real
// PostgreSQL against the database configured in the root .env. Both sides by default.
//
// Conventions (README.md, "Testing and verification", `pnpm test:db`):
// - Go: a test that needs the database skips visibly when none is configured, so it shows as
//   skipped in `pnpm test`. This command runs `go test ./...` once without database settings to
//   find those tests, then again with the database, where they must pass.
// - API: test files are named `*.db-spec.ts`, which the API's own vitest config does not include.
// `pnpm test` and `pnpm verify` therefore stay free of a database.
// Both sides receive the POSTGRES_* settings and TEST_DATABASE_REQUIRED=1.
//
// Honest results: an unreachable database, a missing toolchain or a failing test is FAIL;
// a skipped test, or a side with no database-backed tests, is SKIPPED. Anything other than
// PASS on every selected side exits nonzero, so a skip is never reported as a pass.
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { connect } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { commandExists, runCommand } from "./lib/commands.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { countByStatus, printHeading, printResultTable } from "./lib/output.mjs";
import { fromRepositoryRoot, repositoryRoot } from "./lib/repo-root.mjs";

const GATEWAY_DIRECTORY = fromRepositoryRoot("services", "gateway");
const VITEST_CONFIG_PATH = fromRepositoryRoot("scripts", "vitest.db.config.mjs");
// Set for every test this command runs: a database test can treat a missing database as a failure.
const DATABASE_REQUIRED_VARIABLE = "TEST_DATABASE_REQUIRED";
const DATABASE_CONNECT_TIMEOUT_MS = 3_000;
const SIDES = ["gateway", "api"];

/** Resolves true when a TCP connection to the database address opens in time. */
function databaseIsReachable(host, port) {
  return new Promise((resolveReachable) => {
    const socket = connect({ host, port, timeout: DATABASE_CONNECT_TIMEOUT_MS });
    const finish = (reachable) => {
      socket.destroy();
      resolveReachable(reachable);
    };
    socket.once("connect", () => finish(true));
    socket.once("timeout", () => finish(false));
    socket.once("error", () => finish(false));
  });
}

/** Runs a command, echoing its stdout line by line through `onStdoutLine`; resolves the exit code. */
function runWithLineOutput(command, commandArguments, { env, cwd, onStdoutLine }) {
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
function withoutDatabaseSettings(environment) {
  return Object.fromEntries(
    Object.entries(environment).filter(
      ([name]) => !name.startsWith("POSTGRES_") && name !== DATABASE_REQUIRED_VARIABLE,
    ),
  );
}

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

const requestedSides = process.argv.slice(2);
const unknownSides = requestedSides.filter((side) => !SIDES.includes(side));
if (unknownSides.length > 0) {
  console.error(`Usage: pnpm test:db [${SIDES.join("|")}] (unknown: ${unknownSides.join(", ")})`);
  process.exit(2);
}
const selectedSides = requestedSides.length > 0 ? requestedSides : SIDES;

const { fileFound, environment: rootEnvironment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}
const environment = { ...rootEnvironment, [DATABASE_REQUIRED_VARIABLE]: "1" };

const databaseHost = environment.POSTGRES_HOST || "localhost";
const databasePort = Number(environment.POSTGRES_PORT || 5432);
const databaseAddress = `${databaseHost}:${databasePort}`;
const results = [];

if (await databaseIsReachable(databaseHost, databasePort)) {
  results.push({ name: "database", status: "PASS", detail: `${databaseAddress} reachable` });
  for (const side of selectedSides) {
    printHeading(`test:db ${side}`);
    results.push({ name: side, ...(await SIDE_RUNNERS[side](environment)) });
  }
} else {
  results.push({
    name: "database",
    status: "FAIL",
    detail: `cannot connect to ${databaseAddress}; start PostgreSQL (pnpm infra:up) or fix POSTGRES_HOST/POSTGRES_PORT`,
  });
  for (const side of selectedSides) {
    results.push({ name: side, status: "SKIPPED", detail: "not run: database unreachable" });
  }
}

printHeading("Database test summary");
printResultTable("CHECK", results);
const counts = countByStatus(results);
console.log(`\n${counts.PASS} passed, ${counts.FAIL} failed, ${counts.SKIPPED} skipped`);
process.exit(counts.FAIL === 0 && counts.SKIPPED === 0 ? 0 : 1);
