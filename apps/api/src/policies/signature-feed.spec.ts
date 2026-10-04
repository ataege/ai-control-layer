import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { validateSignatureFeed } from "./signature-feed.js";

const committedFeedBytes = readFileSync(
  resolve(import.meta.dirname, "../../../../config/attack-signatures.json"),
);
// The digest Go pins in services/gateway/internal/security/feed_file_test.go.
const COMMITTED_FEED_DIGEST = "ff6ff4fef7e7091a50c1e416fab5a7b1aa825b55d783ada38de43b42a399d98c";

const committedText = committedFeedBytes.toString("utf8");
const encode = (text: string) => new TextEncoder().encode(text);

describe("validateSignatureFeed", () => {
  it("accepts the committed feed with the digest Go pins and keeps its exact text", () => {
    const validation = validateSignatureFeed(committedFeedBytes);
    expect(validation.valid).toBe(true);
    if (!validation.valid) return;
    expect(validation.fileDigest).toBe(COMMITTED_FEED_DIGEST);
    expect(validation.sourceText).toBe(committedText);
    expect(validation.feed.revision).toBe("feed_v2");
    expect(validation.feed.rules.map((rule) => rule.id)).toEqual([
      "prompt_ignore_previous_v1",
      "prompt_ignore_all_previous_v1",
      "prompt_ignore_the_previous_v1",
      "prompt_ignore_prior_v1",
      "prompt_ignore_all_prior_v1",
      "prompt_disregard_previous_v1",
      "prompt_disregard_all_previous_v1",
      "prompt_disregard_prior_v1",
      "code_exec_python_import_v1",
      "unsafe_deserialization_pickle_v1",
      "model_repo_trust_remote_code_v1",
    ]);
  });

  // Each case is a feed the Go parser refuses (or that this mirror refuses more strictly).
  const refused: [string, string][] = [
    ["a regex rule", committedText.replace('"normalized_substring"', '"regex"')],
    [
      "a capitalized pattern",
      committedText.replace('"ignore previous instructions"', '"Ignore previous instructions"'),
    ],
    [
      "a double space in a pattern",
      committedText.replace('"ignore previous instructions"', '"ignore  previous instructions"'),
    ],
    ["a too short pattern", committedText.replace('"__import__("', '"__"')],
    [
      "a space before a parenthesis in a pattern",
      committedText.replace('"__import__("', '"__import__ ("'),
    ],
    [
      "a space around an equals sign in a pattern",
      committedText.replace('"trust_remote_code=true"', '"trust_remote_code = true"'),
    ],
    ["a redact response", committedText.replace('"response": "block"', '"response": "redact"')],
    ["an unknown boundary", committedText.replace('"action_proposal"]', '"file_system"]')],
    [
      "an unknown key",
      committedText.replace('"schema_version": 1,', '"schema_version": 1, "loader": "x",'),
    ],
    [
      "a duplicate key",
      committedText.replace('"schema_version": 1,', '"schema_version": 1, "schema_version": 1,'),
    ],
    [
      "schema_version written as 1.0",
      committedText.replace('"schema_version": 1,', '"schema_version": 1.0,'),
    ],
    [
      "an issuer the gateway does not trust",
      committedText.replace('"issuer": "task-passport-security"', '"issuer": "other-publisher"'),
    ],
    ["a bad revision", committedText.replace('"revision": "feed_v2"', '"revision": "../feed"')],
    ["a control character", committedText.replace("Sample managed", "Sample\\u0007managed")],
    ["trailing data", `${committedText}{}`],
    ["no rules", committedText.replace(/"rules": \[[\s\S]*\]\n\}/, '"rules": []\n}')],
  ];
  it.each(refused)("refuses %s", (_name, text) => {
    expect(text).not.toBe(committedText);
    expect(validateSignatureFeed(encode(text)).valid).toBe(false);
  });

  it("refuses invalid UTF-8 and oversized files", () => {
    expect(validateSignatureFeed(new Uint8Array([0x7b, 0xff, 0x7d])).valid).toBe(false);
    expect(validateSignatureFeed(encode(committedText + " ".repeat(64 * 1024))).valid).toBe(false);
  });
});
