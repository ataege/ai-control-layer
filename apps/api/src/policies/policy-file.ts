import { createHash } from "node:crypto";
import { isScalar, parseDocument, visit } from "yaml";
import { z } from "zod";

// Validation of policy.yaml against the schema documented in config/README.md (the draft of the
// "policy activation and catalog revision" contract). Any deviation rejects the whole file.

/** Upper bound on the file size; the sample is under 2 KiB, so this leaves room without being unbounded. */
export const POLICY_FILE_MAX_BYTES = 64 * 1024;

/** Stable reason code for every rejected import or reload (report 1.2 reason vocabulary). */
export const POLICY_REJECTION_REASON = "policy_reload_rejected";

/** At most this many issues are reported, so one broken file cannot produce an unbounded error. */
const MAX_REPORTED_ISSUES = 20;

const modelTagPattern = /^[a-z0-9][a-z0-9_.:-]{0,63}$/;
const feedFileNamePattern = /^(?!.*\.\.)[A-Za-z0-9_-][A-Za-z0-9_.-]*\.json$/;
const feedRevisionPattern = /^[a-z0-9][a-z0-9_.-]{0,63}$/;
const signatureRuleIdPattern = /^[a-z0-9][a-z0-9_]{0,63}$/;

/** A list whose entries must be distinct. */
function uniqueList<Item extends z.ZodType>(itemSchema: Item) {
  return z
    .array(itemSchema)
    .refine((entries) => new Set(entries).size === entries.length, "entries must be unique");
}

/** A non-empty list of distinct boundaries, each one the guard supports. */
function boundaryList<const Boundary extends string>(supportedBoundaries: readonly Boundary[]) {
  return uniqueList(z.enum(supportedBoundaries)).min(1);
}

const positiveCount = z.number().int().positive();
const guardMode = z.enum(["block", "redact"]);

export const policyFileSchema = z
  .strictObject({
    schema_version: z.literal(1),
    allowed_models: uniqueList(z.string().regex(modelTagPattern)).min(1),
    budgets: z.strictObject({
      calls_total: positiveCount,
      calls_agent: positiveCount,
      calls_security: positiveCount,
      tokens_total: positiveCount,
      request_timeout_seconds: positiveCount,
      local_max_concurrency: positiveCount,
      run_expiry_minutes: positiveCount,
      tool_attempts: positiveCount,
      corrections: positiveCount,
    }),
    controls: z.strictObject({
      secret_pattern: z.strictObject({
        enabled: z.boolean(),
        mode: guardMode,
        boundaries: boundaryList(["model_input", "tool_result"]),
      }),
      semantic_injection: z.strictObject({
        enabled: z.boolean(),
        mode: guardMode,
        // Provisional range, pending the open item `classifier prompt and verdict schema`.
        threshold: z.number().min(0).max(1),
        boundaries: boundaryList(["model_input", "tool_result", "action_proposal"]),
      }),
      signature_match: z.strictObject({
        enabled: z.boolean(),
        boundaries: boundaryList(["model_input", "tool_result", "action_proposal"]),
      }),
    }),
    signatures: z.strictObject({
      path: z.string().regex(feedFileNamePattern),
      revision: z.string().regex(feedRevisionPattern),
      disabled_rules: uniqueList(z.string().regex(signatureRuleIdPattern)),
    }),
    reports: z.strictObject({
      enabled_templates: uniqueList(
        z.enum(["internal_investigation_v1", "vendor_reconciliation_v1"]),
      ),
    }),
  })
  .superRefine((policy, context) => {
    const { budgets } = policy;
    // Purpose sub-limits are ceilings inside the shared total, not entitlements that add up to it.
    for (const subLimit of ["calls_agent", "calls_security"] as const) {
      if (budgets[subLimit] > budgets.calls_total) {
        context.addIssue({
          code: "custom",
          path: ["budgets", subLimit],
          message: `${subLimit} must not exceed calls_total`,
        });
      }
    }
    if (budgets.request_timeout_seconds >= budgets.run_expiry_minutes * 60) {
      context.addIssue({
        code: "custom",
        path: ["budgets", "request_timeout_seconds"],
        message: "request_timeout_seconds must be shorter than run_expiry_minutes",
      });
    }
  });

export type PolicyFile = z.infer<typeof policyFileSchema>;

/** One reason a file was rejected: where in the file, and what is wrong. Never echoes file content. */
export interface PolicyFileIssue {
  path: string;
  message: string;
}

export type PolicyFileValidation =
  | { valid: true; policy: PolicyFile; sourceText: string; fileDigest: string }
  | { valid: false; fileDigest: string; issues: PolicyFileIssue[] };

/** Keys whose values must be written as plain decimal integers, not as 24.0, 0x18 or 0o30. */
const integerValuePaths: readonly (readonly string[])[] = [
  ["schema_version"],
  ...Object.keys(policyFileSchema.shape.budgets.shape).map((budgetKey) => ["budgets", budgetKey]),
];
const decimalIntegerSource = /^[0-9]+$/;

