import { mkdtemp, readFile, rm, writeFile, mkdir } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, it } from "vitest";
import { readPolicyFiles, REPOSITORY_POLICY_PATH } from "./read-policy-files.js";
import { POLICY_FILE_MAX_BYTES } from "./policy-file.js";

it("reads the fixed repository policy and its named feed without rewriting bytes", async () => {
  const files = await readPolicyFiles(REPOSITORY_POLICY_PATH);
  expect(files.sourceFileName).toBe("policy.yaml");
  expect(Buffer.from(files.fileBytes)).toEqual(await readFile(REPOSITORY_POLICY_PATH));
  expect(files.feed?.sourceFileName).toBe("attack-signatures.json");
});
it("bounds oversized files, leaves missing feeds to validation and refuses non-files", async () => {
  const directory = await mkdtemp(join(tmpdir(), "policy-files-test-"));
  try {
    const path = join(directory, "policy.yaml");
    await writeFile(path, await readFile(REPOSITORY_POLICY_PATH));
    expect((await readPolicyFiles(path)).feed).toBeUndefined();
    await writeFile(path, "a".repeat(POLICY_FILE_MAX_BYTES + 20));
    const files = await readPolicyFiles(path);
    expect(files.fileBytes.byteLength).toBe(POLICY_FILE_MAX_BYTES + 1);
    expect(files.feed).toBeUndefined();
    await mkdir(join(directory, "not-a-file"));
    await expect(readPolicyFiles(join(directory, "not-a-file"))).rejects.toThrow(
      "not a regular file",
    );
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
