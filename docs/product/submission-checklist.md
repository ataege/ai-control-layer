# Submission checklist (RS-09)

Owner: researcher, document owner and presenter; a person submits, never an automatic step. Status:
**built from the official documents on 3 October 2026; not yet filled in.** Sources: competition
rules [S9] §5, §6, §12 and §13; general HackYeah rules [S11] §4.3, §5.8 and §6; report 1.2, "Submission
package and deadline controls" and the hours 21-24 row of "Proposed 24-hour implementation
sequence". Requirement IDs refer to [requirements.md](requirements.md).

**Deadline.** Submission on HackTribe no later than **11:00 AM on 4 October 2026**, confirmed in
writing by the organizers on 3 October 2026: "until 11:00 AM tomorrow" (organizer question 2).
"Any alterations and modifications made after the statutory time is expired will not be considered
by the Jury." ([S9] §13)

## 0. HackTribe text to paste (final, 4 October)

The organizers' announcement, relayed by the lead on 3 October 2026 at about 19:30 (verbatim in
[organizer-questions.md](organizer-questions.md), answer 2): a first draft needs "a few sentences about
the project you are working on, information about the category you are competing in", and "After
submitting the draft, you can still freely edit and update your project until 11:00 AM tomorrow." The
"Published" option "does not matter at this stage". If the first draft is not submitted yet, submit
the **short description** below with the category now, then paste the final text.

- [ ] **Project title:** `Task Passport`
- [ ] **Category:** AI Control Layer (Goldman Sachs)
- [ ] **Team name:** **[FILL team name]** (the team decides)
- [ ] **Team members (1 to 6):** **[FILL members]**. The git history of `main` shows three authors:
      Ata Ege, Noyan67 and Batın Adıgüzel. The user confirms the names, whether to add anyone else, and
      that each member is not excluded by rule 6 ([S9] §6, G-4).
- [ ] **Short description** (for the first draft; also usable as a one-paragraph summary):

> Task Passport is an AI control layer that governs one bounded agent task. A Go gateway issues an
> immutable task passport (the tools, records, recipients, report templates and model budget the task
> may use) and checks every governed model call and tool action against it, with deterministic rules
> and a separately metered semantic check on a local model. The demonstration, on synthetic invoices
> and a simulated outbox, shows an agent that may read an internal note and contact the vendor, but
> whose report built from that note cannot be exported to the vendor; the task still finishes through
> a separate vendor report built only from approved fields.

- [ ] **Final description** (paste before 11:00 AM; fill **[FILL build]** with the frozen commit, or
      keep `cdfee55` if the rerun did not happen, as section 3 says):

> **What it is.** Task Passport is an AI control layer that governs one bounded agent task. A Go
> gateway issues an immutable task passport (the tools, records, recipients, report templates and
> model budget the task may use) and checks every governed model call and tool action against it.
> Deterministic checks decide scope, destination and report lineage first. A separately metered
> semantic check on a local model (`qwen3.5:4b` through Ollama) can block or redact untrusted text but
> never grants access, and it fails closed.
>
> **What it shows.** On synthetic invoices and a simulated outbox (a database record; no email is
> sent), an agent may read an internal investigation note and the vendor is a permitted recipient, yet
> a report built from that note inherits its Internal only restriction and its export to the vendor is
> denied; approval cannot override it. The task still finishes through a separate vendor report
> rendered from approved invoice fields, reviewed as an exact action and queued once. The denied export
> is shown with a labelled replay of a scripted proposal through the real gate, not as a live-model
> action.
>
> **Control-layer deliverables.** An editable policy file with validated live import and a last-good
> fallback; a four-rule attack-signature feed (trusted by authenticated import and SHA-256, not
> signed); budgets reserved before every model call, with separate agent and security allowances; a
> security summary and a sanitized JSON and CSV audit export; performance telemetry; a small
> control-evaluation adapter; and a one-command test suite. `pnpm verify:controls` passed 1,145 cases
> on build **[FILL build]** (live model: 28 of 29 labels matched on our fixture set, 0 false
> positives, 1 false negative whose action the gate still denies).
>
> **Limits.** No universal prompt-injection detection; the protection covers two report templates and
> stored lineage; the semantic check does not run on the four tools' action proposals; the audit
> export is not tamper-proof; there are no automatic model retries. **Pre-event work:** the starter and
> the design documents were prepared before the start, and the project was built with AI coding
> assistance (Claude Code); see `docs/preparation-record.md`. Stack: Next.js, NestJS, Go, PostgreSQL,
> Ollama (MIT) and `qwen3.5:4b` (Apache 2.0). Repository:
> https://github.com/ataege/ai-control-layer

