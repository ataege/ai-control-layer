import { expect, it, vi, beforeEach } from "vitest";
import type { DataSource, EntityManager } from "typeorm";
import { PoliciesService } from "./policies.service.js";
import { importPolicyFile } from "./policy-catalog-importer.js";
import { readPolicyFiles, REPOSITORY_POLICY_PATH } from "./read-policy-files.js";

vi.mock("./read-policy-files.js", () => ({
  REPOSITORY_POLICY_PATH: "/repository/config/policy.yaml",
  readPolicyFiles: vi.fn(),
}));
vi.mock("./policy-catalog-importer.js", () => ({ importPolicyFile: vi.fn() }));
const findOneBy = vi.fn();
const findOneByOrFail = vi.fn();
const manager = { findOneBy, findOneByOrFail } as unknown as EntityManager;
const transaction = vi.fn((fn: (value: EntityManager) => Promise<unknown>) => fn(manager));
const database = { isInitialized: true, manager, transaction };
const service = new PoliciesService(database as unknown as DataSource);
beforeEach(() => {
  vi.clearAllMocks();
  database.isInitialized = true;
  vi.mocked(readPolicyFiles).mockResolvedValue({
    sourceFileName: "policy.yaml",
    fileBytes: new Uint8Array([1]),
  });
  vi.mocked(importPolicyFile).mockResolvedValue({
    accepted: true,
    unchanged: false,
    revisionId: "3",
    fileDigest: "a".repeat(64),
    activeRevisionId: "2",
    feedRevisionId: "1",
    feedDigest: "b".repeat(64),
  });
  findOneByOrFail.mockResolvedValue({ revision: "feed_v1" });
});
it("reads only the server path and records verified actor provenance inside the transaction", async () => {
  const actor = "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01";
  expect((await service.reload(actor)).feedRevision).toBe("feed_v1");
  expect(readPolicyFiles).toHaveBeenCalledWith(REPOSITORY_POLICY_PATH);
  expect(importPolicyFile).toHaveBeenCalledWith(
    manager,
    expect.objectContaining({ provenance: { importSource: "reload", importedBy: actor } }),
  );
});
it("refuses file and database failures without exposing their messages", async () => {
  vi.mocked(readPolicyFiles).mockRejectedValue(new Error("secret-file-path"));
  await expect(service.reload("actor")).rejects.toMatchObject({
    status: 503,
    message: "Policy reload unavailable",
  });
  expect(transaction).not.toHaveBeenCalled();
  database.isInitialized = false;
  await expect(service.pointer()).rejects.toMatchObject({ status: 503 });
});
it("reports pointer lock contention as revision_pending", async () => {
  transaction.mockRejectedValueOnce({ code: "55P03" });
  await expect(service.reload("actor")).rejects.toMatchObject({
    status: 409,
    response: { code: "revision_pending" },
  });
});
it("refuses a missing pointer and an unavailable status dependency", async () => {
  findOneBy.mockResolvedValue(null);
  await expect(service.pointer()).rejects.toMatchObject({ status: 503 });
  findOneBy.mockRejectedValue(new Error("database-secret"));
  await expect(service.pointer()).rejects.toMatchObject({
    status: 503,
    message: "Policy status unavailable",
  });
});
