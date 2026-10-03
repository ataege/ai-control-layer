# Presentation draft, ten slides (RS-08)

Owner: researcher, document owner and presenter. Status: **draft content only.** Every result, number,
screenshot and statement of what works is a placeholder marked **[replace with final-build
evidence]**. The PDF is produced from the final build at M5 and M6 (SH-32, X-59), with at most 10
slides ([S9] §5e). Structure: report 1.2, "Proposed ten slide presentation structure". Claims come
only from [claim-to-proof.md](claim-to-proof.md) rows marked verified; a claim that fails or is cut
leaves its slide and moves to the limitations on slide 10.

Each slide lists the judging criterion it mainly serves (weights per [S9] §11 / [S10] §8, open item
`scoring weights`): robustness and guardrails 30%, architecture and performance 20%, security
reporting 20%, self-testing suite 20% or 15%, practicality and scalability 10% or 15%.

Language: English ([S9] §5). Terms follow the report's working glossary (passport, admission, exact
action approval, reservation, simulated outbox, source manifest, Internal only, Vendor shareable,
field projection, semantic security control, control catalog, signature feed).

---

## Slide 1. Task Passport

Report: "Task Passport, team and the bounded-work problem." Criterion: framing.

- Title: **Task Passport**: a control layer for one bounded agent task.
- Positioning (CL-01): "Task Passport controls what an agent can do and where the information it uses
  can go."
- The problem in one line: an agent asked to do one job should get authority for that job, with these
  resources and these limits, and nothing more.
- Team name and members [team fills in].
- Footer: "Synthetic records; simulated outbox; local model." Build identifier **[replace with
  final-build evidence]**.

## Slide 2. Permitted pieces, forbidden combination

Report: "Why permitted tools and recipients can still produce a forbidden information flow."
Criterion: robustness and guardrails.

- Reading the internal investigation note: allowed. Sending to Atlas, the correct registered vendor:
  allowed. Sending a report built from that note to Atlas: **not** allowed.
- Report: "Permission to read a source and permission to send to a vendor would not, by themselves,
  authorize sending that source's information to the vendor."
- Visual: the report's Figure 11 (internal report denied, vendor report queued), redrawn to match the
  final build.
- Scope line: two classifications, two fixed templates, four registered tools.

## Slide 3. Architecture

Report: "Simple service architecture and the reusable Go control boundary." Criterion: architecture
and performance; also answers [S10] §3.1b ("a simple diagram presenting the architecture").

- Diagram: Figure 1, checked against the final build (`architecture specification version`).
- Next.js (task and security views) → NestJS (identity, catalog import, public API, exports) → Go
  (admission, hybrid checks, provenance, approvals, budgets, execution, telemetry) → PostgreSQL
  (`app`, `runtime`, `demo` schemas) → local model through Ollama.
- CL-02: every model request and tool effect in the demonstration goes through Go. Scope: the
  registered tools and the configured model.
- CL-29: runs on our machine with a local model, no paid services. Model: **[replace with final-build
  evidence: model name and ID]**.

## Slide 4. Passport, configuration and deterministic restrictions

Report: "Passport, centralized configuration and independent deterministic restrictions." Criterion:
robustness and guardrails.

- The passport: tools, records, destination, templates, approved models, limits, expiry; immutable
  after admission (CL-03).
- Deterministic checks run first: scope, destination, report lineage (CL-04, CL-10).
- One policy file, `config/policy.yaml`: guards, block or redact, thresholds, allowed models, budgets
  (CL-15). Excerpt of the submitted file **[replace with final-build evidence]**.
- "Optional detector changes do not grant new tools, records, destinations or export rights."

## Slide 5. The semantic check

Report: "Budgeted semantic security check with a real allowed/blocked example." Criterion: robustness
and guardrails.

- A separate, metered security call to the local model inspects tool results and inputs before they
  reach the agent (CL-10).
- Real example from the final build: one benign input allowed, one hostile note blocked, one secret
  redacted, with verdicts **[replace with final-build evidence]** (CL-11, CL-13).
- Fails closed: timeout, unavailable model, malformed verdict or exhausted security allowance pauses
  or denies (CL-12).
- A semantic verdict restricts, never grants (AGENTS.md guardrail 8). Evaluation: **[replace with final-build evidence:
  fixture count, model, prompt version, false positives, false negatives]**, worded "on this fixture
  set". No universal-detection claim.

## Slide 6. The denied export

Report: "Internal report export denied, cosmetic workaround denied and unchanged outbox." Criterion:
robustness and guardrails.

