// Database-backed test of the policy import (API-32). Runs only through the database test command
// (SH-21), against a migrated PostgreSQL. Every test runs in a transaction that is rolled back, so the
// immutable revision rows it creates never persist.
import "reflect-metadata";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { DataSource, type QueryRunner } from "typeorm";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";
import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { buildTypeOrmOptions } from "../database/typeorm-options.js";
import { ControlCatalogPointer } from "./control-catalog-pointer.entity.js";
import { ControlCatalogRevision } from "./control-catalog-revision.entity.js";
import { importPolicyFile } from "./policy-catalog-importer.js";
import { digestPolicyBytes, validatePolicyFile } from "./policy-file.js";

const samplePolicyBytes = readFileSync(
  resolve(import.meta.dirname, "../../../../config/policy.yaml"),
);
const invalidPolicyBytes = new TextEncoder().encode(
  samplePolicyBytes.toString("utf8").replace("calls_agent: 12", "calls_agent: 30"),
);

// Migrations are not loaded: the database must already be migrated (pnpm db:migration:run).
const dataSource = new DataSource({
  ...buildTypeOrmOptions(parseEnvironment(databaseEnvironmentSchema, process.env)),
  migrations: [],
});
let queryRunner: QueryRunner;

beforeAll(async () => {
  await dataSource.initialize();
});

afterAll(async () => {
  if (dataSource.isInitialized) {
    await dataSource.destroy();
  }
});

beforeEach(async () => {
  queryRunner = dataSource.createQueryRunner();
  await queryRunner.startTransaction();
});

afterEach(async () => {
  await queryRunner.rollbackTransaction();
  await queryRunner.release();
});

/** Starts from a catalog with no pointer row, as before the very first import. */
async function clearPointer(): Promise<void> {
  await queryRunner.manager.delete(ControlCatalogPointer, { id: 1 });
}

async function readPointer(): Promise<ControlCatalogPointer> {
  return queryRunner.manager.findOneByOrFail(ControlCatalogPointer, { id: 1 });
}

describe("importPolicyFile", () => {
  it("stores the first valid import as an immutable revision and activates it", async () => {
    await clearPointer();

    const outcome = await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy.yaml",
      fileBytes: samplePolicyBytes,
    });

    expect(outcome).toMatchObject({
      accepted: true,
      activated: true,
      fileDigest: digestPolicyBytes(samplePolicyBytes),
    });
    if (!outcome.accepted) return;
    const revision = await queryRunner.manager.findOneByOrFail(ControlCatalogRevision, {
      id: outcome.revisionId,
    });
    expect(revision.fileDigest).toBe(digestPolicyBytes(samplePolicyBytes));
    expect(revision.sourceText).toBe(samplePolicyBytes.toString("utf8"));
    expect(revision).toMatchObject({ importSource: "command", importedBy: null, schemaVersion: 1 });
    const sampleValidation = validatePolicyFile(samplePolicyBytes);
    expect(sampleValidation.valid && revision.content).toEqual(
      sampleValidation.valid && sampleValidation.policy,
    );
    const pointer = await readPointer();
    expect(pointer).toMatchObject({
      requestedRevisionId: outcome.revisionId,
      activeRevisionId: outcome.revisionId,
      validatedRevisionId: null,
      lastError: null,
    });
  });

  it("only requests a later valid import and leaves the active revision in place", async () => {
    await clearPointer();
    const first = await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy.yaml",
      fileBytes: samplePolicyBytes,
    });

    const second = await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy.yaml",
      fileBytes: samplePolicyBytes,
    });

    expect(first.accepted && second.accepted).toBe(true);
    if (!first.accepted || !second.accepted) return;
    expect(second.revisionId).not.toBe(first.revisionId);
    expect(second).toMatchObject({ activated: false, activeRevisionId: first.revisionId });
    expect(await readPointer()).toMatchObject({
      requestedRevisionId: second.revisionId,
      activeRevisionId: first.revisionId,
    });
  });

  it("records a rejection on the pointer, stores no revision and keeps the last-known-good", async () => {
    await clearPointer();
    const accepted = await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy.yaml",
      fileBytes: samplePolicyBytes,
    });
    const revisionCountBefore = await queryRunner.manager.count(ControlCatalogRevision);
    const requestedBefore = (await readPointer()).requestedRevisionId;

    const rejected = await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy-invalid.yaml",
      fileBytes: invalidPolicyBytes,
    });

    if (!accepted.accepted) throw new Error("the sample policy must be accepted");
    expect(rejected).toEqual({
      accepted: false,
      fileDigest: digestPolicyBytes(invalidPolicyBytes),
      issues: [{ path: "budgets.calls_agent", message: "calls_agent must not exceed calls_total" }],
      activeRevisionId: accepted.revisionId,
    });
    expect(await queryRunner.manager.count(ControlCatalogRevision)).toBe(revisionCountBefore);
    const pointer = await readPointer();
    expect(pointer.activeRevisionId).toBe(accepted.revisionId);
    expect(pointer.requestedRevisionId).toBe(requestedBefore);
    expect(pointer.lastErrorAt).toBeInstanceOf(Date);
    expect(pointer.lastError).toMatchObject({
      reason: "policy_reload_rejected",
      source_file_name: "policy-invalid.yaml",
      file_digest: digestPolicyBytes(invalidPolicyBytes),
      issues: [{ path: "budgets.calls_agent" }],
    });
  });

  it("clears the recorded rejection once a valid file is imported", async () => {
    await clearPointer();
    await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy-invalid.yaml",
      fileBytes: invalidPolicyBytes,
    });
    expect(await readPointer()).toMatchObject({ activeRevisionId: null });

    await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy.yaml",
      fileBytes: samplePolicyBytes,
    });

    expect(await readPointer()).toMatchObject({ lastError: null, lastErrorAt: null });
  });

  it("keeps a stored revision immutable", async () => {
    const outcome = await importPolicyFile(queryRunner.manager, {
      sourceFileName: "policy.yaml",
      fileBytes: samplePolicyBytes,
    });
    if (!outcome.accepted) throw new Error("the sample policy must be accepted");

    for (const statement of [
      `UPDATE app.control_catalog_revisions SET source_text = 'changed' WHERE id = $1`,
      `DELETE FROM app.control_catalog_revisions WHERE id = $1`,
    ]) {
      // A savepoint keeps the surrounding test transaction usable after the expected failure.
      await queryRunner.query("SAVEPOINT immutable_revision_check");
      await expect(queryRunner.query(statement, [outcome.revisionId])).rejects.toMatchObject({
        code: "23001",
      });
      await queryRunner.query("ROLLBACK TO SAVEPOINT immutable_revision_check");
    }
  });

  it("refuses to run outside a transaction", async () => {
    await expect(
      importPolicyFile(dataSource.manager, {
        sourceFileName: "policy.yaml",
        fileBytes: samplePolicyBytes,
      }),
    ).rejects.toThrow("inside a database transaction");
  });
});
