#!/usr/bin/env node
// Runs a command with the root .env loaded: node scripts/with-env.mjs <command> [args...]
// Real environment variables win over the file. Exit code and signals are propagated.
import { runCommand } from "./lib/commands.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { repositoryRoot } from "./lib/repo-root.mjs";

const [command, ...commandArguments] = process.argv.slice(2);

if (!command) {
  console.error("Usage: node scripts/with-env.mjs <command> [args...]");
  process.exit(2);
}

const { fileFound, environment } = loadRootEnvironment();
if (!fileFound) {
  console.error(MISSING_ENV_FILE_MESSAGE);
  process.exit(1);
}

process.exit(
  await runCommand(command, commandArguments, { env: environment, cwd: repositoryRoot }),
);