- The internal report shows Internal only and its source trail (CL-05). Screenshot **[replace with
  final-build evidence]**.
- `queue_report` to the correct Atlas address: denied, `report_export_restricted`. Outbox count before
  0, after 0 **[replace with final-build evidence]**.
- Renamed "Public summary": still Internal only (CL-06, subject to `rename operation`). Approval cannot
  override it (CL-07).
- Label: "Simulated outbox" and, if used, "Replay: scripted proposal" (storyboard, beats 5 and 6).

## Slide 7. The task still finishes

Report: "Approved-field vendor continuation, exact review and one committed effect." Criterion:
robustness and guardrails; practicality.

- A new `vendor_reconciliation_v1` report, rendered by the server from approved invoice fields only;
  no internal note, no model prose (CL-08). Screenshot **[replace with final-build evidence]**.
- The reviewer sees the exact content and recipient and approves; it is queued once; a changed
  recipient or content invalidates the approval (CL-09).
- Outbox count 0 → 1; outbox bytes equal reviewed bytes **[replace with final-build evidence]**.

## Slide 8. Live configuration, budgets and judge input

Report: "Live policy/feed change, model/resource budgets and ad-hoc judge interaction." Criterion:
robustness; architecture; practicality.

- Edit `policy.yaml` or the signature feed, reload: the next decision records the new revision; an
  invalid file is rejected and the last accepted revision stays (CL-14, CL-15, CL-16). Before and
  after **[replace with final-build evidence]**.
- Agent and security calls share one ceiling with per-purpose limits; reserved before dispatch;
  exhausted allowance stops the next call (CL-17, CL-18). Usage view **[replace with final-build
  evidence]**.
- Judges send their own input through the same gates with the judge client (CL-23).
- Signature-feed scope: text rules at the inspected boundaries (D-5).

## Slide 9. Test results, audit export and performance

Report: "Actual automated test outcomes, audit export and performance observations." Criterion:
self-testing suite; security reporting; architecture and performance.

- One command: `make verify-controls` (or `pnpm verify:controls`). Results: **[replace with
  final-build evidence: total cases, passed, failed, which are fixtures and which use the live
  model]** (CL-22). Failed cases are shown, not hidden.
- Security summary and a sanitized JSON or CSV audit export sample **[replace with final-build
  evidence]** (CL-20). Metrics refresh by authenticated polling (D-8).
- Performance: deterministic gate time against semantic and model time **[replace with final-build
  evidence: counts, p50, p95, machine, model, warmup, payload size, concurrency]** (CL-21), measured
  on an idle machine with the method of decision 22. "Report count, errors, p50 and p95 only after
  collecting observations."

## Slide 10. Integration, scope and limitations

Report: "Developer integration, delivered scope, limitations and evidence/demo links." Criterion:
practicality and scalability.

- Integration: a small documented adapter contract for governed model calls and tool proposals
  (CL-24); the invoice agent is a demonstration client.
- Delivered scope **[replace with final-build evidence: the verified CL rows]**.
- Deferred: agent-to-agent and agent-to-MCP integration (D-4); commercial API spending is a documented
  estimated-cost rule, not an implemented ledger (CL-19, D-6); broad exploit coverage (D-5).
- Limitations, from the report: no universal prompt-injection detection; the simulated outbox is not
  email delivery; export protection covers the two templates and stored lineage only; the audit
  export is not tamper-proof; "Production readiness is a future validation effort". Also: repeated
  identical start-run requests create separate runs (`command idempotency keys` open); the signature
  feed is trusted by authenticated import and SHA-256, with no signing key. Plus every failed or cut
  claim **[replace with final-build evidence]**.
- Pre-event work and external resources, cited as HackYeah FAQ F-13 and F-14 require: the starter
  and design documents with their true preparation dates, Ollama, the model and the main libraries
  (decision 8) **[replace with the agreed
  text]**.
- Links: repository https://github.com/ataege/ai-control-layer, submitted commit **[replace with
  final-build evidence]**, demo recording if allowed.

---

## Producing the PDF (M5 and M6)

1. Wait for X-59 (the final build identifier and recaptured evidence, SH-32).
2. Replace every **[replace with final-build evidence]** marker; delete any slide line whose claim is
   not verified, and add it to slide 10 as a limitation.
3. Run the wording checks of [claim-to-proof.md](claim-to-proof.md), "Wording checks at freeze".
4. Export to PDF; confirm it has at most 10 pages; record its SHA-256 in
   [submission-checklist.md](submission-checklist.md).
5. Rehearse it with the storyboard (SH-33) and the pitch length the organizers give (question 4).
