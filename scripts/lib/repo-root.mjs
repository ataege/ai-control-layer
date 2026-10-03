// Resolves the repository root from this file's location (scripts/lib -> root),
// so every script works regardless of the directory it is started from.
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const libraryDirectory = dirname(fileURLToPath(import.meta.url));

export const repositoryRoot = resolve(libraryDirectory, "..", "..");

/** Absolute path of a file or directory inside the repository. */
export function fromRepositoryRoot(...pathSegments) {
  return resolve(repositoryRoot, ...pathSegments);
}

export const rootEnvFilePath = fromRepositoryRoot(".env");
export const rootEnvExamplePath = fromRepositoryRoot(".env.example");
