#!/usr/bin/env node
// The one-command control test suite (`pnpm verify:controls`, or `make verify-controls`), SH-47 and
// report 1.2 "Validation plan and evidence matrix", "One command test contract".
//
// 1. Preflight: Node, pnpm and Go versions, the root .env, PostgreSQL on loopback, and the local
//    model (Ollama with MODEL_NAME, default qwen3.5:4b, decision 6). Missing pieces print the fix.
// 2. Isolation: the dedicated test database `<POSTGRES_DB>_test`, recreated, migrated and seeded
//    through the same helpers as `pnpm test:db` (scripts/lib/test-database.mjs). The demo database
//    is never written. `--reuse` keeps the existing test database instead of recreating it.
// 3. Deterministic part: Go unit and database tests (`go test -json`), the API unit and `*.db-spec.ts`
//    tests (vitest JSON) and the fixture self-checks (node:test, JSON lines).
// 4. Live part, labelled "live model": the opt-in live semantic tests against the real local model.
//    A label mismatch is recorded as false_positive or false_negative and does not fail the suite
//    (lead's decision, 3 October 2026); `--strict-live` makes it fail. A guard failure fails. An
//    unavailable model makes the live part INCOMPLETE and the command exits nonzero, unless
//    `--no-live` is given; then the summary says "INCOMPLETE: live model cases not run".
// 5. Results: .verify-controls/results-<timestamp>.json (gitignored) with the commit, versions,
//    model and digest, and every case with its category, source, outcome and duration.
// Any failure exits nonzero; a skip is never a pass. VERIFY_CONTROLS_INJECT_FAILURE=1 makes one real
// Go test fail (TestSuiteInjectedFailure) to show that.
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync, existsSync } from "node:fs";
import { tmpdir, platform, arch } from "node:os";
import { join, relative } from "node:path";

import { readCommandOutput, runCommand } from "./lib/commands.mjs";
import { databaseAddress, databaseIsReachable, hostIsLoopback } from "./lib/database-probe.mjs";
import { loadRootEnvironment } from "./lib/env-file.mjs";
import { colorize, printHeading } from "./lib/output.mjs";
import { fromRepositoryRoot, repositoryRoot } from "./lib/repo-root.mjs";
import {
  prepareTestDatabase,
  runWithLineOutput,
  testEnvironmentFor,
} from "./lib/test-database.mjs";

const GATEWAY_DIRECTORY = fromRepositoryRoot("services", "gateway");
const VITEST_DB_CONFIG = fromRepositoryRoot("scripts", "vitest.db.config.mjs");
const NODE_REPORTER = fromRepositoryRoot("scripts", "lib", "node-test-json-reporter.mjs");
const RESULTS_DIRECTORY = fromRepositoryRoot(".verify-controls");
const DEFAULT_MODEL = "qwen3.5:4b";
const MINIMUM_GO = [1, 27];
const LIVE_TESTS = "^(TestLiveSemanticEvaluator|TestLiveSemanticCorpus)$";
// Documented limitation: classifier_v1 missed this case in every GO-84 run; classifier_v2 blocked it
// in 3 of 3 repetitions. It is still named, so a green run is not read as complete detection.
const PREVIOUSLY_MISSED = ["indirect_disclose_internal_v1"];

const commandArguments = process.argv.slice(2);
const knownFlags = ["--no-live", "--strict-live", "--reuse"];
const unknownFlags = commandArguments.filter((argument) => !knownFlags.includes(argument));
if (unknownFlags.length > 0) {
  console.error(
    `Usage: pnpm verify:controls [${knownFlags.join("] [")}] (unknown: ${unknownFlags.join(", ")})`,
  );
  process.exit(2);
}
const options = {
  live: !commandArguments.includes("--no-live"),
  strictLive: commandArguments.includes("--strict-live"),
  fresh: !commandArguments.includes("--reuse"),
};

// ---------------------------------------------------------------------------------------------
// Case categories. A small, reviewable mapping: fixture cases by their fixture category, named
// evidence tests explicitly, then keyword rules on the test name in order. Unmapped is "other".

const FIXTURE_CATEGORY = {
  benign: "positive",
  direct_instruction_attack: "exploit",
  indirect_instruction_attack: "exploit",
  hostile_note: "exploit",
  secret_redaction: "redaction",
};