- [ ] After the first draft: replace it with the final description before 11:00 AM on 4 October.

## 1. Before the freeze (M5, by about 08:00)

- [ ] Organizer answers recorded in [organizer-questions.md](organizer-questions.md), or their
      absence stated: decision 8 (pre-event work and disclosure), start and deadline, scoring
      weights, judge access, pitch length.
- [x] The five HackYeah FAQ answers recorded ([requirements.md](requirements.md), "HackYeah FAQ";
      read by the lead's session on 3 October 2026).
- [ ] One project in this category only (FAQ F-4: "you can submit only one project in each
      category"; F-5 strongly discourages one project in several categories).
- [ ] Pre-event work disclosed in all three places FAQ F-13 names ("you must fairly cite or note
      that in your presentation, code, etc."): the HackTribe description, slide 10 and the
      repository (`docs/preparation-record.md`), with the true preparation dates from
      `docs/preparation-record.md`, not git timestamps. Final wording follows the organizers' answer
      to question 1.
- [ ] External resources indicated in the submission (FAQ F-14: "Remember to fairly indicate it in
      your submission"): the main libraries, Ollama and the model, from the RS-06 license list.
- [ ] Rule 6 ([S9] §6, R-08): the lead confirms for each member that they are not "related to or
      affined with members of the "AI Control Layer" Competition Jury" or an employee "promising the
      prize". Members confirmed: \_\_\_ (G-4).
- [ ] Licenses recorded for every dependency added after the starter, Ollama and the model weights
      (RS-06, G-3), so the [S11] §6.2 declaration ("does not infringe any third-party rights") holds.
- [ ] The repository has no license file (checked 3 October 2026). Team decision: add one, or state
      that all rights are reserved. Either way [S9] §14 keeps copyright with the authors.
- [ ] Every claim in the slides and the description is verified in [claim-to-proof.md](claim-to-proof.md);
      failed and cut claims are listed as limitations.

## 2. HackTribe fields ([S9] §5, R-03 to R-07)

All in English.

| Field                                     | Value                                                                                        | Done |
| ----------------------------------------- | -------------------------------------------------------------------------------------------- | ---- |
| a) Project title                          | Proposed: "Task Passport". Team confirms.                                                    | [ ]  |
| b) Team name                              | \_\_\_ (team decides)                                                                        | [ ]  |
| c) Team members (1 to 6)                  | \_\_\_ Include the research and presentation owner in the count.                             | [ ]  |
| d) Project description                    | Draft from the verified claims only (section 4 below).                                       | [ ]  |
| e) PDF presentation, at most 10 slides    | From [presentation.md](presentation.md) and the final build. Count the slides in the PDF.    | [ ]  |
| Optional: code repository                 | https://github.com/ataege/ai-control-layer (public, checked 3 October 2026)                  | [ ]  |
| Optional: demo links, snapshots, graphics | Recording of the final build, labelled with its build identifier, if the organizers allow it | [ ]  |

## 3. Freeze (M6, SH-34)

The freeze time is the team's decision (SH-34); leave enough time to capture evidence and submit
before 11:00. After the freeze, only critical fixes, each followed by recapturing the evidence it
affects.

