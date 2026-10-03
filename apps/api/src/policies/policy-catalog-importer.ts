import type { EntityManager } from "typeorm";
import { ControlCatalogPointer } from "./control-catalog-pointer.entity.js";
import { ControlCatalogRevision } from "./control-catalog-revision.entity.js";
import {
  POLICY_REJECTION_REASON,
  validatePolicyFile,
  type PolicyFile,
  type PolicyFileIssue,
} from "./policy-file.js";
import { SignatureFeedRevision } from "./signature-feed-revision.entity.js";
import { validateSignatureFeed, type SignatureFeed } from "./signature-feed.js";

/** The single pointer row (CHECK id = 1 in the migration). */
const CATALOG_POINTER_ID = 1;

/** How long an import waits for the pointer row lock held by another import or reload. */
const POINTER_LOCK_TIMEOUT = "5s";

export interface PolicyImportRequest {
  /** File name only, for operators and the audit trail; never used to open anything. */
  sourceFileName: string;
  fileBytes: Uint8Array;
  /**
   * The signature feed the policy names in `signatures.path`. Required while `signature_match` is
   * enabled; the policy and its feed are accepted or rejected together.
   */
  feed?: { sourceFileName: string; fileBytes: Uint8Array };
}

export type PolicyImportOutcome =
  | {
      accepted: true;
      /** The new revision, now the requested revision; Go validates and activates it (GO-73). */
      revisionId: string;
      fileDigest: string;
      /** The active revision, unchanged by the import; null before the gateway's first activation. */
      activeRevisionId: string | null;
      /** The stored feed revision row (new or reused), or null when the policy imports no feed. */
      feedRevisionId: string | null;
      feedDigest: string | null;
    }
  | {
      accepted: false;
      fileDigest: string;
      issues: PolicyFileIssue[];
      /** The last-known-good revision, unchanged by the rejection; null when none was ever accepted. */
      activeRevisionId: string | null;
    };

/**
 * Imports policy.yaml and the signature feed it names through the explicit command (API-32, API-34
 * feed part). A valid policy with a valid, matching feed becomes a new immutable catalog revision and
 * the requested revision; the feed's exact bytes are stored (or the stored row with the same bytes is
 * reused). The import never activates: the gateway validates the requested revision and switches the
 * active revision and feed together (GO-73, `catalog activation protocol`). An invalid policy or feed
 * stores nothing: the rejection is recorded on the pointer and the active revision stays.
 *
 * The caller supplies a transactional EntityManager, so the rows and the pointer change commit
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

  const reject = async (
    fileDigest: string,
    issues: PolicyFileIssue[],
  ): Promise<PolicyImportOutcome> => {
    pointer.lastError = {
      reason: POLICY_REJECTION_REASON,
      source_file_name: request.sourceFileName,
      file_digest: fileDigest,
      issues,
    };
    pointer.lastErrorAt = new Date();
    await transactionManager.save(pointer);
    return { accepted: false, fileDigest, issues, activeRevisionId: pointer.activeRevisionId };
  };

  if (!validation.valid) {
    return reject(validation.fileDigest, validation.issues);
  }

  // The feed is validated and stored before the revision, so a policy whose feed Go would refuse
  // never becomes a revision at all.
  let feedRevision: SignatureFeedRevision | null = null;
  if (request.feed !== undefined || validation.policy.controls.signature_match.enabled) {
    const feedResult = await storeSignatureFeed(
      transactionManager,
      validation.policy,
      request.feed,
    );
    if (!feedResult.stored) {
      return reject(validation.fileDigest, feedResult.issues);
    }
    feedRevision = feedResult.revision;
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

  pointer.requestedRevisionId = revision.id;
  pointer.lastError = null;
  pointer.lastErrorAt = null;
  await transactionManager.save(pointer);

  return {
    accepted: true,
    revisionId: revision.id,
    fileDigest: validation.fileDigest,
    activeRevisionId: pointer.activeRevisionId,
    feedRevisionId: feedRevision?.id ?? null,
    feedDigest: feedRevision?.fileDigest ?? null,
  };
}

type FeedStoreResult =
  { stored: true; revision: SignatureFeedRevision } | { stored: false; issues: PolicyFileIssue[] };

/**
 * Validates the feed against the Go grammar and the policy that names it, then stores its exact
 * bytes, or reuses the stored row when the same issuer and revision already hold the same bytes.
 * The same revision with different bytes is refused: changed rules need a new revision.
 */
async function storeSignatureFeed(
  transactionManager: EntityManager,
  policy: PolicyFile,
  feedFile: PolicyImportRequest["feed"],
): Promise<FeedStoreResult> {
  if (feedFile === undefined) {
    return { stored: false, issues: [{ path: "feed", message: "signature feed file is missing" }] };
  }
  const feedValidation = validateSignatureFeed(feedFile.fileBytes);
  if (!feedValidation.valid) {
    return { stored: false, issues: feedValidation.issues };
  }
  const mismatch = feedMismatch(feedValidation.feed, policy.signatures);
  if (mismatch !== null) {
    return { stored: false, issues: [mismatch] };
  }
  const { feed, sourceText, fileDigest } = feedValidation;
  const existing = await transactionManager.findOneBy(SignatureFeedRevision, {
    issuer: feed.issuer,
    revision: feed.revision,
  });
  if (existing !== null) {
    if (existing.fileDigest !== fileDigest) {
      return {
        stored: false,
        issues: [
          {
            path: "feed.revision",
            message: "this feed revision is already stored with different bytes; bump the revision",
          },
        ],
      };
    }
    return { stored: true, revision: existing };
  }
  const revision = await transactionManager.save(
    transactionManager.create(SignatureFeedRevision, {
      issuer: feed.issuer,
      revision: feed.revision,
      sourceFileName: feedFile.sourceFileName,
      sourceText,
      fileDigest,
      content: feed,
      importSource: "command",
      importedBy: null,
    }),
  );
  return { stored: true, revision };
}

/** The checks Go's SettingsFromCatalog makes between a policy and its feed. */
function feedMismatch(
  feed: SignatureFeed,
  signatures: PolicyFile["signatures"],
): PolicyFileIssue | null {
  if (feed.revision !== signatures.revision) {
    return { path: "signatures.revision", message: "the feed file holds a different revision" };
  }
  const ruleIds = new Set(feed.rules.map((rule) => rule.id));
  if (signatures.disabled_rules.some((ruleId) => !ruleIds.has(ruleId))) {
    return { path: "signatures.disabled_rules", message: "a disabled rule is not in the feed" };
  }
  return null;
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