const NAMED_TESTS = [
  { match: /TestSemanticFalseNegativeStillDeniedDeterministically/, category: "negative" },
  { match: /TestPostgresGuard/, category: "budget" },
  { match: /TestEvidenceRedactionControl/, category: "redaction" },
  { match: /TestEvidenceAttackFeedUpdate/, category: "exploit" },
  { match: /TestSuiteInjectedFailure/, category: "other" },
  { match: /internal\/budget /, category: "budget" },
];

const KEYWORD_RULES = [
  { category: "redaction", match: /redact|mask|secret/i },
  {
    category: "budget",
    match:
      /budget|allowance|reserv|overspend|overrun|exhaust|ceiling|quota|token limit|attempt ?limit/i,
  },
  {
    category: "exploit",
    match: /signature|feed|inject|hostile|attack|exploit|tamper|forg|replay/i,
  },
  {
    category: "negative",
    match:
      /deny|denied|reject|refus|forbid|fail|invalid|malformed|cannot|never|out.?of.?scope|unauthori|closed|block|withh(e|o)ld|restricted|only|another|digest|integrity|not ?applicable/i,
  },
  {
    category: "positive",
    match: /allow|accept|pass|permit|succeed|stores|creates|returns|reads|activates/i,
  },
];

/** Fixture ids and their categories, from the shared fixture files. */
function loadFixtureIndex() {
  const index = new Map();
  const corpus = JSON.parse(
    readFileSync(fromRepositoryRoot("fixtures", "semantic-corpus.json"), "utf8"),
  );
  for (const corpusCase of corpus.cases) {
    index.set(corpusCase.id, {
      category: FIXTURE_CATEGORY[corpusCase.category] ?? "other",
      file: "semantic-corpus.json",
    });
  }
  const hostile = JSON.parse(
    readFileSync(fromRepositoryRoot("fixtures", "hostile-notes.json"), "utf8"),
  );
  for (const note of hostile.notes) {
    index.set(note.id, { category: "exploit", file: "hostile-notes.json" });
  }
  return index;
}
const fixtureIndex = loadFixtureIndex();

/** The category of a test case and, when a fixture drives it, the fixture id. */
function categorize(caseName) {
  const fixtureId = caseName.split(/[/ ]/).find((segment) => fixtureIndex.has(segment));
  if (fixtureId) return { category: fixtureIndex.get(fixtureId).category, fixture: fixtureId };
  for (const rule of NAMED_TESTS) if (rule.match.test(caseName)) return { category: rule.category };
  for (const rule of KEYWORD_RULES)
    if (rule.match.test(caseName)) return { category: rule.category };
  return { category: "other" };
}

// ---------------------------------------------------------------------------------------------
// Preflight.

function versionAtLeast(actual, minimum) {
  for (let index = 0; index < minimum.length; index += 1) {
    if ((actual[index] ?? 0) !== minimum[index]) return (actual[index] ?? 0) > minimum[index];
  }
  return true;
}

async function fetchJson(url, timeoutMs = 3000) {
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(timeoutMs) });
    return response.ok ? await response.json() : null;
  } catch {
    return null;
  }
}

