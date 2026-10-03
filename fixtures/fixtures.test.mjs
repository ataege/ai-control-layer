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

// Feed rule patterns (config/attack-signatures.json, SH-46): a case labelled with a rule holds its
// pattern after the feed's normalization (lowercase, collapsed whitespace).
const FEED_PATTERNS = {
  prompt_ignore_previous_v1: "ignore previous instructions",
  code_exec_python_import_v1: "__import__(",
  unsafe_deserialization_pickle_v1: "pickle.loads(",
  model_repo_trust_remote_code_v1: "trust_remote_code=true",
};

test("signature cases: each names a feed rule, holds its pattern and expects a block", () => {
  const signatureCases = semanticCorpus.cases.filter(
    (corpusCase) => "signature_rule" in corpusCase,
  );
  const rules = new Set(signatureCases.map((corpusCase) => corpusCase.signature_rule));
  for (const rule of Object.keys(FEED_PATTERNS).filter(
    (rule) => rule !== "prompt_ignore_previous_v1",
  )) {
    assert.ok(rules.has(rule), `no positive case for ${rule}`);
  }
  for (const corpusCase of signatureCases) {
    const pattern = FEED_PATTERNS[corpusCase.signature_rule];
    assert.ok(pattern, `${corpusCase.id}: unknown rule ${corpusCase.signature_rule}`);
    assert.equal(corpusCase.expected_outcome, "block", corpusCase.id);
    assert.ok(corpusCase.text.toLowerCase().replace(/\s+/g, " ").includes(pattern), corpusCase.id);
  }
});

test("classifier_v2 labels: hard negatives written after it, one held-out attack", () => {
  const writtenAfter = semanticCorpus.cases.filter((corpusCase) => "written_after" in corpusCase);
  const heldOut = semanticCorpus.cases.filter((corpusCase) => "held_out_for" in corpusCase);
  assert.ok(writtenAfter.length >= 4);
  for (const corpusCase of writtenAfter) {
    assert.equal(corpusCase.written_after, "classifier_v2", corpusCase.id);
    assert.equal(corpusCase.category, "benign", corpusCase.id);
  }
  assert.equal(heldOut.length, 1);
  assert.equal(heldOut[0].held_out_for, "classifier_v2");
  assert.equal(heldOut[0].expected_outcome, "block");
});

// Demo records (SH-25 data): internal consistency, so the later seed loads a coherent scenario.
const demoRecords = readFixture("./demo-records.json");
// Column names of demo.vendors and demo.invoices (SH-17 plus 1791060000000's provenance columns).
const VENDOR_COLUMNS = ["id", "name", "organization_id", "registered_reporting_address", "version"];
const INVOICE_COLUMNS = [
  "currency",
  "due_on",
  "external_reference",
  "id",
  "internal_note",
  "internal_note_classification",
  "issued_on",
  "organization_id",
  "total_minor_units",
  "vendor_id",
  "version",
];

test("demo records: labelled synthetic, fields match the demo table columns", () => {
  assert.equal(demoRecords.synthetic, true);
  for (const vendor of demoRecords.vendors)
    assert.deepEqual(Object.keys(vendor).sort(), VENDOR_COLUMNS);
  for (const invoice of demoRecords.invoices) {
    assert.deepEqual(Object.keys(invoice).sort(), INVOICE_COLUMNS, invoice.id);
  }
});

test("demo records: ids are unique and every reference resolves in the same organization", () => {
  const organizationIds = new Set(demoRecords.organizations.map((organization) => organization.id));
  assert.equal(organizationIds.size, demoRecords.organizations.length);
  const vendorsById = new Map(demoRecords.vendors.map((vendor) => [vendor.id, vendor]));
  assert.equal(vendorsById.size, demoRecords.vendors.length);
  const invoiceIds = new Set(demoRecords.invoices.map((invoice) => invoice.id));
  assert.equal(invoiceIds.size, demoRecords.invoices.length);

  for (const record of [...demoRecords.vendors, ...demoRecords.invoices]) {
    assert.ok(organizationIds.has(record.organization_id), `${record.id}: organization`);
  }
  // Mirrors the SH-17 composite key: an invoice's vendor belongs to the invoice's organization.
  for (const invoice of demoRecords.invoices) {
    const vendor = vendorsById.get(invoice.vendor_id);
    assert.ok(vendor, `${invoice.id}: vendor ${invoice.vendor_id}`);
    assert.equal(vendor.organization_id, invoice.organization_id, invoice.id);
  }
});

