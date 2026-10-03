import type { EntityManager } from "typeorm";
import { ControlCatalogPointer } from "./control-catalog-pointer.entity.js";
import { ControlCatalogRevision } from "./control-catalog-revision.entity.js";
import {
  POLICY_REJECTION_REASON,
  validatePolicyFile,
  type PolicyFileIssue,
} from "./policy-file.js";

/** The single pointer row (CHECK id = 1 in the migration). */
const CATALOG_POINTER_ID = 1;

/** How long an import waits for the pointer row lock held by another import or reload. */
const POINTER_LOCK_TIMEOUT = "5s";

export interface PolicyImportRequest {
  /** File name only, for operators and the audit trail; never used to open anything. */
  sourceFileName: string;
  fileBytes: Uint8Array;
}

export type PolicyImportOutcome =
  | {
      accepted: true;
      revisionId: string;
      fileDigest: string;
      /** True when this import became the active revision (only the very first valid import does). */
      activated: boolean;
      activeRevisionId: string;
    }
  | {
      accepted: false;
      fileDigest: string;
      issues: PolicyFileIssue[];
      /** The last-known-good revision, unchanged by the rejection; null when none was ever accepted. */
      activeRevisionId: string | null;
    };

/**
 * Imports policy.yaml through the explicit command (API-32). A valid file becomes a new immutable
 * revision and the requested revision; the first valid import also becomes active. Later imports wait
 * for the reload flow (API-33, `catalog activation protocol`) to activate them. An invalid file never
 * becomes a revision: the rejection is recorded on the pointer and the active revision stays.
 *
 * The caller supplies a transactional EntityManager, so the revision and the pointer change commit
 * together or not at all.
 */
export async function importPolicyFile(
  transactionManager: EntityManager,
  request: PolicyImportRequest,
): Promise<PolicyImportOutcome> {
  if (!transactionManager.queryRunner?.isTransactionActive) {
    throw new Error("importPolicyFile must run inside a database transaction");
  }

  const validation = validatePolicyFile(request.fileBytes);
  // Fail with lock_not_available instead of waiting forever behind another import or reload.
  await transactionManager.query(`SET LOCAL lock_timeout = '${POINTER_LOCK_TIMEOUT}'`);
  const pointer = await lockCatalogPointer(transactionManager);

  if (!validation.valid) {
    pointer.lastError = {
      reason: POLICY_REJECTION_REASON,
      source_file_name: request.sourceFileName,
      file_digest: validation.fileDigest,
      issues: validation.issues,
    };
    pointer.lastErrorAt = new Date();
    await transactionManager.save(pointer);
    return {
      accepted: false,
      fileDigest: validation.fileDigest,
      issues: validation.issues,
      activeRevisionId: pointer.activeRevisionId,
    };
  }

  const revision = await transactionManager.save(
    transactionManager.create(ControlCatalogRevision, {
      schemaVersion: validation.policy.schema_version,
      sourceFileName: request.sourceFileName,
      sourceText: validation.sourceText,
      fileDigest: validation.fileDigest,
      content: validation.policy,
      // The explicit command has no authenticated actor; the reload (API-33) records its actor.
      importSource: "command",
      importedBy: null,
    }),
  );

  const activated = pointer.activeRevisionId === null;
  pointer.requestedRevisionId = revision.id;
  if (activated) {
    pointer.activeRevisionId = revision.id;
  }
  pointer.lastError = null;
  pointer.lastErrorAt = null;
  await transactionManager.save(pointer);

  return {
    accepted: true,
    revisionId: revision.id,
    fileDigest: validation.fileDigest,
    activated,
    activeRevisionId: pointer.activeRevisionId ?? revision.id,
  };
}

/** Creates the pointer row on first use and locks it, so concurrent imports apply one at a time. */
async function lockCatalogPointer(
  transactionManager: EntityManager,
): Promise<ControlCatalogPointer> {
  await transactionManager
    .createQueryBuilder()
    .insert()
    .into(ControlCatalogPointer)
    .values({ id: CATALOG_POINTER_ID })
    .orIgnore()
    .execute();
  return transactionManager.findOneOrFail(ControlCatalogPointer, {
    where: { id: CATALOG_POINTER_ID },
    lock: { mode: "pessimistic_write" },
  });
}
