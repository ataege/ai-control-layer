// Run with: node --test apps/web/scripts/lib/secret-verdict.test.mjs
import assert from "node:assert/strict";
import { test } from "node:test";

import { classifySecretVerdict, MODEL_DEPENDENT_DENIALS } from "./secret-verdict.mjs";

const SECRET = "demo-only-pass-4471";
const base = { actionId: null, safeMessage: "x", controls: [], content: null };

test("a redaction with its marker passes", () => {
  const verdict = {
    ...base,
    decision: "redact",
    reasonCode: "content_redacted",
    content: { text: "password [REDACTED:password], rotate monthly" },
  };
  assert.deepEqual(classifySecretVerdict(verdict, SECRET), { status: "pass", detail: "" });
});

test("a model-dependent semantic denial is a note, never a failure", () => {
  for (const reasonCode of MODEL_DEPENDENT_DENIALS) {
    const result = classifySecretVerdict({ ...base, decision: "deny", reasonCode }, SECRET);
    assert.equal(result.status, "note", reasonCode);
    assert.match(result.detail, new RegExp(`deny/${reasonCode}`));
  }
});

test("a leaked secret fails whatever the decision, even a note-worthy denial", () => {
  const leaked = {
    ...base,
    decision: "deny",
    reasonCode: "semantic_injection_detected",
    safeMessage: `withheld ${SECRET}`,
  };
  const result = classifySecretVerdict(leaked, SECRET);
  assert.equal(result.status, "fail");
  assert.match(result.detail, /deny\/semantic_injection_detected/);
});

test("an allow fails and names its reason code", () => {
  const result = classifySecretVerdict({ ...base, decision: "allow", reasonCode: null }, SECRET);
  assert.equal(result.status, "fail");
  assert.match(result.detail, /allow\/null/);
});

test("a redaction without the marker fails", () => {
  for (const content of [null, { text: "password hunter2 was removed" }]) {
    const result = classifySecretVerdict(
      { ...base, decision: "redact", reasonCode: "content_redacted", content },
      SECRET,
    );
    assert.equal(result.status, "fail");
    assert.match(result.detail, /redact\/content_redacted/);
  }
});

test("any other denial is unexpected and fails with its reason code", () => {
  const result = classifySecretVerdict(
    { ...base, decision: "deny", reasonCode: "run_not_active" },
    SECRET,
  );
  assert.equal(result.status, "fail");
  assert.match(result.detail, /unexpected decision deny\/run_not_active/);
});
