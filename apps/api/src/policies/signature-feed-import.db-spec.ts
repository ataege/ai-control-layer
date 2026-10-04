// Database-backed test of the signature feed half of the policy import (API-34 feed part). Runs only
// through the database test command, against a migrated PostgreSQL. Every test runs in a transaction
// that is rolled back, so the immutable rows it creates never persist.
import "reflect-metadata";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { DataSource, type QueryRunner } from "typeorm";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { ControlCatalogPointer } from "./control-catalog-pointer.entity.js";
import { ControlCatalogRevision } from "./control-catalog-revision.entity.js";
import { importPolicyFile, type PolicyImportRequest } from "./policy-catalog-importer.js";
import { digestPolicyBytes } from "./policy-file.js";
import { SignatureFeedRevision } from "./signature-feed-revision.entity.js";

const configDirectory = resolve(import.meta.dirname, "../../../../config");
const policyText = readFileSync(resolve(configDirectory, "policy.yaml"), "utf8");
const feedBytes = readFileSync(resolve(configDirectory, "attack-signatures.json"));
const feedText = feedBytes.toString("utf8");
// The digest Go pins for the committed feed (internal/security feed_file_test.go).
const COMMITTED_FEED_DIGEST = "ff6ff4fef7e7091a50c1e416fab5a7b1aa825b55d783ada38de43b42a399d98c";

const encode = (text: string) => new TextEncoder().encode(text);
const digestOf = (text: string) => digestPolicyBytes(encode(text));
const signaturesOff = policyText.replace(
  /signature_match:\n(\s+)enabled: true/,
  "signature_match:\n$1enabled: false",
);
const policyNamingNextFeed = policyText.replace("revision: feed_v2", "revision: feed_v3");
const nextFeedText = feedText.replace('"revision": "feed_v2"', '"revision": "feed_v3"');

function request(policy: string, feed?: string): PolicyImportRequest {
  return {
    sourceFileName: "policy.yaml",
    fileBytes: encode(policy),
    feed:
      feed === undefined
        ? undefined
        : { sourceFileName: "attack-signatures.json", fileBytes: encode(feed) },
  };
}

const dataSource = new DataSource({
  ...buildTypeOrmOptions(parseEnvironment(databaseEnvironmentSchema, process.env)),
  migrations: [],
});
let queryRunner: QueryRunner;

beforeAll(async () => {
  await dataSource.initialize();
});
afterAll(async () => {
  if (dataSource.isInitialized) await dataSource.destroy();
});
beforeEach(async () => {
  queryRunner = dataSource.createQueryRunner();
  await queryRunner.startTransaction();
  // Start from a catalog with no pointer row, as before the very first import.
  await queryRunner.manager.delete(ControlCatalogPointer, { id: 1 });
});
afterEach(async () => {
  await queryRunner.rollbackTransaction();
  await queryRunner.release();
});

const readPointer = () => queryRunner.manager.findOneByOrFail(ControlCatalogPointer, { id: 1 });
const counts = async () => ({
  revisions: await queryRunner.manager.count(ControlCatalogRevision),
  feeds: await queryRunner.manager.count(SignatureFeedRevision),
});

