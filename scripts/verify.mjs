#!/usr/bin/env node
// Full quality gate (`pnpm verify`): runs the root scripts below in order and prints a summary.
// Needs no .env, no database and no running service. Every step runs even after a failure.
import { readFileSync } from "node:fs";

import { commandExists, runCommand } from "./lib/commands.mjs";
import { countByStatus, printHeading, printResultTable } from "./lib/output.mjs";
import { fromRepositoryRoot, repositoryRoot } from "./lib/repo-root.mjs";

const VERIFICATION_STEPS = [
  "check:instructions",
  "format:check",
  "lint",
  "typecheck",
  "test",
  "build",
];

// Steps that include a gateway task and therefore need the Go toolchain.
const GO_DEPENDENT_STEPS = new Set(["format:check", "lint", "typecheck", "test", "build"]);

const rootScripts =
  JSON.parse(readFileSync(fromRepositoryRoot("package.json"), "utf8")).scripts ?? {};
const goAvailable = commandExists("go", { versionArguments: ["version"] });
const stepResults = [];
let interruptedBySignal = null;

// The running step receives the signal as well; remaining steps are then skipped.
for (const signalName of ["SIGINT", "SIGTERM"]) {
  process.on(signalName, () => {
    interruptedBySignal = signalName;
  });
}

const formatDuration = (durationMs) => `${(durationMs / 1000).toFixed(1)}s`;

if (!goAvailable) {
  console.error(
    "[verify] Go toolchain not found on PATH: the gateway's format, lint, typecheck, test and build tasks cannot pass.",
  );
}

for (const stepName of VERIFICATION_STEPS) {
  if (interruptedBySignal) {
    stepResults.push({
      name: stepName,
      status: "SKIPPED",
      detail: `not run (${interruptedBySignal})`,
    });
    continue;
  }
  if (!(stepName in rootScripts)) {
    stepResults.push({
      name: stepName,
      status: "SKIPPED",
      detail: "no such script in package.json",
    });
    continue;
  }

  printHeading(`verify: ${stepName}`);
  const startedAt = Date.now();
  const exitCode = await runCommand("pnpm", ["run", stepName], { cwd: repositoryRoot });
  const duration = formatDuration(Date.now() - startedAt);

  if (exitCode === 0) {
    stepResults.push({ name: stepName, status: "PASS", detail: duration });
  } else {
    const blockedByMissingGo = !goAvailable && GO_DEPENDENT_STEPS.has(stepName);
    const goHint = blockedByMissingGo
      ? "; Go is not installed, so the gateway part cannot pass"
      : "";
    stepResults.push({
      name: stepName,
      status: "FAIL",
      detail: `exit code ${exitCode} after ${duration}${goHint}`,
    });
  }
}

printHeading("Verification summary");
printResultTable("STEP", stepResults);

const counts = countByStatus(stepResults);
console.log(`\n${counts.PASS} passed, ${counts.FAIL} failed, ${counts.SKIPPED} skipped`);

// A skipped step means the gate did not fully run, so it cannot count as a success.
const verificationSucceeded = counts.FAIL === 0 && counts.SKIPPED === 0;
process.exit(interruptedBySignal ? 130 : verificationSucceeded ? 0 : 1);
