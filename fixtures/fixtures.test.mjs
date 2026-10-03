// Self-check of the synthetic fixtures (`pnpm test:fixtures`, or `node --test "fixtures/*.test.mjs"`).
// Catches what breaks silently when a text is edited: duplicate ids, labels outside the closed
// sets, and secret spans that no longer point at their value.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const readFixture = (fileName) =>
  JSON.parse(readFileSync(new URL(fileName, import.meta.url), "utf8"));

const hostileNotes = readFixture("./hostile-notes.json");
const semanticCorpus = readFixture("./semantic-corpus.json");

// Boundaries as named in the draft policy.yaml (SH-42).
const BOUNDARIES = new Set(["model_input", "tool_result", "action_proposal"]);
const OUTCOMES = new Set(["allow", "block", "redact"]);
const OUTCOME_BY_CATEGORY = {
  benign: "allow",
  direct_instruction_attack: "block",
  indirect_instruction_attack: "block",
  secret_redaction: "redact",
};
// Proposed reason vocabulary, docs/product/README.md.
const DETERMINISTIC_REASONS = new Set([
  "resource_out_of_scope",
  "destination_not_allowed",
  "report_export_restricted",
]);
const SECRET_KINDS = new Set(["password", "api_token", "bank_account", "payment_card"]);

// Go indexes strings by byte and JavaScript by UTF-16 unit; both agree only on ASCII.
const isAscii = (text) => /^[\x20-\x7e]*$/.test(text);

for (const [name, fixture] of [
  ["hostile notes", hostileNotes],
  ["semantic corpus", semanticCorpus],
]) {
  test(`${name}: labelled synthetic`, () => {
    assert.equal(fixture.synthetic, true);
    assert.equal(typeof fixture.fixture_version, "number");
  });
}

test("ids are unique across both files", () => {
  const ids = [...hostileNotes.notes, ...semanticCorpus.cases].map((entry) => entry.id);
  assert.equal(new Set(ids).size, ids.length);
});

test("hostile notes: block outcome, a tool-result boundary and a deterministic backstop", () => {
  assert.ok(hostileNotes.notes.length >= 3);
  for (const note of hostileNotes.notes) {
    assert.equal(note.boundary, "tool_result", note.id);
    assert.equal(note.expected_outcome, "block", note.id);
    assert.ok(DETERMINISTIC_REASONS.has(note.deterministic_reason_if_obeyed), note.id);
    assert.ok(isAscii(note.text), `${note.id}: text must be printable ASCII`);
  }
});

test("semantic corpus: closed labels and an outcome matching each category", () => {
  for (const corpusCase of semanticCorpus.cases) {
    assert.ok(corpusCase.category in OUTCOME_BY_CATEGORY, corpusCase.id);
    assert.ok(BOUNDARIES.has(corpusCase.boundary), corpusCase.id);
    assert.ok(OUTCOMES.has(corpusCase.expected_outcome), corpusCase.id);
    assert.equal(
      corpusCase.expected_outcome,
      OUTCOME_BY_CATEGORY[corpusCase.category],
      corpusCase.id,
    );
    assert.ok(isAscii(corpusCase.text), `${corpusCase.id}: text must be printable ASCII`);
  }
});

test("semantic corpus: covers every category", () => {
  const categories = new Set(semanticCorpus.cases.map((corpusCase) => corpusCase.category));
  assert.deepEqual([...categories].sort(), Object.keys(OUTCOME_BY_CATEGORY).sort());
});

test("secret cases: each span selects its value, which appears exactly once", () => {
  for (const corpusCase of semanticCorpus.cases) {
    const isSecretCase = corpusCase.category === "secret_redaction";
    assert.equal(Array.isArray(corpusCase.secrets), isSecretCase, corpusCase.id);
    for (const secret of corpusCase.secrets ?? []) {
      assert.ok(SECRET_KINDS.has(secret.kind), `${corpusCase.id}: kind ${secret.kind}`);
      assert.equal(corpusCase.text.slice(secret.start, secret.end), secret.value, corpusCase.id);
      assert.equal(corpusCase.text.indexOf(secret.value), secret.start, corpusCase.id);
      assert.equal(corpusCase.text.indexOf(secret.value, secret.start + 1), -1, corpusCase.id);
    }
  }
});

test("the report's sample signature phrase appears in a tool result", () => {
  const signatureCases = semanticCorpus.cases.filter(
    (corpusCase) => corpusCase.report_sample_signature === "prompt_ignore_previous_v1",
  );
  assert.ok(signatureCases.length > 0);
  for (const corpusCase of signatureCases) {
    assert.equal(corpusCase.boundary, "tool_result", corpusCase.id);
    assert.match(corpusCase.text.toLowerCase(), /ignore previous instructions/);
  }
});