describe("importPolicyFile with the signature feed", () => {
  it("stores the exact feed bytes with the pinned digest and only requests the revision", async () => {
    const outcome = await importPolicyFile(queryRunner.manager, request(policyText, feedText));

    expect(outcome).toMatchObject({
      accepted: true,
      activeRevisionId: null,
      feedDigest: COMMITTED_FEED_DIGEST,
    });
    if (!outcome.accepted) return;
    const feed = await queryRunner.manager.findOneByOrFail(SignatureFeedRevision, {
      id: outcome.feedRevisionId!,
    });
    expect(feed).toMatchObject({
      issuer: "task-passport-security",
      revision: "feed_v2",
      fileDigest: COMMITTED_FEED_DIGEST,
      importSource: "command",
      importedBy: null,
    });
    expect(feed.sourceText).toBe(feedText);
    // The gateway validates the requested revision and activates it with its feed (GO-73).
    expect(await readPointer()).toMatchObject({
      requestedRevisionId: outcome.revisionId,
      activeRevisionId: null,
      activeFeedRevisionId: null,
    });
  });

  it("reuses the stored feed row when the same bytes are imported again", async () => {
    const first = await importPolicyFile(queryRunner.manager, request(policyText, feedText));
    const before = await counts();
    const second = await importPolicyFile(queryRunner.manager, request(policyText, feedText));

    expect(first.accepted && second.accepted).toBe(true);
    if (!first.accepted || !second.accepted) return;
    expect(second.feedRevisionId).toBe(first.feedRevisionId);
    expect((await counts()).feeds).toBe(before.feeds);
  });

  // Each refusal stores neither a catalog revision nor a feed row and leaves the pointer's
  // requested, active and feed bindings as they were.
  const refusals: [string, string, string | undefined, string][] = [
    [
      "changed bytes under an already stored feed revision",
      policyText,
      feedText.replace("Sample managed", "Changed managed"),
      "feed.revision",
    ],
    [
      "a feed from an issuer the gateway does not trust",
      policyText,
      feedText.replace('"issuer": "task-passport-security"', '"issuer": "other-publisher"'),
      "feed.issuer",
    ],
    [
      "a policy naming another feed revision",
      policyNamingNextFeed,
      feedText,
      "signatures.revision",
    ],
    [
      "a disabled rule the feed does not have",
      policyText.replace("disabled_rules: []", "disabled_rules: [no_such_rule_v1]"),
      feedText,
      "signatures.disabled_rules",
    ],
    [
      "a feed Go would refuse",
      policyText,
      feedText.replace('"normalized_substring"', '"regex"'),
      "feed.rules.0.pattern_type",
    ],
    ["a missing feed while signatures are enabled", policyText, undefined, "feed"],
  ];
  it.each(refusals)(
    "refuses %s and leaves the pointer unchanged",
    async (_name, policy, feed, issuePath) => {
      const accepted = await importPolicyFile(queryRunner.manager, request(policyText, feedText));
      if (!accepted.accepted) throw new Error("the sample policy and feed must be accepted");
      const pointerBefore = await readPointer();
      const before = await counts();

      const outcome = await importPolicyFile(queryRunner.manager, request(policy, feed));

      expect(outcome.accepted).toBe(false);
      if (outcome.accepted) return;
      expect(outcome.issues.map((issue) => issue.path)).toContain(issuePath);
      expect(await counts()).toEqual(before);
      const pointerAfter = await readPointer();
      expect(pointerAfter).toMatchObject({
        requestedRevisionId: pointerBefore.requestedRevisionId,
        activeRevisionId: pointerBefore.activeRevisionId,
        activeFeedRevisionId: pointerBefore.activeFeedRevisionId,
        lastError: { reason: "policy_reload_rejected" },
      });
    },
  );

  it("accepts a policy with signatures off without a feed", async () => {
    const outcome = await importPolicyFile(queryRunner.manager, request(signaturesOff));

    expect(outcome).toMatchObject({ accepted: true, feedRevisionId: null, feedDigest: null });
  });

  it("stores a new feed revision next to the old one without touching the active binding", async () => {
    const first = await importPolicyFile(queryRunner.manager, request(policyText, feedText));
    if (!first.accepted) throw new Error("the sample policy and feed must be accepted");
    // Stand in for the gateway's activation of the first revision and its feed (GO-73).
    await queryRunner.manager.update(
      ControlCatalogPointer,
      { id: 1 },
      {
        activeRevisionId: first.revisionId,
        activeFeedRevisionId: first.feedRevisionId,
      },
    );

    const later = await importPolicyFile(
      queryRunner.manager,
      request(policyNamingNextFeed, nextFeedText),
    );

    if (!later.accepted) throw new Error("the feed_v3 policy and feed must be accepted");
    expect(later.feedRevisionId).not.toBe(first.feedRevisionId);
    expect(await readPointer()).toMatchObject({
      requestedRevisionId: later.revisionId,
      activeRevisionId: first.revisionId,
      activeFeedRevisionId: first.feedRevisionId,
    });
  });

  // Go binds the feed by (trusted issuer, revision), so another issuer's row with the same revision is a
  // different feed: it is neither reused nor a reason to refuse the import.
  it("ignores a stored feed row of another issuer when reusing or refusing", async () => {
    const insertOtherIssuer = (sourceText: string) =>
      queryRunner.query(
        `INSERT INTO app.signature_feed_revisions
           (issuer, revision, source_file_name, source_text, file_digest, content, import_source)
         VALUES ('other-publisher', 'feed_v2', 'attack-signatures.json', $1, $2, '{}', 'command')
         RETURNING id::text`,
        [sourceText, digestOf(sourceText)],
      ) as Promise<{ id: string }[]>;
    // Same bytes under another issuer: not reused, a trusted row is stored beside it.
    const [other] = await insertOtherIssuer(feedText);
    const first = await importPolicyFile(queryRunner.manager, request(policyText, feedText));
    if (!first.accepted) throw new Error("the sample policy and feed must be accepted");
    expect(first.feedRevisionId).not.toBe(other?.id);
    const stored = await queryRunner.manager.findOneByOrFail(SignatureFeedRevision, {
      id: first.feedRevisionId!,
    });
    expect(stored.issuer).toBe("task-passport-security");
    // Another issuer holding other bytes under the revision does not make a re-import "different bytes".
    await queryRunner.query(`DELETE FROM app.control_catalog_pointer`);
    const second = await importPolicyFile(queryRunner.manager, request(policyText, feedText));
    expect(second.accepted && second.feedRevisionId).toBe(first.feedRevisionId);
  });

  it("keeps a stored feed row immutable", async () => {
    const outcome = await importPolicyFile(queryRunner.manager, request(policyText, feedText));
    if (!outcome.accepted) throw new Error("the sample policy and feed must be accepted");
    await queryRunner.query("SAVEPOINT immutable_feed_check");
    await expect(
      queryRunner.query(
        `UPDATE app.signature_feed_revisions SET source_text = 'changed' WHERE id = $1`,
        [outcome.feedRevisionId],
      ),
    ).rejects.toMatchObject({ code: "23001" });
    await queryRunner.query("ROLLBACK TO SAVEPOINT immutable_feed_check");
  });
});