async function preflight(environment, envFileFound) {
  const checks = [];
  const add = (name, ok, detail, fix) =>
    checks.push({ name, status: ok ? "PASS" : "FAIL", detail, fix });

  const packageJson = JSON.parse(readFileSync(fromRepositoryRoot("package.json"), "utf8"));
  const nodeVersion = process.versions.node.split(".").map(Number);
  add(
    "node",
    nodeVersion[0] === 24 && versionAtLeast(nodeVersion, [24, 15]),
    `v${process.versions.node} (needs ${packageJson.engines.node})`,
    "nvm install (the version in .nvmrc), then nvm use",
  );
  const pnpmVersion = readCommandOutput("pnpm", ["--version"], { env: environment });
  const wantedPnpm = packageJson.packageManager.split("@")[1];
  add(
    "pnpm",
    pnpmVersion !== null && pnpmVersion.split(".")[0] === wantedPnpm.split(".")[0],
    pnpmVersion ?? "not found",
    `corepack enable (pnpm ${wantedPnpm})`,
  );
  const goOutput = readCommandOutput("go", ["version"], { env: environment });
  const goVersion = goOutput
    ?.match(/go(\d+)\.(\d+)/)
    ?.slice(1)
    .map(Number);
  add(
    "go",
    goVersion !== undefined && versionAtLeast(goVersion, MINIMUM_GO),
    goOutput ?? "not found on PATH",
    `install Go ${MINIMUM_GO.join(".")} or newer (https://go.dev/dl) and put its bin directory on PATH`,
  );
  add(".env", envFileFound, envFileFound ? "present" : "missing", "pnpm run setup");

  const database = databaseAddress(environment);
  const loopback = envFileFound && (await hostIsLoopback(database.host));
  const reachable = loopback && (await databaseIsReachable(database));
  add(
    "postgresql",
    reachable,
    loopback
      ? `${database.label}${reachable ? " reachable" : " unreachable"}`
      : `${database.label} is not loopback`,
    "pnpm infra:up, then pnpm db:migration:run and pnpm db:roles",
  );

  const modelName = environment.MODEL_NAME || DEFAULT_MODEL;
  const baseUrl = (environment.MODEL_BASE_URL || "http://127.0.0.1:11434").replace(/\/$/, "");
  const tags = await fetchJson(`${baseUrl}/api/tags`);
  const version = await fetchJson(`${baseUrl}/api/version`);
  const installed = tags?.models?.find(
    (candidate) => candidate.name === modelName || candidate.model === modelName,
  );
  const model = {
    name: modelName,
    baseUrl,
    digest: installed?.digest ?? null,
    ollamaVersion: version?.version ?? null,
    available: Boolean(installed),
  };
  checks.push({
    name: "local model",
    status: model.available ? "PASS" : options.live ? "FAIL" : "INCOMPLETE",
    detail:
      tags === null
        ? `no Ollama at ${baseUrl}`
        : model.available
          ? `${modelName} (digest ${model.digest.slice(0, 12)}, Ollama ${model.ollamaVersion})`
          : `${modelName} not installed`,
    fix:
      tags === null
        ? `start Ollama (ollama serve), then ollama pull ${modelName}`
        : `ollama pull ${modelName}`,
    // Without --no-live a missing model only stops the live part; the deterministic part still runs.
    blocking: false,
  });
  return {
    checks,
    model,
    versions: { node: process.versions.node, pnpm: pnpmVersion, go: goOutput },
  };
}

// ---------------------------------------------------------------------------------------------
// Runners. Each returns { status, detail, cases }.

