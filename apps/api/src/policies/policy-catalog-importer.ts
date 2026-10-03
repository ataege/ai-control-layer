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
import {
  TRUSTED_FEED_ISSUER,
  validateSignatureFeed,
  type SignatureFeed,
} from "./signature-feed.js";

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
      /**
       * True when the policy and its feed equal the current revision (validated, or requested and not
       * yet checked): nothing was written and `revisionId` is that current revision.
       */
      unchanged: boolean;
    }
  | {
      accepted: false;
      fileDigest: string;
      issues: PolicyFileIssue[];
      /** The last-known-good revision, unchanged by the rejection; null when none was ever accepted. */
      activeRevisionId: string | null;
      /**
       * Set when a valid import was refused only because this requested revision has not been checked
       * by the gateway yet: nothing was written, and the operator waits for activation and retries.
       */
      pendingRevisionId?: string;
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
    // The gateway's rejection of the revision that is still requested stays: it is the truth about
    // that request, and the gateway only recognizes its own record, so overwriting it would make the
    // next check validate and reject the same revision again and replace this record within a second.
    // The import's issues are always returned (the command prints them and exits 1); they are stored
    // on the pointer only when no such gateway rejection is pending.
    if (!holdsPendingGatewayRejection(pointer)) {
      pointer.lastError = {
        reason: POLICY_REJECTION_REASON,
        source_file_name: request.sourceFileName,
        file_digest: fileDigest,
        issues,
      };
      pointer.lastErrorAt = new Date();
      await transactionManager.save(pointer);
    }
    return { accepted: false, fileDigest, issues, activeRevisionId: pointer.activeRevisionId };
  };

  if (!validation.valid) {
    return reject(validation.fileDigest, validation.issues);
  }

  // Everything up to the first write is read-only: the feed is validated and compared with what is
  // stored, then the import may turn out to be unchanged or refused, and only then are rows written.
  // A policy whose feed Go would refuse never becomes a revision at all.
  let checkedFeed: CheckedFeed | null = null;
  if (request.feed !== undefined || validation.policy.controls.signature_match.enabled) {
    const feedResult = await checkSignatureFeed(
      transactionManager,
      validation.policy,
      request.feed,
    );
    if (!feedResult.ok) {
      return reject(validation.fileDigest, feedResult.issues);
    }
    checkedFeed = feedResult;
  }

  // An import that changes nothing is a no-op. A new revision with the same digest would move the
  // pointer, and pending work is bound to the old revision: the executor would refuse it
  // (source_policy_changed), so a judge's repeated import of an unchanged file during a review wait
  // would void the approval. "Unchanged" means the same policy digest and the same feed (issuer,
  // revision, digest) as the current revision, and only a sound current revision counts: validated by
  // the gateway, or requested and still waiting for it. A rejected request, or an active revision an
  // old import bootstrapped without validation, needs a fresh revision to be retried or healed.
  const current = await unchangedCurrentRevision(
    transactionManager,
    pointer,
    validation.fileDigest,
    checkedFeed,
  );
  if (current !== null) {
    return {
      accepted: true,
      unchanged: true,
      revisionId: current.id,
      fileDigest: validation.fileDigest,
      activeRevisionId: pointer.activeRevisionId,
      feedRevisionId: checkedFeed?.existing?.id ?? null,
      feedDigest: checkedFeed?.existing?.fileDigest ?? null,
    };
  }

  // A second import inside one activation tick could replace a request the gateway has not checked
  // (good A, then bad B: A is never validated, B is rejected, the old revision stays). Refuse until
  // the pending request has been checked; nothing is written, and last_error is left alone.
  const pendingRevisionId = pendingUncheckedRevision(pointer);
  if (pendingRevisionId !== null) {
    return {
      accepted: false,
      fileDigest: validation.fileDigest,
      issues: [
        {
          path: "(import)",
          message: `revision ${pendingRevisionId} is still being validated; wait for activation and retry (without a running gateway, run pnpm catalog:activate)`,
        },
      ],
      activeRevisionId: pointer.activeRevisionId,
      pendingRevisionId,
    };
  }

  let feedRevision: SignatureFeedRevision | null = null;
  if (checkedFeed !== null) {
    feedRevision = await storeCheckedFeed(transactionManager, checkedFeed);
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
    unchanged: false,
    revisionId: revision.id,
    fileDigest: validation.fileDigest,
    activeRevisionId: pointer.activeRevisionId,
    feedRevisionId: feedRevision?.id ?? null,
    feedDigest: feedRevision?.fileDigest ?? null,
  };
}

/** True when last_error is the gateway's own rejection (stage gateway_validation) of the requested revision. */
function holdsPendingGatewayRejection(pointer: ControlCatalogPointer): boolean {
  const record = pointer.lastError;
  return (
    record !== null &&
    pointer.requestedRevisionId !== null &&
    record.stage === "gateway_validation" &&
    String(record.revision_id) === pointer.requestedRevisionId
  );
}

