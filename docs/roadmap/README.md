# Task Passport roadmap

**Status.** This roadmap is a plan derived from the project report,
`docs/product/task-passport-project-report.docx` (version 1.1, "Report provenance and safe
continuation", 3 October 2026), the architecture specification `docs/product/project-architecture.md`,
the competition rules `docs/product/competition-rules.pdf` and the repository at commit `789bcd7`.
Tasks that report 1.1 changed carry a "Report 1.1 change" line; where that line and the older task
text disagree, the change line wins. Nothing in it is implemented: every task is open and every
sync point is unreached. Sizes are estimates, not a schedule. The milestone windows are relative to
the report's 24-hour coding window, which the competition rules fix at 11:00 on 3 October 2026 to
11:00 on 4 October 2026 (see "Milestones").
The plan changes when the report changes.

## How to use this roadmap

**Read the full report first.** AGENTS.md requires it at the start of every session, before the
first task: "read the full report from beginning to end, including the appendices. A skim, a search
or a single section is not enough." Then read `docs/product/project-architecture.md` and
`docs/product/README.md` for the decisions recorded since. AGENTS.md, section "Read the project report
first", gives the conversion commands; the text drops the ten figures, which correspond to the three
Mermaid diagrams in `docs/product/project-architecture.md`.

### The three files

| File                          | Holds                                                                                                                                                                                                                                                                                                                                                                        |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/roadmap/README.md`      | This file, the shared spine: sides and people, tiers, milestones, a task overview of the shared and researcher tasks, the work before the coding window, the cross-side interfaces, the sync points (X), the shared track (SH), the researcher track (RS), open decisions, what is not on the roadmap and the definition of done. The side files treat it as their contract. |
| `docs/roadmap/go.md`          | GO tasks: the Go side, with a task overview at the top.                                                                                                                                                                                                                                                                                                                      |
| `docs/roadmap/web-and-api.md` | API tasks (NestJS) and WEB tasks (Next.js): the Next.js + NestJS side, with a task overview at the top.                                                                                                                                                                                                                                                                      |

### Checkboxes

- `- [ ]` is an open task. `- [x]` is a done task: its "Done when" was observed and the checks in
  "Definition of done" ran, with their results quoted.
- The task owner ticks the box in the commit that completes the task, quotes the commands and their
  results in that commit, and pushes it (AGENTS.md, "Committing and pushing"). Integration owns `docs` except `docs/product`
  (AGENTS.md ownership table) and keeps the three files consistent.
- A task that cannot finish because of an open decision stays open; its "Blocked by" names the
  decision.
- A dropped task stays open too: it keeps `- [ ]`, and its title line ends with "Dropped: reason".
  It is never ticked, so it satisfies no "Depends on" and reaches no sync point.

### IDs

- This file holds `SH-nn` (shared track), `RS-nn` (researcher track) and `X-nn` (sync points).
  `docs/roadmap/go.md` holds `GO-nn`. `docs/roadmap/web-and-api.md` holds `API-nn` (NestJS) and
  `WEB-nn` (Next.js).
- Milestones are P and M0 to M6 (see "Milestones"). Tasks are numbered in milestone order.
- IDs are permanent. Nothing is renumbered. A dropped task keeps its ID and gets "Dropped: reason".
  A new task takes the next free number, so a late addition can sit outside milestone order.
- A side file refers to the other side only through X and SH IDs, never through the other side's
  task IDs. SH and RS tasks in this file refer to side work only through X IDs.
- "Depends on" lists the tasks that must be done before this task can be done, that is, before its
  box is ticked; it fixes the order in which tasks are ticked, not when work on them starts. Work
  may start earlier against the frozen contracts and their fixtures as they land, or against an
  interface the owners of both tasks agree, never against a proposal that is not decided, and every
  stand-in is replaced before the task is ticked: "A mocked response can unblock interface
  development, but it must be replaced or clearly labeled before any result is presented as a
  working capability." ("Delivery scope and six person ownership"). "Needs" works the same way.
  SH-14, RS-05 and RS-06 stay open until M6, and SH-23 until M3, so no task lists them there; a task
  that uses their work refers to it in its text.
- "Needs" lists the sync points a task consumes; "Provides" lists the ones it delivers. The rules,
  including the one for sync points with two providers, are in "Sync points".
- Landing the frozen contracts is SH-11 (the web + API implementer), and SH-39 for the operator context
  contract (X-14) once decisions 4 and 7 are recorded. `docs/roadmap/web-and-api.md` adds no
  separate landing task; `docs/roadmap/go.md` provides X-15, the Go DTO mirrors.
- Open items that have no number in `docs/product/README.md` get a fixed citation string in "Open
  decisions and blockers". Side files cite these strings verbatim, backticks included, in "Blocked
  by".

### Sizes

`Size: S (estimate 1-3 h)` gives a class and an hour range. The class follows the midpoint of the
range, or the single value when only one is given: S up to 4 h, M above 4 h up to 10 h, L above
10 h up to 20 h; a task above 20 h is split. Person-hour ranges are classed the same way.

### Scope changes

The roadmap follows the report and does not extend it. AGENTS.md: "If the feature you are about to
build is not covered there, ask the document owner and the owning implementer before inventing
architecture." When a design decision changes, or the chosen model, contracts, policy or
implementation scope changes ("Sources and evidence register"), the document owner (the lead until
`researcher role` is settled) updates the report's decision record, contracts, demonstration and claim-to-proof list together
(AGENTS.md, "Implementation workflow", step 6); this roadmap changes after that. Mentor feedback
becomes "a short requirements update, not uncontrolled feature additions" ("Research documentation
and submission workflow").

## Sides and people

The user split the work into two sides. The report describes six roles by responsibility ("Delivery
scope and six person ownership", table "Proposed team ownership"). This team has two implementers and
the lead, so each person holds several of the report's roles:

| Side                  | Person and report roles                                                                                                                                                                                                                                                                   | Agents and paths                                                                | Roadmap file                  |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | ----------------------------- |
| Go side               | Go implementer: Implementer 3 (agent runtime), Implementer 4 (enforcement) and the Go parts of Implementer 5 (four tool adapters, trusted source manifests, deterministic internal and vendor rendering, transactional effects)                                                           | go, every package of `services/gateway`                                         | `docs/roadmap/go.md`          |
| Next.js + NestJS side | Web + API implementer: Implementer 1 (interface) and Implementer 2 (application API, shared contracts, source-policy and projection configuration)                                                                                                                                        | frontend (`apps/web`, `packages/ui`); nestjs (`apps/api`, `packages/contracts`) | `docs/roadmap/web-and-api.md` |
| Shared track          | Implementer 5's data and integration work. Default, to be confirmed (`shared-track assignment`): migrations and seeds by the web + API implementer, with table definitions from the Go implementer; database roles, Compose and deployment, smoke checks and evidence capture by the lead | integration, infrastructure                                                     | this file                     |
| Researcher track      | Researcher, document owner, and presenter: no person assigned (`researcher role`); the lead acts as document owner until then                                                                                                                                                             | `docs/product`; reviewer                                                        | this file                     |

- The lead helps both sides when needed; that help is not fixed capacity.
- Inside one side the tasks are done by one person, in sequence. The report's "Build the vertical path
  in parallel" holds across the two sides only.
- The transactional effects are the Go implementer's (GO-34).
- Moving work between people is a team decision.

**Effort split (estimates, not a schedule).** Four independent estimates of the MVP, made on
2026-10-03 from the current starter with tests included, agree on the shape and disagree on the size.
They predate report 1.1 and contain no report-provenance work.

- Go is the larger side in every estimate: 54 to 58 percent of the non-shared engineering effort at
  the midpoints. The totals run from 117-236 h (lowest method) to 210-397 h (highest).
- Every total is above what two implementers can deliver in the 24-hour window: at most 48
  person-hours working all 24 hours with no break, 36 to 40 h under one estimate's own assumption of
  18 to 20 focused hours per person, plus the lead's help, which is not fixed capacity.
- Per side at the midpoints, Tier A is about 128.5 h on the Go side and about 90 h on the Next.js +
  NestJS side, each for one person against at most 24 h. The report 1.1 tasks add about 27-57 h, of
  which 22-45 h is Tier A.
- Removing every Tier B and Tier C task still leaves the plan far above the window; "Tiers" says what
  that requires. The tiers set the order of cuts; they do not make the work fit.

## Tiers

| Tier     | Meaning                                                                                                                                                                                                      | Report basis                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A        | Needed for the smallest credible vertical slice, or for one of the four protections the report keeps intact under time pressure.                                                                             | "the smallest credible vertical slice is one authorized invoice/report workflow, an Internal only report whose vendor export is denied, a separately generated Vendor shareable report from approved fields, one exact-action approval and one limit-triggered stop" (report 1.1) ("Threat model limits and unresolved design choices"); "Keep service-side authorization, scope enforcement, pre-dispatch limits, and exact-action approval intact." ("Relative implementation milestones and critical dependencies") |
| B        | The rest of the report's MVP requirements, acceptance evidence and critical checks.                                                                                                                          | Table "Proposed MVP requirements and acceptance evidence" ("Functional requirements MVP boundary and deferred scope"); table "Proposed critical checks" ("Validation plan and evidence matrix")                                                                                                                                                                                                                                                                                                                        |
| C        | Optional breadth the report says to cut first: for example server-sent events after authenticated polling, extra screens, polish beyond the exit conditions, the optional unknown-outcome failure rehearsal. | "If time becomes tight, keep a fixed server-owned task template instead of a general policy editor, use one worker process with durable jobs instead of distributed scheduling, and use authenticated polling before adding SSE if necessary." ("Relative implementation milestones and critical dependencies"); "The team should first remove optional breadth when delivery is at risk, while keeping the checks required to substantiate the control-layer proposition." ("Design decision record")                 |
| Deferred | The report's deferred features. They are not on this roadmap; see "Not on this roadmap".                                                                                                                     | "Functional requirements MVP boundary and deferred scope", list "Deferred features"                                                                                                                                                                                                                                                                                                                                                                                                                                    |

Under time pressure, Tier C goes first and Tier A stays. For Tier B the report's rules apply: "If a
critical check fails, either fix it or narrow the supported behavior and the claims." ("Risk register
and scope controls"), and "A missing protection must become an explicit limitation, not a silently
mocked success." ("Relative implementation milestones and critical dependencies").

Keeping Tier A does not make the plan fit: by this roadmap's sizes Tier A alone is above the window
(see "Effort split" and "Milestones"), so cutting Tier C and narrowing Tier B are not enough.
Narrowing the supported behaviour and the claims inside Tier A is a team decision that this roadmap
does not make: the team agrees it, the document owner records it as a scope change (see "Scope
changes"), and the side files change after that. Inside Tier A the report's order still holds: keep
"service-side authorization, scope enforcement, pre-dispatch limits, and exact-action approval
intact", and a protection that is still missing becomes "an explicit limitation, not a silently
mocked success" ("Relative implementation milestones and critical dependencies"; in the report that
sentence directly follows the four protections, so it covers Tier A as well as Tier B; AGENTS.md
guardrail 7 says the same).

Roadmap rule, not a report tier: the submission itself is also Tier A. That covers the
claim-to-proof list (RS-05), the presentation (RS-08) and the submission checklist and submission
(RS-09). Report basis: "Maintain a claim-to-proof list: each presentation claim names a demo step,
observed record, or completed check." ("Research documentation and submission workflow"); "The
non-negotiable quality threshold is a truthful account of the implemented boundary." ("Risk
register and scope controls"); and, for hours 21-24, "finalize documentation and artifacts, and
submit using the confirmed organizer requirements" ("Relative implementation milestones and
critical dependencies").

The submission depends on Tier B work: RS-08 and RS-09 wait on SH-32, which provides X-59, and on
SH-35, which needs X-62, and through them on SH-31, SH-33, SH-34, SH-29, SH-30 and SH-09 and on the
side tasks behind the sync points those tasks need. These keep Tier B because their content narrows
under time pressure, but only as far as RS-08 and RS-09 still get what they use, and none is
skipped: each completes with what the final build delivers, and failed, unverified and cut items
are recorded as such and stated as known limitations ("Keep failed or unverified checks visible."
("Sources and evidence register")).

Roadmap rule on dependencies, so that Tier A can stay when Tier B is cut: no Tier A task lists a
Tier B or C task under "Depends on", and none waits for a sync point, or a part of one, that only
Tier B or C tasks provide. A prerequisite that a Tier A task cannot do without is Tier A itself.
Tier B work that a Tier A task uses only when it exists is named in that task's text, or, for
evidence, listed under "Needs" without being waited for and recorded as not verified while it is
missing (see "Sync points"). Two exceptions remain: RS-08 and RS-09 depend on the closing tasks
above, which are narrowed, never skipped; and SH-23 waits for X-32, which only Tier B work
provides, because decision 5's repository rule requires that work whatever its tier.

**Consequences of report 1.1 for the tiers.** These follow from the new Tier A definition; they are
not a team narrowing:

- "one denied out-of-scope operation" left the slice, so GO-30 is Tier B and moves to M4, and X-37 and
  X-38 are needed by M4. The report keeps the out-of-scope read as a supporting check; keeping it in
  the main demonstration is the team's call.
- The Internal only report with its denied export, the separately generated vendor report and the
  safe-continuation path entered the slice, so GO-63 to GO-67, API-28 to API-30, WEB-27, WEB-28, SH-40
  and SH-41 are Tier A, and GO-29 is Tier A because a continuation in the same run needs it.
- Implementer 4's new first integrated deliverable ("even if an approval is submitted") lands in GO-69,
  which the rule makes Tier B (the approval override is a critical check outside the slice); the team
  may raise it.
- The report says the broader architecture "should not displace this complete enforcement and
  safe-continuation path".

## Milestones

The windows are the report's relative windows from the table "Proposed 24-hour implementation
sequence" ("Relative implementation milestones and critical dependencies"). The report: "The
implementation plan assumes a 24-hour coding window; confirmed rules and the actual submission
deadline take precedence." The competition rules (`docs/product/competition-rules.pdf`, times as
confirmed by the team lead) fix the window at 11:00 on 3 October 2026 to 11:00 on 4 October 2026, so
the report's windows map to the clock times below. Changes after 11:00 on 4 October are not
considered, so the submission happens before then. The focus and exit columns quote report 1.1. The
last column lists the sync points that must be reached by the end of the milestone, which is their
"Needed by" in "Sync points".

| Milestone | Report window | Clock time              | Team focus (report 1.1)                                                                                                                                                                                                                                       | Exit condition (report 1.1)                                                                                                                                                                                                                                                                                                                 | Sync points needed by the end                                                                      |
| --------- | ------------- | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| P         | None          | before 11:00, 3 October | Not in the report. Roadmap: decisions, environment setup, container validation and organizer questions.                                                                                                                                                       | The report gives no window or exit condition for P. Roadmap condition, not a quote: the decisions the first contracts depend on are recorded or carried into M0 as open, every machine passes `pnpm verify`, and the organizers' answer on decision 8 is recorded or its absence is stated. Code written before 11:00 waits on that answer. | X-01                                                                                               |
| M0        | Hours 0-2     | 11:00-13:00, 3 October  | "Confirm rules and sponsor expectations; freeze task, contracts, tool arguments, policy fixture, and schema ownership. Bring up the starter and provider connection. Fix the two classifications, two report templates and approved vendor field projection." | "A single documented workflow, working service connectivity, and an agreed contract example for each command and event."                                                                                                                                                                                                                    | X-03 to X-06                                                                                       |
| M1        | Hours 2-6     | 13:00-17:00             | "Build the vertical path in parallel: task form, authenticated facade, admission, worker, live model call, one read tool, and event feed."                                                                                                                    | "A real operator starts a run; Go executes a permitted tool; the interface displays the actual persisted result."                                                                                                                                                                                                                           | X-07 to X-32                                                                                       |
| M2        | Hours 6-10    | 17:00-21:00             | "Complete the four adapters, scoped reads, trusted source manifests, both fixed report templates, inherited classifications, destination checks and bounded feedback."                                                                                        | "An internal report is retained for authorized internal viewing; its vendor export is denied; a separate approved-field vendor report can be created."                                                                                                                                                                                      | X-33 to X-36, X-63, X-64, X-68 to X-72, X-75 (X-65 only if the replay is triggered through NestJS) |
| M3        | Hours 10-14   | 21:00-01:00             | "Add frozen action previews, authorized approval decisions, approval consumption, model/tool reservations, and controlled stopping."                                                                                                                          | "The reviewed action executes once; changed content and depleted allowance cannot dispatch an operation."                                                                                                                                                                                                                                   | X-39 to X-46, X-66, X-67, X-77                                                                     |
| M4        | Hours 14-18   | 01:00-05:00, 4 October  | "Exercise concurrency and waiting-state recovery; inspect context and safe events; finish reset and deployment procedures. Verify label tampering, missing lineage, rename attempts and changed source-policy versions."                                      | "Critical checks have recorded outcomes, and the team can reset fixtures and repeat the workflow."                                                                                                                                                                                                                                          | X-02, X-37, X-38, X-47 to X-57, X-73, X-74, X-76 (X-60 is optional, Tier C)                        |
| M5        | Hours 18-21   | 05:00-08:00             | "Polish comprehension and error states; rehearse the complete story; capture evidence from the final build."                                                                                                                                                  | "A reviewer can understand the task boundary, attempted action, decision, actual effect, and limitation without narration filling gaps."                                                                                                                                                                                                    | X-58, X-59 (X-61 is optional, Tier C)                                                              |
| M6        | Hours 21-24   | 08:00-11:00             | "Freeze features, fix critical faults, finalize documentation and artifacts, and submit using the confirmed organizer requirements."                                                                                                                          | "Submission checklist is complete; the demonstration matches the submitted build and its documented limitations." Submission on HackTribe before 11:00.                                                                                                                                                                                     | X-62                                                                                               |

The competition's judging weights (robustness of the solution and quality of guardrails 30%,
architecture and performance efficiency 20%, security reporting 20%, completeness of the self-testing
suite 20%, practical implementability and scalability 10%) make the critical checks and their
evidence scored work, not only demonstration support.

Notes:

- The report's sequence names little web or API work in hours 6-10 (M2), and it does not name several
  items of the ownership table: for the interface the passport summary, run timeline, terminal states,
  report classification, source trail, export denial and alternative template; for the application API
  authentication, membership checks, task configuration and source-policy and projection
  configuration. The side roadmaps place that work in M2, except the parts the M1 exit already needs:
  the M1 focus names an "authenticated facade", so authentication and the membership check on the
  start-run command come at M1.
- The authentication design (decision 7) is on hold, and the M1 exit needs "A real operator". Decisions
  3 and 4 wait on decision 7 (SH-02, SH-03). While the hold stands, the M1 exit cannot be reached, and
  by the "Depends on" column of "Sync points" neither can X-14, X-22, X-23, X-26, X-27, X-28, X-31,
  X-40, X-42, X-43, X-55 and X-56, nor the optional X-60 and the conditional X-65. Through their
  providing tasks more sync points wait, among them X-44 to X-47, X-50 to X-54, X-57 to X-59 and X-62,
  and X-25, X-29, X-30, X-41 and X-64 when a Go endpoint serves them. X-17 waits on decision 7 for the
  user records, so SH-15 stays open, and with it X-18, X-21, X-34 and X-68 (SH-41 depends on SH-15),
  and through X-68 the chain of GO-63 and the evidence X-72 and X-75. X-71 does not wait. The task,
  policy, template and tool tables and their seed can still be written and applied during the hold.
  This roadmap records that and does not resolve it.
- "Progress should be measured by integrated behavior rather than completed screens or isolated
  modules." If the early checkpoint is late, "remove optional breadth before adding more features."
- Load against the windows (sums of this roadmap's task ranges by placement, report 1.1 tasks
  included, the researcher track excluded; estimates, not a schedule): P 10-25.5 h, M0 14-38.5 h, M1
  93-198 h, M2 65-140.5 h, M3 41-81.5 h, M4 32.5-81.5 h, M5 11-32 h, M6 4-11 h. Capacity from the two
  implementers working the whole window with no break: M0 4, M1 8, M2 8, M3 8, M4 8, M5 6 and M6 6
  person-hours, plus the lead's help. Per-person loads can be summed: every GO task is the Go
  implementer's, every API and WEB task the web + API implementer's, and the SH tasks follow "Sides
  and people".

First integrated deliverables, from the table "Proposed team ownership" (report 1.1). The milestone is
this roadmap's placement, not the report's:

| Report role                               | Person                                                         | First integrated deliverable (report 1.1)                                                                                        | Milestone |
| ----------------------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | --------- |
| Implementer 1: interface                  | Web + API implementer                                          | "Start a run and display a real persisted event through the NestJS API."                                                         | M1        |
| Implementer 2: application API            | Web + API implementer                                          | "An authenticated command reaches Go with verifiable actor and organization context."                                            | M1        |
| Implementer 3: agent runtime              | Go implementer                                                 | "A live model proposes a typed tool action within a recorded allowance."                                                         | M1        |
| Implementer 4: enforcement                | Go implementer                                                 | "An Internal only report cannot be exported to the correct vendor, even if an approval is submitted." (GO-66 at M2, GO-69 at M3) | M3        |
| Implementer 5: data and integration       | Go implementer (rendering); web + API implementer (migrations) | "The stored vendor report uses only approved invoice fields and creates one matching outbox effect." (GO-67 at M2, GO-47 at M3)  | M3        |
| Researcher, document owner, and presenter | Not assigned (the lead as document owner)                      | "A verified brief and a storyboard with observable evidence for every claimed protection."                                       | M1        |

## Before the coding window (P)

The report gives no window for this work, and this roadmap limits it to the four kinds below. Code
written before the coding window, including fixes to the starter, waits on the organizers' answer on
reuse of pre-event work (decision 8 in `docs/product/README.md`, RS-01). The report: "Do not presume
that pre-event code or prepared assets are eligible; the document owner should resolve this from
official rules before those materials are used." ("Delivery scope and six person ownership"). From
M0 on, the same answer gates every task that changes the starter's code: SH-11, SH-12, SH-13,
SH-17, SH-20, SH-21, SH-30, SH-37 and SH-39 need X-01, and every other code task reaches X-01
through its "Depends on" and "Needs", or lists it under "Needs". This roadmap assumes the answer
lets the team use the starter; a negative or conditional answer goes to the document owner first,
and the plan changes after that (see "Scope changes").

The step-by-step list is "Before implementation starts" in `docs/team-workflow.md`; it is not copied
here. The full tasks are in "Shared track" and "Researcher track".

### Decisions

SH-01 to SH-06 are the decide tasks: the authentication mechanism (decision 7), the browser to API
path (decision 3), the operator context sent to Go (decision 4), the model provider (decision 6), the
read path, and the single Go executor connection (a detail of decision 2). `docs/team-workflow.md`
says to start with the browser to API path and the operator context, "because the first contracts
depend on them"; both depend on decision 7, which is on hold. SH-07 records the contract owners, the
shared-track assignment and the researcher role.

### Environment setup

SH-08: each person installs Go 1.27 or newer and Docker with the Compose plugin, then quotes a
`pnpm verify` result. Go and Docker were not installed on the preparation machine.

### Container validation

SH-09: Compose, the Dockerfiles and full-container mode have never been executed, because Docker was
unavailable during preparation. One person with Docker runs them once and records the results in
`README.md`, section "Verification status".

### Organizer questions

RS-01 asks the organizers and sponsor mentors the questions the report lists, decision 8 first. RS-02
asks for cross-track rules only if another track is considered.

## Cross-side interfaces

### Contracts to freeze first

The report: "Freeze the small set of shared contracts before parallel development: start-run
request, passport representation, action proposal, approval decision, run state, and safe event.
Record one owner per contract and per Go module." ("Delivery scope and six person ownership"). The
seventh row is a repository addition from `docs/product/README.md`.

| Contract                                                                    | What it carries (report)                                                                                                                                                                                                                                                                                        | Owner        | Sync point |
| --------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ---------- |
| Start-run request                                                           | The operator's choices: "which invoices to reconcile, which vendor is involved, where a report may go, which action needs review, and how long the run may continue". "The authenticated actor and organization must be supplied through verified context rather than trusted from agent arguments."            | not recorded | X-07       |
| Passport representation                                                     | The field groups of "Passport fields and their purpose": identity ("Organization, initiating actor and run reference"), versions, capabilities, data and destinations, limits and lifetime.                                                                                                                     | not recorded | X-08       |
| Action proposal                                                             | One proposed action with a stable identifier, a registered tool and typed canonical arguments within "Proposed tool argument boundaries"; for review, "the tool, canonical arguments, recipient, affected resources, relevant versions, exact outbound content, passport reference, policy version and expiry". | not recorded | X-09       |
| Approval decision                                                           | Approve or reject one stored action: "The command sent back to Go identifies that stored action; it does not replace it with a new browser-supplied payload."                                                                                                                                                   | not recorded | X-10       |
| Run state                                                                   | Status, usage and terminal reason. The interface "would clearly distinguish approval waiting, rejection, expiry, execution success, and an uncertain result."                                                                                                                                                   | not recorded | X-11       |
| Safe event                                                                  | "Decision events should connect the run, action, policy version, matched rule, outcome and actual effect. Masked summaries support the timeline without copying confidential arguments into general logs."                                                                                                      | not recorded | X-12       |
| Authenticated operator context (repository addition, depends on decision 7) | Not in the report's list. The report requires that "Go must verify service identity and authenticated operator context"; the mechanism is decision 4.                                                                                                                                                           | not recorded | X-14       |
| Report and lineage summary (report 1.1)                                     | "Shared schemas should define supported requests, report lineage summaries and errors": the report's classification, source manifest summary, template and projection versions, content hash and destination class. Shape and storage depend on `report storage`. Sync point X-71.                              |

Frozen in the same M0 session (report, "Hours 0-2"): the four tools' typed arguments (inside X-09),
the reason vocabulary and the decision and error fields (X-13), the task definition and policy
fixture (X-06), schema ownership with table names (SH-10), and the tool-result contract (SH-10):
per tool, the fields it returns to the worker within the X-06 field rules, and the opaque references
for protected values ("Each tool returns an explicit field allowlist; protected values remain opaque
references."). The report: "The runtime developer depends on the action schema and tool-result
contract." ("Relative implementation milestones and critical dependencies", "Critical path and
sensible reductions"). It stays inside the Go side (Diagram 1: ADAPTERS -> FILTER -> RUNNER, all in
GO), so it has no sync point; the Go implementer records it, and `docs/roadmap/go.md` plans it.

**Report 1.1 additions to the freeze.** "Freeze report classification, source manifest, template
version, projection-rule version and export denial fields alongside those contracts." The same M0
session fixes "the two classifications, two report templates and approved vendor field projection"
(Hours 0-2); the field list is open item `vendor projection fields`. Contract content changes: the
start-run request adds "which internal evidence may be consulted, which report types may be created";
the passport adds "allowed report templates" and "source authority" (shape: `passport report fields`);
a report queue action "also binds its immutable report ID, content hash, trusted source manifest and
digest, classification, template and projection versions, and allowed destinations"; `create_report`
takes "Scoped source references and registered template identifier"; the safe event connects "the run,
action, policy version, matched rule, report ID, template version, classification, lineage-check
outcome and actual effect"; start-run and approval commands may carry an idempotency key (`command
idempotency keys`). The architecture's event names are illustrative input.

Rules for every contract:

- "Shared schemas should define supported requests, report lineage summaries and errors, while Go
  remains the authority for action canonicalization, artifact classification and execution." (report
  1.1) ("Technical architecture and service ownership")
- "The TypeScript applications and Go runtime should describe the same identifiers, enum values,
  timestamps, error semantics, and action integrity rules." ("Illustrative passport and interface
  contracts")
- Money, if it is introduced, "should use an explicitly defined decimal or minor-unit representation
  rather than relying on cross-language floating-point equality." ("Exact action approval versioning
  and execution rechecks")
- Every change follows "Changing a shared contract" in `docs/team-workflow.md`: type, JSON Schema and
  fixtures together in `packages/contracts`, then the Go DTOs, then the consumers. "Never change a
  response shape on one side only."
- Repository facts: every schema needs a fixture and a typed sample, or
  `pnpm --filter @workspace/contracts run test` fails; `packages/contracts` compiles with
  `erasableSyntaxOnly`, so TypeScript enums cannot be used there; the Go fixture test
  (`services/gateway/internal/health/dto_test.go`) lists fixtures by explicit case, so a new fixture
  stays unchecked on the Go side until a case is added.

### Browser operations

Exactly as the report's table "Proposed browser and runtime operations" gives them ("Illustrative
passport and interface contracts"):

| Browser operation through NestJS  | Runtime or read behavior                                  | Authority                                                                |
| --------------------------------- | --------------------------------------------------------- | ------------------------------------------------------------------------ |
| `POST /api/runs`                  | `POST /internal/runs` issues the passport and durable job | Verified actor and organization; authoritative task and policy versions. |
| `POST /api/runs/{id}/cancel`      | Persist cancellation before future dispatches             | Actor may manage this run within this organization.                      |
| `POST /api/actions/{id}/approval` | Approve or reject the stored action                       | Authorized reviewer; exact action integrity and expiry.                  |
| `GET /api/runs/{id}`              | Read authorized run and usage view                        | Organization and object access checked on every read.                    |
| `GET /api/runs/{id}/events`       | Read sanitized events with cursor or SSE                  | Authorized subscription; no unrestricted raw payload stream.             |

How the browser reaches these operations is decision 3. How NestJS serves the two reads is the read
path. The table has no operation for the task form's options (`form options`), for the reviewer's
exact payload (`review payload read`) or for the stored report and registered template from which
the interface renders the final result (`stored report read`).

### Internal runtime operations

The report names one internal path, `POST /internal/runs`. Every other internal operation is named at
the M0 freeze (SH-10) by its owner. The report: "NestJS would expose these operations and call
corresponding private Go endpoints. Go must verify service identity and authenticated operator
context, then authorize the command against its organization and run. A signed user identifier alone
is insufficient if the user lacks permission to review the particular action." ("Technical
architecture and service ownership", "Interfaces and repository strategy").

| Public operation                                        | Internal operation                                                                                             | Name                                                                                                             | Provider                                                                                                          | Sync point |
| ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ---------- |
| `POST /api/runs`                                        | Admit the request; issue the passport, run and durable job, or reject with an explanation                      | `POST /internal/runs` (report)                                                                                   | Go side: Go implementer                                                                                           | X-28       |
| `POST /api/runs/{id}/cancel`                            | Persist cancellation before future dispatches                                                                  | `POST /internal/runs/:id/cancel` (architecture proposal)                                                         | Go side: Go implementer                                                                                           | X-42       |
| `POST /api/actions/{id}/approval`                       | Approve or reject the stored action                                                                            | `POST /internal/actions/:id/approval` (architecture proposal)                                                    | Go side: Go implementer                                                                                           | X-40       |
| `GET /api/runs/{id}`                                    | Run and usage view                                                                                             | A private Go endpoint named at M0, or a runtime view, per the read path                                          | Go side (private endpoint) or shared track (SH-16 views), per SH-05                                               | X-29       |
| `GET /api/runs/{id}/events`                             | Sanitized events by cursor                                                                                     | As above                                                                                                         | Go side (private endpoint) or shared track (SH-16 view), per SH-05                                                | X-30       |
| None in the report                                      | Exact review payload for authorized reviewers                                                                  | As above; open item `review payload read`                                                                        | Go side (private endpoint) or shared track (SH-27 view), per SH-05                                                | X-41       |
| None in the report                                      | Task form options (template, vendor, invoice set, allowed destination, approval requirement, available limits) | A private Go endpoint named at M0, a read grant on SH-18's records, or app records, per open item `form options` | Go side (endpoint), shared track (SH-18's records) or Next.js + NestJS side (app records), per `form options`     | X-25       |
| None in the report                                      | Stored report and registered template                                                                          | A private Go endpoint named at M0, or a view and read grant, per open item `stored report read`                  | Go side (private endpoint) or shared track (SH-24 view; the read grant is part of X-35), per `stored report read` | X-64       |
| `GET /api/runs/{id}/events` (optional, Tier C)          | Sanitized events as server-sent events, only after authenticated polling works                                 | A private Go endpoint, only if events stream from Go                                                             | Go side, if events stream from Go, per SH-05                                                                      | X-60       |
| None in the report; the document owner records it first | Start the labelled replay (X-36) of one stored prohibited proposal in a named run                              | Named at M0 by its owner, only if `replay entry` has the interface trigger it                                    | Go side: the replay owner recorded in SH-07                                                                       | X-65       |

Every internal command carries service identity and the verified operator context (X-26); Go
verifies both before it acts (X-27).

**Architecture specification.** It names `POST /internal/runs/:id/cancel` ("authorize and persist
cancellation") and `POST /internal/actions/:id/approval` ("approve or reject an exact action"); these
are the proposal adopted at the M0 freeze unless the Go implementer records otherwise. Both carry an
identifier in the path. For the reads it lists "Read authorized runtime view" and "NestJS SSE from
safe, ordered event records", which is one option of open item `read path`. It also specifies
"signed, short-lived operator context containing the user and organization", an input to decision 4,
not the decision.

### Reason vocabulary (proposed)

Report 1.1 proposes and does not fix: `resource_out_of_scope`, `destination_not_allowed`,
`report_export_restricted`, `report_lineage_missing`, `source_policy_changed`, `template_not_allowed`,
`approval_required`, `approval_expired`, `action_changed`, `resource_version_changed`,
`allowance_exhausted`, `run_cancelled` and `outcome_unknown`. "These names are proposed contract
vocabulary. Denial may identify an authorized alternative template without granting extra scope. The
UI, runtime, tests, and evidence should use the same vocabulary." The M0 freeze (SH-10) records the
vocabulary the team adopts and maps the architecture's illustrative event names (`report.created`,
`report.export_denied`, `report.safe_template_offered` and others) to it. A policy decision is allow,
deny or approval required; "A transport or configuration error is not an allow decision." Responses
"should include a stable reason code, a safe operator message, and relevant action or run references".

### Schema ownership

| Schema    | Main records (report 1.1)                                                                           | Table names (architecture proposal, adopted at the M0 freeze unless recorded otherwise)                                                              | Write authority                              | Read across                                                                                                                                |
| --------- | --------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `app`     | "Memberships, task versions, source and template policy versions, tool definitions and revocations" | users, organizations, memberships, task_templates, task_versions, policy_versions, tool_definitions, report_templates, projection_rules, revocations | NestJS                                       | Go admission reads task and policy versions; the gate reads current revocations; provenance reads trusted template and projection versions |
| `runtime` | "Passports, runs, context manifests, jobs, actions, approvals, usage and decision events"           | passports, runs, jobs, model_calls, actions, approvals, execution_attempts, report_lineage, budget_reservations, usage_entries, audit_events         | Go                                           | NestJS reads an authorized view if `read path` chooses views                                                                               |
| `demo`    | "Synthetic invoices, vendors, immutable reports with lineage, and simulated outbox"                 | versioned invoices and vendor records, classified reports, outbox_messages                                                                           | "Go tool adapters through restricted access" | provenance reads source versions and report metadata; NestJS none (`stored report read`)                                                   |

- Where lineage and context manifests live is `report storage` and `internal report rendering`; where
  source-field labels and recipient rules live is `source classification storage`.
- The `app` record list is the union of both sources (`app-schema record list`).
- Migrations are written by the integration role (the web + API implementer by default); "One migration
  set owns all schemas." Report content and its trusted lineage commit atomically.
- Table names are decided at the M0 freeze (SH-10), with the architecture's names as the proposal.
- Every query uses the verified organization context; "An invoice ID or action ID is a reference, not
  authorization."

### The read path (open, for the document owner)

**Report 1.1 and the architecture.** The architecture is consistent for views: "NestJS has read access
only to authorized runtime views needed for the interface"; "GET /api/runs/:id: Read authorized runtime
view"; "NestJS SSE from safe, ordered event records". The report still says "NestJS would expose these
operations and call corresponding private Go endpoints" and also "NestJS should receive only the
runtime views necessary for the interface". The item stays open.

The report is inconsistent about how NestJS serves the run, usage and event reads:

- Figure 1 has an edge from the activity API (FEED) to the runtime schema (RUNDB) labelled "Read
  authorized event view", and the operations table says "Read authorized run and usage view". Both
  suggest that NestJS reads sanitized runtime views.
- "Interfaces and repository strategy" says "NestJS would expose these operations and call
  corresponding private Go endpoints".

The document owner decides (SH-05); this roadmap does not. The sync points that depend on it (X-29,
X-30, X-41, X-60) name both possible providers. The side files plan for either outcome and mark the
affected tasks "Blocked by: `read path`". On the shared track, SH-16 (run, usage and event views)
and SH-27 (review payload view) deliver the view outcome. The NestJS read grant is part of X-35
(SH-26), because until SH-26 every service connects as the one `POSTGRES_USER` that also runs the
migrations. X-25 depends on open item `form options` in the same way: the side files plan for each
outcome its row names and mark the affected tasks "Blocked by: `form options`". On the shared track,
SH-18 delivers the read-grant outcome. X-64 depends on open item `stored report read` in the same
way: on the shared track, SH-24 delivers the view outcome, and the NestJS read grant is part of
X-35 (SH-26).

### Request id and error envelope (in the starter)

- **Request id.** Header `x-request-id` (constant `REQUEST_ID_HEADER` in `@workspace/contracts`).
  Each hop accepts an inbound id only if it matches `^[A-Za-z0-9._-]{1,64}$` and generates one
  otherwise. The API echoes it, logs it, puts it in every error envelope and sends it on its gateway
  calls; the gateway echoes it and logs it as `request_id`. Every new call between the services
  propagates it. "Request IDs help connect NestJS and Go diagnostics, but are not authorization
  tokens." ("Durable state idempotency audit and uncertain outcomes")
- **Error envelope.** Every failure that is not a health or diagnostics report uses `ErrorResponse`
  from `@workspace/contracts`, in the API, the gateway and the web proxy: `error.code`,
  `error.message`, `statusCode`, `requestId`, `timestamp` and an optional `path`. Codes are stable
  (`bad_request`, `unauthorized`, `forbidden`, `not_found`, `method_not_allowed`, `internal_error`,
  `not_implemented`, plus the proxy codes), and 5xx messages are fixed safe texts. The schema
  `packages/contracts/schemas/error.schema.json` allows no extra properties, so the report's reason
  code and action or run references (X-13) are a contract change through the procedure above.
  Details: `docs/architecture.md`, sections "Request id flow" and "Error envelope".

## Sync points

A sync point is one thing that crosses the boundary between the two sides, or between a side and the
shared track. The list is derived from the report's interfaces and the table "Proposed browser and
runtime operations", from every Diagram 1 edge that crosses the WEB, NEST, GO or POSTGRES subgraph
boundary, from the run lifecycle in Diagram 2 and from the 24-hour sequence. The side files use
exactly these IDs.

- The task that delivers a sync point lists it under "Provides"; every task that consumes it lists it
  under "Needs".
- A sync point with two providers is reached when both have delivered; each side file marks
  "Provides" on the task that delivers its half.
- A provider may deliver its share through several tasks of one side file. Each of them lists
  `X-nn (part)` under "Provides", with a short note of its part; that provider has delivered when
  all of them are done, and the side file lists each set in its "Coverage".
- A task that needs only one part of a sync point with several providers names the part under
  "Needs", as in `X-43 (approval part)`, and waits only for the tasks that deliver that part.
- "Needed by" is the milestone whose work or exit condition needs it. A provider that will be late
  says so at once. A mocked response "must be replaced or clearly labeled before any result is
  presented as a working capability." ("Delivery scope and six person ownership")
- An evidence sync point (X-37, X-38, X-44 to X-46, X-50 to X-57, X-63, X-67) means: the behaviour
  is implemented, its test runs, and the evidence the report lists for the check (quoted in the row)
  can be retrieved from the build. SH-31 does not wait for X-50 to X-57, nor SH-28 for X-67: a check
  or beat whose sync point is unreached when the task records it is recorded as not verified, with
  the reason, and stays visible ("Keep failed or unverified checks visible.").

| ID   | What becomes available                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Provider                                                                                                                                                                | Consumers                                                                                                  | Depends on                                                                                        | Needed by                                    |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| X-01 | The organizers' answer on reuse of pre-event work and how to disclose it, recorded in `docs/product/README.md`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Shared track: researcher (RS-01)                                                                                                                                        | Everyone who uses the starter or code written before the window                                            | `decision 8 in docs/product/README.md`                                                            | P (at the latest the start of M0)            |
| X-02 | The container path executed once (Compose, the three Dockerfiles, full-container mode), results in `README.md` "Verification status"                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: the lead, infrastructure (SH-09)                                                                                                                          | Shared track (SH-30); both sides (their images)                                                            | nothing                                                                                           | M4 (planned in P)                            |
| X-03 | The starter running with service connectivity on every implementer machine, with `pnpm verify`, `pnpm dev` and `pnpm smoke` results quoted                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Shared track: the lead with each person (SH-12)                                                                                                                         | Both sides                                                                                                 | X-01                                                                                              | M0                                           |
| X-04 | The model provider credential readable by Go only: name in `.env.example`, gateway Compose map only, kept out of the web and API children of the dev runner and out of the API process, which loads the root `.env` itself (SH-13), in the smoke leak list                                                                                                                                                                                                                                                                                                                                     | Shared track: the lead, infrastructure (SH-13)                                                                                                                          | Go side: Go implementer; shared track (SH-22)                                                              | `decision 6 in docs/product/README.md`                                                            | M0                                           |
| X-05 | The official requirements sheet (verified brief)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Shared track: researcher (RS-03)                                                                                                                                        | Both sides; shared track                                                                                   | X-01                                                                                              | M0                                           |
| X-06 | The frozen task definition and policy fixture content (tools, resources, destination, field rules, approval rule, limit values chosen by the team), recorded in `docs/product/README.md`                                                                                                                                                                                                                                                                                                                                                                                                       | Shared track: SH-10                                                                                                                                                     | Go side; Next.js + NestJS side; shared track (SH-18, SH-25); researcher (RS-04)                            | nothing                                                                                           | M0                                           |
| X-07 | The start-run request contract in `packages/contracts`: type, JSON Schema and fixture with the agreed example                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Shared track: SH-11, landed by the web + API implementer                                                                                                                | the web + API implementer; Go side: Go implementer                                                         | `contract owners`                                                                                 | M1                                           |
| X-08 | The passport representation contract                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-11, landed by the web + API implementer                                                                                                                | the web + API implementer; Go side: Go implementer                                                         | `contract owners`; `passport in the run view`                                                     | M1                                           |
| X-09 | The action proposal contract, with the four tools' typed arguments and the stored review content                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Shared track: SH-11, landed by the web + API implementer                                                                                                                | Go side: Go implementer; the web + API implementer                                                         | `contract owners`; `record versions`; `exact reviewed material`; `canonical arguments`            | M1                                           |
| X-10 | The approval decision contract: approve or reject one stored action identifier, with no replacement payload                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Shared track: SH-11, landed by the web + API implementer                                                                                                                | the web + API implementer; Go side: Go implementer                                                         | `contract owners`                                                                                 | M1                                           |
| X-11 | The run state contract: status, usage view and terminal reason for every outcome in Diagram 2                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Shared track: SH-11, landed by the web + API implementer                                                                                                                | the web + API implementer; Go side: Go implementer                                                         | `contract owners`; `final result format`                                                          | M1                                           |
| X-12 | The safe event contract: sanitized, ordered events with masked metadata, a cursor and a label for replayed proposals                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-11, landed by the web + API implementer                                                                                                                | the web + API implementer; Go side                                                                         | `contract owners`; `replay entry` (replay label)                                                  | M1                                           |
| X-13 | The reason vocabulary and the decision and error fields (stable reason code, safe operator message, action or run references) in the error envelope                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-11, landed by the web + API implementer                                                                                                                | Both sides; shared track (SH-23, SH-28)                                                                    | `contract owners`                                                                                 | M1                                           |
| X-14 | The authenticated operator context contract (repository addition)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Shared track: SH-39, landed by the web + API implementer                                                                                                                | the web + API implementer; Go side: internal API owner                                                     | `contract owners`; `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md` | M1                                           |
| X-15 | Go DTO mirrors of X-07 to X-14, with strict fixture-decode tests that list every new fixture                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Go side: owner recorded in SH-07                                                                                                                                        | the web + API implementer; shared track (SH-14, SH-22)                                                     | X-07 to X-14; `Go package owners`                                                                 | M1                                           |
| X-16 | The demo specification and storyboard: the nine beats with the observable proof each needs                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Shared track: researcher (RS-04)                                                                                                                                        | Both sides; shared track (SH-28, SH-33)                                                                    | X-05; X-06                                                                                        | M1                                           |
| X-17 | The app-schema entity classes with `schema: "app"` in the shared options factory, with the uuid settings that keep startup free of extension creation                                                                                                                                                                                                                                                                                                                                                                                                                                          | Next.js + NestJS side: web + API implementer                                                                                                                            | Shared track: the web + API implementer (SH-15)                                                            | `decision 7 in docs/product/README.md` (user records)                                             | M1                                           |
| X-18 | The `app` schema created and the app tables migrated                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-15                                                                                                                                                     | the web + API implementer; Go side: Go implementer                                                         | X-17                                                                                              | M1                                           |
| X-19 | The runtime tables for passports, runs, jobs, actions, decision events and usage records, with the constraints the Go owners agreed                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-16                                                                                                                                                     | Go side: Go implementer                                                                                    | nothing                                                                                           | M1                                           |
| X-20 | The demo tables for invoices and vendors, with organization references, each invoice's vendor reference and record versions                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Shared track: SH-17                                                                                                                                                     | Go side: Go implementer                                                                                    | `record versions`                                                                                 | M1                                           |
| X-21 | The seeded task template, policy version and tool definitions and the minimal synthetic records, loaded by an explicit seed command                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-18                                                                                                                                                     | Go side: Go implementer; the web + API implementer                                                         | X-06; `app-schema seed ownership`                                                                 | M1                                           |
| X-22 | The seeded demonstration operator, organization and membership, labelled as a development demonstration, with a real credential check                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Shared track: SH-19, with the web + API implementer                                                                                                                     | the web + API implementer; Go side                                                                         | `decision 7 in docs/product/README.md`; `app-schema seed ownership`                               | M1                                           |
| X-23 | The authentication and operator-context secrets wired, or a record that none is needed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Shared track: SH-20                                                                                                                                                     | the web + API implementer; Go side; shared track (SH-19)                                                   | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                    | M1                                           |
| X-24 | A documented database-backed test command                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Shared track: SH-21                                                                                                                                                     | Both sides; shared track (SH-16, SH-24, SH-26, SH-27, SH-31, SH-38)                                        | nothing                                                                                           | M1                                           |
| X-25 | The data for the task form's options (template, vendor, invoice set, allowed destination, approval requirement, available limits) readable by NestJS                                                                                                                                                                                                                                                                                                                                                                                                                                           | Decided with open item `form options`: shared track (SH-18's records; the NestJS read grant is part of X-35), Go side (endpoint) or Next.js + NestJS side (app records) | the web + API implementer                                                                                  | `form options`                                                                                    | M1                                           |
| X-26 | Every runtime command from NestJS carries the verified actor and organization context                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Next.js + NestJS side: web + API implementer                                                                                                                            | Shared track (SH-22)                                                                                       | X-14; `decision 4 in docs/product/README.md`                                                      | M1                                           |
| X-27 | Go verifies service identity and the operator context on every internal command and rejects the rest with the shared envelope                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Go implementer                                                                                                                                                 | the web + API implementer; shared track (SH-22)                                                            | X-14; `decision 4 in docs/product/README.md`; `Go package owners`                                 | M1                                           |
| X-28 | `POST /internal/runs`: passport, run and job stored in one transaction, or a rejection that names the scope or limit that must change and creates no passport                                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Go implementer                                                                                                                                                 | the web + API implementer; shared track (SH-22)                                                            | X-07; X-08; X-13; X-27; `command timeout budget`                                                  | M1                                           |
| X-29 | The run and usage view (status, usage and terminal reason; the passport too if `passport in the run view` says so) readable by NestJS                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Go side (private endpoint) or shared track (SH-16 views; the read grant is part of X-35), per the read-path decision (SH-05)                                            | the web + API implementer; shared track (SH-22)                                                            | `read path`; X-11; `passport in the run view`                                                     | M1                                           |
| X-30 | Sanitized events readable by cursor                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Go side (private endpoint) or shared track (SH-16 view; the read grant is part of X-35), per SH-05                                                                      | the web + API implementer; shared track (SH-22)                                                            | `read path`; X-12                                                                                 | M1                                           |
| X-31 | `POST /api/runs`, `GET /api/runs/{id}` and `GET /api/runs/{id}/events` reachable behind authentication through the chosen browser path                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Next.js + NestJS side: web + API implementer                                                                                                                            | Shared track (SH-22, SH-23, SH-28)                                                                         | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                    | M1                                           |
| X-32 | The gateway readiness check reports the worker (repository rule of decision 5), through a shared contract change                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Go side: Go implementer (contract through SH-14)                                                                                                                        | the web + API implementer (diagnostics); shared track (SH-23)                                              | `worker readiness`                                                                                | M1                                           |
| X-33 | The report table with an organization reference, and the simulated outbox with a uniqueness constraint tied to the action identifier                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-24                                                                                                                                                     | Go side: Go implementer                                                                                    | `record versions`                                                                                 | M2                                           |
| X-34 | The complete synthetic fixture set: seeded discrepancy, registered reporting address, out-of-scope invoice, hostile note, second organization, small configured allowance                                                                                                                                                                                                                                                                                                                                                                                                                      | Shared track: SH-25                                                                                                                                                     | Go side; Next.js + NestJS side; researcher                                                                 | X-06; `decision 7 in docs/product/README.md` (second organization's sign-in)                      | M2                                           |
| X-35 | The service database roles and grants, with the per-service credentials wired                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Shared track: SH-26                                                                                                                                                     | Both sides                                                                                                 | `decision 2 in docs/product/README.md`; `read path`                                               | M2                                           |
| X-36 | A labelled deterministic adversarial action replay through the real gate and execution path                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Go side: owner recorded in SH-07                                                                                                                                        | Shared track (SH-28, SH-33); researcher; the web + API implementer (replay label)                          | X-12; X-34; `Go package owners`; `replay entry`                                                   | M2                                           |
| X-37 | Resource boundary evidence: "Denial record plus unchanged excluded records and absence of an execution attempt."                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Go side: Go implementer                                                                                                                                                 | Shared track (SH-28, SH-36); researcher                                                                    | X-34; X-36                                                                                        | M2                                           |
| X-38 | Destination boundary evidence: "Stored proposal, rule decision, and outbox comparison."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Go side: Go implementer                                                                                                                                                 | Shared track (SH-28, SH-36); researcher                                                                    | X-33; X-34                                                                                        | M2                                           |
| X-39 | The approval, reservation and usage tables, with single-consumption constraints and identifiable dispatched attempts                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-27                                                                                                                                                     | Go side: Go implementer                                                                                    | `dispatched attempts`                                                                             | M3                                           |
| X-40 | The internal approval decision command (named at M0): Go checks reviewer authority, action integrity and expiry, and stores the grant and the continuation in one transaction                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Go implementer                                                                                                                                                 | the web + API implementer                                                                                  | X-10; X-27                                                                                        | M3                                           |
| X-41 | The exact review payload (recipient, rendered content, referenced report and version, reason for review), readable by authorized reviewers only                                                                                                                                                                                                                                                                                                                                                                                                                                                | Go side (private endpoint) or shared track (SH-27 view; the read grant is part of X-35), per SH-05                                                                      | the web + API implementer                                                                                  | `read path`; `review payload read`; X-09                                                          | M3                                           |
| X-42 | The internal cancel command (named at M0), which persists cancellation before future dispatches                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Go side: Go implementer                                                                                                                                                 | the web + API implementer                                                                                  | X-27                                                                                              | M3                                           |
| X-43 | `POST /api/actions/{id}/approval` (the approval part) and `POST /api/runs/{id}/cancel` (the cancel part) reachable behind authentication                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Next.js + NestJS side: web + API implementer                                                                                                                            | Shared track (SH-23, SH-28, SH-33)                                                                         | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                    | M3                                           |
| X-44 | Legitimate task evidence: "Report references, expected discrepancy, one outbox row, and completed run events."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Go side (report, discrepancy, outbox row, run events); Next.js + NestJS side (run start, approval, event display)                                                       | Shared track (SH-28); researcher                                                                           | X-34                                                                                              | M3                                           |
| X-45 | Approval integrity evidence: "Tampered proposal outcome and no matching business effect."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Go side: Go implementer                                                                                                                                                 | Shared track (SH-28); researcher                                                                           | X-09; `exact reviewed material`                                                                   | M3                                           |
| X-46 | Limit-triggered stop evidence (demo beat 9): "The next request is rejected before dispatch; a terminal reason is visible and the ledger does not record an unaccounted call." It is also the challenge table's Unpredictable costs evidence: "Run a bounded retry scenario and show execution stopping at the configured limit, with usage and uncertainty recorded."; the retry part follows `model call retries`                                                                                                                                                                             | Go side: Go implementer; Next.js + NestJS side: web + API implementer (terminal reason)                                                                                 | Shared track (SH-28); researcher                                                                           | X-34; `model call retries` (the retry part)                                                       | M3                                           |
| X-47 | Current revocation records written by NestJS in the `app` schema and read by Go before dispatch and before execution                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Next.js + NestJS side: web + API implementer (write path); shared track: SH-38 (the table and the Go read grant)                                                        | Go side: Go implementer                                                                                    | `revocation reads`; `decision 2 in docs/product/README.md`; X-66                                  | M4                                           |
| X-48 | A documented reset command                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Shared track: SH-29                                                                                                                                                     | Everyone                                                                                                   | X-34                                                                                              | M4                                           |
| X-49 | A documented deployment procedure for the demonstration environment                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-30                                                                                                                                                     | Everyone; researcher                                                                                       | X-01; X-02                                                                                        | M4                                           |
| X-50 | Field minimization evidence: "Inspected serialized tool result, model request fixture, and event payload."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Go side: Go implementer; Next.js + NestJS side: web + API implementer (safe activity views)                                                                             | Shared track (SH-31); researcher                                                                           | X-06; X-12; `Go package owners`                                                                   | M4                                           |
| X-51 | Approval replay evidence: "Concurrent request results, one consumed grant, and one outbox row."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Go side: Go implementer                                                                                                                                                 | Shared track (SH-31)                                                                                       | X-39                                                                                              | M4                                           |
| X-52 | Budget concurrency evidence: "Reservation and usage rows with reconciled totals and denied competing request."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Go side: Go implementer                                                                                                                                                 | Shared track (SH-31)                                                                                       | X-39                                                                                              | M4                                           |
| X-53 | Unknown usage evidence: "Retained reservation and visibly uncertain estimated-cost state."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Go side: Go implementer; Next.js + NestJS side: web + API implementer                                                                                                   | Shared track (SH-31)                                                                                       | `decision 6 in docs/product/README.md`                                                            | M4                                           |
| X-54 | Waiting-state restart evidence: "State before/after restart and exactly one resulting effect."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Go side: Go implementer                                                                                                                                                 | Shared track (SH-31)                                                                                       | `dispatched attempts`                                                                             | M4                                           |
| X-55 | Cancellation and expiry evidence: "Cancellation/expiry timestamp, subsequent denial, and dispatch records."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Go side: Go implementer; Next.js + NestJS side: web + API implementer (cancel command)                                                                                  | Shared track (SH-31)                                                                                       | X-42; X-43 (cancel part)                                                                          | M4                                           |
| X-56 | Organization access evidence: "Rejected read/command requests and absence of runtime mutation."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Next.js + NestJS side: web + API implementer (public path); Go side: internal API owner (internal boundary)                                                             | Shared track (SH-31)                                                                                       | X-34                                                                                              | M4                                           |
| X-57 | Database execution transaction evidence: "Fault-injection outcome and consistent action/business state."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Go side: Go implementer                                                                                                                                                 | Shared track (SH-31)                                                                                       | `decision 2 in docs/product/README.md`                                                            | M4                                           |
| X-58 | The interface ready for the rehearsal: all nine beats observable, with labels for the simulated outbox, replays and estimated cost                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Next.js + NestJS side: web + API implementer                                                                                                                            | Shared track (SH-33); researcher                                                                           | X-16                                                                                              | M5                                           |
| X-59 | The build identifier of the final build, with its recaptured evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Shared track: SH-32                                                                                                                                                     | Researcher (RS-08, RS-09); both sides                                                                      | nothing                                                                                           | M5                                           |
| X-60 | Optional, Tier C: server-sent events for the activity feed, only after authenticated polling works                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Next.js + NestJS side: web + API implementer; also the Go side if events stream from Go, per SH-05                                                                      | the web + API implementer                                                                                  | `read path`; `decision 3 in docs/product/README.md`                                               | None (optional)                              |
| X-61 | Optional, Tier C: an unknown-outcome failure rehearsal that pauses the action for operator attention instead of repeating it                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Go side                                                                                                                                                                 | Researcher                                                                                                 | X-34                                                                                              | None (optional)                              |
| X-62 | The technical handoff text for each side's area: setup, architecture and boundaries, tool contracts, known limitations                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Go side and Next.js + NestJS side                                                                                                                                       | Shared track (SH-35); researcher                                                                           | nothing                                                                                           | M6                                           |
| X-63 | Permitted reconciliation evidence (M2 exit, demo beat 3): "Returned fields obey policy; the report references authorized records and contains the expected discrepancy."                                                                                                                                                                                                                                                                                                                                                                                                                       | Go side: Go implementer                                                                                                                                                 | Shared track (SH-36); researcher                                                                           | X-33; X-34                                                                                        | M2                                           |
| X-64 | The stored report (stable identifier, version, source references, structured content) and its registered template, readable by NestJS, so the interface "renders its substantive content from the stored report and registered template" rather than from model prose                                                                                                                                                                                                                                                                                                                          | Decided with open item `stored report read`: Go side (private endpoint) or shared track (SH-24 view; the read grant is part of X-35)                                    | the web + API implementer                                                                                  | `stored report read`; `final result format`; X-33                                                 | M2                                           |
| X-65 | Conditional, only if `replay entry` has the interface trigger it: the internal operation (named at M0) that starts the labelled replay (X-36) of one stored prohibited proposal in a named run, called by a NestJS facade operation with the verified operator context; bounded and labelled, never "an unrestricted runtime command interface". The report's table "Proposed browser and runtime operations" has no such public operation, so the document owner records it first (see "Scope changes"); the side files add the tasks that provide and consume it once this outcome is chosen | Go side: the replay owner recorded in SH-07                                                                                                                             | the web + API implementer (the facade operation); the web + API implementer (the control that triggers it) | X-27; X-36; `Go package owners`; `replay entry`                                                   | M2 if this outcome is chosen; otherwise none |
| X-66 | The app-schema revocation entity class with `schema: "app"`, registered in the shared options factory, in the shape agreed under `revocation reads`                                                                                                                                                                                                                                                                                                                                                                                                                                            | Next.js + NestJS side: web + API implementer                                                                                                                            | Shared track: the web + API implementer (SH-38)                                                            | `revocation reads`                                                                                | M3                                           |
| X-67 | Productive continuation evidence (demo beat 6): "The original useful output is completed within the same run and allowance; correction attempts are counted."                                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Go implementer                                                                                                                                                 | Shared track (SH-28); researcher                                                                           | X-34; X-36                                                                                        | M3                                           |
| X-68 | The fixed report template and projection rule records in `app` (`internal_investigation_v1`, `vendor_reconciliation_v1`, the vendor projection rule), migrated and readable by Go                                                                                                                                                                                                                                                                                                                                                                                                              | Next.js + NestJS side: web + API implementer, API-28 (part: entities); shared track: SH-41 (part: migration and Go read)                                                | Go side (GO-32, GO-63, GO-65)                                                                              | `vendor projection fields` (rule content)                                                         | M2                                           |
| X-69 | The report lineage records, and the context manifest records if chosen, where `report storage` places them                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Shared track: SH-40                                                                                                                                                     | Go side (GO-63, GO-71); Next.js + NestJS side (API-30)                                                     | `report storage`; `internal report rendering` (context part)                                      | M2                                           |
| X-70 | The trusted source-field classifications and recipient rules, stored and readable by Go                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Per `source classification storage`: API-29 with SH-41 (`app`), or SH-17 and SH-25 (`demo`)                                                                             | Go side (GO-63)                                                                                            | `source classification storage`                                                                   | M2                                           |
| X-71 | The report and lineage summary contract in `packages/contracts`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Shared track: SH-11                                                                                                                                                     | Both sides                                                                                                 | `contract owners`                                                                                 | M2                                           |
| X-72 | Inherited restriction evidence: "Trusted source manifest and label; export denial rule; unchanged outbox count."                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Go side: GO-66                                                                                                                                                          | Shared track (SH-28, SH-36); researcher                                                                    | X-34; X-36                                                                                        | M2                                           |
| X-73 | Label and rename tampering evidence: "Stored provenance before and after attempt; denied export or rejected unsupported mutation."                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Go side: GO-68                                                                                                                                                          | Shared track (SH-31); researcher                                                                           | X-34; `rename operation` (rename part)                                                            | M4                                           |
| X-74 | Missing lineage evidence: "Rejected report proposal or export denial with no outbox row."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Go side: GO-68                                                                                                                                                          | Shared track (SH-31)                                                                                       | X-34                                                                                              | M4                                           |
| X-75 | Approved external projection evidence: "Serialized report content, selected fields, source versions, template and projection versions."                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Go side: GO-67                                                                                                                                                          | Shared track (SH-28, SH-36); researcher                                                                    | X-34; X-68                                                                                        | M2                                           |
| X-76 | Source or template policy changes evidence: "Old approval is rejected; no stale report is queued."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Go side: GO-70                                                                                                                                                          | Shared track (SH-31)                                                                                       | X-34; X-47                                                                                        | M4                                           |
| X-77 | Approval cannot override classification evidence: "Submitted approval or replayed grant; export remains denied with no outbox effect."                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Go side: GO-69                                                                                                                                                          | Shared track (SH-31); researcher                                                                           | X-34; X-40                                                                                        | M3                                           |

**Report 1.1 changes to existing sync points.** These amend the rows above; where a row and this table
disagree, this table wins. Every provider or consumer named "Implementer N" or "owner recorded in
SH-07" is the person in "Sides and people": GO-side work is the Go implementer's, API and WEB work the
web + API implementer's.

| ID                     | Change                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| X-06                   | Adds the two classifications, the two report templates, the approved vendor field projection (list per `vendor projection fields`), the field rules ("Internal note allowed for investigation; external report uses approved invoice projection"), allowed report templates and the source policy version.                                                                                                                                           |
| X-07, X-10             | Depends on adds `command idempotency keys` (the key field only).                                                                                                                                                                                                                                                                                                                                                                                     |
| X-08                   | Carries allowed report templates and the projection or source-policy fields; depends on adds `passport report fields`.                                                                                                                                                                                                                                                                                                                               |
| X-09                   | Carries the report-action fields and the `create_report` arguments; depends on no longer lists `exact reviewed material` (settled by report 1.1).                                                                                                                                                                                                                                                                                                    |
| X-12                   | Carries the report 1.1 event links (report ID, template version, classification, lineage-check outcome); the replay label now covers the export replay and the supporting rehearsals.                                                                                                                                                                                                                                                                |
| X-13                   | The 13 proposed reason codes; a denial may name an authorized alternative template.                                                                                                                                                                                                                                                                                                                                                                  |
| X-15                   | Also mirrors X-71; provider the Go implementer; depends on no longer lists `Go package owners`.                                                                                                                                                                                                                                                                                                                                                      |
| X-16, X-58             | The storyboard has nine beats, not eight.                                                                                                                                                                                                                                                                                                                                                                                                            |
| X-19                   | Table names: the architecture's proposal.                                                                                                                                                                                                                                                                                                                                                                                                            |
| X-20                   | Invoices are versioned; the internal investigation note is an authorized field, with its label if `source classification storage` chooses `demo`.                                                                                                                                                                                                                                                                                                    |
| X-21                   | The seed adds the two report templates and the projection rule.                                                                                                                                                                                                                                                                                                                                                                                      |
| X-25                   | The form options add internal evidence and report types.                                                                                                                                                                                                                                                                                                                                                                                             |
| X-27, X-36, X-50, X-65 | Provider the Go implementer; depends on no longer lists `Go package owners`. X-36's replayed proposals are the export of a genuinely created internal report and the out-of-scope read.                                                                                                                                                                                                                                                              |
| X-33                   | The report table holds immutable reports with classification, template and projection versions, content hash and destination class; depends on adds `report storage`.                                                                                                                                                                                                                                                                                |
| X-37, X-38             | Needed by M4 instead of M2; consumer SH-31 instead of SH-28 and SH-36.                                                                                                                                                                                                                                                                                                                                                                               |
| X-41                   | The review payload adds "the report identifier, content hash, source_manifest and its digest, classification, template and projection versions, exact recipient and exact outbound content".                                                                                                                                                                                                                                                         |
| X-45                   | Depends on no longer lists `exact reviewed material`.                                                                                                                                                                                                                                                                                                                                                                                                |
| X-46                   | Demo beat 9 instead of 8: "Scope denial creates no effect; exhausted allowance prevents another dispatch; neither uses broader authority."                                                                                                                                                                                                                                                                                                           |
| X-47                   | Revocations include source and template revocations.                                                                                                                                                                                                                                                                                                                                                                                                 |
| X-63                   | Demo beats 3 and 4: "Reads are authorized; ... the discrepancy is supported by A01 and A02" and the stored "Go-derived Internal only classification, source versions and immutable content hash"; "model context retains the internal restriction" comes from GO-71 (Tier B) and is recorded as not verified while missing.                                                                                                                          |
| X-64                   | Adds classification, template and projection versions, content hash, destination class and the lineage summary, with restricted source content only to authorized users; depends on adds `report storage`.                                                                                                                                                                                                                                           |
| X-67                   | Productive continuation evidence, demo beat 7: "Go renders a new report directly from approved invoice fields and records the projection rule; internal free text is absent." with "The blocked internal export continues to a newly rendered vendor report and one reviewed outbox effect; exhausted or unauthorized recovery stops." Providers GO-29 (part: continuation and stop) and GO-47 (part: reviewed outbox effect); Tier A; needed by M3. |

### Diagram 1 boundary crossings

Diagram 1 (section 1 of `docs/product/project-architecture.md`, the report's Figures 1-3) places UI in
WEB; API, CONFIG, FACADE and FEED in NEST; INTERNAL, ADMIT, RUNNER, MODEL, GATE, APPROVAL, EXEC,
ADAPTERS, FILTER and STATE in GO; APPDB, RUNDB and DEMODB in POSTGRES; USER and LLM outside. Every
edge that crosses a subgraph boundary:

| Edge (label)                                                   | Crosses          | Where it is tracked                                                                                                                                                                                                                               |
| -------------------------------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| USER -> UI                                                     | outside -> WEB   | The operator uses the interface; inside the Next.js + NestJS side                                                                                                                                                                                 |
| UI -> API ("Authenticated HTTPS")                              | WEB -> NEST      | Inside the Next.js + NestJS side (decisions 3 and 7); its results for the shared track are X-31 and X-43                                                                                                                                          |
| FEED -> UI ("SSE updates")                                     | NEST -> WEB      | Inside the Next.js + NestJS side, Tier C after authenticated polling; X-60 if events stream from Go                                                                                                                                               |
| FACADE -> INTERNAL ("Private authenticated API")               | NEST -> GO       | X-26, X-27, X-28, X-40, X-42; also X-29, X-30 and X-41 if the read path uses Go endpoints; X-25 and X-64 if `form options` and `stored report read` choose Go endpoints; X-60 if events stream from Go; X-65 if the interface triggers the replay |
| CONFIG -> APPDB ("Owns application writes")                    | NEST -> POSTGRES | X-17, X-18, X-47, X-66                                                                                                                                                                                                                            |
| FEED -> RUNDB ("Read authorized event view")                   | NEST -> POSTGRES | X-29, X-30, X-41 (read path open); X-35 for the grant                                                                                                                                                                                             |
| ADMIT -> APPDB ("Read authoritative task and policy versions") | GO -> POSTGRES   | X-18, X-21, X-35                                                                                                                                                                                                                                  |
| GATE -> APPDB ("Read current revocations")                     | GO -> POSTGRES   | X-47, X-35                                                                                                                                                                                                                                        |
| STATE -> RUNDB ("Owns runtime writes")                         | GO -> POSTGRES   | X-19, X-39, X-35                                                                                                                                                                                                                                  |
| ADAPTERS -> DEMODB ("Narrow database permissions")             | GO -> POSTGRES   | X-20, X-33, X-35                                                                                                                                                                                                                                  |
| MODEL -> LLM ("Governed model requests and responses")         | GO -> outside    | X-04; the provider is decision 6                                                                                                                                                                                                                  |

The edges INTERNAL -> ADMIT ("Start run"), INTERNAL -> APPROVAL ("Operator decision") and INTERNAL ->
STATE ("Cancel or revoke") stay inside GO; their callers are X-28, X-40 and X-42. The report: "The
diagram's internal cancel-or-revoke path should not imply that Go can directly edit every
NestJS-owned policy record." Revocation writes stay in NestJS (X-47).

### Diagram 2 run lifecycle crossings

Diagram 2 (section 2 of `docs/product/project-architecture.md`, the report's Figures 4-9). Every step
whose effect leaves one service:

| Lifecycle step                                                                         | Crosses                   | Sync points                                                 |
| -------------------------------------------------------------------------------------- | ------------------------- | ----------------------------------------------------------- |
| START -> AUTH: the operator creates a task; NestJS authenticates and checks membership | Next.js -> NestJS         | Inside the Next.js + NestJS side; X-31 for the shared track |
| AUTH -> ADMIT: NestJS forwards the request with verified operator context              | NestJS -> Go              | X-26, X-27, X-28                                            |
| VALID -> REJECT: "Reject task with explanation"                                        | Go -> NestJS -> Next.js   | X-28, X-13                                                  |
| ADMIT, QUEUE: Go reads task and policy versions; stores passport, run and job          | Go -> PostgreSQL          | X-18, X-19, X-21                                            |
| MODEL: Go sends minimized context to the approved model provider                       | Go -> provider            | X-04                                                        |
| USAGE, MFAIL: usage recorded; an uncertain reservation is retained                     | Go -> NestJS (read)       | X-29, X-53                                                  |
| WAIT -> HUMAN: awaiting approval persisted, interface notified, operator reviews       | Go -> NestJS -> Next.js   | X-29, X-30 (polled state and events), X-41 (review payload) |
| HUMAN -> APPROVED: "NestJS sends authenticated decision to Go"                         | Next.js -> NestJS -> Go   | X-40, X-43 (approval part)                                  |
| APPROVED -> STOP ("Cancel"), and cancellation at any step                              | Next.js -> NestJS -> Go   | X-42, X-43 (cancel part)                                    |
| RECHECK: current revocations and run cancellation                                      | Go -> PostgreSQL (`app`)  | X-47                                                        |
| EXEC, SUCCESS: registered tool effect with the stable action identifier                | Go -> PostgreSQL (`demo`) | X-20, X-33                                                  |
| UNKNOWN -> ATTENTION: "Operator attention required"                                    | Go -> NestJS -> Next.js   | X-29; X-61 (optional rehearsal)                             |
| STOP, FAILED, COMPLETE, REJECT: terminal states with their recorded reasons            | Go -> NestJS -> Next.js   | X-11, X-29; X-64 (the completed run's report)               |

The 24-hour sequence appears in "Milestones": its last column lists, per window, the sync points
that must be reached.

**Report 1.1.** The diagrams are now the Mermaid source in `docs/product/project-architecture.md`
(report Figures 1 to 9). Diagram 1 gains a GO component, report provenance, with two new crossings:
PROVENANCE to APPDB ("Read trusted template and projection versions", X-68, X-35) and PROVENANCE to
DEMODB ("Read source versions and report metadata", X-69, X-70, X-35). MODEL to LLM is bidirectional.
The internal cancel edge is now INTERNAL to STATE ("Authorize runtime cancellation"); revocation writes
stay in NestJS (X-47). Diagram 2 gains REPORTACTION to LINEAGE (trusted sources, template and
projection resolved; X-68, X-69, X-70), the denial with its export reason and offered vendor template
(X-13, X-30) and REPORTBUILD (report, lineage and completion stored atomically; X-33, X-69); the review
payload carries the report fields (X-41).

### Diagram 3 report information flow crossings

Source: section 5 of `docs/product/project-architecture.md` (report Figure 10). TASK (X-06, X-08);
SOURCES (X-20, X-70); the internal report shown to an authorized viewer (X-64); BLOCK and OFFER shown in
the interface (X-13, X-30); REVIEW (X-40, X-41, X-43); OUTBOX (X-33). RENAME has no operation in either
source (`rename operation`).

## Task overview

Every shared-track and researcher task in this file, one row each, in milestone order. 50 tasks: 37 Tier A, 12 Tier B, 1 Tier C. 10 name decision 7 (the authentication hold) under "Blocked by". The side tasks are listed in the overviews of `go.md` and `web-and-api.md`. Generated from the task blocks below on 2026-10-03. The task blocks are the source of truth: when you add, drop or rename a task, update its row in the same change.

### Shared track

| ID    | When | Tier | Owner                                                                                                                                             | Size         | Task                                                                            | Blocked by                                                                                                                                                                                                                                                                                                                                                         |
| ----- | ---- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| SH-01 | P    | A    | Web + API implementer (report role: Implementer 2, application API)                                                                               | S, 0.5-2 h   | Decide: authentication mechanism (decision 7)                                   | `decision 7 in docs/product/README.md`: the authentication design is on hold by the user's decision of 2026-10-03; this task waits until the hold is lifted                                                                                                                                                                                                        |
| SH-02 | P    | A    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API)                                                | S, 0.5-2 h   | Decide: browser to API path (decision 3)                                        | `decision 7 in docs/product/README.md` (what the browser carries depends on it)                                                                                                                                                                                                                                                                                    |
| SH-03 | P    | A    | Web + API implementer with the Go implementer                                                                                                     | S, 1-2 h     | Decide: operator context to Go (decision 4)                                     | `decision 7 in docs/product/README.md`                                                                                                                                                                                                                                                                                                                             |
| SH-04 | P    | A    | Go implementer with the lead (infrastructure)                                                                                                     | S, 0.5-2 h   | Decide: model provider, model and accounting rule (decision 6)                  | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-05 | P    | A    | Document owner (the lead until `researcher role` is settled) with both implementers                                                               | S, 0.5-2 h   | Decide: read path for the run, usage, event and review views                    | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-06 | P    | A    | the lead with the Go implementer                                                                                                                  | S, 0.5-1 h   | Decide: single Go executor connection (decision 2 detail)                       | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-07 | P    | A    | the lead with the web + API implementer                                                                                                           | S, 0.5-1 h   | Record the contract owners, the shared-track assignment and the researcher role | nothing (settles `contract owners`, `shared-track assignment`, `researcher role`)                                                                                                                                                                                                                                                                                  |
| SH-08 | P    | A    | each person runs it; the lead coordinates                                                                                                         | S, 0.5-1.5 h | Prepare every implementer machine                                               | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-09 | P    | B    | the lead (infrastructure)                                                                                                                         | S, 2-4 h     | Validate the container path                                                     | a fix it needs before the coding window waits on `decision 8 in docs/product/README.md`                                                                                                                                                                                                                                                                            |
| SH-10 | M0   | A    | the whole team; the web + API implementer coordinates the contracts; the document owner records the outcomes                                      | M, 3-10 h    | Freeze the task, contracts, tool arguments, policy fixture and schema ownership | `contract owners`; the names of the read operations also `read path`; `vendor projection fields` (the field list)                                                                                                                                                                                                                                                  |
| SH-11 | M0   | A    | Web + API implementer (report role: Implementer 2, application API)                                                                               | M, 2-7 h     | Land the frozen contracts in `packages/contracts`                               | `contract owners`                                                                                                                                                                                                                                                                                                                                                  |
| SH-12 | M0   | A    | the lead with both implementers                                                                                                                   | S, 2.5-5 h   | Bring up the starter and confirm service connectivity                           | `decision 8 in docs/product/README.md`                                                                                                                                                                                                                                                                                                                             |
| SH-13 | M0   | A    | the lead (infrastructure) with the Go implementer and, for the API's own `.env` loading, the web + API implementer                                | S, 1-2 h     | Wire the model provider credential for Go only                                  | `decision 6 in docs/product/README.md`                                                                                                                                                                                                                                                                                                                             |
| SH-14 | M0   | A    | each recorded contract owner with the web + API implementer, who also takes schema changes with the Go implementer's table definitions            | S, 1-4 h     | Run a quick shared review for every contract, schema and event-name change      | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-37 | M0   | A    | the lead (infrastructure) with the Go implementer                                                                                                 | S, 0.5-1.5 h | Wire the non-secret Go variables, if the Go owners add any                      | `decision 6 in docs/product/README.md` (the model name and pricing only)                                                                                                                                                                                                                                                                                           |
| SH-39 | M1   | A    | Web + API implementer with the Go implementer                                                                                                     | S, 0.5-1 h   | Freeze and land the authenticated operator context contract                     | `contract owners`; `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                                                                                                                                                                                                  |
| SH-15 | M1   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-3 h     | Create the `app` schema and the app-schema migration                            | `decision 7 in docs/product/README.md` for user records                                                                                                                                                                                                                                                                                                            |
| SH-16 | M1   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data); the Go implementer supplies the table definitions | S, 2-5 h     | Write the first runtime-schema migration                                        | `read path` (the views only)                                                                                                                                                                                                                                                                                                                                       |
| SH-17 | M1   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-2 h     | Write the first demo-schema migration                                           | `record versions`; `source classification storage` (label fields only)                                                                                                                                                                                                                                                                                             |
| SH-18 | M1   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-3 h     | Seed the policy fixture and the minimal synthetic records                       | `app-schema seed ownership`; `repository layout` (seed location only)                                                                                                                                                                                                                                                                                              |
| SH-19 | M1   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-2 h     | Seed the demonstration operator, organization and membership                    | `decision 7 in docs/product/README.md`; `app-schema seed ownership`                                                                                                                                                                                                                                                                                                |
| SH-20 | M1   | A    | the lead (infrastructure) with the web + API implementer and the Go implementer                                                                   | S, 1-2 h     | Wire the authentication and operator-context secrets                            | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                                                                                                                                                                                                                     |
| SH-21 | M1   | A    | not assigned by the default (`shared-track assignment`)                                                                                           | S, 1-3 h     | Provide a database-backed test command                                          | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-22 | M1   | A    | not assigned by the default (`shared-track assignment`), with both implementers                                                                   | M, 4-12 h    | Integrate the vertical path across the services                                 | `decision 7 in docs/product/README.md`; `decision 3 in docs/product/README.md`; `decision 4 in docs/product/README.md`; `read path`                                                                                                                                                                                                                                |
| SH-23 | M1   | A    | the lead (integration and infrastructure)                                                                                                         | S, 2-5 h     | Extend the smoke and leak checks                                                | `smoke under login`                                                                                                                                                                                                                                                                                                                                                |
| SH-24 | M2   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-3 h     | Write the demo-schema migration for reports and the simulated outbox            | `record versions`; `stored report read` (the report view only); `report storage`                                                                                                                                                                                                                                                                                   |
| SH-25 | M2   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 2-4 h     | Complete the synthetic fixture set                                              | `decision 7 in docs/product/README.md` (the second organization's sign-in); `source classification storage`                                                                                                                                                                                                                                                        |
| SH-26 | M2   | B    | the lead (database roles, by default) with both implementers                                                                                      | M, 2-9 h     | Create the service database roles and grants                                    | `decision 2 in docs/product/README.md`; `read path`; `form options` (the form-option read grant only); `stored report read` (the report read grant only); `decision 4 in docs/product/README.md` and `decision 7 in docs/product/README.md` (the Go read grant on the records behind operator and reviewer authority only); `report storage` (lineage grants only) |
| SH-36 | M2   | A    | the lead with the Go implementer                                                                                                                  | S, 1-3 h     | Observe the M2 exit across the services                                         | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-40 | M2   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data); the Go implementer supplies the definitions       | S, 1-3 h     | Write the migration for report lineage (and context manifests if chosen)        | `report storage`; `internal report rendering` (context part)                                                                                                                                                                                                                                                                                                       |
| SH-41 | M2   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-2 h     | Write the app-schema migration for templates and projection rules               | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-27 | M3   | A    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-3 h     | Write the runtime-schema migration for approvals, reservations and usage        | `dispatched attempts`; `read path` and `review payload read` (the review view only)                                                                                                                                                                                                                                                                                |
| SH-28 | M3   | A    | the lead with the document owner                                                                                                                  | M, 3-6 h     | Capture evidence for the vertical-slice checks                                  | `demonstration baseline`                                                                                                                                                                                                                                                                                                                                           |
| SH-38 | M3   | B    | Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data)                                                    | S, 1-2 h     | Write the app-schema migration for the revocation records                       | `revocation reads`; `decision 2 in docs/product/README.md` (the grant only)                                                                                                                                                                                                                                                                                        |
| SH-29 | M4   | B    | not assigned by the default (`shared-track assignment`)                                                                                           | S, 1-4 h     | Write and document the reset command                                            | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-30 | M4   | B    | the lead (infrastructure)                                                                                                                         | S, 2-6 h     | Write the deployment procedure for the demonstration environment                | `deployment network` (the network part)                                                                                                                                                                                                                                                                                                                            |
| SH-31 | M4   | B    | the lead with the document owner                                                                                                                  | M, 3-8 h     | Record outcomes for the remaining critical checks                               | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-32 | M5   | B    | the lead with the document owner                                                                                                                  | S, 1-3 h     | Recapture the evidence from the final build                                     | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-33 | M5   | B    | Researcher, document owner, and presenter (not assigned; `researcher role`), with both implementers and the lead                                  | M, 3-12 h    | Rehearse the storyboard end to end                                              | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-34 | M6   | B    | the whole team; coordination not assigned by the default (`shared-track assignment`)                                                              | S, 1-3 h     | Freeze features and fix critical faults                                         | nothing                                                                                                                                                                                                                                                                                                                                                            |
| SH-35 | M6   | B    | not assigned by the default (`shared-track assignment`), with both implementers                                                                   | S, 1-3 h     | Assemble the technical handoff                                                  | nothing                                                                                                                                                                                                                                                                                                                                                            |

