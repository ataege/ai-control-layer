// The end-to-end script's judgement of a secret-bearing text (scripts/lib/secret-verdict.mjs),
// tested here so `pnpm test` and `pnpm verify` run it.
import { describe, expect, it } from "vitest";
import {
  classifySecretVerdict,
  MODEL_DEPENDENT_DENIALS,
} from "../../scripts/lib/secret-verdict.mjs";

const SECRET = "demo-only-pass-4471";
const base = { actionId: null, safeMessage: "x", controls: [], content: null };

describe("classifySecretVerdict", () => {
  it("passes a redaction with its marker", () => {
    const verdict = {
      ...base,
      decision: "redact",
      reasonCode: "content_redacted",
      content: { text: "password [REDACTED:password], rotate monthly" },
    };
    expect(classifySecretVerdict(verdict, SECRET)).toEqual({ status: "pass", detail: "" });
  });

  it.each(MODEL_DEPENDENT_DENIALS)(
    "makes a semantic denial (%s) a note, never a failure",
    (reasonCode) => {
      const result = classifySecretVerdict({ ...base, decision: "deny", reasonCode }, SECRET);
      expect(result.status).toBe("note");
      expect(result.detail).toContain(`deny/${reasonCode}`);
    },
  );

  it("fails a leaked secret whatever the decision, even a note-worthy denial", () => {
    const result = classifySecretVerdict(
      {
        ...base,
        decision: "deny",
        reasonCode: "semantic_injection_detected",
        safeMessage: `withheld ${SECRET}`,
      },
      SECRET,
    );
    expect(result.status).toBe("fail");
    expect(result.detail).toContain("deny/semantic_injection_detected");
  });

  it("fails an allow and names its reason code", () => {
    const result = classifySecretVerdict({ ...base, decision: "allow", reasonCode: null }, SECRET);
    expect(result.status).toBe("fail");
    expect(result.detail).toContain("allow/null");
  });

  it.each([null, { text: "password hunter2 was removed" }])(
    "fails a redaction without the marker (%j)",
    (content) => {
      const result = classifySecretVerdict(
        { ...base, decision: "redact", reasonCode: "content_redacted", content },
        SECRET,
      );
      expect(result.status).toBe("fail");
      expect(result.detail).toContain("redact/content_redacted");
    },
  );

  it("fails any other denial as unexpected, with its reason code", () => {
    const result = classifySecretVerdict(
      { ...base, decision: "deny", reasonCode: "run_not_active" },
      SECRET,
    );
    expect(result.status).toBe("fail");
    expect(result.detail).toContain("unexpected decision deny/run_not_active");
  });
});
