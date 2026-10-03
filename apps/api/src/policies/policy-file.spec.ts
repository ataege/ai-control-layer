import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { parseDocument } from "yaml";
import {
  digestPolicyBytes,
  POLICY_FILE_MAX_BYTES,
  validatePolicyFile,
  type PolicyFileValidation,
} from "./policy-file.js";

// The documented sample the judges edit; the tests start from it so the sample itself stays valid.
const samplePolicyBytes = readFileSync(
  resolve(import.meta.dirname, "../../../../config/policy.yaml"),
);

/** Marks a key to delete instead of a value to set. */
const REMOVE = Symbol("remove");

function textBytes(text: string): Uint8Array {
  return new TextEncoder().encode(text);
}

/** The sample with values set (or removed) at dotted paths, serialized back to YAML. */
function sampleWith(edits: Record<string, unknown>): Uint8Array {
  const document = parseDocument(samplePolicyBytes.toString("utf8"));
  for (const [dottedPath, value] of Object.entries(edits)) {
    const pathSegments = dottedPath.split(".");
    if (value === REMOVE) {
      document.deleteIn(pathSegments);
    } else {
      document.setIn(pathSegments, value);
    }
  }
  return textBytes(document.toString());
}

function issuePaths(validation: PolicyFileValidation): string[] {
  expect(validation.valid).toBe(false);
  return validation.valid ? [] : validation.issues.map((issue) => issue.path);
}

