#!/usr/bin/env node
// First-time setup (`pnpm run setup`): reports prerequisites and prepares the root .env.
// It installs nothing, never overwrites a non-empty value and never prints a secret. The local-model
// check only warns: setup never fails because Ollama or the model is missing.
import { existsSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { parseEnv } from "node:util";

import { readCommandOutput } from "./lib/commands.mjs";
import { GENERATED_SECRETS } from "./lib/generated-secrets.mjs";
import { isModelListed } from "./lib/local-model.mjs";
import { printHeading, printStatus } from "./lib/output.mjs";
import {
  fromRepositoryRoot,
  repositoryRoot,
  rootEnvExamplePath,
  rootEnvFilePath,
} from "./lib/repo-root.mjs";

const MINIMUM_GO_VERSION = "1.27";

// Keys that are filled with their .env.example default when an existing .env has them empty, so an
// .env created before the default existed still works. The value is not a secret.
const FILL_WHEN_EMPTY = ["MODEL_NAME"];

// ---------------------------------------------------------------- versions

function parseVersion(versionText) {
  const numericParts = String(versionText).match(/\d+(?:\.\d+){0,2}/)?.[0];
  if (!numericParts) return null;
  const [major = 0, minor = 0, patch = 0] = numericParts.split(".").map(Number);
  return [major, minor, patch];
}

function compareVersions(leftVersion, rightVersion) {
  for (let partIndex = 0; partIndex < 3; partIndex += 1) {
    if (leftVersion[partIndex] !== rightVersion[partIndex]) {
      return leftVersion[partIndex] - rightVersion[partIndex];
    }
  }
  return 0;
}

/** Supports the simple ranges used in "engines": space-separated comparators such as ">=24.15.0 <25". */
function satisfiesRange(versionText, rangeText) {
  const actualVersion = parseVersion(versionText);
  if (!actualVersion) return false;
  return rangeText
    .trim()
    .split(/\s+/)
    .every((comparatorText) => {
      const [, operator = "=", boundText] = comparatorText.match(/^(>=|<=|>|<|=)?v?(.+)$/) ?? [];
      const boundVersion = parseVersion(boundText);
      if (!boundVersion) return false;
      const difference = compareVersions(actualVersion, boundVersion);
      return {
        ">=": difference >= 0,
        "<=": difference <= 0,
        ">": difference > 0,
        "<": difference < 0,
        "=": difference === 0,
      }[operator];
    });
}

// ----------------------------------------------------------- prerequisites

/** Returns true when every required prerequisite (Node, pnpm) is satisfied. */
function reportPrerequisites() {
  printHeading("Prerequisites");
  const rootPackage = JSON.parse(readFileSync(fromRepositoryRoot("package.json"), "utf8"));
  let requiredPrerequisitesMet = true;

  const requiredNodeRange = rootPackage.engines?.node;
  if (!requiredNodeRange || satisfiesRange(process.versions.node, requiredNodeRange)) {
    printStatus("ok", `Node.js ${process.versions.node} (required: ${requiredNodeRange ?? "any"})`);
  } else {
    requiredPrerequisitesMet = false;
    printStatus("fail", `Node.js ${process.versions.node} does not satisfy ${requiredNodeRange}`);
  }

  const pinnedPnpmVersion = rootPackage.packageManager?.match(/^pnpm@([\w.-]+)/)?.[1];
  const installedPnpmVersion = readCommandOutput("pnpm", ["--version"], { cwd: repositoryRoot });
  if (!installedPnpmVersion) {
    requiredPrerequisitesMet = false;
    printStatus("fail", `pnpm not found on PATH (required: ${pinnedPnpmVersion ?? "any"})`);
  } else if (pinnedPnpmVersion && installedPnpmVersion !== pinnedPnpmVersion) {
    printStatus(
      "warn",
      `pnpm ${installedPnpmVersion} differs from the pinned ${pinnedPnpmVersion}`,
    );
  } else {
    printStatus("ok", `pnpm ${installedPnpmVersion}`);
  }

  const goVersionOutput = readCommandOutput("go", ["version"]);
  const goVersion = goVersionOutput?.match(/go(\d+\.\d+(?:\.\d+)?)/)?.[1];
  if (!goVersion) {
    printStatus(
      "warn",
      `Go not found on PATH. Go ${MINIMUM_GO_VERSION}+ is needed to run, test and build the gateway on the host.`,
    );
  } else if (!satisfiesRange(goVersion, `>=${MINIMUM_GO_VERSION}`)) {
    printStatus("warn", `Go ${goVersion} is older than the required ${MINIMUM_GO_VERSION}`);
  } else {
    printStatus("ok", `Go ${goVersion}`);
  }

  const dockerVersion = readCommandOutput("docker", ["--version"]);
  const composeVersion = dockerVersion && readCommandOutput("docker", ["compose", "version"]);
  if (!dockerVersion) {
    printStatus(
      "warn",
      "Docker not found on PATH. It is needed for `pnpm infra:up` (PostgreSQL) and `pnpm stack:up`.",
    );
  } else if (!composeVersion) {
    printStatus("warn", "Docker found, but the Compose plugin (`docker compose`) is missing.");
  } else {
    printStatus("ok", `${dockerVersion}; ${composeVersion}`);
  }

  return requiredPrerequisitesMet;
}

// -------------------------------------------------------------------- .env

const escapeForRegExp = (text) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * Returns the content with `key=` given the value `makeValue()` returns, or null when the line has an
 * unusual syntax. It only matches an assignment whose value is empty.
 */
function fillEmptyAssignment(content, key, makeValue) {
  const emptyAssignment = new RegExp(
    `^([ \\t]*(?:export[ \\t]+)?${escapeForRegExp(key)}[ \\t]*=)[ \\t]*(?:""|'')?[ \\t]*(?=\\r?$)`,
    "m",
  );
  if (!emptyAssignment.test(content)) return null;
  return content.replace(
    emptyAssignment,
    (_line, assignmentPrefix) => `${assignmentPrefix}${makeValue()}`,
  );
}

/**
 * Returns the new .env content plus the names of the keys that were added or generated.
 * Existing non-empty values are left byte-for-byte untouched.
 */
function planEnvFile(exampleContent, existingContent) {
  const exampleValues = parseEnv(exampleContent);
  const isNewFile = existingContent === null;
  const currentValues = parseEnv(isNewFile ? exampleContent : existingContent);
  let nextContent = isNewFile ? exampleContent : existingContent;
  const generatedKeys = [];
  const appendedKeys = [];

  // Fill generated keys that are present but empty, in place.
  for (const [secretKey, { generate: generateSecret }] of Object.entries(GENERATED_SECRETS)) {
    if (!(secretKey in currentValues) || currentValues[secretKey] !== "") continue;
    const filled = fillEmptyAssignment(nextContent, secretKey, generateSecret);
    if (filled === null) continue; // unusual syntax: leave the line alone
    nextContent = filled;
    generatedKeys.push(secretKey);
  }

  // Fill the keys that have a default in .env.example when an existing .env has them empty.
  const defaultedKeys = [];
  for (const defaultedKey of FILL_WHEN_EMPTY) {
    if (isNewFile || currentValues[defaultedKey] !== "" || !exampleValues[defaultedKey]) continue;
    const filled = fillEmptyAssignment(
      nextContent,
      defaultedKey,
      () => exampleValues[defaultedKey],
    );
    if (filled === null) continue;
    nextContent = filled;
    defaultedKeys.push(defaultedKey);
  }

  // Append keys that exist in .env.example but are missing from an existing .env.
  const missingKeys = Object.keys(exampleValues).filter((key) => !(key in currentValues));
  if (missingKeys.length > 0) {
    const appendedLines = missingKeys.map((missingKey) => {
      const generateSecret = GENERATED_SECRETS[missingKey]?.generate;
      if (generateSecret) generatedKeys.push(missingKey);
      else appendedKeys.push(missingKey);
      return `${missingKey}=${generateSecret ? generateSecret() : exampleValues[missingKey]}`;
    });
    const separator = nextContent.endsWith("\n") || nextContent === "" ? "" : "\n";
    nextContent += `${separator}\n# Added by setup (new keys from .env.example)\n${appendedLines.join("\n")}\n`;
  }

  return { nextContent, generatedKeys, appendedKeys, defaultedKeys, isNewFile };
}

/** Returns false when .env could not be prepared. */
function prepareEnvFile() {
  printHeading("Environment file");
  if (!existsSync(rootEnvExamplePath)) {
    printStatus("fail", ".env.example is missing; cannot create .env");
    return false;
  }
  const exampleContent = readFileSync(rootEnvExamplePath, "utf8");
  const existingContent = existsSync(rootEnvFilePath)
    ? readFileSync(rootEnvFilePath, "utf8")
    : null;
  const { nextContent, generatedKeys, appendedKeys, defaultedKeys, isNewFile } = planEnvFile(
    exampleContent,
    existingContent,
  );

  if (nextContent !== existingContent) {
    // Write to a temporary file first so an interrupted run cannot truncate .env.
    const temporaryPath = `${rootEnvFilePath}.tmp-${process.pid}`;
    writeFileSync(temporaryPath, nextContent, { mode: 0o600 });
    renameSync(temporaryPath, rootEnvFilePath);
  }

  if (isNewFile) printStatus("ok", "created .env from .env.example (readable by your user only)");
  else if (nextContent === existingContent)
    printStatus("ok", ".env is already complete; left unchanged");
  else printStatus("ok", "updated .env; existing values were kept");

  for (const generatedKey of generatedKeys) {
    printStatus("ok", `generated ${generatedKey} (value not shown)`);
  }
  for (const appendedKey of appendedKeys) {
    printStatus("ok", `added missing key ${appendedKey} with its default value`);
  }
  for (const defaultedKey of defaultedKeys) {
    printStatus(
      "ok",
      `${defaultedKey} was empty; set it to its default ${parseEnv(exampleContent)[defaultedKey]}`,
    );
  }

  // Report generated keys that still have no value (for example a hand-edited unusual line).
  const finalValues = parseEnv(nextContent);
  let envFileComplete = true;
  for (const secretKey of Object.keys(GENERATED_SECRETS)) {
    if (!finalValues[secretKey]) {
      envFileComplete = false;
      printStatus("fail", `${secretKey} is empty in .env; set it manually`);
    }
  }
  return envFileComplete;
}

// ------------------------------------------------------------- local model

/**
 * Warns, never fails: the model is named in .env, and `ollama list` should show it. A missing Ollama
 * or model only means runs would fail closed until it is pulled.
 */
function reportLocalModel() {
  printHeading("Local model");
  const modelName = (
    existsSync(rootEnvFilePath) ? parseEnv(readFileSync(rootEnvFilePath, "utf8")) : {}
  ).MODEL_NAME;
  if (!modelName) {
    printStatus(
      "warn",
      "MODEL_NAME is empty in .env: the gateway starts, but every model call fails closed. Set it to qwen3.5:4b.",
    );
    return;
  }
  const listOutput = readCommandOutput("ollama", ["list"]);
  if (listOutput === null) {
    printStatus(
      "warn",
      `could not read \`ollama list\` (Ollama is not installed or not running), so ${modelName} was not checked. Start Ollama, then: ollama pull ${modelName}`,
    );
  } else if (!isModelListed(listOutput, modelName)) {
    printStatus(
      "warn",
      `\`ollama list\` does not show ${modelName}. Run: ollama pull ${modelName}`,
    );
  } else {
    printStatus("ok", `Ollama has ${modelName} (the MODEL_NAME in .env)`);
  }
}

// -------------------------------------------------------------------- main

const prerequisitesMet = reportPrerequisites();
const envFileReady = prepareEnvFile();
reportLocalModel();

printHeading("Next steps");
// The first-run order of the README's "Quick start"; `pnpm install` already ran before setup.
console.log("  pnpm infra:up           start PostgreSQL in Docker (host development mode)");
console.log("  pnpm db:migration:run   apply the migrations");
console.log("  pnpm db:roles           give the gateway's database role its password");
console.log("  pnpm db:seed            load the synthetic demo records and the policy catalog");
console.log(
  "  ollama pull qwen3.5:4b  the model .env names as MODEL_NAME (docs/setup.md, section 7)",
);
console.log("  pnpm dev                run web, api and gateway on the host");
console.log("  pnpm smoke              check the running services");
console.log("  pnpm reset:demo         later: restore the demo data");
console.log("  pnpm verify             lint, typecheck, test and build");

if (!prerequisitesMet || !envFileReady) {
  console.log("");
  printStatus("fail", "setup finished with problems; see the FAIL lines above");
  process.exit(1);
}
