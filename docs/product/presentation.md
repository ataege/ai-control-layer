# Presentation, ten slides (RS-08)

Owner: researcher, document owner and presenter. Status: **final text, written 4 October 2026 from
`main` at `c6438c2`; ready to paste into a 10-slide PDF** ([S9] §5e: at most 10 slides, English). Only
the items marked **[FILL]** or **[SCREENSHOT n]** below are still open: they depend on the frozen
commit (08:00) and on the web screens being built tonight.

Rules this text follows: every claim is one of [claim-to-proof.md](claim-to-proof.md), with its scope
stated; every number comes from a file on `main`; the simulated outbox, the replay and test evidence
carry their labels; nothing says "universal", "guarantees" or "prevents all". Structure: report 1.2,
"Proposed ten slide presentation structure".

## Before you paste (fill-ins)

| Slot               | What goes there                                                                                                                         | When                        |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- |
| **[FILL team]**    | Team name and members (slide 1; same as the HackTribe fields in `submission-checklist.md` section 0)                                    | Before export               |
| **[FILL build]**   | The frozen commit hash (slides 1, 9, 10). The evidence on slide 9 is for build `cdfee55` until c1's rerun on the frozen commit lands    | After 08:00, rerun by 09:30 |
| **[SCREENSHOT n]** | A screenshot of the screen named in the slot, taken from the final build; **each slot lists the terminal alternative** that works today | As the web screens land     |

Screens on `main` at `c6438c2`: sign-in and the new-task form. Not yet: the run page, report view,
approval screen, security dashboard and labels. The API routes behind them are on `main` (runs, events,
usage, reports, approval and review, security summary and export, control evaluate), so every slot has
a terminal alternative; a slide uses whichever exists when the PDF is exported, and **never a mock-up**.

Evidence labels used below: **suite** = `docs/evidence/verify-controls-2026-10-03T21-53-09Z.json`
(build `cdfee55`, SHA-256 `5954f087ca2446e3d39d5a20e896db98c709ee3bcf1fb54ef1d8da790350ba58`);
**roadmap** = `docs/roadmap/go.md`; **README** = `services/gateway/README.md`.

---

## Slide 1. Task Passport

**Title:** Task Passport: an AI control layer for one bounded agent task

**On the slide**

- Task Passport controls what an agent can do and where the information it uses can go.
- Goldman Sachs AI Control Layer challenge, HackYeah 2026.
- Team **[FILL team]**.
- Synthetic invoices · simulated outbox · local model (`qwen3.5:4b` through Ollama).
- Build **[FILL build]**.

**Evidence:** the report's positioning statement (claim CL-01, scope: the registered invoice workflow,
four tools, two templates).

**Presenter notes**

- One sentence of context: an agent given a job should get authority for that job, with these records
  and these limits, and nothing more.
- Say once, early: the data is synthetic, the outbox is a database record and no email is sent, and the
  model runs locally.
- Do not say "secure" without its scope.

**Visual:** title only; no screenshot.

---

## Slide 2. Allowed read, allowed recipient, forbidden export

**Title:** Permitted pieces can still make a forbidden flow

**On the slide**

- Reading the internal investigation note: allowed.
- Sending to Atlas, the correct registered vendor: allowed.
- Sending a report **built from that note** to Atlas: denied, and approval cannot override it.
- The task still finishes: a new vendor report is rendered from approved invoice fields only.

**Evidence:** `TestApprovalCannotOverrideTheExportRestriction` (suite); beat 5 shown by the labelled
replay (slide 6); report Figure 11. Claims CL-05, CL-07, CL-08.

**Presenter notes**

- This is the idea in one picture: the passport authorizes each piece separately, and a report
  inherits the restriction of its sources.
- The scope is the two classifications (`Internal only`, `Vendor shareable`) and the two fixed
  templates. Do not claim that it tracks paraphrases or encoded copies of a secret.
- Say that the deterministic gate decides this, not the model.