### Researcher track

| ID    | When | Tier | Owner                                                                       | Size     | Task                                                                            | Blocked by |
| ----- | ---- | ---- | --------------------------------------------------------------------------- | -------- | ------------------------------------------------------------------------------- | ---------- |
| RS-01 | P    | A    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 1-3 h | Ask the organizers and sponsor mentors the report's questions, decision 8 first | nothing    |
| RS-02 | P    | C    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 0.5 h | Ask for cross-track rules only if another track is considered                   | nothing    |
| RS-03 | M0   | A    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 1-3 h | Write the official requirements sheet                                           | nothing    |
| RS-04 | M1   | A    | Researcher, document owner, and presenter (not assigned; `researcher role`) | M, 3-6 h | Write the demo specification and storyboard                                     | nothing    |
| RS-05 | M1   | A    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 1-3 h | Keep the claim-to-proof list                                                    | nothing    |
| RS-06 | M1   | B    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 1-2 h | Keep the source register                                                        | nothing    |
| RS-07 | M2   | B    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 2-4 h | Compare existing controls from primary documentation                            | nothing    |
| RS-08 | M5   | A    | Researcher, document owner, and presenter (not assigned; `researcher role`) | M, 3-6 h | Prepare the presentation from the final build                                   | nothing    |
| RS-09 | M6   | A    | Researcher, document owner, and presenter (not assigned; `researcher role`) | S, 2-5 h | Complete the submission checklist and submit                                    | nothing    |

