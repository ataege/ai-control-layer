#!/usr/bin/env node
// Docker Compose wrapper: node scripts/compose.mjs <infra-up|infra-down|stack-up|stack-down> [--debug]
// infra-* manage PostgreSQL only (host development); stack-* manage the full-container mode.
// The down commands never remove volumes, so database data survives.
import { existsSync } from "node:fs";

import { readCommandOutput, runCommand } from "./lib/commands.mjs";
import { composeProjectArguments } from "./lib/compose-arguments.mjs";
import { MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { repositoryRoot, rootEnvFilePath } from "./lib/repo-root.mjs";

const FULL_STACK_PROFILE = ["--profile", "full"];

const COMPOSE_ACTIONS = {
  "infra-up": { globalArguments: [], commandArguments: ["up", "--detach", "--wait", "postgres"] },
  "infra-down": { globalArguments: [], commandArguments: ["down"] },
  "stack-up": {
    globalArguments: FULL_STACK_PROFILE,
    commandArguments: ["up", "--detach", "--build", "--wait"],
  },
  "stack-down": { globalArguments: FULL_STACK_PROFILE, commandArguments: ["down"] },
};

const cliArguments = process.argv.slice(2);
const debugRequested = cliArguments.includes("--debug");
const [actionName, ...unexpectedArguments] = cliArguments.filter(
  (cliArgument) => cliArgument !== "--debug",
);
const action = COMPOSE_ACTIONS[actionName];

if (!action || unexpectedArguments.length > 0) {
  console.error(
    `Usage: node scripts/compose.mjs <${Object.keys(COMPOSE_ACTIONS).join("|")}> [--debug]`,
  );
  process.exit(2);
}

if (!readCommandOutput("docker", ["--version"])) {
  console.error("[compose] Docker was not found on PATH. Install and start Docker, then retry.");
  process.exit(127);
}
if (!readCommandOutput("docker", ["compose", "version"])) {
  console.error("[compose] The Docker Compose plugin (`docker compose`) is not available.");
  process.exit(127);
}
if (!existsSync(rootEnvFilePath)) {
  console.error(`[compose] ${MISSING_ENV_FILE_MESSAGE}`);
  process.exit(1);
}

// --debug adds the override that publishes the gateway port on localhost.
const dockerArguments = [
  ...composeProjectArguments({ debug: debugRequested }),
  ...action.globalArguments,
  ...action.commandArguments,
];

const composeExitCode = await runCommand("docker", dockerArguments, { cwd: repositoryRoot });
if (composeExitCode !== 0) {
  console.error(`[compose] ${actionName} failed (exit code ${composeExitCode})`);
}
process.exit(composeExitCode);
