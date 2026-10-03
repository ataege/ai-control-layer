// The `docker compose` arguments that select this repository's project: the root as project
// directory, the root .env (which may set COMPOSE_PROJECT_NAME) and the Compose file(s).
import { fromRepositoryRoot, repositoryRoot, rootEnvFilePath } from "./repo-root.mjs";

/** Arguments that follow `docker`; `debug` adds the override that publishes the gateway port. */
export function composeProjectArguments({ debug = false } = {}) {
  const composeFileArguments = ["-f", fromRepositoryRoot("infra", "compose.yaml")];
  if (debug) composeFileArguments.push("-f", fromRepositoryRoot("infra", "compose.debug.yaml"));
  return [
    "compose",
    "--project-directory",
    repositoryRoot,
    "--env-file",
    rootEnvFilePath,
    ...composeFileArguments,
  ];
}