## Shared track

Owners follow the default of `shared-track assignment`: the web + API implementer for migrations and
seeds, the lead for roles, Compose, deployment, smoke and evidence; tasks the default does not name
say so, plus whole-team sessions. Where
one exists, a size comes from the shared rows of the four estimates; the others are this roadmap's
estimates. Every size is a planning estimate, never a schedule.

### P (before the coding window)

- [ ] **SH-01 · Decide: authentication mechanism (decision 7)**
  - **Report 1.1 change:** Recorded in `docs/product/README.md` by the document owner.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 0.5-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `apps/api/src/auth`, `AGENTS.md`
  - Work: Options: (1) the recorded proposal, proposed, not decided: server-side sessions with an
    opaque random token stored as a SHA-256 hash in an HttpOnly SameSite=Lax cookie; Argon2id through
    `@node-rs/argon2`, with Node's built-in scrypt as fallback; a seeded demo operator created by an
    explicit seed command with passwords generated into `.env` and labelled as a development
    demonstration; roles operator, reviewer, policy administrator and audit reader, more than one per
    membership; one organization per user and two organizations seeded; password hashes and sessions
    in tables separate from memberships; routes without a `/v1` prefix; sessions of 7 days revoked on
    logout. (2) Another credential and session mechanism the owner proposes. Either option keeps
    AGENTS.md guardrail 2 (a real credential check; no allow-all guard, fabricated identity or fake
    login; `UnimplementedAuthProvider` answers 501 until a real provider replaces it); facts verified
    on 2026-10-03: Express 5 has no `req.cookies` without a parser, `argon2` 0.45.1 needs an
    `allowBuilds` entry under pnpm 11 while `@node-rs/argon2` 2.2.1 does not, and any new package is a
    lockfile change through integration. Owner in `docs/product/README.md`: nestjs (the web + API implementer).
  - Done when: the outcome is recorded as decision 7 in `docs/product/README.md` by the document owner
    (document owner).
  - Tests: none (a decision).
  - Report: "Technical architecture and service ownership"; "Architecture and chart reading guide"
    (Interpreting the full architecture); "Functional requirements MVP boundary and deferred scope"
    (Deferred features)
  - Blocked by: `decision 7 in docs/product/README.md`: the authentication design is on hold by the user's decision of 2026-10-03; this task waits until the hold is lifted

