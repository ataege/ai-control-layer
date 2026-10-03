#!/usr/bin/env node
// Starts the development services on the host: node scripts/dev.mjs [web|api|gateway]
// Without an argument all three run together and stop together.
import { commandExists, runCommand } from "./lib/commands.mjs";
import { loadRootEnvironment, MISSING_ENV_FILE_MESSAGE } from "./lib/env-file.mjs";
import { repositoryRoot } from "./lib/repo-root.mjs";
import { superviseServices } from "./lib/supervisor.mjs";

const SERVICE_NAMES = ["web", "api", "gateway"];

const requestedArguments = process.argv.slice(2);
const requestedService = requestedArguments[0];

if (
  requestedArguments.length > 1 ||
  (requestedService && !SERVICE_NAMES.includes(requestedService))
) {
  console.error(`Usage: node scripts/dev.mjs [${SERVICE_NAMES.join("|")}]`);
  process.exit(2);
}

const { fileFound, environment } = loadRootEnvironment();
if (!fileFound) {
  console.error(`[dev] ${MISSING_ENV_FILE_MESSAGE}`);
  process.exit(1);
}

const selectedServiceNames = requestedService ? [requestedService] : SERVICE_NAMES;

// Fail early with a clear message instead of letting the gateway child die mid-start.
if (
  selectedServiceNames.includes("gateway") &&
  !commandExists("go", { env: environment, versionArguments: ["version"] })
) {
  console.error("[dev] Go toolchain not found on PATH. The gateway needs Go 1.27 or newer.");
  console.error(
    "[dev] Install Go, or start the other services alone with `pnpm dev:web` and `pnpm dev:api`.",
  );
  process.exit(1);
}

// The web app and the API import the compiled contracts, so build them once up front.
if (selectedServiceNames.some((serviceName) => serviceName !== "gateway")) {
  console.error("[dev] building @workspace/contracts...");
  const contractsBuildExitCode = await runCommand(
    process.platform === "win32" ? "npx.cmd" : "npx",
    ["pnpm", "--filter", "@workspace/contracts", "run", "build"],
    { env: environment, cwd: repositoryRoot },
  );
  if (contractsBuildExitCode !== 0) {
    console.error(
      `[dev] building @workspace/contracts failed (exit code ${contractsBuildExitCode})`,
    );
    process.exit(contractsBuildExitCode);
  }
}

// Mirrors the web block in infra/compose.yaml: the web app must never hold the
// service token or the database credentials, so its child gets a filtered environment.
const WEB_FORBIDDEN_VARIABLE_PATTERN = /^(GATEWAY_SERVICE_TOKEN$|POSTGRES_)/;
const webEnvironment = Object.fromEntries(
  Object.entries(environment).filter(
    ([variableName]) => !WEB_FORBIDDEN_VARIABLE_PATTERN.test(variableName),
  ),
);

const services = selectedServiceNames.map((serviceName) => ({
  name: serviceName,
  command: process.platform === "win32" ? "npx.cmd" : "npx",
  commandArguments: ["pnpm", "--filter", serviceName, "run", "dev"],
  cwd: repositoryRoot,
  env: serviceName === "web" ? webEnvironment : environment,
}));

process.exit(await superviseServices(services, { env: environment }));
