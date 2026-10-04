// Entry point of the explicit policy import command (API-32), started by scripts/import-policy.mjs.
// It is never imported by the Nest app: nothing imports a policy at application startup.
import "reflect-metadata";
import { DataSource } from "typeorm";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { importPolicyFile } from "./policy-catalog-importer.js";
import { readPolicyFiles } from "./read-policy-files.js";

/** PostgreSQL lock_not_available: another import or reload holds the catalog pointer. */
const LOCK_NOT_AVAILABLE = "55P03";

const [policyFilePath] = process.argv.slice(2);
if (policyFilePath === undefined) {
  console.error("Usage: import-policy.command.ts <path to policy.yaml>");
  process.exit(2);
}

const dataSource = new DataSource({
  ...buildTypeOrmOptions(parseEnvironment(databaseEnvironmentSchema, process.env)),
  logging: false,
});

try {
  const files = await readPolicyFiles(policyFilePath);
  await dataSource.initialize();
  const outcome = await dataSource.transaction((transactionManager) =>
    importPolicyFile(transactionManager, files),
  );

  if (outcome.accepted && outcome.unchanged) {
    // Nothing was written: the file equals the current revision, so work bound to it stays valid.
    console.log(
      `unchanged: revision ${outcome.revisionId} is already current (sha256 ${outcome.fileDigest}); nothing was written.`,
    );
    process.exitCode = 0;
  } else if (outcome.accepted) {
    console.log(
      `Accepted ${policyFilePath} as catalog revision ${outcome.revisionId} (sha256 ${outcome.fileDigest}).`,
    );
    if (outcome.feedRevisionId !== null) {
      console.log(
        `Signature feed stored as feed revision ${outcome.feedRevisionId} (sha256 ${outcome.feedDigest}).`,
      );
    }
    console.log(
      `requested revision ${outcome.revisionId}; the gateway validates and activates it` +
        (outcome.activeRevisionId === null
          ? " (no revision is active yet)."
          : ` (revision ${outcome.activeRevisionId} stays active until then).`),
    );
    process.exitCode = 0;
  } else if (outcome.pendingRevisionId !== undefined) {
    // A valid file, refused only because the gateway has not checked the pending request yet.
    console.error(
      `Not imported: revision ${outcome.pendingRevisionId} is still being validated; wait for activation and retry (without a running gateway, run pnpm catalog:activate). Nothing was written.`,
    );
    process.exitCode = 1;
  } else {
    console.error(
      `Rejected ${policyFilePath} (policy_reload_rejected, sha256 ${outcome.fileDigest}):`,
    );
    for (const issue of outcome.issues) {
      console.error(`  ${issue.path}: ${issue.message}`);
    }
    console.error(
      outcome.activeRevisionId === null
        ? "No revision has ever been accepted, so no catalog is active."
        : `Revision ${outcome.activeRevisionId} stays active.`,
    );
    process.exitCode = 1;
  }
} catch (error) {
  // Connection and file errors: name the failure, never the credentials.
  const errorCode = (error as { code?: unknown }).code;
  console.error(
    errorCode === LOCK_NOT_AVAILABLE
      ? "Policy import failed: the catalog pointer is locked by another import or reload; try again."
      : `Policy import failed: ${typeof errorCode === "string" ? errorCode : (error as Error).name}`,
  );
  process.exitCode = 1;
} finally {
  if (dataSource.isInitialized) {
    await dataSource.destroy();
  }
}
