// Shared helpers for the web launchers (dev.mjs and start.mjs).
import { spawn } from "node:child_process";
import path from "node:path";

const DEFAULT_WEB_PORT = 3000;
const FORWARDED_SIGNALS = ["SIGINT", "SIGTERM", "SIGHUP"];

/** Absolute path of apps/web, independent of the caller's working directory. */
export const webAppDirectory = path.resolve(import.meta.dirname, "../..");

/** Port from WEB_PORT, or 3000 when it is unset. Exits on an invalid value. */
export function resolveWebPort() {
  const rawPort = process.env.WEB_PORT;
  if (rawPort === undefined || rawPort.trim() === "") {
    return DEFAULT_WEB_PORT;
  }
  const port = Number(rawPort);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    console.error("[web] WEB_PORT must be a whole number between 1 and 65535.");
    process.exit(1);
  }
  return port;
}

/**
 * Runs a script with the current Node binary and resolves with its exit code.
 * Termination signals are passed on so the child never outlives the launcher.
 */
export function runNodeScript(scriptPath, scriptArguments, { cwd, env }) {
  return new Promise((resolve) => {
    const childProcess = spawn(process.execPath, [scriptPath, ...scriptArguments], {
      cwd,
      env,
      stdio: "inherit",
    });

    const signalHandlers = FORWARDED_SIGNALS.map((signalName) => {
      const forwardSignal = () => childProcess.kill(signalName);
      process.on(signalName, forwardSignal);
      return [signalName, forwardSignal];
    });
    const removeSignalHandlers = () => {
      for (const [signalName, forwardSignal] of signalHandlers) {
        process.off(signalName, forwardSignal);
      }
    };

    childProcess.on("error", (spawnError) => {
      removeSignalHandlers();
      console.error(`[web] could not start ${scriptPath}: ${spawnError.message}`);
      resolve(1);
    });
    childProcess.on("exit", (exitCode, signalName) => {
      removeSignalHandlers();
      // A child stopped by a signal has no exit code; report a failure instead.
      resolve(exitCode ?? (signalName ? 1 : 0));
    });
  });
}
