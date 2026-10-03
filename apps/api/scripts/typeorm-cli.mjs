#!/usr/bin/env node
// Runs the TypeORM CLI for this package under the ts-node ESM loader.
// Usage: node scripts/typeorm-cli.mjs <create|generate|show|run|revert> [Name] [extra CLI flags]
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

const apiDirectory = resolve(import.meta.dirname, "..");
const typeormCliPath = resolve(apiDirectory, "node_modules/typeorm/cli-ts-node-esm.js");
const dataSourcePath = "src/database/data-source.ts";
const migrationsDirectory = "src/database/migrations";

const commandsWithName = new Set(["create", "generate"]);
const commandsWithDataSource = new Set(["generate", "show", "run", "revert"]);

const [command, ...remainingArguments] = process.argv.slice(2);

if (!commandsWithName.has(command) && !commandsWithDataSource.has(command)) {
  console.error("Usage: typeorm-cli.mjs <create|generate|show|run|revert> [Name] [flags]");
  process.exit(1);
}

const cliArguments = [`migration:${command}`];
if (commandsWithDataSource.has(command)) {
  // Short message up front; data-source.ts validates the same variables in full.
  const requiredVariables = ["POSTGRES_HOST", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB"];
  const missingVariables = requiredVariables.filter((variableName) => !process.env[variableName]);
  if (missingVariables.length > 0) {
    console.error(
      `Missing environment variables: ${missingVariables.join(", ")}.\n` +
        "Run this command from the repository root (pnpm db:migration:...), which loads .env, " +
        "or export the POSTGRES_* variables first.",
    );
    process.exit(1);
  }
  cliArguments.push("-d", dataSourcePath);
}
if (commandsWithName.has(command)) {
  const [migrationName, ...extraFlags] = remainingArguments;
  // A plain class-style name keeps the file inside the migrations directory.
  if (migrationName === undefined || !/^[A-Za-z][A-Za-z0-9]*$/.test(migrationName)) {
    console.error(
      `Usage: pnpm migration:${command} <Name>  (letters and digits, e.g. AddExampleTable)`,
    );
    process.exit(1);
  }
  cliArguments.push(`${migrationsDirectory}/${migrationName}`, ...extraFlags);
} else {
  cliArguments.push(...remainingArguments);
}

// Relative paths above resolve against the package directory, wherever the script was started.
const result = spawnSync(process.execPath, [typeormCliPath, ...cliArguments], {
  cwd: apiDirectory,
  stdio: "inherit",
});
process.exit(result.status ?? 1);