- [ ] **SH-02 · Decide: browser to API path (decision 3)**
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: A · Size: S (estimate 0.5-2 h)
  - Depends on: SH-01 · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `apps/web/src/server/upstream-proxy.ts`, `apps/api/src/app.setup.ts`
  - Work: Options: (1) a same-origin Next.js route handler that forwards an allowlist of top-level
    API prefixes (auth, runs, actions), the session cookie and a fixed header set, and streams the
    response: proposed, not decided; (2) direct browser calls to the API with credentialed CORS (the
    API allows GET, HEAD and OPTIONS without credentials today). Facts verified on 2026-10-03:
    Next.js rewrites resolve at build time and do not fit the image build; a forwarder must build
    upstream headers from an allowlist, reject encoded slashes, backslashes and dot segments, relay
    every `Set-Cookie` and send `Cache-Control: no-cache, no-transform` for event streams.
    Repository facts for option (1): today's proxy drops the caller's query string, answers 502 for
    an empty body (a 204 cannot pass) and bounds the whole exchange, body included, by 10 s
    (`apps/web/src/server/upstream-proxy.ts`). Owner in `docs/product/README.md`: frontend with
    nestjs.
  - Done when: the outcome is recorded as decision 3 in `docs/product/README.md` by the document owner.
  - Tests: none (a decision).
  - Report: "Architecture and chart reading guide" ("The browser must never receive provider
    credentials, tool credentials, internal service secrets, or an unrestricted runtime command
    interface"); "Relative implementation milestones and critical dependencies" (authenticated
    polling before SSE)
  - Blocked by: `decision 7 in docs/product/README.md` (what the browser carries depends on it)

