# Submission checklist (RS-09)

Owner: researcher, document owner and presenter; a person submits, never an automatic step. Status:
**built from the official documents on 3 October 2026; not yet filled in.** Sources: competition
rules [S9] §5, §6, §12 and §13; general HackYeah rules [S11] §4.3, §5.8 and §6; report 1.2, "Submission
package and deadline controls" and the hours 21-24 row of "Proposed 24-hour implementation
sequence". Requirement IDs refer to [requirements.md](requirements.md).

**Deadline.** Submission on HackTribe no later than **11:00 on 4 October 2026** (the lead's reading;
the printed "11:00 PM" is organizer question 2). Until the organizers confirm, work to 11:00.
"Any alterations and modifications made after the statutory time is expired will not be considered
by the Jury." ([S9] §13)

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

Record each item at the freeze and again if a critical fix changes it:

| Item                                      | How to record it                                                                                                                     | Value |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | ----- |
| Submitted commit (build identifier, X-59) | `git rev-parse HEAD` on `main` after the last merge                                                                                  |       |
| Branch pushed                             | `git status` shows nothing to push; the hash is on `origin/main`                                                                     |       |
| Policy file                               | `shasum -a 256 config/policy.yaml`, plus the active catalog revision ID                                                              |       |
| Signature feed                            | `shasum -a 256 config/attack-signatures.json`, plus its revision                                                                     |       |
| Model                                     | `ollama list` name and ID of the model in `allowed_models` (decision 6)                                                              |       |
| Test results                              | The machine-readable result file of the control suite (X-89) and its SHA-256                                                         |       |
| Performance measurements                  | The benchmark output (X-95, SH-50) and its SHA-256                                                                                   |       |
| Presentation PDF                          | `shasum -a 256` of the submitted PDF                                                                                                 |       |
| Evidence bundle                           | SHA-256 of each screenshot, recording and export sample used on the slides (SH-32)                                                   |       |
| Project report                            | `task-passport-project-report.docx` brought to the submitted model, contracts, policy and scope (RS-09 "Done when"), and its SHA-256 |       |

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
