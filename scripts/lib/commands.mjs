// Helpers to check for external tools and to run a command to completion.
import { spawn, spawnSync } from "node:child_process";
import { constants as osConstants } from "node:os";

/**
 * Runs `<command> <args>` and returns its trimmed stdout, or null when the command
 * is missing, fails or does not answer in time. Never throws.
 */
export function readCommandOutput(command, commandArguments, { env = process.env, cwd } = {}) {
  const result = spawnSync(command, commandArguments, {
    env,
    cwd,
    encoding: "utf8",
    timeout: 15_000,
    stdio: ["ignore", "pipe", "ignore"],
  });
  if (result.error || result.status !== 0) return null;
  return result.stdout.trim();
}

/** True when the executable can be started from PATH. */
export function commandExists(
  command,
  { env = process.env, versionArguments = ["--version"] } = {},
) {
  const result = spawnSync(command, versionArguments, { env, stdio: "ignore", timeout: 15_000 });
  return !result.error;
}

/** Conventional shell exit code for a process ended by a signal (128 + signal number). */
export function exitCodeForSignal(signalName) {
  return 128 + (osConstants.signals[signalName] ?? 0);
}

const FORWARDED_SIGNALS = ["SIGINT", "SIGTERM", "SIGHUP"];

/**
 * Runs a command with inherited stdio and resolves with its exit code.
 * The child stays in our process group, so Ctrl+C reaches it directly; signals sent
 * only to this process are forwarded. A missing executable resolves with 127.
 */
export function runCommand(command, commandArguments, { env = process.env, cwd } = {}) {
  return new Promise((resolveExitCode) => {
    const child = spawn(command, commandArguments, { env, cwd, stdio: "inherit", shell: process.platform === "win32" });
    const signalForwarders = new Map();

    for (const signalName of FORWARDED_SIGNALS) {
      const forwardSignal = () => {
        if (child.exitCode === null && child.signalCode === null) child.kill(signalName);
      };
      signalForwarders.set(signalName, forwardSignal);
      process.on(signalName, forwardSignal);
    }
    const removeSignalForwarders = () => {
      for (const [signalName, forwardSignal] of signalForwarders) {
        process.off(signalName, forwardSignal);
      }
    };

    child.on("error", (spawnError) => {
      removeSignalForwarders();
      const reason =
        spawnError.code === "ENOENT" ? `"${command}" was not found on PATH` : spawnError.message;
      console.error(`Cannot run "${command}": ${reason}`);
      resolveExitCode(spawnError.code === "ENOENT" ? 127 : 126);
    });
    child.on("exit", (exitCode, signalName) => {
      removeSignalForwarders();
      resolveExitCode(signalName ? exitCodeForSignal(signalName) : (exitCode ?? 1));
    });
  });
}
