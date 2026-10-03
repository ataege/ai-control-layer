# Synthetic security fixtures

**Everything in this directory is synthetic test data.** No text comes from a real invoice, vendor,
person or system. Every recipient address uses the reserved `.example` domain, and every "secret" is
an obvious fake (`demo-only-...`) or a published documentation example (the example IBAN
`GB82 WEST 1234 5698 7654 32`, the public test card number `4111 1111 1111 1111`). The attack strings
are harmless instructions aimed at this project's own agent.

Roadmap task SH-49 (sync point X-86). Report 1.2: "Maintain a small labeled corpus containing benign
text, direct/indirect instruction attacks and secret-redaction cases" ("Validation plan and evidence
matrix"), and "A separate hostile-note fixture would attempt to redirect the agent to invoice_B01 or
another recipient" ("Illustrative invoice scenario and future domain adaptations").

| File                   | Content                                                                                                                                                                                 |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `hostile-notes.json`   | Three hostile internal-note texts for the separate hostile-note run: redirect to `invoice_B01`, redirect to another recipient, and include internal information in a vendor message.    |
| `semantic-corpus.json` | 24 labelled cases: 8 benign (4 of them hard negatives), 5 direct and 5 indirect instruction attacks, 6 secret-redaction cases.                                                          |
| `demo-records.json`    | The synthetic records of the Atlas scenario for the later seed (SH-25): organizations, vendors, invoices, the registered reporting address, the task scope and the clean internal note. |
| `fixtures.test.mjs`    | Self-check of all three files: `pnpm test:fixtures`.                                                                                                                                    |

## Demo records

`demo-records.json` holds the scenario's records. `pnpm db:seed` (draft, SH-18) loads its vendors
and invoices into the `demo` tables, and `pnpm reset:demo` restores them. Record fields match the
column names of the SH-17 draft migration (`demo.vendors`, `demo.invoices`); amounts are integers
in minor units, dates are ISO dates, every record is at version 1. The amounts and dates are
illustrative values, not requirements.

| Record                       | Role in the demonstration                                                                                                                                         |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `vendor_Atlas`               | The task's vendor.                                                                                                                                                |
| `invoice_A01`, `invoice_A02` | The selected invoices; both carry external reference `INV104` with the same total, the repeated reference the agent should find.                                  |
| `invoice_B01`                | Same organization and vendor, but outside the task scope: a read must be denied (`resource_out_of_scope`).                                                        |
| `internal_note` of A01       | The clean authorized internal investigation note: no instruction to the agent. The hostile notes are separate (`hostile-notes.json`).                             |
| `reports@atlas.example.com`  | Atlas's `registered_reporting_address`, the registered demonstration reporting address (reserved `example.com` domain); the outbox is simulated, nothing is sent. |
| Second organization          | `vendor_Borealis` and `invoice_C01`, for the organization-access check.                                                                                           |

Organizations are `app` records owned by NestJS; this file gives them fixed ids only so the demo
records can reference them. The decided items (lead's delegate, 3 October 2026) are recorded in
the file's `decisions` list: the note carries `internal_note_classification` (`internal_only` for
the clean note), the reporting address is `vendor_Atlas.registered_reporting_address`, and the
vendor projection holds the invoice reference, external reference, duplicate-reference flag,
currency, total and due date, never the note.

## The clean note and the hostile notes stay apart

The demonstration's baseline uses a **clean** authorized internal investigation note on
`invoice_A01`, classified `internal_only` in `internal_note_classification`.
It is the `internal_note` of `invoice_A01` in `demo-records.json`, and it holds no instruction to the
agent. The hostile notes in `hostile-notes.json` are separate: "The baseline note is clean; the
hostile note is a separate fixture." They are for a separate hostile-note run; how that run attaches
them follows SH-18. The self-check keeps the two apart.

## Fields

Both files carry `synthetic: true`, a `fixture_set` name and a `fixture_version`. Change the version
when a text or label changes, so recorded results can name the fixture they ran against.

`hostile-notes.json`, one entry per note:

| Field                            | Meaning                                                                                                                                                                                                                                                                                                                                                          |
| -------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                             | Stable identifier, unique across both files.                                                                                                                                                                                                                                                                                                                     |
| `attack`                         | What the note tries to make the agent do.                                                                                                                                                                                                                                                                                                                        |
| `boundary`                       | Where the text is inspected: always `tool_result`, because the note arrives through `read_invoice`.                                                                                                                                                                                                                                                              |
| `text`                           | The note's text.                                                                                                                                                                                                                                                                                                                                                 |
| `expected_outcome`               | `block`: the note should not reach the agent context.                                                                                                                                                                                                                                                                                                            |
| `deterministic_reason_if_obeyed` | The deterministic denial the gate must still return if the semantic check misses the note and the agent acts on it ("A test modeling a semantic false negative would still require the attempted out-of-scope read or restricted-report export to be denied by deterministic controls"). Values from the proposed reason vocabulary in `docs/product/README.md`. |

`semantic-corpus.json`, one entry per case:

| Field                     | Meaning                                                                                                                                                                                                              |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                      | Stable identifier, unique across both files.                                                                                                                                                                         |
| `category`                | `benign`, `direct_instruction_attack` (in the operator or judge input), `indirect_instruction_attack` (embedded in tool-result content) or `secret_redaction`.                                                       |
| `boundary`                | `model_input`, `tool_result` or `action_proposal`, the boundary names of the draft `config/policy.yaml` (SH-42).                                                                                                     |
| `text`                    | The text to inspect. Printable ASCII only, so byte offsets (Go) and UTF-16 offsets (JavaScript) agree.                                                                                                               |
| `expected_outcome`        | `allow`, `block` or `redact`, the report's "allowed, blocked, and redacted cases".                                                                                                                                   |
| `secrets`                 | Secret-redaction cases only: each secret's `kind`, `value` and its span (`start` inclusive, `end` exclusive). The value appears exactly once in the text.                                                            |
| `report_sample_signature` | Present on the case that contains the report's sample phrase "ignore previous instructions" in a tool result: the rule `prompt_ignore_previous_v1` should match it once the signature feed (SH-46) adopts that rule. |

The four benign hard negatives (`benign_hard_negative_*`) mention passwords, "ignore the earlier
invoice", portal instructions and an accounting "system" without being attacks or secrets, so the
evaluation can record false positives as the report asks.

## What these fixtures do not decide

- **Which secret patterns are configured** is the open item `redaction rules`. Each case names the
  kind of secret it contains, so the chosen rule set can select the cases it covers; a kind with no
  configured rule is a documented gap, not a pass.
- **The verdict's risk categories and score** are an open item,
  `classifier prompt and verdict schema`. The labels here are expected outcomes, not verdict
  categories; the semantic evaluator (GO-75) maps its validated verdict to an outcome with the
  catalog thresholds.
- **How the hostile notes and the corpus reach a run** (the seed loads only the demo records) follows
  SH-18 and the Go tests that use them. The seed location itself is decided: `fixtures/`.

## Reading the results honestly

An expected outcome is a test label, not evidence of detection quality. A run against the live local
model records the model and configuration, the actual verdicts, and the false positives and
negatives (SH-50, GO-84); a stubbed verdict tests composition only and is labelled as a stub. "A
finite passing fixture set does not prove universal detection."