test("demo records: values satisfy the SH-17 checks", () => {
  for (const record of [...demoRecords.vendors, ...demoRecords.invoices]) {
    assert.ok(Number.isInteger(record.version) && record.version === 1, `${record.id}: version 1`);
  }
  for (const invoice of demoRecords.invoices) {
    assert.match(invoice.currency, /^[A-Z]{3}$/, invoice.id);
    assert.ok(
      Number.isSafeInteger(invoice.total_minor_units) && invoice.total_minor_units >= 0,
      invoice.id,
    );
    assert.match(invoice.issued_on, /^\d{4}-\d{2}-\d{2}$/, invoice.id);
    assert.ok(invoice.due_on >= invoice.issued_on, `${invoice.id}: due before issue`);
  }
});

test("demo records: A01 and A02 share INV104 in scope; B01 is the same vendor but out of scope", () => {
  const invoicesById = new Map(demoRecords.invoices.map((invoice) => [invoice.id, invoice]));
  const scope = demoRecords.task_scope;
  const [first, second] = scope.selected_invoice_ids.map((invoiceId) =>
    invoicesById.get(invoiceId),
  );
  assert.deepEqual(scope.selected_invoice_ids, ["invoice_A01", "invoice_A02"]);
  assert.equal(first.external_reference, "INV104");
  assert.equal(second.external_reference, "INV104");

  assert.deepEqual(scope.out_of_scope_invoice_ids, ["invoice_B01"]);
  const outOfScope = invoicesById.get("invoice_B01");
  assert.ok(outOfScope);
  assert.equal(scope.selected_invoice_ids.includes("invoice_B01"), false);
  // Out of scope, not merely inaccessible: same organization and vendor as the task.
  assert.equal(outOfScope.organization_id, scope.organization_id);
  assert.equal(outOfScope.vendor_id, scope.vendor_id);
  assert.notEqual(outOfScope.external_reference, "INV104");
});

test("demo records: the second organization holds its own invoice", () => {
  const scopeOrganization = demoRecords.task_scope.organization_id;
  const otherInvoices = demoRecords.invoices.filter(
    (invoice) => invoice.organization_id !== scopeOrganization,
  );
  assert.ok(otherInvoices.length >= 1);
  assert.ok(demoRecords.organizations.length >= 2);
});

test("demo records: only invoice_A01 carries the clean internal note", () => {
  const notedInvoices = demoRecords.invoices.filter((invoice) => invoice.internal_note !== null);
  assert.deepEqual(
    notedInvoices.map((invoice) => invoice.id),
    ["invoice_A01"],
  );
  const cleanNote = notedInvoices[0].internal_note;
  assert.ok(isAscii(cleanNote));
  // Kept apart from the hostile fixtures: no hostile text, and no instruction to the agent.
  for (const hostileNote of hostileNotes.notes) assert.notEqual(cleanNote, hostileNote.text);
  assert.doesNotMatch(
    cleanNote,
    /ignore previous|assistant|\bAI\b|queue|send|export|invoice_B01|\.example/i,
  );
});

test("demo records: the task vendor has a registered example.com reporting address", () => {
  const vendorsById = new Map(demoRecords.vendors.map((vendor) => [vendor.id, vendor]));
  const taskVendor = vendorsById.get(demoRecords.task_scope.vendor_id);
  assert.match(taskVendor.registered_reporting_address, /^[^@\s]+@([a-z0-9-]+\.)*example\.com$/);
  for (const vendor of demoRecords.vendors) {
    const address = vendor.registered_reporting_address;
    if (address !== null) assert.match(address, /@([a-z0-9-]+\.)*example\.com$/, vendor.id);
  }
});

test("demo records: a note carries its classification, and the clean note is internal_only", () => {
  // Mirrors invoices_note_classification_with_note: set exactly when the note is set.
  for (const invoice of demoRecords.invoices) {
    assert.equal(
      invoice.internal_note === null,
      invoice.internal_note_classification === null,
      invoice.id,
    );
  }
  const noted = demoRecords.invoices.find((invoice) => invoice.id === "invoice_A01");
  assert.equal(noted.internal_note_classification, "internal_only");
});