- [ ] **SH-03 · Decide: operator context to Go (decision 4)**
  - **Report 1.1 change:** The architecture's shape, "signed, short-lived operator context containing the user and organization", is an input, proposed by the architecture specification, not decided; the conflict is decision 4's status.
  - Owner: Web + API implementer with the Go implementer · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: SH-01 · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/internal/httpserver/middleware.go`,
    `apps/api/src/gateway-client/gateway-client.service.ts`
  - Work: The requirement is settled: Go verifies service identity and the authenticated operator
    context, authorizes each command against its organization and run, and checks reviewer authority
    itself. Options from `docs/product/README.md`: (1) extend the starter's service token with an
    operator context Go can verify; (2) replace the service token with another service-identity
    mechanism that carries the operator context. No proposal is recorded; the outcome also names the
    trusted records Go reads for reviewer authority. Owner in `docs/product/README.md`: nestjs with go.
  - Done when: the outcome is recorded as decision 4 in `docs/product/README.md` by the document owner.
  - Tests: none (a decision).
  - Report: "Technical architecture and service ownership" (Interfaces and repository strategy);
    "Threat model limits and unresolved design choices" ("The service token in the starter requires
    replacement or extension for authenticated operator context")
  - Blocked by: `decision 7 in docs/product/README.md`

- [ ] **SH-04 · Decide: model provider, model and accounting rule (decision 6)**
  - Owner: Go implementer with the lead (infrastructure) · Tier: A · Size: S (estimate 0.5-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `.env.example`, `services/gateway/internal/config/config.go`
  - Work: Options: any one provider and model the owner proposes; no proposal is recorded. Settled
    constraints: one provider, called only by Go, which holds the credentials; one new variable in
    `.env.example` (name only), read by Go alone; a provider library beyond `net/http`, `slog` and pgx
    is a team decision and a `go.sum` change coordinated with integration. The outcome includes the
    documented accounting rule for estimated cost. Owner in `docs/product/README.md`: go (the Go implementer) with the lead (infrastructure).
  - Done when: the provider, the model and the accounting rule are recorded as decision 6 in
    `docs/product/README.md` by the document owner.
  - Tests: none (a decision).
  - Report: "Report purpose and design status" (one model provider); "Atomic allowances hard limits
    and estimated cost" ("The selected provider and model should have a documented accounting rule");
    "Relative implementation milestones and critical dependencies" (provider connection in hours 0-2)
  - Blocked by: nothing

- [ ] **SH-05 · Decide: read path for the run, usage, event and review views**
  - **Report 1.1 change:** Option 1 (views) now has the architecture's statements ("NestJS has read access only to authorized runtime views", "Read authorized runtime view", "NestJS SSE from safe, ordered event records"); option 2 keeps the report's "call corresponding private Go endpoints".
  - Owner: Document owner (the lead until `researcher role` is settled) with both implementers · Tier: A · Size: S (estimate 0.5-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: Options: (1) NestJS reads sanitized runtime views through a read-only grant, created by the
    migrations (Figure 1, "Read authorized event view"; operations table, "Read authorized run and
    usage view"); (2) NestJS calls private Go endpoints for the run, usage, event and review reads
    ("NestJS would expose these operations and call corresponding private Go endpoints"); the
    gateway's 30 s write timeout ends any long-lived stream from Go. No proposal is recorded, and this
    roadmap does not resolve it. Owner: the document owner (session record of 2026-10-03), with nestjs
    and go.
  - Done when: the outcome is recorded in `docs/product/README.md` by the document owner.
  - Tests: none (a decision).
  - Report: "Architecture and chart reading guide" (Figure 1 and Figure 2); "Technical architecture
    and service ownership"; "Data ownership and the transition from starter to product"; "Illustrative
    passport and interface contracts" (Proposed browser and runtime operations)
  - Blocked by: nothing

- [ ] **SH-06 · Decide: single Go executor connection (decision 2 detail)**
  - **Report 1.1 change:** Option 1 is now the architecture's design and the report's recommendation; the outcome records its adoption. Report content and its lineage join the transaction ("so an artifact cannot exist without its restrictions").
  - Owner: the lead with the Go implementer · Tier: A · Size: S (estimate 0.5-1 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/internal/database/database.go`
  - Work: Options: (1) the report's recommendation, not adopted: "one Go executor connection with
    explicit permissions for the necessary runtime writes and limited demo report and outbox
    operations", so the effect, its completion record and its event share one transaction; (2)
    separate connections with different credentials, which "would not provide that atomicity
    automatically". The direction of decision 2 (explicit, narrow privileges per service; NestJS
    keeps separate application privileges) is settled. Owner in `docs/product/README.md`: the lead
    (database roles, by default).
  - Done when: the outcome is recorded under decision 2 in `docs/product/README.md` by the document owner.
  - Tests: none (a decision).
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Data ownership and the
    transition from starter to product"
  - Blocked by: nothing

