// Entry point of the explicit policy import command (API-32), started by scripts/import-policy.mjs.
// It is never imported by the Nest app: nothing imports a policy at application startup.
import "reflect-metadata";
import { open } from "node:fs/promises";
import { basename } from "node:path";
import { DataSource } from "typeorm";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { importPolicyFile } from "./policy-catalog-importer.js";
import { POLICY_FILE_MAX_BYTES } from "./policy-file.js";

/** PostgreSQL lock_not_available: another import or reload holds the catalog pointer. */
const LOCK_NOT_AVAILABLE = "55P03";

/**
 * Reads at most one byte more than the size bound, and only from a regular file, so a huge file or a
 * device never gets buffered. The validator then rejects anything above the bound.
 */
async function readBoundedPolicyFile(filePath: string): Promise<Uint8Array> {
  const fileHandle = await open(filePath, "r");
  try {
    if (!(await fileHandle.stat()).isFile()) {
      throw Object.assign(new Error("not a regular file"), { code: "ENOTREGULAR" });
    }
    const buffer = new Uint8Array(POLICY_FILE_MAX_BYTES + 1);
    let bytesRead = 0;
    while (bytesRead < buffer.byteLength) {
      const { bytesRead: chunkLength } = await fileHandle.read(
        buffer,
        bytesRead,
        buffer.byteLength - bytesRead,
      );
      if (chunkLength === 0) break;
      bytesRead += chunkLength;
    }
    return buffer.subarray(0, bytesRead);
  } finally {
    await fileHandle.close();
  }
}

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
  const fileBytes = await readBoundedPolicyFile(policyFilePath);
  await dataSource.initialize();
  const outcome = await dataSource.transaction((transactionManager) =>
    importPolicyFile(transactionManager, {
      sourceFileName: basename(policyFilePath),
      fileBytes,
    }),
  );

  if (outcome.accepted) {
    console.log(
      `Accepted ${policyFilePath} as catalog revision ${outcome.revisionId} (sha256 ${outcome.fileDigest}).`,
    );
    console.log(
      outcome.activated
        ? `Revision ${outcome.revisionId} is the first accepted revision and is now active.`
        : `Revision ${outcome.revisionId} is requested; revision ${outcome.activeRevisionId} stays active until the reload activates the new one.`,
    );
    process.exitCode = 0;
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