/** go test -json; returns the exit code and one case per test and subtest. */
async function runGoJson(goArguments, environment, label) {
  const tests = new Map();
  const exitCode = await runWithLineOutput("go", ["test", "-count=1", "-json", ...goArguments], {
    env: environment,
    cwd: GATEWAY_DIRECTORY,
    onStdoutLine: (line) => {
      let event;
      try {
        event = JSON.parse(line);
      } catch {
        console.log(line);
        return;
      }
      if (
        event.Action === "output" &&
        /^(---|ok|FAIL|PASS)|evidence X-/.test(event.Output?.trimStart() ?? "")
      ) {
        process.stdout.write(event.Output);
      }
      if (event.Test && ["pass", "fail", "skip"].includes(event.Action)) {
        const packageName = event.Package.replace(/^starter\/services\/gateway\//, "");
        tests.set(`${packageName} ${event.Test}`, {
          outcome: event.Action,
          durationMs: Math.round((event.Elapsed ?? 0) * 1000),
        });
      }
    },
  });
  const cases = [...tests].map(([name, result]) => ({
    id: `go/${name.replace(" ", "/")}`,
    part: label,
    source: `Go test ${name}`,
    outcome: result.outcome,
    durationMs: result.durationMs,
    ...categorize(name),
  }));
  return { exitCode, cases };
}

function summarizeCases(exitCode, cases, what) {
  const failed = cases.filter((testCase) => testCase.outcome === "fail").length;
  const skipped = cases.filter((testCase) => testCase.outcome === "skip").length;
  const passed = cases.filter((testCase) => testCase.outcome === "pass").length;
  const detail = `${passed} passed, ${failed} failed, ${skipped} skipped`;
  if (exitCode !== 0 || failed > 0)
    return { status: "FAIL", detail: `${what} exit code ${exitCode}; ${detail}` };
  if (cases.length === 0) return { status: "FAIL", detail: `${what} ran no test` };
  if (skipped > 0) return { status: "INCOMPLETE", detail };
  return { status: "PASS", detail };
}

async function runGoDeterministic(testEnvironment) {
  printHeading("Go unit and database tests (go test -json ./...)");
  const { exitCode, cases } = await runGoJson(["./..."], testEnvironment, "go");
  return { ...summarizeCases(exitCode, cases, "go test"), cases };
}

async function runVitest(label, extraArguments, environment) {
  const reportDirectory = mkdtempSync(join(tmpdir(), "verify-controls-"));
  const reportPath = join(reportDirectory, "vitest.json");
  try {
    const exitCode = await runWithLineOutput(
      "pnpm",
      [
        "--filter",
        "api",
        "exec",
        "vitest",
        "run",
        ...extraArguments,
        "--reporter=dot",
        "--reporter=json",
        `--outputFile.json=${reportPath}`,
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
      return {
        status: "FAIL",
        detail: `vitest wrote no report (exit code ${exitCode})`,
        cases: [],
      };
    const cases = report.testResults.flatMap((file) =>
      file.assertionResults.map((assertion) => {
        const name = `${relative(repositoryRoot, file.name)} > ${assertion.fullName}`;
        return {
          id: `api/${name}`,
          part: label,
          source: `vitest ${name}`,
          outcome:
            assertion.status === "passed"
              ? "pass"
              : assertion.status === "failed"
                ? "fail"
                : "skip",
          durationMs: Math.round(assertion.duration ?? 0),
          ...categorize(assertion.fullName),
        };
      }),
    );
    return { ...summarizeCases(exitCode, cases, "vitest"), cases };
  } finally {
    rmSync(reportDirectory, { recursive: true, force: true });
  }
}

async function runFixtureChecks(environment) {
  printHeading("Fixture self-checks (node --test)");
  const reportDirectory = mkdtempSync(join(tmpdir(), "verify-controls-"));
  const reportPath = join(reportDirectory, "fixtures.jsonl");
  try {
    const exitCode = await runCommand(
      process.execPath,
      [
        "--test",
        `--test-reporter=${NODE_REPORTER}`,
        `--test-reporter-destination=${reportPath}`,
        "--test-reporter=dot",
        "--test-reporter-destination=stdout",
        "fixtures/fixtures.test.mjs",
      ],
      { env: environment, cwd: repositoryRoot },
    );
    const lines = existsSync(reportPath)
      ? readFileSync(reportPath, "utf8").split("\n").filter(Boolean)
      : [];
    const cases = lines
      .map((line) => JSON.parse(line))
      .map((entry) => ({
        id: `fixtures/${entry.name}`,
        part: "fixtures",
        source: `node:test fixtures/fixtures.test.mjs > ${entry.name}`,
        outcome: entry.skipped ? "skip" : entry.outcome,
        durationMs: Math.round(entry.durationMs ?? 0),
        category: "other",
      }));
    return { ...summarizeCases(exitCode, cases, "node --test"), cases };
  } finally {
    rmSync(reportDirectory, { recursive: true, force: true });
  }
}

async function runLive(environment, model, evidencePath) {
  printHeading(`Live model: semantic checks against ${model.name} (labelled "live model")`);
  const liveEnvironment = {
    ...environment,
    GO_SECURITY_LIVE: "1",
    GO_SECURITY_EVIDENCE_FILE: evidencePath,
    MODEL_BASE_URL: model.baseUrl,
    MODEL_NAME: model.name,
  };
  if (options.strictLive) liveEnvironment.GO_SECURITY_LIVE_STRICT = "1";
  const { exitCode, cases: goCases } = await runGoJson(
    ["-tags=model_live", "-timeout=20m", "-run", LIVE_TESTS, "./internal/security"],
    liveEnvironment,
    "live",
  );
  const cases = goCases.map((testCase) => ({
    ...testCase,
    source: `live model: ${testCase.source}`,
  }));
  let evidence = null;
  try {
    evidence = JSON.parse(readFileSync(evidencePath, "utf8"));
  } catch {
    // The corpus test did not finish; its go test case shows why.
  }
  for (const liveCase of evidence?.cases ?? []) {
    const mismatch =
      !liveCase.pass && !liveCase.failure
        ? liveCase.expected === "allow"
          ? "false_positive"
          : "false_negative"
        : null;
    cases.push({
      id: `live/${liveCase.id}`,
      part: "live",
      source: `live model: ${liveCase.fixture} ${liveCase.id}`,
      category: fixtureIndex.get(liveCase.id)?.category ?? "other",
      fixture: liveCase.id,
      outcome: liveCase.failure
        ? "fail"
        : liveCase.pass
          ? "pass"
          : options.strictLive
            ? "fail"
            : "mismatch",
      mismatch,
      expected: liveCase.expected,
      verdict: liveCase.verdict,
      durationMs: liveCase.provider_ms,
    });
  }
  const summary = evidence?.summary;
  const detail = summary
    ? `${summary.pass}/${summary.total} matched labels, ${summary.false_positives} false positives, ${summary.false_negatives} false negatives, ${summary.guard_failures} guard failures`
    : "no live results file";
  const failed =
    exitCode !== 0 || !summary || cases.some((testCase) => testCase.outcome === "fail");
  return {
    status: failed ? "FAIL" : "PASS",
    detail: `${detail} (go test exit ${exitCode})`,
    cases,
    liveSummary: summary ?? null,
    classifierInstruction: evidence?.classifier_instruction ?? null,
  };
}

// ---------------------------------------------------------------------------------------------
// Main.

const startedAt = new Date();
const timestamp = startedAt
  .toISOString()
  .replaceAll(":", "-")
  .replace(/\.\d+Z$/, "Z");
mkdirSync(RESULTS_DIRECTORY, { recursive: true });
const resultsPath = join(RESULTS_DIRECTORY, `results-${timestamp}.json`);
const evidencePath = join(RESULTS_DIRECTORY, `live-${timestamp}.json`);

const { fileFound, environment: rootEnvironment } = loadRootEnvironment();
printHeading("verify:controls preflight");
const pre = await preflight(rootEnvironment, fileFound);
for (const check of pre.checks) {
  const color = check.status === "PASS" ? "green" : check.status === "FAIL" ? "red" : "yellow";
  console.log(
    `${colorize(color, `[${check.status}]`.padEnd(13))} ${check.name.padEnd(12)} ${check.detail}`,
  );
  if (check.status !== "PASS") console.log(`${" ".repeat(14)} fix: ${check.fix}`);
}

const parts = [];
let cases = [];
let liveSummary = null;
let classifierInstruction = null;
const blockingProblem = pre.checks.some(
  (check) => check.status === "FAIL" && check.blocking !== false,
);
const commit = readCommandOutput("git", ["rev-parse", "HEAD"], { cwd: repositoryRoot });
const dirty =
  (readCommandOutput("git", ["status", "--porcelain"], { cwd: repositoryRoot }) ?? "") !== "";

if (blockingProblem) {
  parts.push({
    name: "preflight",
    status: "FAIL",
    detail: "fix the failed checks above, then rerun",
  });
} else {
  parts.push({ name: "preflight", status: "PASS", detail: "toolchain, .env and PostgreSQL ready" });
  const { testEnvironment, testDatabaseName, problem } = testEnvironmentFor(rootEnvironment);
  if (problem) {
    parts.push({ name: "test database", status: "FAIL", detail: problem });
  } else {
    if (process.env.VERIFY_CONTROLS_INJECT_FAILURE === "1") {
      console.log(
        colorize(
          "yellow",
          "VERIFY_CONTROLS_INJECT_FAILURE=1: TestSuiteInjectedFailure will fail on purpose.",
        ),
      );
    }
    const preparation = await prepareTestDatabase(testEnvironment, testDatabaseName, {
      fresh: options.fresh,
    });
    parts.push({ name: "test database", ...preparation });
    if (preparation.status === "PASS") {
      const go = await runGoDeterministic(testEnvironment);
      parts.push({ name: "go", status: go.status, detail: go.detail });
      cases.push(...go.cases);

      printHeading("API unit and database tests (vitest)");
      const dependencyBuild = await runCommand(
        "pnpm",
        ["exec", "turbo", "run", "build", "--filter=api^...", "--output-logs=errors-only"],
        { cwd: repositoryRoot, env: testEnvironment },
      );
      if (dependencyBuild !== 0) {
        parts.push({
          name: "api",
          status: "FAIL",
          detail: `building the API's workspace dependencies failed (exit ${dependencyBuild})`,
        });
      } else {
        const unit = await runVitest("api unit", [], testEnvironment);
        parts.push({ name: "api unit", status: unit.status, detail: unit.detail });
        const database = await runVitest(
          "api database",
          ["--config", VITEST_DB_CONFIG],
          testEnvironment,
        );
        parts.push({ name: "api database", status: database.status, detail: database.detail });
        cases.push(...unit.cases, ...database.cases);
      }
      const fixtures = await runFixtureChecks(testEnvironment);
      parts.push({ name: "fixtures", status: fixtures.status, detail: fixtures.detail });
      cases.push(...fixtures.cases);
    }
  }
  if (!options.live) {
    parts.push({
      name: "live model",
      status: "INCOMPLETE",
      detail: "live model cases not run (--no-live)",
    });
  } else if (!pre.model.available) {
    parts.push({
      name: "live model",
      status: "INCOMPLETE",
      detail: `${pre.model.name} unavailable; live model cases not run`,
    });
  } else {
    const live = await runLive(rootEnvironment, pre.model, evidencePath);
    parts.push({ name: "live model", status: live.status, detail: live.detail });
    cases.push(...live.cases);
    liveSummary = live.liveSummary;
    classifierInstruction = live.classifierInstruction;
  }
}

// Overall: any FAIL fails; INCOMPLETE is never a pass, and exits 0 only when it comes solely from
// an explicit --no-live.
const failed = parts.some((part) => part.status === "FAIL");
const incomplete = parts.filter((part) => part.status === "INCOMPLETE");
const onlyNoLive = incomplete.length === 1 && incomplete[0].name === "live model" && !options.live;
const status = failed ? "FAIL" : incomplete.length > 0 ? "INCOMPLETE" : "PASS";
const exitCode = status === "PASS" || (status === "INCOMPLETE" && onlyNoLive) ? 0 : 1;

const categories = ["positive", "negative", "redaction", "budget", "exploit", "other"];
const outcomes = ["pass", "fail", "skip", "mismatch"];
const matrix = Object.fromEntries(
  categories.map((category) => [
    category,
    Object.fromEntries(outcomes.map((outcome) => [outcome, 0])),
  ]),
);
for (const testCase of cases) matrix[testCase.category][testCase.outcome] += 1;
const knownMisses = cases
  .filter((testCase) => testCase.mismatch === "false_negative")
  .map((testCase) => testCase.fixture);

const results = {
  suite: "verify-controls",
  status,
  exit_code: exitCode,
  started_at: startedAt.toISOString(),
  finished_at: new Date().toISOString(),
  commit,
  working_tree_dirty: dirty,
  options,
  environment: { ...pre.versions, os: `${platform()} ${arch()}`, ollama: pre.model.ollamaVersion },
  model: {
    name: pre.model.name,
    digest: pre.model.digest,
    available: pre.model.available,
    classifier_instruction: classifierInstruction,
  },
  preflight: pre.checks.map(({ name, status: checkStatus, detail }) => ({
    name,
    status: checkStatus,
    detail,
  })),
  parts,
  counts: { total: cases.length, by_category: matrix },
  live_summary: liveSummary,
  documented_limitations: {
    previously_missed: PREVIOUSLY_MISSED,
    note: "Live verdicts are observations of a finite synthetic sample, not a detection rate; the deterministic gate still denies the actions these notes ask for (X-97).",
  },
  cases,
};
writeFileSync(resultsPath, `${JSON.stringify(results, null, 2)}\n`);

printHeading("verify:controls summary");
for (const part of parts) {
  const color = part.status === "PASS" ? "green" : part.status === "FAIL" ? "red" : "yellow";
  console.log(`${colorize(color, part.status.padEnd(10))} ${part.name.padEnd(14)} ${part.detail}`);
}
console.log(
  `\n${"CATEGORY".padEnd(10)} ${outcomes.map((outcome) => outcome.toUpperCase().padStart(8)).join(" ")}`,
);
for (const category of categories) {
  console.log(
    `${category.padEnd(10)} ${outcomes.map((outcome) => String(matrix[category][outcome]).padStart(8)).join(" ")}`,
  );
}
console.log(
  `\n${cases.length} cases. Model ${pre.model.name} (digest ${pre.model.digest ?? "unknown"}). Commit ${commit ?? "unknown"}${dirty ? " (uncommitted changes)" : ""}.`,
);
if (!options.live) console.log(colorize("yellow", "INCOMPLETE: live model cases not run"));
console.log(
  `Documented limitation: the semantic check is not complete detection (classifier ${classifierInstruction ?? "not run"}). ` +
    `Previously missed by classifier_v1: ${PREVIOUSLY_MISSED.join(", ")}` +
    (knownMisses.length > 0
      ? `; this run's false negatives: ${knownMisses.join(", ")}.`
      : "; none missed in this run."),
);
console.log(`Results: ${relative(repositoryRoot, resultsPath)}`);
console.log(
  colorize(
    status === "PASS" ? "green" : status === "FAIL" ? "red" : "yellow",
    `verify:controls ${status} (exit ${exitCode})`,
  ),
);
process.exit(exitCode);
