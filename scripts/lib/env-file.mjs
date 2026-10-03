// Reads the single root .env file. Real environment variables always win over the file.
import { existsSync, readFileSync } from "node:fs";
import { parseEnv } from "node:util";

import { rootEnvFilePath } from "./repo-root.mjs";

/** Parses an env file into a plain object, or returns null when the file does not exist. */
export function readEnvFile(envFilePath) {
  if (!existsSync(envFilePath)) return null;
  return { ...parseEnv(readFileSync(envFilePath, "utf8")) };
}

/** Merges file values under the real environment: a variable that is already set is kept. */
export function mergeEnvironment(fileValues, realEnvironment = process.env) {
  return { ...fileValues, ...realEnvironment };
}

/**
 * Loads the root .env and returns the environment to hand to child processes.
 * `fileFound` is false when .env is missing; the caller decides whether that is fatal.
 */
export function loadRootEnvironment(envFilePath = rootEnvFilePath) {
  const fileValues = readEnvFile(envFilePath);
  return {
    fileFound: fileValues !== null,
    environment: mergeEnvironment(fileValues ?? {}),
  };
}

export const MISSING_ENV_FILE_MESSAGE =
  "No .env file found in the repository root. Run `pnpm run setup` first to create it.";
