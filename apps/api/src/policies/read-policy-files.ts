import { open } from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";
import { POLICY_FILE_MAX_BYTES, validatePolicyFile } from "./policy-file.js";
import { SIGNATURE_FEED_MAX_BYTES } from "./signature-feed.js";
import type { PolicyImportRequest } from "./policy-catalog-importer.js";

/** Fixed repository file; no HTTP input chooses a path or supplies configuration bytes. */
export const REPOSITORY_POLICY_PATH = resolve(
  import.meta.dirname,
  "../../../../config/policy.yaml",
);

async function readBoundedFile(filePath: string, maxBytes: number): Promise<Uint8Array> {
  const fileHandle = await open(filePath, "r");
  try {
    if (!(await fileHandle.stat()).isFile()) {
      throw Object.assign(new Error("not a regular file"), { code: "ENOTREGULAR" });
    }
    const buffer = new Uint8Array(maxBytes + 1);
    let bytesRead = 0;
    while (bytesRead < buffer.byteLength) {
      const chunk = await fileHandle.read(buffer, bytesRead, buffer.byteLength - bytesRead);
      if (chunk.bytesRead === 0) break;
      bytesRead += chunk.bytesRead;
    }
    return buffer.subarray(0, bytesRead);
  } finally {
    await fileHandle.close();
  }
}

/** Shared by the explicit CLI and authenticated reload; reads at most the validators' size bounds. */
export async function readPolicyFiles(policyPath: string): Promise<PolicyImportRequest> {
  const fileBytes = await readBoundedFile(policyPath, POLICY_FILE_MAX_BYTES);
  const preliminary = validatePolicyFile(fileBytes);
  let feed: PolicyImportRequest["feed"];
  if (preliminary.valid) {
    const sourceFileName = preliminary.policy.signatures.path;
    try {
      feed = {
        sourceFileName,
        fileBytes: await readBoundedFile(
          join(dirname(policyPath), sourceFileName),
          SIGNATURE_FEED_MAX_BYTES,
        ),
      };
    } catch (error) {
      // The importer decides whether an absent feed is allowed by this policy.
      if ((error as { code?: unknown }).code !== "ENOENT") throw error;
    }
  }
  return { sourceFileName: basename(policyPath), fileBytes, feed };
}