**Freeze plan for today, 4 October (decided by the lead's delegate):**

1. The Go code freezes at **08:00**.
2. Lane c1 then runs `pnpm verify:controls` once more on the frozen commit, in a quiet window, and
   commits its sanitized results under `docs/evidence/`. The slides cite that run.
3. If the rerun fails, or cannot happen by **09:30**, the slides cite the run of build `cdfee55`
   (`docs/evidence/verify-controls-2026-10-03T21-53-09Z.json`) with the words "evidence of build
   `cdfee55`; later changes covered by the deterministic suite only".
4. Whichever run is cited, the submission description and the slides name its commit, the file and
   its SHA-256, and say "on this fixture set". A failed rerun is reported, not hidden.

Fill in below: the frozen commit ___ at ___; rerun started ___, result ___; file ___, SHA-256 ___;
which run the slides cite ___.

Record each item at the freeze and again if a critical fix changes it:

| Item                                      | How to record it                                                                                                                     | Value                                                                                                                                                                                                                                                                                           |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Submitted commit (build identifier, X-59) | `git rev-parse HEAD` on `main` after the last merge                                                                                  |                                                                                                                                                                                                                                                                                                 |
| Branch pushed                             | `git status` shows nothing to push; the hash is on `origin/main`                                                                     |                                                                                                                                                                                                                                                                                                 |
| Policy file                               | `shasum -a 256 config/policy.yaml`, plus the active catalog revision ID                                                              |                                                                                                                                                                                                                                                                                                 |
| Signature feed                            | `shasum -a 256 config/attack-signatures.json`, plus its revision                                                                     |                                                                                                                                                                                                                                                                                                 |
| Model                                     | `ollama list` name and ID of the model in `allowed_models` (decision 6)                                                              |                                                                                                                                                                                                                                                                                                 |
| Test results                              | The machine-readable result file of the control suite (X-89) and its SHA-256                                                         | Plan: rerun on the frozen commit (freeze 08:00, rerun in a quiet window, fallback to the `cdfee55` run at 09:30). Candidate until then: `docs/evidence/verify-controls-2026-10-03T21-53-09Z.json` (SHA-256 `5954f087ca2446e3d39d5a20e896db98c709ee3bcf1fb54ef1d8da790350ba58`, build `cdfee55`) |
| Performance measurements                  | The benchmark output (X-95, SH-50) and its SHA-256                                                                                   |                                                                                                                                                                                                                                                                                                 |
| Presentation PDF                          | `shasum -a 256` of the submitted PDF                                                                                                 |                                                                                                                                                                                                                                                                                                 |
| Evidence bundle                           | SHA-256 of each screenshot, recording and export sample used on the slides (SH-32)                                                   |                                                                                                                                                                                                                                                                                                 |
| Project report                            | `task-passport-project-report.docx` brought to the submitted model, contracts, policy and scope (RS-09 "Done when"), and its SHA-256 |                                                                                                                                                                                                                                                                                                 |

- [ ] Every screenshot and recording names the build identifier above.
- [ ] The demonstration machine runs exactly the submitted commit, policy, feed and model.
- [ ] The technical handoff (SH-35, X-62) is in the repository at the submitted commit: setup, the
      test command, the reset command, where `config/policy.yaml` and the feed are, known
      limitations.

## 4. Project description (draft at M5, final at M6)

Rules for the text: only claims marked verified in [claim-to-proof.md](claim-to-proof.md), each with
its scope; the simulated outbox, synthetic data and estimated cost named as such; the pre-event work
disclosed as the organizers require (decision 8). Skeleton:

1. What it is: a control layer that governs one bounded agent task (positioning statement, CL-01).
2. What it shows: the denied export of an Internal only report to a permitted recipient, and the
   permitted continuation (CL-05, CL-08, CL-09).
3. The control-layer deliverables the criteria ask for: hybrid controls, `policy.yaml` with live
   reload, signature feed, budgets, security summary and audit export, telemetry, the one-command
   test suite, the adapter contract (CL-10 to CL-24), each only if verified.
4. How to run it: setup, test command, reset command, judge client (from the handoff).
5. Limitations: from the "Claims to avoid" and every failed or cut claim.
6. Disclosure of pre-event work (text agreed with the lead, organizer-questions.md) and the
   external resources used (FAQ F-13, F-14).

## 5. Submit

- [ ] Fields entered, PDF uploaded, links checked from a browser that is not signed in.
- [ ] Submitted at \_\_\_ (local time), before 11:00. Screenshot of the confirmation saved with the
      evidence.
- [ ] The submitted commit hash sent to the whole team.

## 6. After the deadline

- [ ] No pushes to `main` that change the submitted product ([S9] §13, [S11] §5.8). Any later work
      goes to a separate branch and is not shown as part of the submission.
- [ ] The live demonstration and the phase-2 pitch run the submitted commit
      (`git checkout <submitted hash>`), with the recorded policy, feed and model.
- [ ] Watch HackYeah Discord for the jury list ([S9] §15) and the finalists.