describe("validatePolicyFile", () => {
  it("accepts the documented sample and digests its exact bytes", () => {
    const validation = validatePolicyFile(samplePolicyBytes);

    expect(validation.valid).toBe(true);
    expect(validation.fileDigest).toBe(
      createHash("sha256").update(samplePolicyBytes).digest("hex"),
    );
    if (validation.valid) {
      expect(validation.sourceText).toBe(samplePolicyBytes.toString("utf8"));
      expect(validation.policy).toEqual(parseDocument(validation.sourceText).toJS());
    }
  });

  // The four invalid cases API-32 names, plus the other rejected-file examples in config/README.md.
  it.each<[string, Record<string, unknown>, string]>([
    ["unknown control", { "controls.custom_plugin": { enabled: true } }, "controls"],
    ["unknown top-level plugin key", { plugins: ["run.sh"] }, "(document)"],
    [
      "script on secret_pattern",
      { "controls.secret_pattern.script": "x" },
      "controls.secret_pattern",
    ],
    [
      "extra key on semantic_injection",
      { "controls.semantic_injection.extra": 1 },
      "controls.semantic_injection",
    ],
    ["feed URL key", { "signatures.url": "https://example.com/feed.json" }, "signatures"],
    ["extra key under reports", { "reports.extra": true }, "reports"],
    ["missing control", { "controls.signature_match": REMOVE }, "controls.signature_match"],
    ["unknown budget key", { "budgets.calls_bonus": 5 }, "budgets"],
    [
      "mode on signature_match",
      { "controls.signature_match.mode": "block" },
      "controls.signature_match",
    ],
    ["model tag with a space", { allowed_models: ["GPT 4"] }, "allowed_models.0"],
    ["model given as a URL", { allowed_models: ["https://example.com/model"] }, "allowed_models.0"],
    ["empty model list", { allowed_models: [] }, "allowed_models"],
    ["duplicate model", { allowed_models: ["qwen3.5:4b", "qwen3.5:4b"] }, "allowed_models"],
    ["agent sub-limit above the total", { "budgets.calls_agent": 30 }, "budgets.calls_agent"],
    [
      "security sub-limit above the total",
      { "budgets.calls_security": 30 },
      "budgets.calls_security",
    ],
    ["zero token budget", { "budgets.tokens_total": 0 }, "budgets.tokens_total"],
    ["fractional limit", { "budgets.corrections": 1.5 }, "budgets.corrections"],
    ["limit as text", { "budgets.calls_total": "24" }, "budgets.calls_total"],
    [
      "timeout equal to the run lifetime",
      { "budgets.request_timeout_seconds": 900 },
      "budgets.request_timeout_seconds",
    ],
    [
      "timeout longer than the run",
      { "budgets.request_timeout_seconds": 1200 },
      "budgets.request_timeout_seconds",
    ],
    ["feed URL", { "signatures.path": "https://example.com/feed.json" }, "signatures.path"],
    ["feed path outside the directory", { "signatures.path": "../feed.json" }, "signatures.path"],
    ["feed name with dots", { "signatures.path": "a..json" }, "signatures.path"],
    ["absolute feed path", { "signatures.path": "/etc/feed.json" }, "signatures.path"],
    [
      "feed revision as a URL",
      { "signatures.revision": "https://example.com/v1" },
      "signatures.revision",
    ],
    [
      "malformed rule ID",
      { "signatures.disabled_rules": ["Bad Rule"] },
      "signatures.disabled_rules.0",
    ],
    [
      "duplicate rule IDs",
      { "signatures.disabled_rules": ["prompt_ignore_previous_v1", "prompt_ignore_previous_v1"] },
      "signatures.disabled_rules",
    ],
    [
      "threshold below 0",
      { "controls.semantic_injection.threshold": -0.1 },
      "controls.semantic_injection.threshold",
    ],
    [
      "threshold above 1",
      { "controls.semantic_injection.threshold": 1.5 },
      "controls.semantic_injection.threshold",
    ],
    [
      "allow as a mode",
      { "controls.secret_pattern.mode": "allow" },
      "controls.secret_pattern.mode",
    ],
    [
      "unsupported boundary",
      { "controls.secret_pattern.boundaries": ["action_proposal"] },
      "controls.secret_pattern.boundaries.0",
    ],
    [
      "no boundary",
      { "controls.signature_match.boundaries": [] },
      "controls.signature_match.boundaries",
    ],
    [
      "unknown template",
      { "reports.enabled_templates": ["public_summary_v1"] },
      "reports.enabled_templates.0",
    ],
    [
      "duplicate templates",
      { "reports.enabled_templates": ["vendor_reconciliation_v1", "vendor_reconciliation_v1"] },
      "reports.enabled_templates",
    ],
    ["wrong schema version", { schema_version: 2 }, "schema_version"],
    ["missing group", { reports: REMOVE }, "reports"],
  ])("rejects %s", (_caseName, edits, expectedPath) => {
    expect(issuePaths(validatePolicyFile(sampleWith(edits)))).toContain(expectedPath);
  });

  it("accepts sub-limits whose sum exceeds the total, because they are ceilings", () => {
    const validation = validatePolicyFile(
      sampleWith({ "budgets.calls_agent": 20, "budgets.calls_security": 20 }),
    );

    expect(validation.valid).toBe(true);
  });

  it.each([
    ["duplicate keys", "schema_version: 1\nschema_version: 1\n"],
    ["aliases", "anchor: &shared [1]\ncopy: *shared\n"],
    ["a custom tag", "schema_version: !!js/function 'return 1'\n"],
    ["several documents", "schema_version: 1\n---\nschema_version: 1\n"],
    ["broken syntax", "schema_version: [1\n"],
  ])("rejects YAML with %s before the schema is checked", (_caseName, text) => {
    expect(issuePaths(validatePolicyFile(textBytes(text)))[0]).toBe("(yaml)");
  });

  it("rejects an empty file", () => {
    expect(issuePaths(validatePolicyFile(textBytes("")))).toEqual(["(document)"]);
  });

  it("rejects bytes that are not UTF-8", () => {
    expect(issuePaths(validatePolicyFile(Uint8Array.of(0xff, 0xfe, 0x00)))).toEqual(["(file)"]);
  });

  it("rejects a file above the size bound without parsing it", () => {
    const oversizedBytes = textBytes(`# ${"x".repeat(POLICY_FILE_MAX_BYTES)}\n`);

    expect(issuePaths(validatePolicyFile(oversizedBytes))).toEqual(["(file)"]);
  });

  it.each([
    ["a float spelling", "calls_total: 24", "calls_total: 24.0", "budgets.calls_total"],
    ["a hexadecimal spelling", "calls_total: 24", "calls_total: 0x18", "budgets.calls_total"],
    ["an octal spelling", "calls_total: 24", "calls_total: 0o30", "budgets.calls_total"],
    ["a float schema version", "schema_version: 1", "schema_version: 1.0", "schema_version"],
  ])("rejects an integer written as %s", (_caseName, original, replacement, expectedPath) => {
    const editedText = samplePolicyBytes.toString("utf8").replace(original, replacement);

    expect(issuePaths(validatePolicyFile(textBytes(editedText)))).toEqual([expectedPath]);
  });

  it("never echoes keys or values from the file in its issues", () => {
    const marker = "sk_live_marker_7f3a";
    const validation = validatePolicyFile(
      sampleWith({
        [marker]: marker,
        [`budgets.${marker}`]: 1,
        "signatures.revision": marker.toUpperCase(),
      }),
    );

    expect(validation.valid).toBe(false);
    expect(JSON.stringify(validation).toLowerCase()).not.toContain(marker);
  });

  it("reports at most 20 issues", () => {
    // 25 unknown templates give 25 issues before the cap.
    const unknownTemplates = Array.from({ length: 25 }, (_, index) => `template_${index}`);
    const validation = validatePolicyFile(
      sampleWith({ "reports.enabled_templates": unknownTemplates }),
    );

    expect(validation.valid ? 0 : validation.issues.length).toBe(20);
  });

  it("rejects control characters, even inside a comment", () => {
    const sampleText = samplePolicyBytes.toString("utf8");

    expect(issuePaths(validatePolicyFile(textBytes(`# note \u0000\n${sampleText}`)))).toEqual([
      "(file)",
    ]);
    expect(issuePaths(validatePolicyFile(textBytes(`# note \u0085\n${sampleText}`)))).toEqual([
      "(file)",
    ]);
  });

  it("keeps a byte order mark in the stored text, so the text still matches the digest", () => {
    const bytesWithMark = Uint8Array.of(0xef, 0xbb, 0xbf, ...samplePolicyBytes);
    const validation = validatePolicyFile(bytesWithMark);

    expect(validation.valid).toBe(true);
    if (validation.valid) {
      expect(digestPolicyBytes(textBytes(validation.sourceText))).toBe(validation.fileDigest);
    }
  });

  it("accepts a file of exactly the size bound and rejects one byte more", () => {
    const sampleText = samplePolicyBytes.toString("utf8");
    const padding = (length: number) => `${sampleText}#${"x".repeat(length)}\n`;
    const exactText = padding(POLICY_FILE_MAX_BYTES - textBytes(padding(0)).byteLength);

    expect(textBytes(exactText).byteLength).toBe(POLICY_FILE_MAX_BYTES);
    expect(validatePolicyFile(textBytes(exactText)).valid).toBe(true);
    expect(issuePaths(validatePolicyFile(textBytes(`${exactText}#`)))).toEqual(["(file)"]);
  });
});
