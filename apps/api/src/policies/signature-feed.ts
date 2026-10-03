import { isScalar, parseDocument, visit } from "yaml";
import { z } from "zod";
import { digestPolicyBytes, type PolicyFileIssue } from "./policy-file.js";

// Validation of the attack-signature feed (config/attack-signatures.json, SH-46) before the import
// stores it. It mirrors the Go parser (services/gateway/internal/security ParseFeed) and is stricter
// where the two languages differ, so a feed Go would refuse is never activated: patterns are
// printable ASCII only, and keys must match exactly.

/** The Go parser's bounds, all in bytes. */
export const SIGNATURE_FEED_MAX_BYTES = 64 * 1024;
const MAX_RULES = 100;
const MIN_PATTERN_BYTES = 3;
const MAX_PATTERN_BYTES = 256;
const MAX_TEXT_BYTES = 1000;
const MAX_REPORTED_ISSUES = 20;

const feedIdentifierPattern = /^[a-z0-9][a-z0-9_.-]{0,63}$/;
const ruleIdentifierPattern = /^[a-z0-9][a-z0-9_]{0,63}$/;
/** An already normalized ASCII pattern: printable, no capitals, no edge or double spaces. */
const normalizedAsciiPattern =
  /^(?!.* {2})[\x21-\x40\x5b-\x7e](?:[\x20-\x40\x5b-\x7e]*[\x21-\x40\x5b-\x7e])?$/;

/** True for the characters Go's unicode.IsControl reports: U+0000 to U+001F and U+007F to U+009F. */
function containsControlCharacter(text: string): boolean {
  for (let index = 0; index < text.length; index += 1) {
    const codeUnit = text.charCodeAt(index);
    if (codeUnit < 0x20 || (codeUnit >= 0x7f && codeUnit <= 0x9f)) {
      return true;
    }
  }
  return false;
}

/** Non-empty printable text of at most 1000 UTF-8 bytes, as the Go parser's boundedText. */
const boundedText = z
  .string()
  .min(1)
  .refine((text) => Buffer.byteLength(text, "utf8") <= MAX_TEXT_BYTES, "text is too long")
  .refine((text) => !containsControlCharacter(text), "text contains control characters");

const boundary = z.enum(["model_input", "tool_result", "action_proposal"]);

const signatureRuleSchema = z.strictObject({
  id: z.string().regex(ruleIdentifierPattern),
  attack_class: z.string().regex(ruleIdentifierPattern),
  description: boundedText,
  pattern_type: z.literal("normalized_substring"),
  pattern: z
    .string()
    .regex(normalizedAsciiPattern)
    .refine(
      (pattern) => pattern.length >= MIN_PATTERN_BYTES && pattern.length <= MAX_PATTERN_BYTES,
      "pattern length is out of range",
    ),
  boundaries: z
    .array(boundary)
    .min(1)
    .refine((entries) => new Set(entries).size === entries.length, "entries must be unique"),
  response: z.literal("block"),
  sources: z.array(boundedText).min(1),
});

/** The only issuer the gateway's activation trusts (catalog.TrustedFeedIssuer); any other is refused here. */
export const TRUSTED_FEED_ISSUER = "task-passport-security";

export const signatureFeedSchema = z.strictObject({
  schema_version: z.literal(1),
  issuer: z.literal(TRUSTED_FEED_ISSUER),
  revision: z.string().regex(feedIdentifierPattern),
  description: boundedText,
  scope: boundedText,
  rules: z
    .array(signatureRuleSchema)
    .min(1)
    .max(MAX_RULES)
    .refine(
      (rules) => new Set(rules.map((rule) => rule.id)).size === rules.length,
      "rule ids must be unique",
    ),
});

export type SignatureFeed = z.infer<typeof signatureFeedSchema>;

export type SignatureFeedValidation =
  | { valid: true; feed: SignatureFeed; sourceText: string; fileDigest: string }
  | { valid: false; fileDigest: string; issues: PolicyFileIssue[] };

function rejected(fileDigest: string, issues: PolicyFileIssue[]): SignatureFeedValidation {
  return {
    valid: false,
    fileDigest,
    issues: issues
      .slice(0, MAX_REPORTED_ISSUES)
      .map((issue) => ({ ...issue, path: `feed.${issue.path}` })),
  };
}

/** Feed keys are lowercase identifiers; anything else in a path is reported without its text. */
const safePathSegment = /^[a-z0-9_]{1,64}$/;

function describePath(pathSegments: readonly PropertyKey[]): string {
  if (pathSegments.length === 0) {
    return "(document)";
  }
  return pathSegments
    .map((segment) =>
      typeof segment === "number" || (typeof segment === "string" && safePathSegment.test(segment))
        ? String(segment)
        : "(key)",
    )
    .join(".");
}

/**
 * Validates the raw feed bytes: bounded size, strict UTF-8, one strict JSON object without duplicate
 * keys, schema_version written as the integer 1, and the closed rule grammar. The digest is computed
 * over these exact bytes, which the import stores unchanged.
 */
export function validateSignatureFeed(fileBytes: Uint8Array): SignatureFeedValidation {
  const fileDigest = digestPolicyBytes(fileBytes);
  if (fileBytes.byteLength > SIGNATURE_FEED_MAX_BYTES) {
    return rejected(fileDigest, [
      { path: "(file)", message: `file is larger than ${SIGNATURE_FEED_MAX_BYTES} bytes` },
    ]);
  }
  let sourceText: string;
  try {
    // ignoreBOM keeps the bytes as they are, so the stored text matches the digest Go checks.
    sourceText = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(fileBytes);
  } catch {
    return rejected(fileDigest, [{ path: "(file)", message: "file is not valid UTF-8" }]);
  }

  let parsedValue: unknown;
  try {
    // Strict JSON syntax: no comments, trailing commas or trailing values.
    parsedValue = JSON.parse(sourceText);
  } catch {
    return rejected(fileDigest, [{ path: "(file)", message: "file is not valid JSON" }]);
  }
  // JSON.parse keeps the last of two equal keys; the YAML parser reports them, as Go does.
  const document = parseDocument(sourceText, {
    schema: "core",
    uniqueKeys: true,
    prettyErrors: false,
  });
  let containsAlias = false;
  visit(document, {
    Alias() {
      containsAlias = true;
      return visit.BREAK;
    },
  });
  if (document.errors.length > 0 || containsAlias) {
    return rejected(fileDigest, [{ path: "(file)", message: "file has duplicate keys" }]);
  }

  const result = signatureFeedSchema.safeParse(parsedValue);
  if (!result.success) {
    return rejected(
      fileDigest,
      result.error.issues.map((issue) => ({
        path: describePath(issue.path),
        message: issue.code === "custom" ? issue.message : "value is invalid",
      })),
    );
  }
  // Go decodes schema_version into an int, so 1.0 or 1e0 would be refused there.
  const versionNode = document.get("schema_version", true);
  if (!isScalar(versionNode) || String(versionNode.source) !== "1") {
    return rejected(fileDigest, [
      { path: "schema_version", message: "must be written as the integer 1" },
    ]);
  }
  return { valid: true, feed: result.data, sourceText, fileDigest };
}