/** True for control characters other than tab, line feed and carriage return; a policy needs none. */
function containsDisallowedControlCharacter(text: string): boolean {
  for (let index = 0; index < text.length; index += 1) {
    const codeUnit = text.charCodeAt(index);
    const isAllowedWhitespace = codeUnit === 0x09 || codeUnit === 0x0a || codeUnit === 0x0d;
    if ((codeUnit < 0x20 && !isAllowedWhitespace) || (codeUnit >= 0x7f && codeUnit <= 0x9f)) {
      return true;
    }
  }
  return false;
}

/** Schema keys are lowercase identifiers; anything else in a path is reported without its text. */
const safePathSegment = /^[a-z0-9_]{1,64}$/;

/** Lowercase hex SHA-256 of the file's bytes, the digest stored with every revision. */
export function digestPolicyBytes(fileBytes: Uint8Array): string {
  return createHash("sha256").update(fileBytes).digest("hex");
}

function rejected(fileDigest: string, issues: PolicyFileIssue[]): PolicyFileValidation {
  return { valid: false, fileDigest, issues: issues.slice(0, MAX_REPORTED_ISSUES) };
}

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

/** A fixed message per issue kind, so no key or value from the file reaches storage or logs. */
function describeIssue(issue: z.core.$ZodIssue): string {
  switch (issue.code) {
    case "unrecognized_keys":
      return `unknown key (${issue.keys.length})`;
    case "invalid_type":
      return `expected ${issue.expected}`;
    case "invalid_value":
      return "value is not one of the allowed values";
    case "invalid_format":
      return "value does not match the required format";
    case "too_small":
      return issue.origin === "array" ? "list is too short" : "value is too small";
    case "too_big":
      return issue.origin === "array" ? "list is too long" : "value is too large";
    case "custom":
      // Only this module's own refinements produce custom issues; their messages hold no file content.
      return issue.message;
    default:
      return "value is invalid";
  }
}

/**
 * Validates the raw bytes of a policy file: bounded size, strict UTF-8 without control characters,
 * one plain YAML document (no aliases, no custom tags, no duplicate keys) and the closed schema with
 * its cross-field rules.
 */
export function validatePolicyFile(fileBytes: Uint8Array): PolicyFileValidation {
  const fileDigest = digestPolicyBytes(fileBytes);

  if (fileBytes.byteLength > POLICY_FILE_MAX_BYTES) {
    return rejected(fileDigest, [
      { path: "(file)", message: `file is larger than ${POLICY_FILE_MAX_BYTES} bytes` },
    ]);
  }

  let sourceText: string;
  try {
    // ignoreBOM keeps a leading byte order mark in the text, so the stored text matches the digest.
    sourceText = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(fileBytes);
  } catch {
    return rejected(fileDigest, [{ path: "(file)", message: "file is not valid UTF-8" }]);
  }
  if (containsDisallowedControlCharacter(sourceText)) {
    return rejected(fileDigest, [{ path: "(file)", message: "file contains control characters" }]);
  }

  // The core schema knows no executable or custom types.
  const document = parseDocument(sourceText, {
    schema: "core",
    uniqueKeys: true,
    prettyErrors: false,
  });
  const yamlProblems = [...document.errors, ...document.warnings];
  if (yamlProblems.length > 0) {
    return rejected(
      fileDigest,
      yamlProblems.map((problem) => ({ path: "(yaml)", message: problem.code })),
    );
  }

  // Aliases are refused outright, which rules out alias expansion attacks.
  let containsAlias = false;
  visit(document, {
    Alias() {
      containsAlias = true;
      return visit.BREAK;
    },
  });
  if (containsAlias) {
    return rejected(fileDigest, [{ path: "(yaml)", message: "aliases are not allowed" }]);
  }

  let parsedValue: unknown;
  try {
    parsedValue = document.toJS({ maxAliasCount: 0 });
  } catch {
    return rejected(fileDigest, [{ path: "(yaml)", message: "document cannot be read" }]);
  }

  const result = policyFileSchema.safeParse(parsedValue);
  if (!result.success) {
    return rejected(
      fileDigest,
      result.error.issues.map((issue) => ({
        path: describePath(issue.path),
        message: describeIssue(issue),
      })),
    );
  }

  const nonDecimalIntegers = integerValuePaths.filter((valuePath) => {
    const valueNode = document.getIn(valuePath, true);
    return !isScalar(valueNode) || !decimalIntegerSource.test(String(valueNode.source));
  });
  if (nonDecimalIntegers.length > 0) {
    return rejected(
      fileDigest,
      nonDecimalIntegers.map((valuePath) => ({
        path: valuePath.join("."),
        message: "must be written as a decimal integer",
      })),
    );
  }

  return { valid: true, policy: result.data, sourceText, fileDigest };
}
