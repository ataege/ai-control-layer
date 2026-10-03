#!/usr/bin/env node
// Runs the explicit policy import (API-32) under the ts-node ESM loader, like the TypeORM CLI wrapper.
// Usage: node scripts/import-policy.mjs [path]; relative to the repository root, default config/policy.yaml.
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

const apiDirectory = resolve(import.meta.dirname, "..");
const repositoryRoot = resolve(apiDirectory, "../..");
const [requestedPath] = process.argv.slice(2);

// The root script runs through scripts/with-env.mjs, which starts in the repository root, so a
// relative path is relative to the repository root.
const policyFilePath = resolve(repositoryRoot, requestedPath ?? "config/policy.yaml");

const requiredVariables = ["POSTGRES_HOST", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"];
const missingVariables = requiredVariables.filter((variableName) => !process.env[variableName]);
if (missingVariables.length > 0) {
  console.error(
    `Missing environment variables: ${missingVariables.join(", ")}.\n` +
      "Run this command from the repository root (pnpm policy:import), which loads .env.",
  );
  process.exit(1);
}

const result = spawnSync(
  process.execPath,
  [
    "--loader",
    "ts-node/esm",
    "--no-warnings",
    "src/policies/import-policy.command.ts",
    policyFilePath,
  ],
  { cwd: apiDirectory, stdio: "inherit" },
);
process.exit(result.status ?? 1);