- [ ] **SH-07 · Record the contract owners, the shared-track assignment and the researcher role**
  - Owner: the lead with the web + API implementer · Tier: A · Size: S (estimate 0.5-1 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/README.md`
  - Work: Agree one owner for each of the eight contracts. Record that every Go module, existing
    package (`cmd/gateway`, `internal/config`, `internal/logging`, `internal/database`,
    `internal/health`, `internal/httpserver`) and conditional Go endpoint (X-25 and X-64 if
    `form options` and `stored report read` choose Go) belongs to the Go implementer. Confirm or
    change the shared-track default (migrations and seeds by the web + API implementer; database roles,
    Compose and deployment, smoke checks and evidence by the lead) and assign the duties it does not
    name. Record who holds the researcher, document owner and presenter role, or that the lead acts as
    document owner.
  - Done when: every contract row in `docs/product/README.md` has an owner, and `docs/product/README.md`
    records the Go modules' owner, the shared-track persons and the researcher role.
  - Tests: none (a record).
  - Report: "Delivery scope and six person ownership"; "Proposed team ownership"
  - Blocked by: nothing (settles `contract owners`, `shared-track assignment`, `researcher role`)

- [ ] **SH-08 · Prepare every implementer machine**
  - **Report 1.1 change:** "each implementer" is "each person"; the lead coordinates.
  - Owner: each person runs it; the lead coordinates · Tier: A · Size: S (estimate 0.5-1.5 h per person)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/team-workflow.md`, `README.md`, `.env.example`
  - Work: Each person installs Go 1.27 or newer and Docker with the Compose plugin, then runs
    `pnpm install`, `pnpm run setup` and `pnpm verify`, as "Before implementation starts" in
    `docs/team-workflow.md` describes. Nothing in the repository installs tools, and bare `pnpm setup`
    is a pnpm built-in that must not be used.
  - Done when: every implementer has quoted a `pnpm verify` result with six of six steps passing, or
    the failing step and the reason.
  - Tests: `pnpm verify` (instructions, format, lint, typecheck, test, build).
  - Report: "Relative implementation milestones and critical dependencies" ("Bring up the starter");
    "Delivery scope and six person ownership" (Starter work versus task specific implementation)
  - Blocked by: nothing

- [ ] **SH-09 · Validate the container path**
  - Owner: the lead (infrastructure) · Tier: B · Size: S (estimate 2-4 h)
  - Depends on: SH-08 · Needs: nothing · Provides: X-02
  - Paths: `infra/compose.yaml`, `infra/compose.debug.yaml`, `infra/docker/web.Dockerfile`,
    `infra/docker/api.Dockerfile`, `infra/docker/gateway.Dockerfile`, `scripts/compose.mjs`,
    `scripts/smoke.mjs`, `README.md`
  - Work: On a machine with Docker, run the container path that has never been executed and record
    the real results in `README.md`, section "Verification status". Container smoke skips every
    gateway check, so also run host development against the real `postgres:18-alpine` image.
  - Done when: `pnpm stack:up`, `pnpm smoke --mode=container` and `pnpm stack:down`, then
    `pnpm infra:up`, `pnpm dev` with `pnpm smoke` and `pnpm infra:down` have run, with their output
    quoted in "Verification status".
  - Tests: `pnpm smoke --mode=container`; `pnpm smoke` in host mode against the image's database.
  - Report: "Delivery scope and six person ownership" ("The prototype needs four deployed
    components"; deployment integration for Implementer 5)
  - Blocked by: a fix it needs before the coding window waits on `decision 8 in docs/product/README.md`

### M0 (hours 0-2)

- [ ] **SH-10 · Freeze the task, contracts, tool arguments, policy fixture and schema ownership**
  - **Report 1.1 change:** Work adds: the two classifications, the two templates and the approved vendor field projection; the report and lineage summary contract and the report freeze fields (classification, source manifest, template version, projection-rule version, export denial); the architecture's names as the proposal (Go packages, NestJS modules, routes, tables, internal endpoints, event examples); and the new open items `report storage`, `internal report rendering`, `source classification storage`, `vendor projection fields`, `passport report fields`, `command idempotency keys`, `repository layout` and `Go package layout`. `exact reviewed material` is settled and leaves the list.
  - Owner: the whole team; the web + API implementer coordinates the contracts; the document owner records the outcomes · Tier: A · Size: M (estimate 3-10 h, person-hours summed over attendees)
  - Depends on: SH-07 · Needs: nothing · Provides: X-06
  - Paths: `docs/product/README.md`, `services/gateway/README.md`, `packages/contracts`
  - Work: In one quick shared review, agree the single task definition, the policy fixture (allowed
    tools, resources, destination, field rules, approval rule and limit values the team chooses; the
    report's example limits are illustrative, not requirements), one example for each command and
    event, the four tools' typed arguments, the tool-result contract (Go-internal, planned in
    `docs/roadmap/go.md`), the reason vocabulary, how the frozen contracts are versioned (the report
    asks for it and names no mechanism), schema ownership with table names, who writes the
    app-schema seed (`app-schema seed ownership`, integration with nestjs), where the trusted
    recipient directory, the task's invoice scope and the registered report template are stored, and
    the internal endpoint names beyond `POST /internal/runs`. Bring the open items the contracts
    touch to the document owner in the same session: `final result format`, `record versions`,
    `exact reviewed material`, `passport in the run view`, `review payload read`, `form options`,
    `stored report read`, `command timeout budget`, `canonical arguments` and `replay entry`. The
    outcomes of SH-01 to SH-06 recorded by then are inputs. The operator context contract (X-14) is
    SH-39's, not part of this freeze.
  - Done when: the M0 exit's "agreed contract example for each command and event", the task
    definition, the policy fixture and how the frozen contracts are versioned are recorded in
    `docs/product/README.md` by the document owner.
  - Tests: none at the freeze; SH-11 runs the contract tests.
  - Report: "Relative implementation milestones and critical dependencies" (Proposed 24-hour
    implementation sequence, Hours 0-2; Critical path and sensible reductions); "Delivery scope and six
    person ownership"; "Illustrative passport and interface contracts"
  - Blocked by: `contract owners`; the names of the read operations also `read path`; `vendor projection fields` (the field list)

- [ ] **SH-11 · Land the frozen contracts in `packages/contracts`**
  - **Report 1.1 change:** Provides adds X-71. Work adds the report and lineage summary contract and the new fields of X-08, X-09, X-12 and X-13.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: M (estimate 2-7 h)
  - Depends on: SH-10 · Needs: X-01 · Provides: X-07, X-08, X-09, X-10, X-11, X-12, X-13, X-71
  - Paths: `packages/contracts/src/index.ts`, `packages/contracts/schemas`,
    `packages/contracts/fixtures`, `packages/contracts/test/fixtures.test.ts`,
    `packages/contracts/schemas/error.schema.json`
  - Work: Land each frozen contract as a type, a JSON Schema and fixtures with the agreed example,
    following "Changing a shared contract" in `docs/team-workflow.md`, and extend the error envelope
    with the reason fields. Apart from SH-39, which lands X-14, this is the only landing task:
    `docs/roadmap/web-and-api.md` adds none, and `docs/roadmap/go.md` provides the Go mirrors
    (X-15). It starts when SH-10 ends and, at its size, finishes in M1: SH-10 meets the M0 exit, and
    the M1 tasks on both sides that need X-07 to X-13 can start from the examples SH-10 records but
    are ticked only after it lands (see "IDs").
  - Done when: every frozen contract has a type, a schema, a fixture and a typed sample in the
    package and follows the versioning rule decided at the M0 freeze (SH-10), so the examples agreed
    in SH-10 exist in code.
  - Tests: the fixture test validates every fixture against its schema, rejects unknown fields and
    wrong enum values and pins one typed sample per schema: `pnpm --filter @workspace/contracts run test`;
    also `pnpm --filter @workspace/contracts run lint`, `typecheck` and `build`.
  - Report: "Delivery scope and six person ownership"; "Illustrative passport and interface contracts"
    (Decision and error semantics); "Risk register and scope controls" (NestJS/Go contract drift)
  - Blocked by: `contract owners`

- [ ] **SH-12 · Bring up the starter and confirm service connectivity**
  - Owner: the lead with both implementers · Tier: A · Size: S (estimate 2.5-5 h, one estimate's total for every machine and the demonstration
    environment)
  - Depends on: SH-08 · Needs: X-01 · Provides: X-03
  - Paths: `scripts/dev.mjs`, `scripts/smoke.mjs`, `README.md`
  - Work: At the start of the coding window, run the starter on every machine and, once the organizers
    name it, in the demonstration environment; quote the results.
  - Done when: the M0 exit's "working service connectivity" holds: `pnpm dev` runs web, API and gateway
    and `pnpm smoke` passes on each machine, with the results quoted.
  - Tests: `pnpm smoke` (the 19 host checks, including the leak check and the token rejection).
  - Report: "Relative implementation milestones and critical dependencies" (Hours 0-2)
  - Blocked by: `decision 8 in docs/product/README.md`

- [ ] **SH-13 · Wire the model provider credential for Go only**
  - Owner: the lead (infrastructure) with the Go implementer and, for the API's own `.env` loading, the web + API implementer · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: SH-04 · Needs: X-01 · Provides: X-04
  - Paths: `.env.example`, `scripts/setup.mjs`, `scripts/dev.mjs`, `infra/compose.yaml`,
    `scripts/smoke.mjs`, `README.md`, `services/gateway/README.md`,
    `apps/api/src/config/app-config.module.ts` (the web + API implementer), `scripts/with-env.mjs`,
    `docs/setup.md`
  - Work: In one change, add the variable the Go implementer names to `.env.example` (name only), the
    gateway's Compose environment map only and the README table; keep it out of the web child and
    the API child in `scripts/dev.mjs` (today the API child receives every `.env` variable); add it
    to the smoke leak list. Setup appends it empty and never prints it; how Go reads it is Go-side
    work. The runner filter alone does not keep it out of the API on the host:
    `apps/api/src/config/app-config.module.ts` loads the root `.env` into `process.env` for every
    key not already set (the container image has no `.env`). The web + API implementer and the lead
    record how the API stops holding it, for example by loading only the keys
    `environmentSchema` declares, or by keeping Go-only secrets out of the file the API reads, and
    update `docs/setup.md`, section 3. `scripts/with-env.mjs` (migrations, and any seed or reset
    command that uses it) passes the full environment: filter it, or record it in `docs/setup.md` as
    a documented exception.
  - Done when: the credential reaches no running service but the gateway, in host and container
    mode, and `pnpm smoke` finds it in no page or asset ("The browser receives neither provider
    credentials nor the development service token").
  - Tests: `pnpm run setup` (appends the name, prints no value); `pnpm dev` followed by `pnpm smoke`
    (leak check); the child environments checked by variable name, as during preparation; and a
    check inside the API process, for example an API spec that loads the config module with the
    credential in a fixture `.env` and absent from the environment, and asserts that `process.env`
    lacks the name (names only, never values). The preparation checks (`README.md`, "Verification
    status") cannot see the API's own load: the `pnpm` stand-in never starts NestJS, and the process
    list shows only the start-time environment.
  - Report: "Technical architecture and service ownership"; "The enforcement loop and data
    minimization" ("Model and tool credentials remain with Go")
  - Blocked by: `decision 6 in docs/product/README.md`

- [ ] **SH-14 · Run a quick shared review for every contract, schema and event-name change**
  - **Report 1.1 change:** Work adds the report lineage, classification and projection shapes and the event names (the architecture's examples are input).
  - Owner: each recorded contract owner with the web + API implementer, who also takes schema changes with the Go implementer's table definitions · Tier: A · Size: S (estimate 1-4 h, person-hours over the window)
  - Depends on: SH-11 · Needs: X-15 · Provides: nothing
  - Paths: `packages/contracts`, `services/gateway/internal/health/dto.go`,
    `apps/api/src/database/migrations`, `docs/team-workflow.md`
  - Work: From M0 to M6, agree every change to a shared shape, table or event name with the affected
    owners before anyone builds on it, and land it through "Changing a shared contract", so the type,
    schema, fixtures, Go DTO and consumers merge together or in direct succession. The worker readiness
    change (X-32) goes through this review. So does `revocation reads`: by M3, the web + API implementer and the Go
    implementer, with the lead for the grant, agree the app-schema revocation records and how Go reads them before dispatch and before
    execution (X-47).
  - Done when: at feature freeze no shared change is pending, and the TypeScript and Go fixture tests
    both decode every fixture ("validate serialized contracts").
  - Tests: `pnpm --filter @workspace/contracts run test`; `pnpm --filter gateway run test`;
    `pnpm verify`.
  - Report: "Relative implementation milestones and critical dependencies" ("Schema changes and event
    names therefore require a quick shared review rather than independent invention"); "Risk register
    and scope controls" (NestJS/Go contract drift)
  - Blocked by: nothing

- [ ] **SH-37 · Wire the non-secret Go variables, if the Go owners add any**
  - Owner: the lead (infrastructure) with the Go implementer · Tier: A · Size: S (estimate 0.5-1.5 h over M0 and M1, this roadmap's estimate)
  - Depends on: SH-04 · Needs: X-01 · Provides: nothing
  - Paths: `.env.example`, `infra/compose.yaml`, `README.md`, `services/gateway/README.md`
  - Work: If the Go owners make the model name, the pricing behind estimated cost or a worker
    setting an environment variable instead of a constant, add each in one change, as SH-13 does
    for the credential: `.env.example` (name and non-secret default), the gateway's Compose
    environment map only (`${NAME:-default}`, as `DATABASE_TIMEOUT_MS` is passed today) and the
    tables in `README.md` and `services/gateway/README.md`. No other file changes: `pnpm run setup`
    already appends a key missing from an existing `.env` with its default, the dev runner passes
    the full environment to the gateway, and a value that is not a secret needs no smoke leak
    entry. The passport's limits are not among these variables; they come from the policy fixture
    (X-06). If the Go owners use constants, record that and close the task.
  - Done when: each variable the Go owners added is in `.env.example` with its default and in both
    README tables, and reaches the gateway with its `.env` value on the host and in full-container
    mode ("The prototype needs four deployed components"); or the task records that none is needed.
  - Tests: `pnpm run setup` against an existing `.env` (adds each new key with its default); with a
    value the gateway's validation rejects, `pnpm dev` and, where Docker exists, `pnpm stack:up`
    each give a gateway validation log line naming the variable, which shows the value arrives in
    both modes and the Compose map does not drop it; a valid value then starts the gateway;
    `pnpm stack:down`; `pnpm exec prettier --check infra scripts`.
  - Report: "Delivery scope and six person ownership" (deployment integration for Implementer 5);
    "Atomic allowances hard limits and estimated cost" ("based on the configured model, estimated
    input and the permitted output ceiling"; "The selected provider and model should have a
    documented accounting rule")
  - Blocked by: `decision 6 in docs/product/README.md` (the model name and pricing only)

### M1 (hours 2-6)

- [ ] **SH-39 · Freeze and land the authenticated operator context contract**
  - Owner: Web + API implementer with the Go implementer · Tier: A · Size: S (estimate 0.5-1 h, split from SH-11's estimate)
  - Depends on: SH-01, SH-03, SH-07, SH-10 · Needs: X-01 · Provides: X-14
  - Paths: `docs/product/README.md`, `packages/contracts/src/index.ts`,
    `packages/contracts/schemas`, `packages/contracts/fixtures`,
    `packages/contracts/test/fixtures.test.ts`
  - Work: Once decisions 4 and 7 are recorded, agree the operator context contract with its recorded
    owner in a quick shared review and land it as SH-11 lands the others: a type, a JSON Schema and
    a fixture with the agreed example, following "Changing a shared contract" in
    `docs/team-workflow.md`. It is a repository addition, not one of the report's six contracts.
  - Done when: X-14 has a type, a schema, a fixture and a typed sample in the package and follows
    the versioning rule decided at the M0 freeze (SH-10) ("Shared contracts should be versioned and
    agreed before parallel implementation").
  - Tests: the fixture test validates the X-14 fixture against its schema, rejects unknown fields
    and pins its typed sample: `pnpm --filter @workspace/contracts run test`; also `lint`,
    `typecheck` and `build`.
  - Report: "Technical architecture and service ownership" (Interfaces and repository strategy);
    "Threat model limits and unresolved design choices" ("The service token in the starter requires
    replacement or extension for authenticated operator context")
  - Blocked by: `contract owners`; `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **SH-15 · Create the `app` schema and the app-schema migration**
  - **Report 1.1 change:** Work adds `organizations` with the user records (decision 7).
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: SH-10 · Needs: X-17 · Provides: X-18
  - Paths: `apps/api/src/database/migrations`, `apps/api/src/database/typeorm-options.ts`,
    `db/migrations/README.md`, `docs/team-workflow.md`, `AGENTS.md`, `CLAUDE.md`,
    `.claude/agents/nestjs.md`, `.claude/agents/integration.md`
  - Work: Generate the app-schema migration from the NestJS entities, add
    `CREATE SCHEMA IF NOT EXISTS app` by hand with the matching `DROP SCHEMA` in `down`, review the
    SQL and apply it. `synchronize`, `migrationsRun` and `dropSchema` stay false, and the uuid
    settings from X-17 keep startup free of extension creation. Whichever of SH-15, SH-16 and SH-17
    lands first also rewrites the "No migration exists yet" sentences in `AGENTS.md` ("Commands";
    edit it, then `cp AGENTS.md CLAUDE.md`), `.claude/agents/nestjs.md` and
    `.claude/agents/integration.md` to match the repository; integration owns these files.
  - Done when: the `app` schema and the app tables exist after `pnpm db:migration:run`, and starting
    the API on a fresh database creates no table and no extension (report: these settings "need to be
    preserved when the first product migrations are added").
  - Tests: `pnpm db:migration:run`, `pnpm db:migration:show`, `pnpm db:migration:revert`, then
    `pnpm db:migration:run` again; `pnpm dev:api` against a fresh database, then a table listing;
    `pnpm check:instructions` when the instruction files changed.
  - Report: "Data ownership and the transition from starter to product" (Proposed database ownership);
    "Design decision record" (TypeORM migration toolchain)
  - Blocked by: `decision 7 in docs/product/README.md` for user records

- [ ] **SH-16 · Write the first runtime-schema migration**
  - **Report 1.1 change:** Table names: the architecture's proposal (passports, runs, jobs, model_calls, actions and the others in "Schema ownership"); the Go implementer supplies the definitions.
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data); the Go implementer supplies the table definitions · Tier: A · Size: S (estimate 2-5 h)
  - Depends on: SH-10 · Needs: X-24 · Provides: X-19; X-29 and X-30 if the read path uses views
  - Paths: `apps/api/src/database/migrations`
  - Work: Hand-write (`pnpm db:migration:create <Name>`) `CREATE SCHEMA IF NOT EXISTS runtime`, with
    the matching `DROP SCHEMA` in `down`, and the runtime tables the vertical path needs
    (passports, runs, jobs with leases, actions, decision events and the usage records of the first
    model step), with the names from SH-10 and the constraints the Go owners agree in SH-14, among them
    a unique stable action identifier. If the read path uses views, add the sanitized run, usage and
    event views here.
  - Done when: the tables and constraints exist after `pnpm db:migration:run`, revert cleanly, and match
    the frozen names (Figure 4: "The passport, run, and job are stored together").
  - Tests: migration round trip (`pnpm db:migration:run`, `pnpm db:migration:revert`,
    `pnpm db:migration:run`); a database-backed test through X-24 shows a second action with the same
    identifier rejected.
  - Report: "Data ownership and the transition from starter to product"; "Durable state idempotency
    audit and uncertain outcomes"; "Architecture and chart reading guide" (Figure 4)
  - Blocked by: `read path` (the views only)

- [ ] **SH-17 · Write the first demo-schema migration**
  - **Report 1.1 change:** Work adds versioned invoices and the internal investigation note as an authorized field of a synthetic invoice, with its classification field if `source classification storage` chooses `demo`; in that case it provides X-70 (part).
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: SH-10 · Needs: X-01 · Provides: X-20
  - Paths: `apps/api/src/database/migrations`
  - Work: Hand-write `CREATE SCHEMA IF NOT EXISTS demo`, with the matching `DROP SCHEMA` in `down`,
    and the synthetic invoice and vendor tables with the organization reference, each invoice's
    vendor reference and the record version the rechecks compare, using the names from SH-10.
  - Done when: the tables exist after `pnpm db:migration:run`, carry an organization reference, a
    vendor reference on each invoice and a record version, and revert cleanly.
  - Tests: migration round trip (`pnpm db:migration:run`, `pnpm db:migration:revert`,
    `pnpm db:migration:run`).
  - Report: "Data ownership and the transition from starter to product"; "Threat model limits and
    unresolved design choices" ("Record-version rechecks require versions and comparison semantics")
  - Blocked by: `record versions`; `source classification storage` (label fields only)

- [ ] **SH-18 · Seed the policy fixture and the minimal synthetic records**
  - **Report 1.1 change:** Work adds the seed of the two report templates and the projection rule; the seed file location follows `repository layout` (the architecture proposes `db/seeds`, feasibility unverified).
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: SH-15, SH-17 · Needs: X-06 ·
    Provides: X-21; X-25 if `form options` chooses a read grant
  - Paths: `package.json`, `scripts/with-env.mjs`, `README.md`
  - Work: Write an explicit, documented seed command (a root script owned by integration, calling a
    file whose location integration agrees with infrastructure) that loads the frozen task template,
    policy version and tool definitions and the first synthetic vendor and invoices. Nothing runs it at
    startup, the interface labels the data as sample data, and the report's Atlas records are
    illustrative: the content follows X-06.
  - Done when: after the seed command Go admission can read the authoritative task and policy versions
    and `read_invoice` can read a permitted invoice; a service started on an empty database loads
    nothing.
  - Tests: run the seed command twice on an empty database and confirm the documented second-run
    behaviour; start the services without it and confirm no seed data appears.
  - Report: "Relative implementation milestones and critical dependencies" (policy fixture frozen in
    hours 0-2); "Illustrative passport and interface contracts" (Concrete synthetic business example);
    "Delivery scope and six person ownership"
  - Blocked by: `app-schema seed ownership`; `repository layout` (seed location only)

- [ ] **SH-19 · Seed the demonstration operator, organization and membership**
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: SH-01, SH-15 · Needs: X-23 · Provides: X-22
  - Paths: `package.json`, `apps/api/src/auth`
  - Work: Add the seeded operator, its organization and its membership through an explicit seed
    command, labelled as a development demonstration, with the real credential check decision 7
    defines; no fabricated identity and no fake login. Any secret the decision 7 mechanism needs
    follows AGENTS.md working rule 8 (untracked `.env` only, never logged or returned) and is wired
    through SH-20.
  - Done when: the seeded operator authenticates through the real credential check, a wrong credential
    is rejected, and nothing is seeded at startup ("One seeded operator can replace enterprise
    onboarding only within a clearly labeled development demonstration").
  - Tests: an API test against the seeded records: the correct credential is accepted, a wrong one is
    rejected, and no identity exists without the check (`pnpm --filter api run test`, or the X-24
    command if it needs the database).
  - Report: "Architecture and chart reading guide" (Interpreting the full architecture)
  - Blocked by: `decision 7 in docs/product/README.md`; `app-schema seed ownership`

- [ ] **SH-20 · Wire the authentication and operator-context secrets**
  - Owner: the lead (infrastructure) with the web + API implementer and the Go implementer · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: SH-01, SH-03 · Needs: X-01 · Provides: X-23
  - Paths: `.env.example`, `scripts/setup.mjs`, `scripts/dev.mjs`, `infra/compose.yaml`,
    `scripts/smoke.mjs`, `README.md`
  - Work: If decisions 4 and 7 need any secret, add each in one change: `.env.example` (name only),
    generation in `scripts/setup.mjs` for random local secrets, the Compose map of each reading service
    only, the filter in `scripts/dev.mjs` for each child that does not read it (today only the web
    child is filtered; a secret the API must not hold also needs SH-13's approach, because the API
    loads the root `.env` itself), the smoke leak list and the README table. If they need none,
    record that and close the task.
  - Done when: each new secret reaches only the services that read it and `pnpm smoke` finds none in
    pages or assets, or the task records that none is needed.
  - Tests: `pnpm run setup`; `pnpm dev` followed by `pnpm smoke`; for each secret a service must not
    hold, the SH-13 checks (the child environment, and the check inside the API process when the API
    must not hold it).
  - Report: "Technical architecture and service ownership" ("The browser receives neither provider
    credentials nor the development service token")
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **SH-21 · Provide a database-backed test command**
  - **Report 1.1 change:** Owner not assigned by the default (`shared-track assignment`).
  - Owner: not assigned by the default (`shared-track assignment`) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: SH-08 · Needs: X-01 · Provides: X-24
  - Paths: `turbo.json`, `package.json`, `scripts/verify.mjs`, `scripts/with-env.mjs`, `README.md`
  - Work: Give both sides one documented command that runs their database-backed tests against a real
    PostgreSQL with `.env` loaded. Turborepo's strict environment mode keeps `POSTGRES_*` from
    `pnpm test`, and `pnpm verify` needs no database by design; integration chooses the mechanism and
    keeps `pnpm verify` free of a database unless the team decides otherwise.
  - Done when: a Go test and an API test that need PostgreSQL run through the documented command, and
    the command fails when the database is down.
  - Tests: the command itself, with the database up and down.
  - Report: "Validation plan and evidence matrix" (Interpreting results honestly); "Atomic allowances
    hard limits and estimated cost" ("PostgreSQL transactions and row locks can protect reservation
    updates and execution claims")
  - Blocked by: nothing

- [ ] **SH-22 · Integrate the vertical path across the services**
  - **Report 1.1 change:** Owner not assigned by the default; the run includes the internal report, the denied export and the vendor report once M2 delivers them.
  - Owner: not assigned by the default (`shared-track assignment`), with both implementers · Tier: A · Size: M (estimate 4-12 h)
  - Depends on: SH-12, SH-16, SH-18, SH-19 · Needs: X-04, X-15, X-26, X-27, X-28, X-29, X-30, X-31 ·
    Provides: nothing
  - Paths: `scripts/smoke.mjs`, `docs/architecture.md`, `README.md`
  - Work: Run "operator to Next.js to NestJS to Go to a registered adapter" with the real services, fix
    contract drift with the owners through SH-14, and record each landed module in "Product modules" in
    `docs/architecture.md`.
  - Done when: the M1 exit is observed: "A real operator starts a run; Go executes a permitted tool; the
    interface displays the actual persisted result."
  - Tests: the end-to-end path run by hand with the results quoted; `pnpm smoke` with the checks
    SH-23 adds at M1.
  - Report: "Relative implementation milestones and critical dependencies" (Hours 2-6; "The early
    checkpoint is a complete authenticated path from the interface to a live model, through one
    governed tool, back to a persisted effect and visible event")
  - Blocked by: `decision 7 in docs/product/README.md`; `decision 3 in docs/product/README.md`; `decision 4 in docs/product/README.md`; `read path`

- [ ] **SH-23 · Extend the smoke and leak checks**
  - Owner: the lead (integration and infrastructure) · Tier: A · Size: S (estimate 2-5 h over M1 to M3)
  - Depends on: SH-13, SH-20 · Needs: X-13, X-31, X-32, X-43 (approval part) · Provides: nothing
  - Paths: `scripts/smoke.mjs`, `scripts/lib/http-probe.mjs`, `README.md`
  - Work: Add checks for the new routes and pages, for internal commands rejected without valid
    service identity or operator context, and for every new secret in the leak list; scan API
    responses and logs as well as web assets. Health and diagnostics stay public unless the team
    decides otherwise, and how smoke reaches pages behind a login follows decision 7. Extend again
    at M3 for approval, and for cancel once the cancel part of X-43 is delivered. The gateway
    readiness check is the only check that consumes X-32; the leak and service-identity checks do
    not wait on it.
  - Done when: `pnpm smoke` covers every public operation and every secret in use and passes against the
    running stack ("a hidden URL is not a protection").
  - Tests: `pnpm smoke`; `pnpm smoke --mode=container` where Docker exists.
  - Report: "Validation plan and evidence matrix" (Interpreting results honestly); "Risk register and
    scope controls" (Data leakage through secondary views)
  - Blocked by: `smoke under login`

### M2 (hours 6-10)

- [ ] **SH-24 · Write the demo-schema migration for reports and the simulated outbox**
  - **Report 1.1 change:** The report table holds immutable reports with classification, template and projection identifiers and versions, content hash, allowed destination class and server-rendered content; lineage per `report storage` (SH-40); the templates are `app` records (X-68) unless SH-10 records otherwise; outbox name proposal `outbox_messages`.
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: SH-17 · Needs: X-24 · Provides: X-33; X-64 if `stored report read` chooses a view
  - Paths: `apps/api/src/database/migrations`
  - Work: Hand-write the report table (stable identifier, version, organization reference, run,
    source references, template, structured content) and the simulated outbox "with a uniqueness
    constraint tied to the action ID", so a repeated approved action cannot create a second row. If
    SH-10 stores the registered report template in `demo`, add its table here. If
    `stored report read` chooses a view, add the read-only view of the stored report (stable
    identifier, version, source references, structured content) and its registered template here,
    from wherever SH-10 stores the template.
  - Done when: a second outbox row for the same action identifier is rejected by the database, and the
    migration reverts cleanly (acceptance evidence: "A repeated request does not create a second outbox
    message").
  - Tests: a database-backed test through X-24 that inserts two outbox rows for one action identifier;
    migration round trip.
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Illustrative passport and
    interface contracts" (Proposed tool argument boundaries)
  - Blocked by: `record versions`; `stored report read` (the report view only); `report storage`

- [ ] **SH-25 · Complete the synthetic fixture set**
  - **Report 1.1 change:** Work: invoice_A01 and invoice_A02 for Atlas sharing INV104; the authorized internal note on invoice_A01, labelled Internal only by trusted fixture data, with its instruction; invoice_B01 outside the task; Atlas's registered demonstration address; the source fields of the approved projection; the second organization; the small allowance for beat 9. The fixtures support nine demo beats, not eight. Report field: Scenes 1 to 4 and "Supporting rehearsals". Provides X-70 (part) if labels live in `demo`.
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: SH-18, SH-24 · Needs: X-06 · Provides: X-34
  - Paths: `package.json`, `README.md`
  - Work: Extend the seed command with permitted invoices that carry a seeded discrepancy, the vendor's
    registered demonstration reporting address as the only allowed recipient, an invoice outside the
    task, a synthetic invoice note with the hostile instruction, a second organization's records for the
    organization-access check, and the small configured allowance beat 9 uses, all within the limits
    and content frozen in X-06. Unless SH-10 records the registered report template as Go code,
    also seed it where SH-10 stores it, with fields within the X-06 field rules. All data is
    synthetic and labelled.
  - Done when: one documented seed command yields fixtures that support the nine demo beats and the
    critical checks, and the document owner confirms the content matches X-06.
  - Tests: run the seed on an empty database and query the expected records (discrepancy, out-of-scope
    invoice, hostile note, second organization and, if it is a stored record, the registered
    report template).
  - Report: "Illustrative invoice scenario and future domain adaptations" (Scene 2); "Live
    demonstration storyboard and proof checks" (Proposed demo sequence); "Validation plan and evidence
    matrix" (critical check Organization access)
  - Blocked by: `decision 7 in docs/product/README.md` (the second organization's sign-in); `source classification storage`

- [ ] **SH-26 · Create the service database roles and grants**
  - **Report 1.1 change:** Work adds the Go read grant on report templates, projection rules and source labels in `app` and the lineage grants per `report storage`; NestJS read on the classification and lineage summary if `stored report read` chooses a view.
  - Owner: the lead (database roles, by default) with both implementers · Tier: B · Size: M (estimate 2-9 h)
  - Depends on: SH-05, SH-06, SH-16, SH-24 · Needs: X-24 · Provides: X-35
  - Paths: `infra/compose.yaml`, `.env.example`, `scripts/setup.mjs`, `scripts/dev.mjs`,
    `scripts/smoke.mjs`, `infra/README.md`, `README.md`
  - Work: Give each service only the privileges the report assigns: NestJS writes `app`; Go writes
    `runtime` and reads the `app` records that admission and reviewer checks need (the records
    behind operator and reviewer authority follow SH-03's outcome and decision 7's record shape; the
    revocation records' read grant is SH-38's, once their table exists); the adapters reach `demo`
    through narrow permissions, on one executor connection if SH-06 adopts it; NestJS gets runtime
    read access only if the read path uses views, read access to the form-option records only if
    `form options` chooses a read grant, and read access to the stored report view only if
    `stored report read` chooses a view. Role passwords stay in the untracked `.env`, never in a
    migration. Wire each role password in one change, as SH-13 and SH-20 do: `.env.example` (name
    only), generation in `scripts/setup.mjs` (`GENERATED_SECRETS`, never printed), the Compose map
    of its reading service only, kept out of the web child in `scripts/dev.mjs` (a `POSTGRES_` name
    prefix, which the filter already strips, or an extended filter), the smoke leak list in
    `scripts/smoke.mjs` and the README table. The readers of the new credentials change in their
    owners' code. On the host, `scripts/dev.mjs` passes every role password to the API and gateway
    children, and the API also loads the root `.env` itself, so each service holds the other roles'
    passwords: the owners either extend SH-13's approach to them or record that host mode does not
    enforce the credential boundary.
  - Done when: each service connects with its own role and a write outside its authority fails
    ("Credentials and database roles must enforce the intended boundary"), and `pnpm smoke` finds
    no role password in pages or assets.
  - Tests: database-backed tests through X-24: the NestJS role cannot write runtime records, the Go role
    cannot write `app` records, and the executor connection commits an effect, its execution record
    and its event together; `pnpm run setup` (generates each role password, prints none); `pnpm dev`
    with the web child's environment checked by variable name, followed by `pnpm smoke` (the leak
    list includes each role password). No experiment has covered a non-superuser role yet.
  - Report: "Architecture and chart reading guide"; "Technical architecture and service ownership"
    (Proposed ownership: "Explicit service privileges and transactions; no assumed tenant isolation");
    "Data ownership and the transition from starter to product"
  - Blocked by: `decision 2 in docs/product/README.md`; `read path`; `form options` (the form-option read grant only); `stored report read` (the report read grant only); `decision 4 in docs/product/README.md` and `decision 7 in docs/product/README.md` (the Go read grant on the records behind operator and reviewer authority only); `report storage` (lineage grants only)

- [ ] **SH-36 · Observe the M2 exit across the services**
  - **Report 1.1 change:** Work: run the internal investigation, the denied export to Atlas and the vendor report creation; compare the outbox count (zero for the denied artifact) and the stored provenance. Done when: the M2 exit of report 1.1, "An internal report is retained for authorized internal viewing; its vendor export is denied; a separate approved-field vendor report can be created." Report: beats 3 to 5 and 7.
  - Owner: the lead with the Go implementer · Tier: A · Size: S (estimate 1-3 h, this roadmap's estimate)
  - Depends on: SH-25 · Needs: X-63, X-64, X-72, X-75 · Provides: nothing
  - Paths: none (an observation; the results are quoted in the change that ticks the box)
  - Work: With the real services and the seeded fixtures, run a permitted reconciliation and an
    explicit prohibited proposal, and compare the business state before and after the denied
    proposal (record versions, report count, outbox count) with the documented read-only queries
    SH-28 uses until `demonstration baseline` is settled. Label a replayed proposal as a replay.
  - Done when: the M2 exit is observed: "A permitted reconciliation succeeds; an explicit prohibited
    proposal produces no business effect."
  - Tests: the reconciliation and the prohibited proposal run by hand, with the results and the
    before and after state quoted.
  - Report: "Relative implementation milestones and critical dependencies" (Hours 6-10; "Progress
    should be measured by integrated behavior rather than completed screens or isolated modules");
    "Live demonstration storyboard and proof checks" (beat 3); "Risk register and scope controls"
    (Demo proves logs, not prevention)
  - Blocked by: nothing

- [ ] **SH-40 · Write the migration for report lineage (and context manifests if chosen)**
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data); the Go implementer supplies the definitions · Tier: A · Size: S (estimate 1-3 h, this roadmap's estimate)
  - Depends on: SH-16, SH-24 · Needs: X-24 · Provides: X-69
  - Paths: `apps/api/src/database/migrations`
  - Work: Hand-write the lineage tables where `report storage` places them (the architecture proposes
    `report_lineage` in `runtime`), and the context manifest records if `internal report rendering`
    admits model prose. Report content and lineage commit in one transaction ("so an artifact cannot
    exist without its restrictions").
  - Done when: a report row cannot be committed without its lineage rows, and the migration reverts
    cleanly.
  - Tests: a database-backed test through X-24; migration round trip.
  - Report: "MVP" (Inherited report restrictions); "Durable state idempotency audit and uncertain
    outcomes"
  - Blocked by: `report storage`; `internal report rendering` (context part)

- [ ] **SH-41 · Write the app-schema migration for templates and projection rules**
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-2 h, this roadmap's estimate)
  - Depends on: SH-15 · Needs: X-24, X-68 (part: entities) · Provides: X-68 (part: migration); X-70
    (part, if `source classification storage` chooses `app`)
  - Paths: `apps/api/src/database/migrations`
  - Work: Generate the migration from API-28 (and API-29 if chosen); NestJS writes these records and
    Go reads them. The Go read grant is SH-26's (X-35, Tier B); until SH-26 every service connects as
    the one `POSTGRES_USER`, so this task does not wait for it. It depends on SH-15 as SH-38 does, so
    it is not ticked during the hold of decision 7.
  - Done when: the two fixed templates and the projection rule are migrated and readable by Go, and
    the migration reverts cleanly.
  - Tests: migration round trip; a query from the Go side's connection.
  - Report: "MVP" (Trusted template manifests); "Proposed team ownership" (Implementer 2: "Own
    versioned source-policy and projection configuration")
  - Blocked by: nothing

### M3 (hours 10-14)

- [ ] **SH-27 · Write the runtime-schema migration for approvals, reservations and usage**
  - **Report 1.1 change:** The review view adds the report identifier, content hash, source manifest and digest, classification, template and projection versions, exact recipient and exact outbound content; table names: the architecture's proposal.
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: SH-16 · Needs: X-24 · Provides: X-39; X-41 if the read path uses views
  - Paths: `apps/api/src/database/migrations`
  - Work: Hand-write the approval, reservation and usage tables with the constraints the Go owners
    agree in SH-14: a grant bound to one stored action with its expiry and consumed once, reservation
    rows that short row-locked transactions can settle, and the identification of dispatched attempts
    that worker recovery needs. If the read path uses views, add the review payload view (recipient,
    rendered content, referenced report and version, reason for review) here.
  - Done when: a second consumption of one approval and a second claim of one attempt fail at the
    database, and the migration reverts cleanly ("An atomic execution claim consumes the approval
    once").
  - Tests: database-backed tests through X-24 for the single-consumption constraints; migration round
    trip.
  - Report: "Exact action approval versioning and execution rechecks"; "Atomic allowances hard limits
    and estimated cost"; "Threat model limits and unresolved design choices" ("Durable worker recovery
    requires identifiable dispatched attempts")
  - Blocked by: `dispatched attempts`; `read path` and `review payload read` (the review view only)

- [ ] **SH-28 · Capture evidence for the vertical-slice checks**
  - **Report 1.1 change:** Work: the vertical-slice checks are Legitimate task, Inherited restriction, Approved external projection and Approval integrity, plus the limit-triggered stop (beat 9) and the safe continuation (X-67); Resource boundary and Destination boundary move to SH-31. X-67 is now Tier A work, so this task waits for it. Done when: each of the four. Report beats 2, 4, 5, 6, 7, 8 and 9.
  - Owner: the lead with the document owner · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: RS-04, SH-21, SH-25 · Needs: X-13, X-16, X-31, X-36, X-43 (approval part), X-44, X-45, X-46, X-67, X-72, X-75 · Provides: nothing
  - Paths: `docs/product`
  - Work: For Legitimate task, Resource boundary, Destination boundary, Approval integrity, the
    limit-triggered stop and beat 6's continuation in the same run (X-67), record status, build
    identifier, fixture, observed outcome, timestamp and evidence location, and "Compare business
    state and execution records before and after every denied action" (record versions, report
    count, outbox count). Until the document owner names another surface, capture the baseline with
    documented read-only database queries; label every replay as a replay. X-67 comes from Tier B
    work, so this task does not wait for it (see "Sync points").
  - Done when: each of the five has a recorded outcome with its before and after state and the
    evidence "Proposed critical checks" lists, beat 6's continuation has a recorded outcome with the
    proof X-67 quotes, or is recorded as not verified with the reason while X-67 is not reached, and
    the M3 exit is observed: "The reviewed action executes once; changed content and depleted
    allowance cannot dispatch an operation."
  - Tests: the checks themselves, run against the build whose identifier is recorded.
  - Report: "Validation plan and evidence matrix" (critical checks Legitimate task, Resource boundary,
    Destination boundary, Approval integrity); "Live demonstration storyboard and proof checks" (beats 2,
    5, 6, 7 and 8); "Risk register and scope controls" (Demo proves logs, not prevention); "Relative
    implementation milestones and critical dependencies" (Hours 10-14)
  - Blocked by: `demonstration baseline`

- [ ] **SH-38 · Write the app-schema migration for the revocation records**
  - **Report 1.1 change:** The revocation records include source and template revocations.
  - Owner: Web + API implementer by default (`shared-track assignment`; report role: Implementer 5, data) · Tier: B · Size: S (estimate 1-2 h, this roadmap's estimate)
  - Depends on: SH-15, SH-26 · Needs: X-24, X-66 ·
    Provides: X-47 (part: the table and the Go read grant)
  - Paths: `apps/api/src/database/migrations`
  - Work: Generate the migration from the NestJS revocation entity (X-66) with
    `pnpm db:migration:generate <Name>`, review the SQL against the entity and apply it. The `app`
    schema exists since SH-15, so the migration only adds the revocation records. Give the Go role
    read access to them the way SH-26 grants the other `app` records; NestJS keeps the write.
    `synchronize`, `migrationsRun` and `dropSchema` stay false.
  - Done when: the records exist after `pnpm db:migration:run` and revert cleanly, the Go role can
    read them and cannot write them, and starting the API on a fresh database creates no table
    (report table "Proposed database ownership": `app` holds "revocations", write authority NestJS).
  - Tests: migration round trip (`pnpm db:migration:run`, `pnpm db:migration:revert`,
    `pnpm db:migration:run`); a database-backed test through X-24 in which the Go role reads a
    revocation record and its write is refused.
  - Report: "Data ownership and the transition from starter to product" (Proposed database
    ownership); "Exact action approval versioning and execution rechecks" (Versioned policy and
    current revocation)
  - Blocked by: `revocation reads`; `decision 2 in docs/product/README.md` (the grant only)

### M4 (hours 14-18)

- [ ] **SH-29 · Write and document the reset command**
  - **Report 1.1 change:** Owner not assigned by the default. The comparison adds the stored classifications and lineage.
  - Owner: not assigned by the default (`shared-track assignment`) · Tier: B · Size: S (estimate 1-4 h)
  - Depends on: SH-25 · Needs: nothing · Provides: X-48
  - Paths: `package.json`, `README.md`
  - Work: Add an explicit, documented command that restores the synthetic fixtures and the
    demonstration state so the workflow can be repeated. It never runs at startup and never removes the
    database volume.
  - Done when: the M4 exit's "the team can reset fixtures and repeat the workflow" holds: after a full
    run, a reset and a second run, the report count, outbox count and record versions match the first
    run.
  - Tests: run the workflow, reset, run it again, and compare the counts and versions.
  - Report: "Relative implementation milestones and critical dependencies" (Hours 14-18); "Research
    documentation and submission workflow" (the handoff includes the "synthetic-data reset")
  - Blocked by: nothing

- [ ] **SH-30 · Write the deployment procedure for the demonstration environment**
  - **Report 1.1 change:** Work cites the architecture's "NestJS and Go use a private service network. Only the browser-facing entry points are exposed."
  - Owner: the lead (infrastructure) · Tier: B · Size: S (estimate 2-6 h)
  - Depends on: SH-09 · Needs: X-01, X-02 · Provides: X-49
  - Paths: `infra/compose.yaml`, `infra/README.md`, `README.md`, `infra/docker/api.Dockerfile`
  - Work: Document how the four components start, migrate, seed and reset in the environment the
    organizers confirm. The API image cannot run the documented `db:migration:*` path: its runtime
    stage has no pnpm, and the production deploy lacks `apps/api/scripts/typeorm-cli.mjs`, `src` and
    ts-node. TypeORM's own CLI ships with its production dependencies, but running it in the image
    against `dist/database/data-source.js` is unverified. Migrations therefore run from the host, as
    do the root seed and reset commands, unless the team verifies an in-image path; the worker's
    shutdown must fit the gateway's 8 s budget.
  - Done when: a teammate who did not write it brings the stack up from the procedure, runs the
    workflow and resets it ("finish reset and deployment procedures").
  - Tests: `pnpm stack:up`, `pnpm smoke --mode=container` and `pnpm stack:down`, or the host-mode
    commands if the demonstration runs on the host.
  - Report: "Delivery scope and six person ownership" ("The prototype needs four deployed
    components"); "Relative implementation milestones and critical dependencies" (Hours 14-18)
  - Blocked by: `deployment network` (the network part)

- [ ] **SH-31 · Record outcomes for the remaining critical checks**
  - **Report 1.1 change:** Work: 14 checks (Resource boundary, Destination boundary, Label and rename tampering, Missing lineage, Source or template policy changes, Approval cannot override classification and the eight of report 1.0). Done when: each of the eighteen checks has an outcome.
  - Owner: the lead with the document owner · Tier: B · Size: M (estimate 3-8 h)
  - Depends on: SH-28 · Needs: X-24, X-50, X-51, X-52, X-53, X-54, X-55, X-56, X-57, X-37, X-38, X-73, X-74, X-76, X-77 · Provides: nothing
  - Paths: `docs/product`
  - Work: Record the same evidence fields for Field minimization, Approval replay, Budget concurrency,
    Unknown usage, Waiting-state restart, Cancellation and expiry, Organization access and Database
    execution transaction. "Keep failed or unverified checks visible."
  - Done when: the M4 exit's "Critical checks have recorded outcomes" holds: each of the twelve checks is
    recorded as passed or failed, with its evidence, or as not verified, with the reason.
  - Tests: the checks themselves; the concurrency, restart and transaction checks through X-24.
  - Report: "Validation plan and evidence matrix" (Proposed critical checks); "Sources and evidence
    register" (Evidence to add after implementation)
  - Blocked by: nothing

### M5 (hours 18-21)

- [ ] **SH-32 · Recapture the evidence from the final build**
  - Owner: the lead with the document owner · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: SH-28, SH-31 · Needs: nothing · Provides: X-59
  - Paths: `docs/product`
  - Work: Record the build identifier of the build used for the demonstration, recapture every check's
    evidence and every screenshot from it, and recapture again after any material fix.
  - Done when: every evidence entry names the build identifier of the demonstrated build ("capture
    evidence from the final build").
  - Tests: the checks, rerun against that build.
  - Report: "Relative implementation milestones and critical dependencies" (Hours 18-21); "Risk
    register and scope controls" (Final build diverges from presentation)
  - Blocked by: nothing

- [ ] **SH-33 · Rehearse the storyboard end to end**
  - **Report 1.1 change:** Nine beats.
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`), with both implementers and the lead · Tier: B · Size: M (estimate 3-12 h, person-hours)
  - Depends on: RS-04, SH-29, SH-30 · Needs: X-16, X-36, X-43, X-48, X-49, X-58 · Provides: nothing
  - Paths: `docs/product`
  - Work: Run the nine beats on the final build in the demonstration environment, reset between runs,
    and label the replay, the simulated outbox and estimated cost. "Record a successful rehearsal from
    the final implementation and retain its evidence as a fallback if permitted", walk through the
    provider-failure fallback from RS-04, and adapt the timing to the confirmed presentation format.
  - Done when: the M5 exit is observed by someone who did not build the features: "A reviewer can
    understand the task boundary, attempted action, decision, actual effect, and limitation without
    narration filling gaps."
  - Tests: the rehearsal itself.
  - Report: "Live demonstration storyboard and proof checks"; "Relative implementation milestones and
    critical dependencies" (Hours 18-21)
  - Blocked by: nothing

### M6 (hours 21-24)

- [ ] **SH-34 · Freeze features and fix critical faults**
  - **Report 1.1 change:** Coordination not assigned by the default.
  - Owner: the whole team; coordination not assigned by the default (`shared-track assignment`) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: SH-33 · Needs: nothing · Provides: nothing
  - Paths: `README.md`, `docs/product`
  - Work: Stop feature work. Fix each failing critical check or narrow the supported behaviour and the
    claims, and recapture the evidence after each material fix (SH-32).
  - Done when: no feature lands after the freeze, and every known critical fault is fixed or recorded as
    a limitation ("If a critical check fails, either fix it or narrow the supported behavior and the
    claims").
  - Tests: `pnpm verify`, `pnpm smoke`, and the critical checks a fix touched.
  - Report: "Relative implementation milestones and critical dependencies" (Hours 21-24); "Risk
    register and scope controls"
  - Blocked by: nothing

- [ ] **SH-35 · Assemble the technical handoff**
  - **Report 1.1 change:** Owner not assigned by the default.
  - Owner: not assigned by the default (`shared-track assignment`), with both implementers · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: SH-34 · Needs: X-62 · Provides: nothing
  - Paths: `README.md`, `docs/architecture.md`, `docs/setup.md`, `docs/product`
  - Work: Assemble "setup instructions, architecture and boundaries, synthetic-data reset, tool
    contracts, policy fixture, known limitations, dependency disclosures where required, and the
    critical-check outcomes" from the sides' text, with "Product modules" in `docs/architecture.md`
    current.
  - Done when: the handoff contains every item the report lists and matches the submitted build.
  - Tests: a teammate follows the setup instructions on a clean checkout.
  - Report: "Research documentation and submission workflow" (From requirements to verified
    presentation)
  - Blocked by: nothing

## Researcher track

No person holds the researcher role yet (`researcher role`); until the team assigns it, the lead acts
as document owner and these tasks wait. The researcher writes no product code. The four estimates
exclude the researcher's work, so these
sizes are this roadmap's estimates.

### P (before the coding window)

- [ ] **RS-01 · Ask the organizers and sponsor mentors the report's questions, decision 8 first**
  - **Report 1.1 change:** No person holds the role yet: until `researcher role` is settled, X-01 waits unless the lead takes RS-01.
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: nothing · Needs: nothing · Provides: X-01
  - Paths: `docs/product/README.md`, `docs/preparation-record.md`
  - Work: Ask the questions in "Questions to resolve with organizers and sponsor mentors": what
    preparation, starter code, libraries, datasets and generated assets may be reused and how to
    disclose them (decision 8); which submission artifacts, language, presentation format,
    demonstration environment and deadline apply; whether a synthetic finance workflow with a
    simulated outbox is acceptable; which agent risks or developer-integration expectations to
    prioritize; and what evidence would make cost controls and exact-action approvals convincing,
    and whether any sponsor-specific evaluation requirements are available. Also ask for the
    detailed judging criteria, which "have not been established by this report" ("Sources and
    evidence register", S1). Record every answer and the remaining uncertainty.
  - Done when: decision 8's answer is recorded in `docs/product/README.md`, `docs/preparation-record.md`
    matches the required disclosure, and the remaining uncertainty is documented "before reuse or
    submission".
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Delivery scope and six person ownership";
    "Risk register and scope controls" (Rules remain unresolved)
  - Blocked by: nothing

- [ ] **RS-02 · Ask for cross-track rules only if another track is considered**
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: C · Size: S (estimate 0.5 h)
  - Depends on: RS-01 · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: Only if the team considers another track, ask "the exact rules for cross-track submissions,
    reuse, and eligibility". If no other track is considered, this task becomes "Dropped: no other
    track is considered".
  - Done when: the answer is recorded.
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Illustrative invoice scenario and future
    domain adaptations" ("The team should not assume that a single project can be submitted across
    categories")
  - Blocked by: nothing

### M0 (hours 0-2)

- [ ] **RS-03 · Write the official requirements sheet**
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: RS-01 · Needs: nothing · Provides: X-05
  - Paths: `docs/product` (new file, named by the researcher)
  - Work: Write "a concise official requirements sheet" from the official rules and sponsor
    statements, keeping the team's design choices apart from sponsor mandates; later mentor feedback
    becomes a short requirements update. "The team should reconcile the report with the final
    official task description released for the event": record each difference in the sheet and
    settle it through "Scope changes" in time for the SH-10 freeze.
  - Done when: the verified brief, the first half of the researcher's first integrated deliverable,
    is in `docs/product` and the team has read it at M0 ("Confirm rules and sponsor expectations"),
    and it lists each difference between the report and the final official task description with its
    outcome, or states that there is none.
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Mapping the proposal to the Goldman Sachs
    challenge" ("This mapping is a proposed product interpretation, not a Goldman Sachs feature
    specification or judging rubric")
  - Blocked by: nothing

### M1 (hours 2-6)

- [ ] **RS-04 · Write the demo specification and storyboard**
  - **Report 1.1 change:** Nine beats with their proofs; where the export replay is used; "Explain source inheritance and the distinction between a permitted read and a permitted export."
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: RS-03, SH-10 · Needs: X-06 · Provides: X-16
  - Paths: `docs/product` (new file, named by the researcher)
  - Work: Specify the nine beats of "Proposed demo sequence" with the observable proof each needs, the
    screen or record that shows it, where the labelled replay is used, and the labels for the simulated
    outbox, replays and estimated cost. Add the provider-failure fallback: "On provider failure,
    show the actual failure state and distinguish the working gateway demonstration from unavailable
    live model behavior."
  - Done when: the researcher's first integrated deliverable exists: "A verified brief and a storyboard
    with observable evidence for every claimed protection."
  - Tests: none (no code).
  - Report: "Live demonstration storyboard and proof checks"; "Delivery scope and six person ownership"
  - Blocked by: nothing

- [ ] **RS-05 · Keep the claim-to-proof list**
  - **Report 1.1 change:** Requote "the integrated tools, data rules, projections and limits to which it applies" and "A recipient allowlist does not prove that arbitrary free text is safe."; add the report 1.1 limits, including "not universal taint tracking across every possible tool" and "it does not reproduce Fides or establish its formal guarantees".
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: A · Size: S (estimate 1-3 h over the
    window)
  - Depends on: RS-04 · Needs: nothing · Provides: nothing
  - Paths: `docs/product` (new file, named by the researcher)
  - Work: "Maintain a claim-to-proof list: each presentation claim names a demo step, observed
    record, or completed check." Every claim about the product states "the integrated tools, data
    rules and limits to which it applies". Check every claim against "Claims the prototype should
    avoid" and against the report's other claim limits, among them: a recipient allowlist "would not
    prove that arbitrary free-text content is safe"; no "universal detection of personally
    identifiable information or encoded disclosure"; the cost controls are "not universal prevention
    of every possible charge"; cancellation "cannot reverse a committed effect or guarantee that a
    provider stops billing an in-flight request"; "logical module separation inside the trusted Go
    process does not claim containment of a compromised process"; "The simulated outbox does not
    demonstrate remote delivery, provider-side idempotency or recovery from a real financial
    transaction"; the audit stream "should not be called tamper-proof, independently verified or
    legally sufficient unless additional controls are designed and tested"; the unresolved
    implementation choices are "not completed protections"; "Production readiness is a future
    validation effort"; the data controls "do not establish that arbitrary text or information
    already known to a provider is universally safe"; InjecAgent's "historical model-specific attack
    rates are not used as estimates"; the duplicate-reference report "does not determine fraud or
    authorize payment"; the prototype is "not a complete enterprise security platform"; and the
    Qualification column of the table "How the proposed product would be distinguished". Remove
    unsupported language at feature freeze.
  - Done when: at M6 every claim in the presentation points to recorded evidence from the final
    build and names the tools, data rules and limits it applies to, and no presentation or
    submission text crosses a limit listed in Work.
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Mapping the proposal to the Goldman
    Sachs challenge" (Claims the prototype should avoid); "Functional requirements MVP boundary and
    deferred scope"; "The enforcement loop and data minimization"; "Atomic allowances hard limits
    and estimated cost"; "Durable state idempotency audit and uncertain outcomes"; "Threat model
    limits and unresolved design choices"; "Positioning differentiation and credible product
    claims"; "Illustrative passport and interface contracts"; "Phased roadmap beyond the prototype";
    "Sources and evidence register"
  - Blocked by: nothing

- [ ] **RS-06 · Keep the source register**
  - **Report 1.1 change:** Sources S1 to S8; decide whether the hostile-note rehearsal still cites S4; the internal design sources per `design source references`.
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product` (new file, named by the researcher)
  - Work: Keep S1 to S7 and the internal design sources, each backing only the narrow factual
    statement it supports, and check the official challenge page at the event.
  - Done when: every external statement in the presentation and the handoff cites a register entry.
  - Tests: none (no code).
  - Report: "Sources and evidence register"; "Research documentation and submission workflow"
  - Blocked by: nothing

### M2 (hours 6-10)

- [ ] **RS-07 · Compare existing controls from primary documentation**
  - **Report 1.1 change:** Work quote: "Compare relevant authorization and information-flow approaches using primary documentation, including the Fides research [S8]." The claim limit is "Task Passport should not be presented as the invention of tool authorization, approvals, audit logging, or information-flow control."
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: B · Size: S (estimate 2-4 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product` (new file, named by the researcher)
  - Work: "Compare a small number of relevant authorization and agent-control products using primary
    documentation. Record supported features, unavailable information, and overlap." Position Task
    Passport around bounded delegation and useful continuation, subject to what is delivered, with no
    first-of-its-kind claim and no invented market statistics.
  - Done when: the comparison exists and the positioning in the presentation matches it.
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Positioning differentiation and credible
    product claims"
  - Blocked by: nothing

### M5 (hours 18-21)

- [ ] **RS-08 · Prepare the presentation from the final build**
  - **Report 1.1 change:** Adds the positioning statement "Task Passport controls what an agent can do and where the information it uses can go.", the persuasive story and the changed challenge rows.
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: RS-04, SH-32 · Needs: X-59 · Provides: nothing
  - Paths: `docs/product`
  - Work: Explain "the delegated job, why the attempted action was unsafe, how enforcement prevented its
    effect, and how legitimate work continued", replace placeholders with final-build captures, and
    state the limitations and the trust assumptions. Present the recorded check outcomes (SH-28,
    SH-31) by the report's rule: "Report a small synthetic evaluation with its fixture count and
    limitations; do not extrapolate to universal prompt-injection prevention, production
    reliability, or financial savings." For each concern the official brief names (the report lists
    sensitive data exposure, unauthorized actions, unpredictable costs, and preserving innovation
    and developer productivity), as the verified brief confirms them, show its proposed control and
    the evidence that demonstrates it, following the report's table "Challenge concern, proposed
    control and evidence", as the team's interpretation, not a sponsor mandate or judging rubric.
    Use the terms of the report's working glossary.
  - Done when: every claim in the presentation maps to a claim-to-proof entry backed by evidence from the
    build identified in X-59, the evaluation states its fixture count and limitations, and each
    concern the verified brief names has its control and such an entry.
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Positioning differentiation and credible
    product claims"; "Threat model limits and unresolved design choices"; "Validation plan and
    evidence matrix" (Interpreting results honestly); "Mapping the proposal to the Goldman Sachs
    challenge" (Challenge concern, proposed control and evidence); "Terminology for developers and
    presenters" (Working glossary)
  - Blocked by: nothing

### M6 (hours 21-24)

- [ ] **RS-09 · Complete the submission checklist and submit**
  - **Report 1.1 change:** The report is brought up from version 1.1; closes `design source references`.
  - Owner: Researcher, document owner, and presenter (not assigned; `researcher role`) · Tier: A · Size: S (estimate 2-5 h)
  - Depends on: RS-03, RS-08, SH-35 · Needs: X-59 · Provides: nothing
  - Paths: `docs/product`
  - Work: Build the checklist from the confirmed organizer requirements at M0, complete it at M6, and
    submit with them; a person submits, and the submission is never automatic. Before submitting,
    bring `docs/product/task-passport-project-report.docx` up to the chosen model, contracts, policy
    and implementation scope of the submitted build, "so that the submission describes the final
    product accurately".
  - Done when: the M6 exit holds: "Submission checklist is complete; the demonstration matches the
    submitted build and its documented limitations." The report describes the chosen model,
    contracts, policy and implementation scope of the submitted build.
  - Tests: none (no code).
  - Report: "Research documentation and submission workflow"; "Relative implementation milestones and
    critical dependencies" (Hours 21-24); "Sources and evidence register" (Evidence to add after
    implementation)
  - Blocked by: nothing

## Open decisions and blockers

Side files cite the strings in the first column verbatim, backticks included, in "Blocked by". The numbered decisions are
those in `docs/product/README.md`, where the document owner records each outcome. "Settled through" names
what settles or carries the item; "Blocks" lists the IDs in this file that wait on it, and the side
files add theirs.

| Cite as                                | Item                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Status                                                                                                                                                                         | Owner                                                                        | Settled through                                                              | Blocks (this file)                                                                                                                       | Settle by                 |
| -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| `decision 1 in docs/product/README.md` | Schemas `app`, `runtime` and `demo` in one PostgreSQL instance                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Settled by the report                                                                                                                                                          | integration with nestjs                                                      | the report                                                                   | nothing                                                                                                                                  | settled                   |
| `decision 2 in docs/product/README.md` | Database roles; the single Go executor connection                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Settled by the architecture specification: the single Go executor connection, which matches the report's recommendation; report content and lineage join that transaction      | the lead (database roles, by default)                                        | SH-06                                                                        | SH-26, SH-38; X-35, X-47, X-57                                                                                                           | P (M1 at the latest)      |
| `decision 3 in docs/product/README.md` | Browser to API path                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Open; a proposal is recorded, proposed, not decided                                                                                                                            | the web + API implementer                                                    | SH-02                                                                        | SH-22; X-31, X-43, X-60                                                                                                                  | M0                        |
| `decision 4 in docs/product/README.md` | Operator context to Go                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Requirement settled; the two sources differ on the mechanism (the architecture specifies signed, short-lived operator context; the report lists it as unresolved)              | the web + API implementer with the Go implementer                            | SH-03                                                                        | SH-20, SH-22, SH-26 (Go authority-record read grant), SH-39; X-14, X-23, X-26, X-27                                                      | M0                        |
| `decision 5 in docs/product/README.md` | Background worker in Go: PostgreSQL jobs with leases, no broker                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Settled by the report; repository rule: graceful shutdown and the readiness check cover the worker                                                                             | the Go implementer                                                           | the report                                                                   | nothing; how readiness covers the worker is `worker readiness`                                                                           | settled                   |
| `decision 6 in docs/product/README.md` | Model provider, model and accounting rule                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Credentials in Go settled; provider, model and accounting rule open                                                                                                            | the Go implementer with the lead (infrastructure)                            | SH-04                                                                        | SH-13, SH-37 (the model name and pricing only); X-04, X-53                                                                               | P                         |
| `decision 7 in docs/product/README.md` | Authentication mechanism                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open; the design is on hold by the user's decision of 2026-10-03, and nothing is decided                                                                                       | the web + API implementer                                                    | SH-01, which waits for the hold to be lifted                                 | SH-02, SH-03, SH-15, SH-19, SH-20, SH-22, SH-25, SH-26 (Go authority-record read grant), SH-39; X-14, X-17, X-22, X-23, X-31, X-34, X-43 | M0 (the M1 exit needs it) |
| `decision 8 in docs/product/README.md` | Reuse of pre-event work and its disclosure                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Open; the competition rules say work starts no earlier than 11:00 on 3 October 2026 and say nothing about reuse                                                                | the document owner (the lead until `researcher role` is settled)             | RS-01                                                                        | SH-09 (fixes before the window), SH-12; X-01                                                                                             | P                         |
| `read path`                            | Whether NestJS reads sanitized runtime views or calls private Go endpoints for the run, usage, event and review reads                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Open; conflicting between the two sources (the architecture chooses views; the report also names Go endpoints)                                                                 | the document owner, with both implementers                                   | SH-05                                                                        | SH-10 (read operation names), SH-16 (views), SH-22, SH-26, SH-27 (review view); X-29, X-30, X-35, X-41, X-60                             | M0                        |
| `contract owners`                      | One owner for each of the eight contracts                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Not recorded                                                                                                                                                                   | the web + API implementer gives them to the document owner                   | SH-07                                                                        | SH-10, SH-11, SH-39; X-07 to X-14                                                                                                        | P                         |
| `Go package owners`                    | One owner per Go module, including the internal API, the runtime repository and events, data minimization, the replay and the Go DTO mirrors, which the report's team table does not name; the starter's existing packages; and the Go endpoints for X-25 and X-64 if their open items choose Go                                                                                                                                                                                                                                                                                                                           | Settled by the staffing correction: every Go package belongs to the Go implementer; package names are the architecture's proposal (`Go package layout`)                        | the Go implementer                                                           | SH-07                                                                        | X-15, X-27, X-36, X-50, X-65                                                                                                             | P                         |
| `final result format`                  | "Final-output validation requires an output format and a data rule"; the report recommends "a structured status with authorized report references", proposed, not decided                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Open                                                                                                                                                                           | run state contract owner (not recorded) with the Go implementer              | SH-10 brings it to the document owner                                        | X-11, X-64                                                                                                                               | M0                        |
| `record versions`                      | "Record-version rechecks require versions and comparison semantics"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Partly settled: optimistic record versions are the architecture's direction; comparison semantics open; scope adds report, source, template and projection versions            | action proposal contract owner (not recorded) with the Go implementer        | SH-10                                                                        | SH-17, SH-24; X-09, X-20, X-33                                                                                                           | M0                        |
| `exact reviewed material`              | "Freeze the payload, or bind its source records to versions and require a new proposal when they change"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Settled by report 1.1: "Freeze the payload and bind its source records, template and projection to versions."                                                                  | the Go implementer                                                           | SH-10                                                                        | X-09, X-45                                                                                                                               | M0 (M3 at the latest)     |
| `dispatched attempts`                  | "Durable worker recovery requires identifiable dispatched attempts"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Partly settled: the architecture proposes `model_calls` and `execution_attempts`; GO-02 decides the recovery rule                                                              | the Go implementer                                                           | the Go side (`docs/roadmap/go.md`); the tables follow in SH-27               | SH-27; X-39, X-54                                                                                                                        | M3                        |
| `revocation reads`                     | "Current revocation requires a single owner and reliable reads"; the report names NestJS as the owner of app-schema revocations and Go as the enforcer, and leaves the read undefined                                                                                                                                                                                                                                                                                                                                                                                                                                      | Partly settled: Go reads current revocations in `app` directly; the record shape is open; scope adds source and template revocations                                           | the web + API implementer with the Go implementer; grant: the lead           | SH-14                                                                        | SH-38; X-47                                                                                                                              | M3                        |
| `multiple-action responses`            | A model response with several actions "should be rejected or handled by an explicitly defined policy"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Open                                                                                                                                                                           | the Go implementer                                                           | the Go side (`docs/roadmap/go.md`)                                           | nothing in this file (Go-internal)                                                                                                       | M1                        |
| `model call retries`                   | Whether a failed model call is retried. Figure 5 goes from a failed call (Diagram 2: MOK -> MFAIL -> FAILED, "Record failure and known usage / Retain uncertain cost reservation") straight to "Run failed", while "Atomic allowances hard limits and estimated cost" says "Retries are separate attempts and consume allowance", model_call_limit reads "Illustrative hard dispatch count; retries consume allowance.", and "Risk register and scope controls" answers "Rate limits, malformed proposals, or intermittent calls interrupt the loop." with "Use bounded validation/retries and transparent failure states" | Open; a report inconsistency                                                                                                                                                   | document owner, with the Go implementer                                      | the document owner                                                           | X-46 (the retry part)                                                                                                                    | M0 (M1 at the latest)     |
| `canonical arguments`                  | "Canonicalization must be defined deliberately": the canonical representation of the supported argument types, the stored fields the action digest covers and how a change is detected; Go is "the authority for action canonicalization"                                                                                                                                                                                                                                                                                                                                                                                  | Open; no proposal is recorded                                                                                                                                                  | the Go implementer                                                           | the Go side (`docs/roadmap/go.md`); X-09 carries it at the M0 freeze (SH-10) | X-09                                                                                                                                     | P (before the M0 freeze)  |
| `replay entry`                         | How the labelled adversarial action replay enters a run, so beat 6 completes "within the same run and allowance", and how it is marked in the stored action and its events; one option needs the conditional X-65                                                                                                                                                                                                                                                                                                                                                                                                          | Open; basis changed: how a recorded proposal enters a run for the export test (beat 5) and the supporting rehearsal (beat 9)                                                   | the Go implementer                                                           | the Go side (`docs/roadmap/go.md`)                                           | X-12 (replay label), X-36, X-65                                                                                                          | P (before the M0 freeze)  |
| `form options`                         | No listed operation supplies the task form's options (template, vendor, invoice set, destination, approval requirement, limits)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Open                                                                                                                                                                           | document owner                                                               | SH-10                                                                        | SH-26 (form-option read grant); X-25                                                                                                     | M0                        |
| `passport in the run view`             | Whether "Read authorized run and usage view" carries the passport representation the summary shows                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Open                                                                                                                                                                           | document owner, with the passport and run state contract owners              | SH-10                                                                        | X-08, X-29                                                                                                                               | M0                        |
| `review payload read`                  | No listed operation delivers the exact stored action to an authorized reviewer                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open                                                                                                                                                                           | document owner                                                               | SH-10                                                                        | SH-27 (review view); X-41                                                                                                                | M0 (M3 at the latest)     |
| `stored report read`                   | No listed operation delivers the stored report and its registered template to the interface, which "renders its substantive content from the stored report and registered template" (part of the recommended final result, proposed, not decided); Figure 1 gives NestJS no read of `demo`                                                                                                                                                                                                                                                                                                                                 | Open                                                                                                                                                                           | document owner                                                               | SH-10                                                                        | SH-24 (report view), SH-26 (report read grant); X-64                                                                                     | M0 (M2 at the latest)     |
| `demonstration baseline`               | Beat 2 needs the starting record versions, report count and outbox count; the report names no surface for them                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open; basis changed: beat 2 shows the note with its label, invoice versions and the empty outbox; the report names no surface                                                  | document owner                                                               | the document owner                                                           | SH-28                                                                                                                                    | M2                        |
| `app-schema seed ownership`            | Who writes the seed for the operator, memberships, task template, policy version and tool catalog                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Partly settled: the web + API implementer writes the seeds by default (`shared-track assignment`); the seed adds the two report templates and the projection rule              | the web + API implementer                                                    | SH-10, before SH-18                                                          | SH-18, SH-19; X-21, X-22                                                                                                                 | M0                        |
| `command timeout budget`               | The web proxy allows 10 s, `GATEWAY_TIMEOUT_MS` up to 20 s and the gateway's write timeout is 30 s; no budget exists for commands                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Open                                                                                                                                                                           | nestjs with frontend and go                                                  | SH-10                                                                        | X-28                                                                                                                                     | M0                        |
| `smoke under login`                    | Health and diagnostics stay public; how smoke reaches product pages behind a login depends on decision 7                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open                                                                                                                                                                           | the lead                                                                     | the owners, after decision 7                                                 | SH-23                                                                                                                                    | M1                        |
| `worker readiness`                     | The readiness `checks` object is closed and holds only the database, so covering the worker is a shared contract change unless the team meets the rule another way                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Open                                                                                                                                                                           | the Go implementer with the web + API implementer                            | SH-14                                                                        | X-32                                                                                                                                     | M1                        |
| `app-schema record list`               | The report's table lists revocations and no users; Figure 1's `app` node lists users, and its edge GATE -> APPDB reads current revocations from `app`                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Partly settled: the union grows by organizations, task templates, report templates and projection rules; source labels and recipient rules are `source classification storage` | document owner                                                               | `docs/product/README.md` (schema table)                                      | nothing                                                                                                                                  | settled                   |
| `report storage`                       | Where report lineage lives and whether context manifests are a runtime record: lineage in `runtime` (`report_lineage`) per the architecture, or with the immutable reports in `demo` per the report                                                                                                                                                                                                                                                                                                                                                                                                                        | Open; conflicting between the two sources                                                                                                                                      | the document owner, with the Go implementer                                  | the document owner; SH-10 records the tables                                 | SH-24, SH-26, SH-40; X-33, X-64, X-69                                                                                                    | M0 (M2 at the latest)     |
| `internal report rendering`            | Whether the internal report body may hold model-written text and so needs a conservative context manifest (report) or is rendered deterministically (architecture)                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Open; conflicting between the two sources                                                                                                                                      | the document owner, with the Go implementer                                  | the document owner                                                           | SH-40 (context part); X-69 (context part)                                                                                                | M0                        |
| `source classification storage`        | Where the trusted source-field classifications and recipient rules are stored and who writes them; neither source names a table                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Open; conflicting between the two sources                                                                                                                                      | the document owner, with both implementers                                   | the document owner; SH-10 records the table                                  | SH-17, SH-25, SH-26; X-20, X-70                                                                                                          | M0                        |
| `vendor projection fields`             | Which fields the vendor projection may hold; the report's two example lists differ, and "The final approved field list would be a team policy decision"                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Open; a team decision                                                                                                                                                          | the document owner for the disagreement; the team fixes the list at SH-10    | SH-10                                                                        | X-06; X-68                                                                                                                               | M0                        |
| `passport report fields`               | Passport shape for templates, projections and destinations (singular or plural destination; `source_policy_version` or `allowed_projection_rules`)                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Open; conflicting between the two sources                                                                                                                                      | the passport contract owner (not recorded), with the document owner          | SH-10                                                                        | X-08                                                                                                                                     | M0                        |
| `rename operation`                     | Which operation renames or copies a report; neither source defines one, yet the demonstration renames a report "Public summary"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Open; undefined in both                                                                                                                                                        | the document owner                                                           | the document owner; a build task is added once an operation is chosen        | X-73 (rename part)                                                                                                                       | M0 (M4 at the latest)     |
| `policy editor`                        | Whether the Next.js policy editor and the `/policies` route are in scope: the architecture lists them; the report keeps "A policy editor" outside the initial delivery scope                                                                                                                                                                                                                                                                                                                                                                                                                                               | Open; conflicting between the two sources                                                                                                                                      | the document owner                                                           | the document owner                                                           | nothing (no task builds an editor)                                                                                                       | M2                        |
| `list reads`                           | `/runs` and `/approvals` need a run list and a pending-approval list; neither source defines such a read                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open; gap in both                                                                                                                                                              | the document owner                                                           | the document owner; tasks are added after the outcome                        | nothing (no task builds these pages)                                                                                                     | M2                        |
| `command idempotency keys`             | The architecture puts idempotency keys on commands; the report on actions only                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open; architecture only                                                                                                                                                        | the document owner, with the start-run and approval decision contract owners | SH-10                                                                        | X-07, X-10 (the key field only)                                                                                                          | M0                        |
| `repository layout`                    | The architecture shows `db/migrations`, `db/seeds`, `infra/docker-compose.yml` and `docs/diagrams`; this repository keeps migrations in `apps/api/src/database/migrations` (verified reason), Compose in `infra/compose.yaml` and design files in `docs/product`; the seed location is not decided                                                                                                                                                                                                                                                                                                                         | Open; conflicts with verified repository facts                                                                                                                                 | the document owner, with the lead                                            | the document owner                                                           | SH-18 (seed location only)                                                                                                               | M0                        |
| `deployment network`                   | "NestJS and Go use a private service network. Only the browser-facing entry points are exposed." against the current Compose file, which has no network key and has never run                                                                                                                                                                                                                                                                                                                                                                                                                                              | Open; unverified                                                                                                                                                               | the lead (infrastructure), with the document owner                           | SH-30                                                                        | SH-30 (network part)                                                                                                                     | M4                        |
| `Go package layout`                    | Inconsistencies inside the architecture's module list and package tree (data minimization has no package; `api` and `tools` have no module entry; whether `internal/api` replaces the starter's `internal/httpserver`)                                                                                                                                                                                                                                                                                                                                                                                                     | Open                                                                                                                                                                           | the Go implementer; inconsistencies go to the document owner                 | SH-10                                                                        | nothing                                                                                                                                  | M0                        |
| `design source references`             | The report's internal sources (`project-architecture.mmd`, `task-execution-flow.mmd`, `report-information-flow.mmd`, `CLAUDE_SETUP_PROMPT.md`) are not in the repository; one report sentence names a diagram label that changed                                                                                                                                                                                                                                                                                                                                                                                           | Open                                                                                                                                                                           | the document owner                                                           | RS-09 (the report update)                                                    | nothing                                                                                                                                  | M6                        |
| `researcher role`                      | The report's sixth role, "Researcher, document owner, and presenter", has no person; the lead acts as document owner until the team assigns it                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open (staffing correction of 2026-10-03)                                                                                                                                       | the lead                                                                     | SH-07                                                                        | RS-01 to RS-09, SH-05, SH-33; X-01 unless the lead takes RS-01                                                                           | P                         |
| `shared-track assignment`              | Confirm the default: migrations and seeds by the web + API implementer with the Go implementer's table definitions; database roles, Compose and deployment, smoke checks and evidence by the lead. Unassigned by the default: SH-21, SH-22, SH-29, SH-34 coordination, SH-35, SH-08 coordination, lockfiles, instruction files and docs consistency                                                                                                                                                                                                                                                                        | Open (staffing correction of 2026-10-03)                                                                                                                                       | the lead                                                                     | SH-07                                                                        | owner fields of the SH tasks                                                                                                             | P                         |

## Not on this roadmap

These are the report's deferred features and the items it puts outside the initial delivery scope.
Nobody adds them under time pressure; a change goes through the document owner first.

- Deferred features ("Functional requirements MVP boundary and deferred scope"):
  - "Broad enterprise identity federation, complex organizational hierarchies, and production
    onboarding."
  - "Multiple model providers, arbitrary agent frameworks, dynamic executable tool installation, and
    general browser or shell control."
  - "Real payments, production email delivery, live bank records, and connectors requiring access to
    customer systems."
  - "General natural-language policy compilation, automatic declassification, arbitrary safe
    paraphrasing, comprehensive content-loss prevention, and classification of every possible leak."
    (report 1.1)
  - "The prototype does not require a general information-flow engine or arbitrary document
    classification." A free-form template editor is deferred (architecture specification).
  - "Multi-agent delegation chains, cross-region deployment, advanced observability, and
    organization-wide financial chargeback."
  - "A formal security certification, universal attack coverage, or a claim that the prototype can
    control independently accessible credentials."
- Outside the initial delivery scope ("Delivery scope and six person ownership"): "A policy editor,
  arbitrary tool registration, multiple model providers, real email, complex organization
  administration, and a C++ worker".
- Phase 2 and Phase 3 of "Phased roadmap beyond the prototype": a documented tool-adapter interface,
  more task templates, usability research, deployment controls, a limited real integration; stronger
  isolation, secrets rotation, policy administration, operational monitoring, retention controls,
  adversarial testing, independent security review, organization-scale reservations, and more workers
  and providers only on demonstrated need.
- The domain adaptations (Smart City, defensive cyber operations) are "illustrative adaptations, not
  additional MVP commitments" ("Illustrative invoice scenario and future domain adaptations").
- AGENTS.md also keeps out payments of any kind, real or simulated, live email delivery, real bank or
  customer records, Redis, Kafka, Kubernetes and additional services; embeddings and vector databases
  need a team decision.

## Definition of done

A task is done when its "Done when" was observed and all of the following hold. The task owner then
ticks the box.

1. **The implementation workflow in AGENTS.md was followed.** Place it in the service that owns the
   responsibility; contract first, through the recorded owner, landed by nestjs and mirrored by go;
   build inside your paths and hand off the rest; test what can break silently (failure mapping,
   rejection of bad input or credentials, persistence and migrations), never tests that restate a
   constant; verify and quote; record it (the module in "Product modules" in `docs/architecture.md`,
   new variables in `.env.example`, Compose and the README table through infrastructure, design
   changes to the document owner). Then commit and push the task as "Committing and pushing" in
   AGENTS.md describes.
2. **The checks of each touched area ran, from the repository root:**
   - frontend: `pnpm --filter web run lint`, `pnpm --filter web run typecheck`,
     `pnpm --filter web run test`, `pnpm --filter web run build`, `pnpm --filter @workspace/ui run lint`,
     `pnpm --filter @workspace/ui run typecheck`, `pnpm exec prettier --check apps/web packages/ui`.
   - nestjs: `pnpm --filter api run lint`, `pnpm --filter api run typecheck`,
     `pnpm --filter api run test`, `pnpm --filter api run build`; when a contract changed,
     `pnpm --filter @workspace/contracts run lint`, `typecheck`, `test` and `build`;
     `pnpm exec prettier --check apps/api`; `pnpm db:migration:show` when the migration tooling
     changed and a database is reachable.
   - go: `pnpm --filter gateway run format:check`, `pnpm --filter gateway run lint`,
     `pnpm --filter gateway run typecheck`, `pnpm --filter gateway run test`,
     `pnpm --filter gateway run build`.
   - infrastructure: `pnpm run setup`; `pnpm exec prettier --check infra scripts`; `pnpm infra:up` and
     `pnpm infra:down`, and `pnpm stack:up`, `pnpm smoke --mode=container` and `pnpm stack:down`,
     when a container runtime is available; `pnpm dev` followed by `pnpm smoke` when the dev runner
     changed.
   - integration: `pnpm db:migration:show` when a migration changed and a database is reachable;
     `pnpm check:instructions`; `pnpm format:check`; `pnpm verify`; `pnpm smoke` when the services
     are running.
   - Database-backed tests: the command from SH-21 (X-24) once it exists.
   - Then `pnpm verify` for every change, and `pnpm smoke` against the running stack before a merge
     that touches service wiring.
3. **The results are quoted.** Each command appears with its real result. Anything not verified is
   named with the reason (for example no Docker). `pnpm verify` treats a skipped step as a failure.
   The reviewer agent treats a claim without evidence as unverified.
4. **Nothing is mocked silently.** A mocked response "must be replaced or clearly labeled before any
   result is presented as a working capability", and a missing protection becomes a documented
   limitation. The simulated outbox, replays, recordings and estimated cost are labelled.
5. **A critical check has its evidence.** For a report action this includes the stored provenance
   before and after the attempt. Per the validation plan: "Maintain status, build identifier,
   fixture, observed outcome, and evidence reference for each check", and "For each acceptance check,
   record the build identifier, fixture, observed outcome, timestamp, and evidence location." For
   every denied action, "Compare business state and execution records before and after every denied
   action": record versions, report count, outbox count and execution records. "Happy-path screenshots
   alone are insufficient." "Keep failed or unverified checks visible."