/**
 * The requested revision when it is still waiting for the gateway: requested, not the active one (an
 * old import's first revision is requested = active), not validated, and without a recorded gateway
 * rejection. null otherwise.
 */
function pendingUncheckedRevision(pointer: ControlCatalogPointer): string | null {
  const requested = pointer.requestedRevisionId;
  if (
    requested === null ||
    requested === pointer.activeRevisionId ||
    requested === pointer.validatedRevisionId ||
    holdsPendingGatewayRejection(pointer)
  ) {
    return null;
  }
  return requested;
}

/**
 * The current revision (requested, else active) when this import would change nothing: it is sound (see
 * the caller), its policy digest equals the file's and the feed in play is the stored one. null when
 * the import must create a revision.
 */
async function unchangedCurrentRevision(
  transactionManager: EntityManager,
  pointer: ControlCatalogPointer,
  policyDigest: string,
  checkedFeed: CheckedFeed | null,
): Promise<ControlCatalogRevision | null> {
  const currentId = pointer.requestedRevisionId ?? pointer.activeRevisionId;
  if (currentId === null) return null;
  const sound =
    currentId === pointer.validatedRevisionId || currentId === pendingUncheckedRevision(pointer);
  if (!sound) return null;
  const current = await transactionManager.findOneBy(ControlCatalogRevision, { id: currentId });
  if (current === null || current.fileDigest !== policyDigest) return null;
  // The same policy names the same feed revision; it is the same feed when the stored row of the
  // trusted issuer for that revision has the digest of the feed file. No feed in play: nothing differs.
  if (checkedFeed !== null && checkedFeed.existing?.fileDigest !== checkedFeed.fileDigest)
    return null;
  return current;
}

type FeedCheck = ({ ok: true } & CheckedFeed) | { ok: false; issues: PolicyFileIssue[] };

/** A feed that passed every check, with the stored row it equals (null: not stored yet). */
interface CheckedFeed {
  feed: SignatureFeed;
  sourceFileName: string;
  sourceText: string;
  fileDigest: string;
  existing: SignatureFeedRevision | null;
}

/**
 * Validates the feed against the Go grammar and the policy that names it and looks up the stored row of
 * the trusted issuer for its revision, without writing anything. The same revision with different bytes
 * is refused: changed rules need a new revision.
 */
async function checkSignatureFeed(
  transactionManager: EntityManager,
  policy: PolicyFile,
  feedFile: PolicyImportRequest["feed"],
): Promise<FeedCheck> {
  if (feedFile === undefined) {
    return { ok: false, issues: [{ path: "feed", message: "signature feed file is missing" }] };
  }
  const feedValidation = validateSignatureFeed(feedFile.fileBytes);
  if (!feedValidation.valid) {
    return { ok: false, issues: feedValidation.issues };
  }
  const mismatch = feedMismatch(feedValidation.feed, policy.signatures);
  if (mismatch !== null) {
    return { ok: false, issues: [mismatch] };
  }
  const { feed, sourceText, fileDigest } = feedValidation;
  // Go's activation binds the feed by (trusted issuer, revision) (catalog/activation.go), so reuse is
  // looked up the same way: a row of another issuer with the same revision is a different feed. The
  // table's unique (issuer, revision) makes this at most one row; equal bytes reuse it, other bytes
  // under the same revision are refused (changed rules need a new revision).
  const existing = await transactionManager.findOneBy(SignatureFeedRevision, {
    issuer: TRUSTED_FEED_ISSUER,
    revision: feed.revision,
  });
  if (existing !== null && existing.fileDigest !== fileDigest) {
    return {
      ok: false,
      issues: [
        {
          path: "feed.revision",
          message:
            "a different feed is stored under this revision (for example from a manual load); recreate the database, or bump the revision if the rules really changed",
        },
      ],
    };
  }
  return {
    ok: true,
    feed,
    sourceFileName: feedFile.sourceFileName,
    sourceText,
    fileDigest,
    existing,
  };
}

/** Stores a checked feed (exact bytes, with their SHA-256), or returns the stored row it equals. */
async function storeCheckedFeed(
  transactionManager: EntityManager,
  checked: CheckedFeed,
): Promise<SignatureFeedRevision> {
  if (checked.existing !== null) return checked.existing;
  return transactionManager.save(
    transactionManager.create(SignatureFeedRevision, {
      issuer: checked.feed.issuer,
      revision: checked.feed.revision,
      sourceFileName: checked.sourceFileName,
      sourceText: checked.sourceText,
      fileDigest: checked.fileDigest,
      content: checked.feed,
      importSource: "command",
      importedBy: null,
    }),
  );
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
