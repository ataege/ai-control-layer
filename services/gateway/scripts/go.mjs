#!/usr/bin/env node
// Lets pnpm and turbo scripts call the Go toolchain for this module.
// Development aid only: nothing from Node ends up in the Go binary or its image.
import { spawn, spawnSync } from "node:child_process";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const moduleDirectory = resolve(dirname(fileURLToPath(import.meta.url)), "..");
// Not 127: pnpm reports that code as its own "spawn ENOENT" failure.
const MISSING_TOOLCHAIN_EXIT_CODE = 1;
// Separate from the `build` output so a dev run never replaces a release build.
const DEVELOPMENT_BINARY = "bin/gateway-dev";

const MISSING_TOOLCHAIN_GUIDANCE = [
  "[gateway] The Go toolchain was not found on PATH.",
  "[gateway] Install Go 1.27 or newer from https://go.dev/dl/ (or with your OS package manager),",
  "[gateway] open a new terminal, check it with `go version`, then run this command again.",
  "[gateway] Nothing was installed or changed by this script.",
].join("\n");

function exitWithMissingToolchain() {
  console.error(MISSING_TOOLCHAIN_GUIDANCE);
  process.exit(MISSING_TOOLCHAIN_EXIT_CODE);
}

/** Runs one tool to completion and stops the script when it fails. */
function runStep(toolName, toolArguments) {
  const stepResult = spawnSync(toolName, toolArguments, { cwd: moduleDirectory, stdio: "inherit" });
  if (stepResult.error?.code === "ENOENT") exitWithMissingToolchain();
  if (stepResult.error) {
    console.error(`[gateway] could not run ${toolName}: ${stepResult.error.message}`);
    process.exit(1);
  }
  if (stepResult.status !== 0) process.exit(stepResult.status ?? 1);
}

/** `gofmt -l` always exits 0, so the list of files it prints decides the result. */
function checkFormatting() {
  const formatResult = spawnSync("gofmt", ["-l", "."], { cwd: moduleDirectory, encoding: "utf8" });
  if (formatResult.error?.code === "ENOENT") exitWithMissingToolchain();
  if (formatResult.error || formatResult.status !== 0) {
    console.error(`[gateway] gofmt failed: ${formatResult.error?.message ?? formatResult.stderr}`);
    process.exit(1);
  }
  const unformattedFiles = formatResult.stdout.split("\n").filter((fileName) => fileName !== "");
  if (unformattedFiles.length > 0) {
    console.error(
      "[gateway] These files need formatting (run `pnpm --filter gateway run format`):",
    );
    for (const fileName of unformattedFiles) console.error(`  ${fileName}`);
    process.exit(1);
  }
}

/** Compiles the service, runs it in the foreground and forwards stop signals to it. */
function runDevelopmentServer() {
  // Compile first (instead of `go run`) so the service is this script's direct child.
  runStep("go", ["build", "-o", DEVELOPMENT_BINARY, "./cmd/gateway"]);

  // Not detached: the binary stays in the caller's process group, so a group kill
  // (including SIGKILL, which cannot be forwarded) reaches it as well.
  const gatewayProcess = spawn(resolve(moduleDirectory, DEVELOPMENT_BINARY), [], {
    cwd: moduleDirectory,
    stdio: ["ignore", "inherit", "inherit"],
  });
  let stopRequested = false;

  gatewayProcess.on("error", (spawnError) => {
    console.error(`[gateway] could not run ${DEVELOPMENT_BINARY}: ${spawnError.message}`);
    process.exit(1);
  });

  // The gateway handles SIGINT and SIGTERM itself; a repeated signal is harmless.
  for (const signalName of ["SIGINT", "SIGTERM"]) {
    process.on(signalName, () => {
      stopRequested = true;
      gatewayProcess.kill(signalName);
    });
  }
  // The gateway has no SIGHUP handler, so a closed terminal becomes a graceful stop.
  process.on("SIGHUP", () => {
    stopRequested = true;
    gatewayProcess.kill("SIGTERM");
  });

  gatewayProcess.on("exit", (exitCode) => {
    process.exit(stopRequested ? 0 : (exitCode ?? 1));
  });
}

const commands = {
  dev: runDevelopmentServer,
  build: () => runStep("go", ["build", "-trimpath", "-o", "bin/gateway", "./cmd/gateway"]),
  test: () => runStep("go", ["test", "./..."]),
  lint: () => {
    runStep("go", ["vet", "./..."]);
    checkFormatting();
  },
  // Compiles every package and discards the result.
  typecheck: () => runStep("go", ["build", "./..."]),
  format: () => runStep("gofmt", ["-w", "."]),
  "format:check": checkFormatting,
};

const commandName = process.argv[2];
// Object.hasOwn keeps inherited names such as "toString" from being treated as commands.
if (commandName === undefined || !Object.hasOwn(commands, commandName)) {
  console.error(`[gateway] usage: node scripts/go.mjs <${Object.keys(commands).join(" | ")}>`);
  process.exit(2);
}
commands[commandName]();