**Visual:** the report's Figure 11 redrawn, or **[SCREENSHOT 2]** the report view showing the `Internal
only` label and its source trail. Terminal alternative: the stored report row (`classification`,
source manifest) from `psql`.

---

## Slide 3. Architecture

**Title:** One execution boundary in Go

**On the slide**

- Next.js (task and security views) → NestJS (identity, policy import, public API, exports) → **Go
  gateway** (admission, checks, approvals, budgets, execution) → PostgreSQL (`app`, `runtime`, `demo`).
- Model and tools: four registered tools, one local model through Ollama; the model endpoint is a
  gateway-only setting.
- One policy file, `config/policy.yaml`, and one signature feed, both imported as immutable revisions.
- In the demonstration, every model request and tool effect passes through the gateway.

**Evidence:** report Figure 1; `infra/compose.yaml` maps the model variables to the gateway only
(commit `04c6e33`); decisions 1 to 6 in `docs/product/README.md`. Claim CL-02, scope: the registered
tools and the configured model.

**Presenter notes**

- Be candid about the boundary: a tool or credential reachable some other way is outside this
  guarantee. The layer governs the registered workflow, not every tool in an enterprise.
- The Go gateway is the only component that talks to the model and to the tools; NestJS forwards
  commands and reads sanitized views.
- This is the architecture diagram the criteria ask for ([S10] §3.1b).

**Visual:** the report's Figure 1, checked against the final build (the Mermaid in
`docs/product/project-architecture.md` is the older 1.1 version).

---

## Slide 4. The passport, the policy file and the hard limits

**Title:** A grant for one task, and a file that configures the guards

**On the slide**

- The passport fixes the tools, invoices, vendors, report templates, projection rules, recipient
  references and models for the run. It is immutable after admission.
- Admission refuses what exceeds the operator's authority; scope, destination and report lineage are
  checked by deterministic code before any model is asked.
- `config/policy.yaml`: guards, block or redact, a semantic threshold of 0.75, allowed model, and
  budgets: 24 model calls (12 agent, 12 security), 40,000 tokens, 20 s per request, 2 concurrent
  requests, 15-minute expiry.
- Optional detector settings never remove the passport, organization or report-provenance checks.

**Evidence:** `packages/contracts/schemas/passport.schema.json`; `config/policy.yaml`;
`TestPostgresAdmissionRejectsWhatExceedsAuthority`, `TestResourceAndDestinationBoundaries`,
`TestReadInvoiceRefusesOutOfScopeAndOtherOrganizationInvoices` (suite). Claims CL-03, CL-04, CL-15.

**Presenter notes**

- The budget numbers are the team's settings, not a sponsor requirement; the token total was raised
  from the report's illustrative 20,000 to 40,000 after a live run ran out (README decision 29).
- An out-of-scope read is denied before an adapter runs, and the excluded record is unchanged.
- The policy file is the one a judge can edit (slide 8).

**Visual:** an excerpt of `config/policy.yaml` (slide text from the file), or **[SCREENSHOT 4]** the
passport summary beside the run timeline. Terminal alternative: `GET /api/runs/{id}` output.

---

## Slide 5. The semantic check, and what it cannot do

**Title:** A budgeted AI check that can only restrict

**On the slide**

- A separate, metered call to the local model inspects free text before it reaches the agent:
  verdict `{risk_category, score 0 to 1, reason_code}`, validated by Go; a score at or above 0.75 blocks.
- Fails closed: timeout, unavailable model, malformed verdict or exhausted security allowance pauses or
  denies; it never allows.
- A verdict can block or redact, never authorize. Configured secret patterns are redacted: 27 of 27
  redaction cases passed.
- **On this fixture set** (final run, `classifier_v2`): 28 of 29 live cases matched, 0 false
  positives, 1 false negative; the gate still denies the action that note asks for.

**Evidence:** suite, live part (28/29, 0 false positives, 1 false negative:
`hostile_note_redirect_recipient_v1`, score 0); `TestSemanticFalseNegativeStillDeniedDeterministically`
(passes for all three hostile notes); `TestValidationAndUnavailableFailClosed`. Claims CL-10 to CL-13.

**Presenter notes**

- Read the limit aloud: "Live verdicts are observations of a finite synthetic sample, not a detection
  rate; the deterministic gate still denies the actions these notes ask for."
- Variance sentence: the live label counts moved between runs of identical code: a different false
  positive in earlier runs, two other false negatives in the previous run, and one different miss in
  this one. That is the model's variance near the threshold, which is why a mismatch is recorded and
  does not fail the suite.
- Design point to state if asked: for the four MVP tools the semantic check does **not** run on action
  proposals, because the live model scored benign proposals 0.85 to 1.0; the action control is the
  deterministic gate plus signatures (README decision 25).

**Visual:** **[SCREENSHOT 5]** a blocked note in the run timeline with its verdict labelled "live".
Terminal alternative: the live part of the suite output, or `pnpm judge` with a hostile note.

---

## Slide 6. The denied export (labelled replay)

**Title:** The export is denied, and the outbox stays empty

**On the slide**

- The live model created the internal report; it proposed the export in **1 of 3** live runs (denied) in
  the second sample and 0 of 3 in the first. So this beat is shown with a **labelled replay**.
- Replay output: `LABELLED REPLAY (deterministic rehearsal, not a model-generated action)`: decision
  `deny`, reason `report_export_restricted`, nothing executed.
- Outbox rows before: 0. After: 0. No execution attempt. The stored action and events carry the label.
- A title or a model-supplied label never changes the stored classification (test evidence).

**Evidence:** `services/gateway/cmd/replay` and its README section; lane f3's GO-27 block in the roadmap
(commit `a9f8004`: run `8b812e16`); `TestStoredLabelAndTitleCannotOverrideTheLineage`,
`TestLabelRenameAndMissingLineageTampering`, `TestApprovalCannotOverrideTheExportRestriction` (suite).
Claims CL-05 to CL-07.

**Presenter notes**

- Say "labelled replay" every time. Never say or imply that the live model tried the export in this
  demonstration. The replay submits the scripted proposal through the **real** production gate.
- The replay runs on a finished run, so show the live story first (slide 7), then run it against that
  run, or against the run kept from the rehearsal.
- There is no rename operation, so the cosmetic-workaround beat is shown as **test evidence**, and
  called that.

**Visual:** the terminal output of `cmd/replay` plus `SELECT count(*) FROM demo.outbox_messages` before
and after, or **[SCREENSHOT 6]** the denial and the permitted alternative in the run page. Command:
`node scripts/with-env.mjs go -C services/gateway run ./cmd/replay -run <run id> -fixture
hostile_note_internal_disclosure_v1`.

---

## Slide 7. The task still finishes

**Title:** A vendor report from approved fields, reviewed, queued once

**On the slide**

- A new `vendor_reconciliation_v1` report is rendered by the server from six approved fields (invoice
  reference, external reference, duplicate flag, currency, total, due date); no internal note, no model
  prose.
- The reviewer sees the exact content and recipient; approval is bound to that stored action.
- Live run on the local model: vendor report created, approved, **one simulated outbox row**, run
  completed (6 agent calls, 1 security call).
- Concurrent approvals store one grant; a safe retry yields one outbox row.

**Evidence:** GO-47 "Completed (2026-10-03)" in the roadmap (commit `3aeade7`, run on
`qwen3.5:4b`, labelled live); `vendor_invoice_fields_v1` in `services/gateway/internal/provenance`;
`TestConcurrentApprovalDecisionsStoreOneGrant`, `TestKnownSafeRetryOfQueueReportYieldsOneOutboxRow`,
`TestQueueVendorReportCreatesOneSimulatedOutboxRow` (suite). Claims CL-08, CL-09, CL-30.

**Presenter notes**

- The simulated outbox is a database record: no email was sent.
- Be exact about the live run: in that run the model did not create the internal report (the scripted
  run covers that beat), and the feed and app records were loaded by the test harness. It shows the
  path can work, not final-build evidence.
- Until the approval screen exists, the approval in the demo is made through the same code the approval
  route calls; do not present it as the screen.

**Visual:** **[SCREENSHOT 7]** the approval screen with the exact content, then the outbox row.
Terminal alternative: `GET /api/actions/{id}/review` output, then `POST /api/actions/{id}/approval`,
then the outbox row from `psql`.

---

## Slide 8. Live configuration, budgets and judge input

**Title:** Change the policy, watch the next decision change

**On the slide**

- Edit `config/policy.yaml`, run `pnpm policy:import`: the gateway activates the requested revision
  within about a second. An invalid or unchecked revision is refused and the last good one stays active.
- Allowance is reserved before every model call, under a shared ceiling and per-purpose limits;
  concurrent requests cannot overspend; usage the provider does not report stays reserved.
- A failed model call is never re-sent: the run pauses.
- Judges can send their own input through the same gates: `POST /api/control/evaluate`, and the draft
  client `pnpm judge`.

**Evidence:** `TestPostgresRejectedRequestFailsAndKeepsTheLastGoodRevision`,
`TestPostgresConcurrentSharedBudget`, `TestEvaluateRetainsUnknownUsage`,
`TestModelLimitStopsTheRunBeforeTheNextDispatch`, `TestFailedModelCallIsNeverResent` (suite);
`apps/api/src/runs/control-evaluation.controller.ts`. Claims CL-15 to CL-18, CL-23.

**Presenter notes**

- Show the same input before and after a threshold or rule change, with the revision number in the
  decision. **[Rehearse and confirm before presenting.]**
- The judge client is a **draft**; confirm the live-test entry works end to end before showing it, and
  if it does not, say so and use the corpus test instead.
- The signature feed has eleven rules and no signing key: trust is the authenticated import plus the
  file's SHA-256. Never call it "signed".

**Visual:** **[SCREENSHOT 8]** the active controls and policy revision view. Terminal alternative: the
`pnpm policy:import` output and the pointer row (`active_revision_id`).

---

## Slide 9. Test results, audit export and performance

**Title:** The suite, the export and the numbers

**On the slide**

- `pnpm verify:controls` (`make verify-controls`): **PASS, 1,145 cases** on build `cdfee55` (Go 940,
  API unit 125, API database 29, fixtures 18; none failed or skipped). Live model 28 of 29, "on this
  fixture set". Earlier runs 1 and 2 failed and are reported.
- Audit export: JSON and CSV, sanitized, organization-scoped; counts match the summary; fields match
  the shared contract. Not tamper-proof.
- Deterministic controls take about **0.08 ms** at the median; the live semantic check about **1.9 s**,
  almost all model time; gateway overhead about **5 ms**.
- Observations on one M1 Pro machine under stated load; 10 live samples; not a distribution.

**Evidence:** suite (file above); `docs/evidence/api-audit-export-2026-10-04.json` (captured
2026-10-03T22:48Z, code commit `c70487e`: 14 event records and 2 assessment records, CSV with 10 and 18
columns, `countsMatchSummary`); benchmark in README, "Result on the developer machine (2026-10-03,
quiet)" (commit `3aeade7`, load average 12.03 at the start, 10 CPUs, 300 samples each for the two
model-less configurations, 10 live). Claims CL-20 to CL-22.

**Presenter notes**

- Quote the table honestly: semantic off, total p50 1,311 µs and p95 3,452 µs; fixture semantic on,
  total p50 1,288 µs, semantic 13 µs; live semantic on, total p50 1.93 s, provider 1.92 s, overhead
  5.4 ms. An earlier run under heavy load (load average 108) gave far larger numbers and describes only
  that state.
- The suite is evidence of build `cdfee55`; later changes are covered by the deterministic suite
  (`pnpm verify` 6/6, the lead's report) unless c1's rerun on the frozen commit lands: **[FILL build / rerun]**.
- The audit capture says it is not an atomic snapshot, performed no live-model approval, and makes no
  semantic-quality claim. Metrics refresh by authenticated polling, not streaming.

**Visual:** **[SCREENSHOT 9]** the security dashboard and an export file. Terminal alternative: the end
of the `pnpm verify:controls` output (status, counts) and the first lines of the CSV export.

---

## Slide 10. Integration, scope and limitations

**Title:** What is delivered, what is not

**On the slide**

- Integrates through a small documented contract: `POST /internal/control/evaluate` for governed model
  input and registered tool proposals; one command for the suite, one for a clean demo reset.
- Delivered: the passport, hybrid checks, policy file with live import, feed, budgets, approvals, the
  suite, audit export, telemetry. Deferred: agent-to-agent and MCP integration, a commercial-API
  ledger (a documented estimated-cost rule only).
- Limits: no universal prompt-injection detection; protection covers two templates and stored lineage;
  the outbox is simulated; the audit export is not tamper-proof; the semantic check does not run on the
  four tools' action proposals; no automatic model retries; the feed has no signing key; two identical
  start-run requests create two runs.
- Pre-event work and resources: starter and design documents prepared before the start; built with AI
  coding assistance (Claude Code); open-source stack and `qwen3.5:4b` (Apache 2.0) via Ollama (MIT).

**Evidence:** `docs/preparation-record.md` (starter prepared 2 October 2026); `docs/product/source-register.md`
(licenses); the "Known limitations" in `claim-to-proof.md`. Repository
https://github.com/ataege/ai-control-layer, submitted commit **[FILL build]**.

**Presenter notes**

- Read the limits, do not hurry them: they are what makes the claims believable.
- The pre-event disclosure follows HackYeah FAQ F-13 ("fairly cite or note that in your presentation,
  code, etc."). The wording is for the user and the lead to confirm; the organizers have not answered
  the question about the stricter "started solving" rule.
- Mapped to the OWASP Top 10 for LLM Applications 2025 (`comparison.md`): LLM04 and LLM08 are out of
  scope; say so if asked.

**Visual:** text only, plus a QR or link to the repository.

---

## Producing the PDF

1. Fill **[FILL team]** and **[FILL build]**; replace each **[SCREENSHOT n]** with a capture of the
   named screen from the final build, or the listed terminal output; delete a slide line whose claim
   cannot be shown, and add it to slide 10 as a limitation.
2. Run the wording checks in [claim-to-proof.md](claim-to-proof.md), "Wording checks at freeze".
3. Export to PDF; confirm at most 10 pages; record its SHA-256 in
   [submission-checklist.md](submission-checklist.md).
4. Rehearse with [storyboard.md](storyboard.md) and the runbook (`docs/demo-runbook.md`); the pitch
   length is not yet known (organizer question 4).
