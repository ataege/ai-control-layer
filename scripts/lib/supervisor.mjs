// Runs several long-lived processes and guarantees they all stop together.
// Each service gets its own process group so the whole tree (pnpm -> next -> workers,
// pnpm -> gateway wrapper -> compiled binary) can be signalled at once.
// POSIX only (macOS, Linux, WSL): negative-pid kill does not exist on native Windows.
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";

import { exitCodeForSignal } from "./commands.mjs";

const PREFIX_COLORS = ["\x1b[36m", "\x1b[35m", "\x1b[33m", "\x1b[32m", "\x1b[34m"];
const COLOR_RESET = "\x1b[0m";

/**
 * Starts every service and resolves with the exit code the caller should use once all
 * of them have stopped: the signal's conventional code after Ctrl+C / SIGTERM, otherwise
 * non-zero because a dev service is never expected to exit on its own.
 *
 * A service may carry its own `env`; it replaces the shared one for that child.
 *
 * @param {{ name: string, command: string, commandArguments?: string[], cwd?: string,
 *   env?: NodeJS.ProcessEnv }[]} services
 * @param {{ env?: NodeJS.ProcessEnv, gracePeriodMs?: number, label?: string }} [options]
 * @returns {Promise<number>}
 */
export function superviseServices(
  services,
  { env = process.env, gracePeriodMs = 8000, label = "dev" } = {},
) {
  const useColor = Boolean(process.stdout.isTTY) && !env.NO_COLOR;
  const longestNameLength = Math.max(...services.map((service) => service.name.length));
  const runningChildren = new Map();
  let isShuttingDown = false;
  let finalExitCode = 0;

  return new Promise((resolveExitCode) => {
    const signalHandlers = new Map();

    const finishWhenAllStopped = () => {
      if (runningChildren.size > 0) return;
      for (const [signalName, signalHandler] of signalHandlers) {
        process.off(signalName, signalHandler);
      }
      resolveExitCode(finalExitCode);
    };

    const pipeWithPrefix = (stream, serviceName, colorIndex, target) => {
      const paddedName = serviceName.padEnd(longestNameLength);
      const prefix = useColor
        ? `${PREFIX_COLORS[colorIndex % PREFIX_COLORS.length]}${paddedName} |${COLOR_RESET} `
        : `${paddedName} | `;
      createInterface({ input: stream }).on("line", (line) => target.write(`${prefix}${line}\n`));
    };

    const signalGroup = (child, signalName) => {
      try {
        // Negative pid = the whole process group led by the child.
        process.kill(-child.pid, signalName);
      } catch (killError) {
        // The group is already gone (macOS may report EPERM for a group of zombies).
        if (killError.code !== "ESRCH" && killError.code !== "EPERM") throw killError;
      }
    };

    const shutdown = (reason, exitCode) => {
      if (isShuttingDown) return;
      isShuttingDown = true;
      finalExitCode = exitCode;
      console.error(`\n[${label}] ${reason}; stopping ${runningChildren.size} service(s)...`);
      for (const child of runningChildren.values()) signalGroup(child, "SIGTERM");
      // Anything that ignores SIGTERM is force-killed after the grace period.
      const forceKillTimer = setTimeout(() => {
        for (const [serviceName, child] of runningChildren) {
          console.error(
            `[${label}] ${serviceName} did not stop within ${gracePeriodMs} ms, sending SIGKILL`,
          );
          signalGroup(child, "SIGKILL");
        }
      }, gracePeriodMs);
      forceKillTimer.unref();
      finishWhenAllStopped();
    };

    services.forEach((service, serviceIndex) => {
      const child = spawn(service.command, service.commandArguments ?? [], {
        cwd: service.cwd,
        env: { ...(service.env ?? env), FORCE_COLOR: useColor ? "1" : "0" },
        detached: true, // own process group; Ctrl+C reaches it only through shutdown()
        stdio: ["ignore", "pipe", "pipe"],
      });
      runningChildren.set(service.name, child);
      pipeWithPrefix(child.stdout, service.name, serviceIndex, process.stdout);
      pipeWithPrefix(child.stderr, service.name, serviceIndex, process.stderr);

      child.on("error", (spawnError) => {
        runningChildren.delete(service.name);
        const reason =
          spawnError.code === "ENOENT"
            ? `"${service.command}" was not found on PATH`
            : spawnError.message;
        shutdown(`${service.name} could not start (${reason})`, 1);
        finishWhenAllStopped();
      });

      child.on("exit", (exitCode, signalName) => {
        // Sweep stragglers the service leader left behind in its group.
        signalGroup(child, "SIGKILL");
        if (!isShuttingDown) {
          const exitDescription = signalName ? `signal ${signalName}` : `exit code ${exitCode}`;
          // An unexpected exit is a failure even with code 0: dev services run until stopped.
          shutdown(`${service.name} stopped unexpectedly (${exitDescription})`, exitCode || 1);
        }
      });

      // "close" fires after the output streams are drained, so the last lines are not lost.
      child.on("close", () => {
        runningChildren.delete(service.name);
        finishWhenAllStopped();
      });
    });

    for (const signalName of ["SIGINT", "SIGTERM", "SIGHUP"]) {
      const signalHandler = () => shutdown(`received ${signalName}`, exitCodeForSignal(signalName));
      signalHandlers.set(signalName, signalHandler);
      process.on(signalName, signalHandler);
    }
  });
}
