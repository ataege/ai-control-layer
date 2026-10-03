import { ConsoleLogger, type LogLevel } from "@nestjs/common";

const NEST_LEVELS_BY_LOG_LEVEL: Record<string, LogLevel[]> = {
  debug: ["verbose", "debug", "log", "warn", "error", "fatal"],
  info: ["log", "warn", "error", "fatal"],
  warn: ["warn", "error", "fatal"],
  error: ["error", "fatal"],
};

/** JSON logger for the whole app. Unknown levels fall back to "info"; validation reports them later. */
export function createApplicationLogger(logLevel: string | undefined): ConsoleLogger {
  const logLevels = NEST_LEVELS_BY_LOG_LEVEL[logLevel ?? "info"] ?? NEST_LEVELS_BY_LOG_LEVEL.info;
  return new ConsoleLogger({ json: true, colors: false, logLevels });
}
