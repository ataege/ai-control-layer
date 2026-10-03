# Task Passport roadmap

**Status.** This roadmap is a plan derived from the project report,
`docs/product/task-passport-project-report.docx` (version 1.0, design baseline, 3 October 2026), and
from the repository at commit `5c38d8b`. Nothing in it is implemented: every task is open and every
sync point is unreached. Sizes are estimates, not a schedule. The milestone windows are relative to
the report's 24-hour coding window; the organizers' confirmed rules and deadline take precedence.
The plan changes when the report changes.

## How to use this roadmap

**Read the full report first.** AGENTS.md requires it at the start of every session, before the
first task: "read the full report from beginning to end, including the appendices. A skim, a search
or a single section is not enough." Then read `docs/product/README.md` for the decisions recorded
since. AGENTS.md, section "Read the project report first", gives the conversion commands; the text
drops the eight figures, which reproduce `docs/product/task-passport-architecture.svg` and
`docs/product/task-passport-run-lifecycle.svg`.

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
- Landing the frozen contracts is SH-11 (Implementer 2), and SH-39 for the operator context
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
implementation scope changes ("Sources and evidence register"), the document owner (the researcher)
updates the report's decision record, contracts, demonstration and claim-to-proof list together
(AGENTS.md, "Implementation workflow", step 6); this roadmap changes after that. Mentor feedback
becomes "a short requirements update, not uncontrolled feature additions" ("Research documentation
and submission workflow").

## Sides and people

The user split the team into two sides. The report organizes six people by responsibility ("Delivery
scope and six person ownership", table "Proposed team ownership"), and AGENTS.md maps them to the
project agents.

| Side                  | People (report roles)                                                                                                                                                                                                                                                                                                                                                                                      | Agents and paths (AGENTS.md)                                                                    | Roadmap file                  |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | ----------------------------- |
| Go side               | Implementer 3 (agent runtime): Go worker, provider integration, bounded agent loop, model reservations, usage accounting and cancellation checks. Implementer 4 (enforcement): Go admission, immutable passport, argument checks, exact-action approvals, execution claims and denial feedback. Implementer 5 for the four tool adapters (`read_invoice`, `read_vendor`, `create_report`, `queue_report`). | go role, `services/gateway`; one owner per Go package, recorded in `services/gateway/README.md` | `docs/roadmap/go.md`          |
| Next.js + NestJS side | Implementer 1 (interface): task form, passport summary, run timeline, approval preview and terminal states. Implementer 2 (application API): NestJS authentication, membership checks, task configuration, runtime facade and authorized event feed; coordinates shared contracts.                                                                                                                         | frontend (`apps/web`, `packages/ui`); nestjs (`apps/api`, `packages/contracts`)                 | `docs/roadmap/web-and-api.md` |
| Shared track          | Implementer 5 (data and integration): migrations, service database roles, synthetic fixtures, reset procedure and deployment integration. Researcher, document owner, and presenter: official requirements, sponsor clarification, source register, demo specification, claim-to-proof tracking, presentation and submission checklist; no product code.                                                   | integration and infrastructure; the researcher keeps `docs/product` and uses the reviewer agent | this file                     |

- Implementer 5 works on two tracks: the four adapters on the Go side, the data and integration work
  on the shared track.
- The report also gives Implementer 5 the "transactional effects" (a demo effect, its execution
  record and its event committed together). The user's split does not place them; the team records
  their placement with the Go module owners in SH-07.
- Moving people between the sides is a team decision.

**Effort split (estimates, not a schedule).** Four independent estimates of the MVP, made on
2026-10-03 from the current starter with tests included, agree on the shape and disagree on the
size:

- Go is the larger side in every estimate: 54 to 58 percent of the non-shared engineering effort,
  computed as Go / (Next.js + NestJS + Go) at the midpoints. Shared work is 14 to 29 percent of each
  total.
- The totals run from 117-236 h (lowest method) to 210-397 h (highest); the methods differ on
  absolute hours by a factor of about 1.7 to 1.8.
- Every total is above what five implementers can deliver in the 24-hour window at the report's
  testing depth (one estimate assumes 18 to 20 focused hours per implementer; that is its own
  assumption, not a report figure). The estimates already include the reductions the report allows
  when time is tight.
- Every lever the estimates found for an even split recounts work, adds optional or deferred scope,
  cuts evidence the report requires, or needs a decision nobody has made. The tiers below set the
  order of cuts; they do not make the work fit. By this roadmap's own sizes, Tier A alone is 92
  tasks of 172-368.5 h in M0 to M6 (the researcher track excluded), all placed in M0 to M3 (hours
  0-14), against a physical ceiling of 120 person-hours for the whole window (five implementers
  working all 24 hours without a break). At the midpoints, Tier A is 128.5 h in `docs/roadmap/go.md`
  and 88 h in `docs/roadmap/web-and-api.md`, against about 41-50 h for the Go side's 2.3 to 2.5
  people and 36-40 h for the Next.js + NestJS side's two under the 18 to 20 h assumption above.
  Removing every Tier B and Tier C task still leaves the plan above the window; "Tiers" says what
  that requires.

## Tiers

| Tier     | Meaning                                                                                                                                                                                                      | Report basis                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| A        | Needed for the smallest credible vertical slice, or for one of the four protections the report keeps intact under time pressure.                                                                             | "the smallest credible vertical slice is one authorized invoice/report workflow, one denied out-of-scope operation, one exact-action approval and one limit-triggered stop" ("Threat model limits and unresolved design choices"); "Keep service-side authorization, scope enforcement, pre-dispatch limits, and exact-action approval intact." ("Relative implementation milestones and critical dependencies")                                                                                       |
| B        | The rest of the report's MVP requirements, acceptance evidence and critical checks.                                                                                                                          | Table "Proposed MVP requirements and acceptance evidence" ("Functional requirements MVP boundary and deferred scope"); table "Proposed critical checks" ("Validation plan and evidence matrix")                                                                                                                                                                                                                                                                                                        |
| C        | Optional breadth the report says to cut first: for example server-sent events after authenticated polling, extra screens, polish beyond the exit conditions, the optional unknown-outcome failure rehearsal. | "If time becomes tight, keep a fixed server-owned task template instead of a general policy editor, use one worker process with durable jobs instead of distributed scheduling, and use authenticated polling before adding SSE if necessary." ("Relative implementation milestones and critical dependencies"); "The team should first remove optional breadth when delivery is at risk, while keeping the checks required to substantiate the control-layer proposition." ("Design decision record") |
| Deferred | The report's deferred features. They are not on this roadmap; see "Not on this roadmap".                                                                                                                     | "Functional requirements MVP boundary and deferred scope", list "Deferred features"                                                                                                                                                                                                                                                                                                                                                                                                                    |

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

## Milestones

The windows are the report's relative windows from the table "Proposed 24-hour implementation
sequence" ("Relative implementation milestones and critical dependencies"). The report: "The
implementation plan assumes a 24-hour coding window; the organizers' confirmed rules and actual
submission deadline take precedence." ("Delivery scope and six person ownership"). The focus and exit
columns quote the report. The last column lists the sync points that must be reached by the end of
the milestone, which is their "Needed by" in "Sync points".

| Milestone | Report window | Team focus (report)                                                                                                                                                   | Exit condition (report)                                                                                                                                                                                                                                                                                 | Sync points needed by the end                                                  |
| --------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| P         | None          | Not in the report. Roadmap: decisions, environment setup, container validation and organizer questions.                                                               | The report gives no window or exit condition for P. Roadmap condition, not a quote: the decisions the first contracts depend on are recorded or carried into M0 as open, every implementer machine passes `pnpm verify`, and the organizers' answer on decision 8 is recorded or its absence is stated. | X-01                                                                           |
| M0        | Hours 0-2     | "Confirm rules and sponsor expectations; freeze task, contracts, tool arguments, policy fixture, and schema ownership. Bring up the starter and provider connection." | "A single documented workflow, working service connectivity, and an agreed contract example for each command and event."                                                                                                                                                                                | X-03 to X-06                                                                   |
| M1        | Hours 2-6     | "Build the vertical path in parallel: task form, authenticated facade, admission, worker, live model call, one read tool, and event feed."                            | "A real operator starts a run; Go executes a permitted tool; the interface displays the actual persisted result."                                                                                                                                                                                       | X-07 to X-32                                                                   |
| M2        | Hours 6-10    | "Complete four adapters, scoped reads, structured report creation, destination checks, and bounded correction feedback."                                              | "A permitted reconciliation succeeds; an explicit prohibited proposal produces no business effect."                                                                                                                                                                                                     | X-33 to X-38, X-63, X-64 (X-65 only if the replay is triggered through NestJS) |
| M3        | Hours 10-14   | "Add frozen action previews, authorized approval decisions, approval consumption, model/tool reservations, and controlled stopping."                                  | "The reviewed action executes once; changed content and depleted allowance cannot dispatch an operation."                                                                                                                                                                                               | X-39 to X-46, X-66, X-67                                                       |
| M4        | Hours 14-18   | "Exercise concurrency and waiting-state recovery; inspect context and safe events; finish reset and deployment procedures."                                           | "Critical checks have recorded outcomes, and the team can reset fixtures and repeat the workflow."                                                                                                                                                                                                      | X-02, X-47 to X-57 (X-60 is optional, Tier C)                                  |
| M5        | Hours 18-21   | "Polish comprehension and error states; rehearse the complete story; capture evidence from the final build."                                                          | "A reviewer can understand the task boundary, attempted action, decision, actual effect, and limitation without narration filling gaps."                                                                                                                                                                | X-58, X-59 (X-61 is optional, Tier C)                                          |
| M6        | Hours 21-24   | "Freeze features, fix critical faults, finalize documentation and artifacts, and submit using the confirmed organizer requirements."                                  | "Submission checklist is complete; the demonstration matches the submitted build and its documented limitations."                                                                                                                                                                                       | X-62                                                                           |

Notes:

- The report's sequence names no web or API work in hours 6-10 (M2), and it does not name several
  items of the ownership table: for Implementer 1 the passport summary, run timeline and terminal
  states; for Implementer 2 authentication, membership checks and task configuration. The side
  roadmaps place that ownership-table work in M2, except the parts the M1 exit already needs: the M1
  focus names an "authenticated facade", so authentication and the membership check on the start-run
  command come at M1.
- The authentication design (decision 7) is on hold, and the M1 exit needs "A real operator".
  Decisions 3 and 4 wait on decision 7 (SH-02, SH-03). While the hold stands, the M1 exit cannot be
  reached, and by the "Depends on" column of "Sync points" neither can X-14, X-22, X-23, X-26, X-27,
  X-28, X-31, X-40, X-42, X-43, X-55 and X-56 (it needs the second organization's sign-in from
  X-34), nor the optional X-60 and the conditional X-65. That column names direct dependencies
  only; through their providing tasks' "Depends on" and "Needs", more sync points wait, among them
  X-44 to X-47, X-50 to X-54, X-57 to X-59 and X-62, the optional X-61, and X-25, X-29, X-30, X-41
  and X-64 when a Go endpoint serves them (X-25 also when app records do). X-15 (the X-14 mirror)
  waits in part. X-17 waits on decision 7 for the user records, so SH-15, which needs it, stays
  open; X-18 is therefore not reached, nor X-21 (or X-25 where SH-18 provides it), because SH-18
  depends on SH-15, nor X-34, because SH-25 depends on SH-18 and also waits for the second
  organization's sign-in. The task, policy and tool tables and their seed can still be written and
  applied during the hold; SH-15 and SH-18 stay unticked until the user records land. By these
  fields, only X-01 to X-13, X-16, X-19, X-20, X-24, X-32, X-33, X-39 and X-49, and X-29, X-30,
  X-41 and X-64 when views serve them, can be reached while the hold stands. This roadmap records
  that and does not resolve it.
- "Progress should be measured by integrated behavior rather than completed screens or isolated
  modules." If the early checkpoint is late, "remove optional breadth before adding more features."
  ("Relative implementation milestones and critical dependencies")
- Load against the windows (sums of this roadmap's task ranges by placement, the researcher track
  excluded; estimates, not a schedule; capacity is five implementers times the window's hours with
  no break): M0 14-38.5 h against 10 person-hours; M1 93-198 h against 20, of which Tier A
  90.5-191.5 h, Tier B 2.5-6.5 h and no Tier C; M2 42-94.5 h against 20; M3 40-79.5 h against 20;
  M4 29.5-72.5 h against 20; M5 11-32 h against 15; M6 4-11 h against 15. So the optional breadth
  to remove when the early checkpoint is late lies in later milestones: all Tier C (9-21 h) is in
  M4 and M5. Per-person loads cannot be added up yet: SH-07 has not recorded the owners of the Go
  modules the report's team table does not name, and several tasks are person-hours summed over
  attendees or span milestones.

First integrated deliverables, from the table "Proposed team ownership". The milestone is this
roadmap's placement, not the report's:

| Role                                      | First integrated deliverable (report)                                                            | Milestone |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------ | --------- |
| Implementer 1: interface                  | "Start a run and display a real persisted event through the NestJS API."                         | M1        |
| Implementer 2: application API            | "An authenticated command reaches Go with verifiable actor and organization context."            | M1        |
| Implementer 3: agent runtime              | "A live model proposes a typed tool action within a recorded allowance."                         | M1        |
| Implementer 4: enforcement                | "An out-of-scope proposal is denied before the tool adapter runs."                               | M2        |
| Implementer 5: data and integration       | "A permitted action produces one inspectable database effect and its matching execution record." | M2        |
| Researcher, document owner, and presenter | "A verified brief and a storyboard with observable evidence for every claimed protection."       | M1        |

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
depend on them"; both depend on decision 7, which is on hold. SH-07 records one owner per contract and
per Go module.

### Environment setup

SH-08: every implementer installs Go 1.27 or newer and Docker with the Compose plugin, then quotes a
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

Frozen in the same M0 session (report, "Hours 0-2"): the four tools' typed arguments (inside X-09),
the reason vocabulary and the decision and error fields (X-13), the task definition and policy
fixture (X-06), schema ownership with table names (SH-10), and the tool-result contract (SH-10):
per tool, the fields it returns to the worker within the X-06 field rules, and the opaque references
for protected values ("Each tool returns an explicit field allowlist; protected values remain opaque
references."). The report: "The runtime developer depends on the action schema and tool-result
contract." ("Relative implementation milestones and critical dependencies", "Critical path and
sensible reductions"). It stays inside the Go side (Diagram 1: ADAPTERS -> FILTER -> RUNNER, all in
GO), so it has no sync point; Implementer 3 (runtime) and Implementer 5 (adapters) agree it with the
data-minimization owner recorded in SH-07, and `docs/roadmap/go.md` plans it.

Rules for every contract:

- "Shared schemas should define supported requests and errors, while Go remains the authority for
  action canonicalization and execution." ("Technical architecture and service ownership")
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
| `POST /api/runs`                                        | Admit the request; issue the passport, run and durable job, or reject with an explanation                      | `POST /internal/runs` (report)                                                                                   | Go side: Implementer 4                                                                                            | X-28       |
| `POST /api/runs/{id}/cancel`                            | Persist cancellation before future dispatches                                                                  | Named at M0 by its owner                                                                                         | Go side: Implementer 3                                                                                            | X-42       |
| `POST /api/actions/{id}/approval`                       | Approve or reject the stored action                                                                            | Named at M0 by its owner                                                                                         | Go side: Implementer 4                                                                                            | X-40       |
| `GET /api/runs/{id}`                                    | Run and usage view                                                                                             | A private Go endpoint named at M0, or a runtime view, per the read path                                          | Go side (private endpoint) or shared track (SH-16 views), per SH-05                                               | X-29       |
| `GET /api/runs/{id}/events`                             | Sanitized events by cursor                                                                                     | As above                                                                                                         | Go side (private endpoint) or shared track (SH-16 view), per SH-05                                                | X-30       |
| None in the report                                      | Exact review payload for authorized reviewers                                                                  | As above; open item `review payload read`                                                                        | Go side (private endpoint) or shared track (SH-27 view), per SH-05                                                | X-41       |
| None in the report                                      | Task form options (template, vendor, invoice set, allowed destination, approval requirement, available limits) | A private Go endpoint named at M0, a read grant on SH-18's records, or app records, per open item `form options` | Go side (endpoint), shared track (SH-18's records) or Next.js + NestJS side (app records), per `form options`     | X-25       |
| None in the report                                      | Stored report and registered template                                                                          | A private Go endpoint named at M0, or a view and read grant, per open item `stored report read`                  | Go side (private endpoint) or shared track (SH-24 view; the read grant is part of X-35), per `stored report read` | X-64       |
| `GET /api/runs/{id}/events` (optional, Tier C)          | Sanitized events as server-sent events, only after authenticated polling works                                 | A private Go endpoint, only if events stream from Go                                                             | Go side, if events stream from Go, per SH-05                                                                      | X-60       |
| None in the report; the document owner records it first | Start the labelled replay (X-36) of one stored prohibited proposal in a named run                              | Named at M0 by its owner, only if `replay entry` has the interface trigger it                                    | Go side: the replay owner recorded in SH-07                                                                       | X-65       |

Every internal command carries service identity and the verified operator context (X-26); Go
verifies both before it acts (X-27).

### Reason vocabulary (proposed)

The report proposes and does not fix: `resource_out_of_scope`, `destination_not_allowed`,
`approval_required`, `approval_expired`, `action_changed`, `resource_version_changed`,
`allowance_exhausted`, `run_cancelled` and `outcome_unknown`. "These names are proposed contract
vocabulary." The M0 freeze (SH-10) records the vocabulary the team adopts, and it stays stable
"across the UI, runtime, tests, and evidence". A policy decision is allow, deny or approval required;
"A transport or configuration error is not an allow decision." Responses "should include a stable
reason code, a safe operator message, and relevant action or run references". ("Illustrative
passport and interface contracts", "Decision and error semantics")

### Schema ownership

| Schema    | Main records (report)                                                         | Write authority (report)                   | Read across the boundary (Figure 1)                                                                     |
| --------- | ----------------------------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------- |
| `app`     | Memberships, task versions, policy versions, tool definitions and revocations | NestJS                                     | Go admission reads authoritative task and policy versions; the Go action gate reads current revocations |
| `runtime` | Passports, runs, jobs, actions, approvals, usage and decision events          | Go                                         | NestJS reads an authorized event view, if the read path keeps that edge                                 |
| `demo`    | Synthetic invoices, vendors, reports and simulated outbox                     | Go tool adapters through restricted access | None in the figure; the interface still needs the stored report (open item `stored report read`)        |

- "Schemas organize data; they do not automatically isolate tenants or contain a compromised
  service." Every query uses verified organization context; "An invoice ID or action ID is a
  reference, not authorization." ("Data ownership and the transition from starter to product")
- "NestJS should receive only the runtime views necessary for the interface, and should not directly
  edit approval grants, execution states or budget balances."
- The report's table lists revocations and no users; Figure 1's `app` node lists users, and its
  edge GATE -> APPDB ("Read current revocations") reads revocations from `app`. Neither list
  excludes the other, and the schema table in `docs/product/README.md` records both in `app`. The
  shape of the user records waits on `decision 7 in docs/product/README.md`.
- One migration toolchain: TypeORM in `apps/api`, run through `pnpm db:migration:*`. App tables come
  from NestJS entities with `schema: "app"`; runtime and demo tables are hand-written by the migration
  owner (integration, Implementer 5). `migration:generate` never emits `CREATE SCHEMA`, so the first
  migration of each schema writes it by hand; TypeORM's bookkeeping table stays in `public`.
- TypeORM never sets `search_path` and the Go pool sets none, so SQL in NestJS and in Go uses
  schema-qualified names.
- Nothing at application startup runs migrations, creates tables or loads seed data. A uuid
  generated column makes TypeORM create an extension at every startup unless
  `uuidExtension: "pgcrypto"` and `installExtensions: false` are set in
  `apps/api/src/database/typeorm-options.ts` (verified by experiment on 2026-10-03).
- Table names, and where the trusted recipient directory, the task's invoice scope and the
  registered report template are stored, are decided at the M0 freeze (SH-10), not by this roadmap.
  For the directory and the template, each outcome has an owner: a stored record's table comes with
  its schema's migration (in `app`, the entity classes of X-17 and SH-15's migration; in `demo`,
  SH-17 for the directory and SH-24 for the template); SH-25 seeds the template if it is a stored
  record; a template that SH-10 records as Go code is defined with the `create_report` adapter on
  the Go side (`docs/roadmap/go.md`).

### The read path (open, for the document owner)

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

| ID   | What becomes available                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Provider                                                                                                                                                                | Consumers                                                                          | Depends on                                                                                        | Needed by                                    |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| X-01 | The organizers' answer on reuse of pre-event work and how to disclose it, recorded in `docs/product/README.md`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Shared track: researcher (RS-01)                                                                                                                                        | Everyone who uses the starter or code written before the window                    | `decision 8 in docs/product/README.md`                                                            | P (at the latest the start of M0)            |
| X-02 | The container path executed once (Compose, the three Dockerfiles, full-container mode), results in `README.md` "Verification status"                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: Implementer 5, infrastructure role (SH-09)                                                                                                                | Shared track (SH-30); both sides (their images)                                    | nothing                                                                                           | M4 (planned in P)                            |
| X-03 | The starter running with service connectivity on every implementer machine, with `pnpm verify`, `pnpm dev` and `pnpm smoke` results quoted                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Shared track: Implementer 5 with each implementer (SH-12)                                                                                                               | Both sides                                                                         | X-01                                                                                              | M0                                           |
| X-04 | The model provider credential readable by Go only: name in `.env.example`, gateway Compose map only, kept out of the web and API children of the dev runner and out of the API process, which loads the root `.env` itself (SH-13), in the smoke leak list                                                                                                                                                                                                                                                                                                                                     | Shared track: Implementer 5, infrastructure role (SH-13)                                                                                                                | Go side: Implementer 3; shared track (SH-22)                                       | `decision 6 in docs/product/README.md`                                                            | M0                                           |
| X-05 | The official requirements sheet (verified brief)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Shared track: researcher (RS-03)                                                                                                                                        | Both sides; shared track                                                           | X-01                                                                                              | M0                                           |
| X-06 | The frozen task definition and policy fixture content (tools, resources, destination, field rules, approval rule, limit values chosen by the team), recorded in `docs/product/README.md`                                                                                                                                                                                                                                                                                                                                                                                                       | Shared track: SH-10                                                                                                                                                     | Go side; Next.js + NestJS side; shared track (SH-18, SH-25); researcher (RS-04)    | nothing                                                                                           | M0                                           |
| X-07 | The start-run request contract in `packages/contracts`: type, JSON Schema and fixture with the agreed example                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Implementers 1 and 2; Go side: Implementer 4                                       | `contract owners`                                                                                 | M1                                           |
| X-08 | The passport representation contract                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Implementers 1 and 2; Go side: Implementer 4                                       | `contract owners`; `passport in the run view`                                                     | M1                                           |
| X-09 | The action proposal contract, with the four tools' typed arguments and the stored review content                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Go side: Implementers 3, 4 and 5; Implementers 1 and 2                             | `contract owners`; `record versions`; `exact reviewed material`; `canonical arguments`            | M1                                           |
| X-10 | The approval decision contract: approve or reject one stored action identifier, with no replacement payload                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Implementers 1 and 2; Go side: Implementer 4                                       | `contract owners`                                                                                 | M1                                           |
| X-11 | The run state contract: status, usage view and terminal reason for every outcome in Diagram 2                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Implementers 1 and 2; Go side: Implementers 3 and 4                                | `contract owners`; `final result format`                                                          | M1                                           |
| X-12 | The safe event contract: sanitized, ordered events with masked metadata, a cursor and a label for replayed proposals                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Implementers 1 and 2; Go side                                                      | `contract owners`; `replay entry` (replay label)                                                  | M1                                           |
| X-13 | The reason vocabulary and the decision and error fields (stable reason code, safe operator message, action or run references) in the error envelope                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-11, landed by Implementer 2                                                                                                                            | Both sides; shared track (SH-23, SH-28)                                            | `contract owners`                                                                                 | M1                                           |
| X-14 | The authenticated operator context contract (repository addition)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Shared track: SH-39, landed by Implementer 2                                                                                                                            | Implementer 2; Go side: internal API owner                                         | `contract owners`; `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md` | M1                                           |
| X-15 | Go DTO mirrors of X-07 to X-14, with strict fixture-decode tests that list every new fixture                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Go side: owner recorded in SH-07                                                                                                                                        | Implementer 2; shared track (SH-14, SH-22)                                         | X-07 to X-14; `Go package owners`                                                                 | M1                                           |
| X-16 | The demo specification and storyboard: the eight beats with the observable proof each needs                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Shared track: researcher (RS-04)                                                                                                                                        | Both sides; shared track (SH-28, SH-33)                                            | X-05; X-06                                                                                        | M1                                           |
| X-17 | The app-schema entity classes with `schema: "app"` in the shared options factory, with the uuid settings that keep startup free of extension creation                                                                                                                                                                                                                                                                                                                                                                                                                                          | Next.js + NestJS side: Implementer 2                                                                                                                                    | Shared track: Implementer 5 (SH-15)                                                | `decision 7 in docs/product/README.md` (user records)                                             | M1                                           |
| X-18 | The `app` schema created and the app tables migrated                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-15                                                                                                                                                     | Implementer 2; Go side: Implementer 4                                              | X-17                                                                                              | M1                                           |
| X-19 | The runtime tables for passports, runs, jobs, actions, decision events and usage records, with the constraints the Go owners agreed                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-16                                                                                                                                                     | Go side: Implementers 3 and 4                                                      | nothing                                                                                           | M1                                           |
| X-20 | The demo tables for invoices and vendors, with organization references, each invoice's vendor reference and record versions                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Shared track: SH-17                                                                                                                                                     | Go side: Implementers 4 and 5                                                      | `record versions`                                                                                 | M1                                           |
| X-21 | The seeded task template, policy version and tool definitions and the minimal synthetic records, loaded by an explicit seed command                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-18                                                                                                                                                     | Go side: Implementers 4 and 5; Implementer 2                                       | X-06; `app-schema seed ownership`                                                                 | M1                                           |
| X-22 | The seeded demonstration operator, organization and membership, labelled as a development demonstration, with a real credential check                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Shared track: SH-19, with Implementer 2                                                                                                                                 | Implementers 1 and 2; Go side                                                      | `decision 7 in docs/product/README.md`; `app-schema seed ownership`                               | M1                                           |
| X-23 | The authentication and operator-context secrets wired, or a record that none is needed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Shared track: SH-20                                                                                                                                                     | Implementer 2; Go side; shared track (SH-19)                                       | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                    | M1                                           |
| X-24 | A documented database-backed test command                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Shared track: SH-21                                                                                                                                                     | Both sides; shared track (SH-16, SH-24, SH-26, SH-27, SH-31, SH-38)                | nothing                                                                                           | M1                                           |
| X-25 | The data for the task form's options (template, vendor, invoice set, allowed destination, approval requirement, available limits) readable by NestJS                                                                                                                                                                                                                                                                                                                                                                                                                                           | Decided with open item `form options`: shared track (SH-18's records; the NestJS read grant is part of X-35), Go side (endpoint) or Next.js + NestJS side (app records) | Implementers 2 and 1                                                               | `form options`                                                                                    | M1                                           |
| X-26 | Every runtime command from NestJS carries the verified actor and organization context                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Next.js + NestJS side: Implementer 2                                                                                                                                    | Shared track (SH-22)                                                               | X-14; `decision 4 in docs/product/README.md`                                                      | M1                                           |
| X-27 | Go verifies service identity and the operator context on every internal command and rejects the rest with the shared envelope                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: internal API owner (SH-07)                                                                                                                                     | Implementer 2; shared track (SH-22)                                                | X-14; `decision 4 in docs/product/README.md`; `Go package owners`                                 | M1                                           |
| X-28 | `POST /internal/runs`: passport, run and job stored in one transaction, or a rejection that names the scope or limit that must change and creates no passport                                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Implementer 4                                                                                                                                                  | Implementer 2; shared track (SH-22)                                                | X-07; X-08; X-13; X-27; `command timeout budget`                                                  | M1                                           |
| X-29 | The run and usage view (status, usage and terminal reason; the passport too if `passport in the run view` says so) readable by NestJS                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Go side (private endpoint) or shared track (SH-16 views; the read grant is part of X-35), per the read-path decision (SH-05)                                            | Implementer 2; shared track (SH-22)                                                | `read path`; X-11; `passport in the run view`                                                     | M1                                           |
| X-30 | Sanitized events readable by cursor                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Go side (private endpoint) or shared track (SH-16 view; the read grant is part of X-35), per SH-05                                                                      | Implementer 2; shared track (SH-22)                                                | `read path`; X-12                                                                                 | M1                                           |
| X-31 | `POST /api/runs`, `GET /api/runs/{id}` and `GET /api/runs/{id}/events` reachable behind authentication through the chosen browser path                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Next.js + NestJS side: Implementers 1 and 2                                                                                                                             | Shared track (SH-22, SH-23, SH-28)                                                 | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                    | M1                                           |
| X-32 | The gateway readiness check reports the worker (repository rule of decision 5), through a shared contract change                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Go side: Implementer 3 (contract through SH-14)                                                                                                                         | Implementers 1 and 2 (diagnostics); shared track (SH-23)                           | `worker readiness`                                                                                | M1                                           |
| X-33 | The report table with an organization reference, and the simulated outbox with a uniqueness constraint tied to the action identifier                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-24                                                                                                                                                     | Go side: Implementers 3, 4 and 5                                                   | `record versions`                                                                                 | M2                                           |
| X-34 | The complete synthetic fixture set: seeded discrepancy, registered reporting address, out-of-scope invoice, hostile note, second organization, small configured allowance                                                                                                                                                                                                                                                                                                                                                                                                                      | Shared track: SH-25                                                                                                                                                     | Go side; Next.js + NestJS side; researcher                                         | X-06; `decision 7 in docs/product/README.md` (second organization's sign-in)                      | M2                                           |
| X-35 | The service database roles and grants, with the per-service credentials wired                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Shared track: SH-26                                                                                                                                                     | Both sides                                                                         | `decision 2 in docs/product/README.md`; `read path`                                               | M2                                           |
| X-36 | A labelled deterministic adversarial action replay through the real gate and execution path                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Go side: owner recorded in SH-07                                                                                                                                        | Shared track (SH-28, SH-33); researcher; Implementer 1 (replay label)              | X-12; X-34; `Go package owners`; `replay entry`                                                   | M2                                           |
| X-37 | Resource boundary evidence: "Denial record plus unchanged excluded records and absence of an execution attempt."                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Go side: Implementers 4 and 5                                                                                                                                           | Shared track (SH-28, SH-36); researcher                                            | X-34; X-36                                                                                        | M2                                           |
| X-38 | Destination boundary evidence: "Stored proposal, rule decision, and outbox comparison."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Go side: Implementers 4 and 5                                                                                                                                           | Shared track (SH-28, SH-36); researcher                                            | X-33; X-34                                                                                        | M2                                           |
| X-39 | The approval, reservation and usage tables, with single-consumption constraints and identifiable dispatched attempts                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Shared track: SH-27                                                                                                                                                     | Go side: Implementers 3 and 4                                                      | `dispatched attempts`                                                                             | M3                                           |
| X-40 | The internal approval decision command (named at M0): Go checks reviewer authority, action integrity and expiry, and stores the grant and the continuation in one transaction                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Implementer 4                                                                                                                                                  | Implementer 2                                                                      | X-10; X-27                                                                                        | M3                                           |
| X-41 | The exact review payload (recipient, rendered content, referenced report and version, reason for review), readable by authorized reviewers only                                                                                                                                                                                                                                                                                                                                                                                                                                                | Go side (private endpoint) or shared track (SH-27 view; the read grant is part of X-35), per SH-05                                                                      | Implementers 2 and 1                                                               | `read path`; `review payload read`; X-09                                                          | M3                                           |
| X-42 | The internal cancel command (named at M0), which persists cancellation before future dispatches                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Go side: Implementer 3                                                                                                                                                  | Implementer 2                                                                      | X-27                                                                                              | M3                                           |
| X-43 | `POST /api/actions/{id}/approval` (the approval part) and `POST /api/runs/{id}/cancel` (the cancel part) reachable behind authentication                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Next.js + NestJS side: Implementers 1 and 2                                                                                                                             | Shared track (SH-23, SH-28, SH-33)                                                 | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                    | M3                                           |
| X-44 | Legitimate task evidence: "Report references, expected discrepancy, one outbox row, and completed run events."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Go side (report, discrepancy, outbox row, run events); Next.js + NestJS side (run start, approval, event display)                                                       | Shared track (SH-28); researcher                                                   | X-34                                                                                              | M3                                           |
| X-45 | Approval integrity evidence: "Tampered proposal outcome and no matching business effect."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Go side: Implementer 4                                                                                                                                                  | Shared track (SH-28); researcher                                                   | X-09; `exact reviewed material`                                                                   | M3                                           |
| X-46 | Limit-triggered stop evidence (demo beat 8): "The next request is rejected before dispatch; a terminal reason is visible and the ledger does not record an unaccounted call." It is also the challenge table's Unpredictable costs evidence: "Run a bounded retry scenario and show execution stopping at the configured limit, with usage and uncertainty recorded."; the retry part follows `model call retries`                                                                                                                                                                             | Go side: Implementer 3; Next.js + NestJS side: Implementer 1 (terminal reason)                                                                                          | Shared track (SH-28); researcher                                                   | X-34; `model call retries` (the retry part)                                                       | M3                                           |
| X-47 | Current revocation records written by NestJS in the `app` schema and read by Go before dispatch and before execution                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Next.js + NestJS side: Implementer 2 (write path); shared track: SH-38 (the table and the Go read grant)                                                                | Go side: Implementers 3 and 4                                                      | `revocation reads`; `decision 2 in docs/product/README.md`; X-66                                  | M4                                           |
| X-48 | A documented reset command                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Shared track: SH-29                                                                                                                                                     | Everyone                                                                           | X-34                                                                                              | M4                                           |
| X-49 | A documented deployment procedure for the demonstration environment                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Shared track: SH-30                                                                                                                                                     | Everyone; researcher                                                               | X-01; X-02                                                                                        | M4                                           |
| X-50 | Field minimization evidence: "Inspected serialized tool result, model request fixture, and event payload."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Go side: Implementer 5 and the data-minimization owner (SH-07); Next.js + NestJS side: Implementer 2 (safe activity views)                                              | Shared track (SH-31); researcher                                                   | X-06; X-12; `Go package owners`                                                                   | M4                                           |
| X-51 | Approval replay evidence: "Concurrent request results, one consumed grant, and one outbox row."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Go side: Implementer 4                                                                                                                                                  | Shared track (SH-31)                                                               | X-39                                                                                              | M4                                           |
| X-52 | Budget concurrency evidence: "Reservation and usage rows with reconciled totals and denied competing request."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Go side: Implementers 3 and 4                                                                                                                                           | Shared track (SH-31)                                                               | X-39                                                                                              | M4                                           |
| X-53 | Unknown usage evidence: "Retained reservation and visibly uncertain estimated-cost state."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Go side: Implementer 3; Next.js + NestJS side: Implementer 1                                                                                                            | Shared track (SH-31)                                                               | `decision 6 in docs/product/README.md`                                                            | M4                                           |
| X-54 | Waiting-state restart evidence: "State before/after restart and exactly one resulting effect."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Go side: Implementer 3                                                                                                                                                  | Shared track (SH-31)                                                               | `dispatched attempts`                                                                             | M4                                           |
| X-55 | Cancellation and expiry evidence: "Cancellation/expiry timestamp, subsequent denial, and dispatch records."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Go side: Implementer 3; Next.js + NestJS side: Implementer 2 (cancel command)                                                                                           | Shared track (SH-31)                                                               | X-42; X-43 (cancel part)                                                                          | M4                                           |
| X-56 | Organization access evidence: "Rejected read/command requests and absence of runtime mutation."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Next.js + NestJS side: Implementer 2 (public path); Go side: internal API owner (internal boundary)                                                                     | Shared track (SH-31)                                                               | X-34                                                                                              | M4                                           |
| X-57 | Database execution transaction evidence: "Fault-injection outcome and consistent action/business state."                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Go side: Implementers 4 and 5                                                                                                                                           | Shared track (SH-31)                                                               | `decision 2 in docs/product/README.md`                                                            | M4                                           |
| X-58 | The interface ready for the rehearsal: all eight beats observable, with labels for the simulated outbox, replays and estimated cost                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Next.js + NestJS side: Implementers 1 and 2                                                                                                                             | Shared track (SH-33); researcher                                                   | X-16                                                                                              | M5                                           |
| X-59 | The build identifier of the final build, with its recaptured evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Shared track: SH-32                                                                                                                                                     | Researcher (RS-08, RS-09); both sides                                              | nothing                                                                                           | M5                                           |
| X-60 | Optional, Tier C: server-sent events for the activity feed, only after authenticated polling works                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Next.js + NestJS side: Implementer 2; also the Go side if events stream from Go, per SH-05                                                                              | Implementer 1                                                                      | `read path`; `decision 3 in docs/product/README.md`                                               | None (optional)                              |
| X-61 | Optional, Tier C: an unknown-outcome failure rehearsal that pauses the action for operator attention instead of repeating it                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Go side                                                                                                                                                                 | Researcher                                                                         | X-34                                                                                              | None (optional)                              |
| X-62 | The technical handoff text for each side's area: setup, architecture and boundaries, tool contracts, known limitations                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Go side and Next.js + NestJS side                                                                                                                                       | Shared track (SH-35); researcher                                                   | nothing                                                                                           | M6                                           |
| X-63 | Permitted reconciliation evidence (M2 exit, demo beat 3): "Returned fields obey policy; the report references authorized records and contains the expected discrepancy."                                                                                                                                                                                                                                                                                                                                                                                                                       | Go side: Implementers 3, 4 and 5                                                                                                                                        | Shared track (SH-36); researcher                                                   | X-33; X-34                                                                                        | M2                                           |
| X-64 | The stored report (stable identifier, version, source references, structured content) and its registered template, readable by NestJS, so the interface "renders its substantive content from the stored report and registered template" rather than from model prose                                                                                                                                                                                                                                                                                                                          | Decided with open item `stored report read`: Go side (private endpoint) or shared track (SH-24 view; the read grant is part of X-35)                                    | Implementers 2 and 1                                                               | `stored report read`; `final result format`; X-33                                                 | M2                                           |
| X-65 | Conditional, only if `replay entry` has the interface trigger it: the internal operation (named at M0) that starts the labelled replay (X-36) of one stored prohibited proposal in a named run, called by a NestJS facade operation with the verified operator context; bounded and labelled, never "an unrestricted runtime command interface". The report's table "Proposed browser and runtime operations" has no such public operation, so the document owner records it first (see "Scope changes"); the side files add the tasks that provide and consume it once this outcome is chosen | Go side: the replay owner recorded in SH-07                                                                                                                             | Implementer 2 (the facade operation); Implementer 1 (the control that triggers it) | X-27; X-36; `Go package owners`; `replay entry`                                                   | M2 if this outcome is chosen; otherwise none |
| X-66 | The app-schema revocation entity class with `schema: "app"`, registered in the shared options factory, in the shape agreed under `revocation reads`                                                                                                                                                                                                                                                                                                                                                                                                                                            | Next.js + NestJS side: Implementer 2                                                                                                                                    | Shared track: Implementer 5 (SH-38)                                                | `revocation reads`                                                                                | M3                                           |
| X-67 | Productive continuation evidence (demo beat 6): "The original useful output is completed within the same run and allowance; correction attempts are counted."                                                                                                                                                                                                                                                                                                                                                                                                                                  | Go side: Implementers 4 and 3                                                                                                                                           | Shared track (SH-28); researcher                                                   | X-34; X-36                                                                                        | M3                                           |

### Diagram 1 boundary crossings

Diagram 1 (`docs/product/task-passport-architecture.svg`, the report's Figures 1-3) places UI in
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

Diagram 2 (`docs/product/task-passport-run-lifecycle.svg`, the report's Figures 4-8). Every step
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

## Task overview

Every shared-track and researcher task in this file, one row each, in milestone order. 48 tasks: 35 Tier A, 12 Tier B, 1 Tier C. 5 name decision 7 (the authentication hold) under "Blocked by". The side tasks are listed in the overviews of `go.md` and `web-and-api.md`. Generated from the task blocks below on 2026-10-03. The task blocks are the source of truth: when you add, drop or rename a task, update its row in the same change.

### Shared track

| ID    | When | Tier | Owner                                                                                                                                                                | Size         | Task                                                                            | Blocked by                                                                                                                                                                                                                       |
| ----- | ---- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SH-01 | P    | A    | Implementer 2 (application API)                                                                                                                                      | S, 0.5-2 h   | Decide: authentication mechanism (decision 7)                                   | decision 7: the authentication design is on hold by the user's decision of 2026-10-03, this task waits until the hold is lifted                                                                                                  |
| SH-02 | P    | A    | Implementer 1 (interface) with Implementer 2 (application API)                                                                                                       | S, 0.5-2 h   | Decide: browser to API path (decision 3)                                        | decision 7 (what the browser carries depends on it)                                                                                                                                                                              |
| SH-03 | P    | A    | Implementer 2 (application API) with the Go internal API owner (SH-07)                                                                                               | S, 1-2 h     | Decide: operator context to Go (decision 4)                                     | decision 7                                                                                                                                                                                                                       |
| SH-04 | P    | A    | Implementer 3 (agent runtime) with Implementer 5 (infrastructure role)                                                                                               | S, 0.5-2 h   | Decide: model provider, model and accounting rule (decision 6)                  | nothing                                                                                                                                                                                                                          |
| SH-05 | P    | A    | Researcher, document owner, and presenter, with Implementer 2 and the Go owners                                                                                      | S, 0.5-2 h   | Decide: read path for the run, usage, event and review views                    | nothing                                                                                                                                                                                                                          |
| SH-06 | P    | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 0.5-1 h   | Decide: single Go executor connection (decision 2 detail)                       | nothing                                                                                                                                                                                                                          |
| SH-07 | P    | A    | Implementer 2 (application API) for the contracts; Implementers 3, 4 and 5 for the Go modules                                                                        | S, 0.5-1 h   | Record one owner per contract and per Go module                                 | nothing                                                                                                                                                                                                                          |
| SH-08 | P    | A    | Implementer 5 (data and integration) coordinates; each implementer runs it                                                                                           | S, 0.5-1.5 h | Prepare every implementer machine                                               | nothing                                                                                                                                                                                                                          |
| SH-09 | P    | B    | Implementer 5 (data and integration), infrastructure role                                                                                                            | S, 2-4 h     | Validate the container path                                                     | a fix it needs before the coding window waits on decision 8                                                                                                                                                                      |
| SH-10 | M0   | A    | the whole team; Implementer 2 (application API) coordinates the contracts; the researcher records the outcomes                                                       | M, 3-10 h    | Freeze the task, contracts, tool arguments, policy fixture and schema ownership | contract owners, the names of the read operations also read path                                                                                                                                                                 |
| SH-11 | M0   | A    | Implementer 2 (application API)                                                                                                                                      | M, 2-7 h     | Land the frozen contracts in `packages/contracts`                               | contract owners                                                                                                                                                                                                                  |
| SH-12 | M0   | A    | Implementer 5 (data and integration) with each implementer                                                                                                           | S, 2.5-5 h   | Bring up the starter and confirm service connectivity                           | decision 8                                                                                                                                                                                                                       |
| SH-13 | M0   | A    | Implementer 5 (data and integration), infrastructure role, with Implementer 3 (agent runtime) and, for the API's own `.env` loading, Implementer 2 (application API) | S, 1-2 h     | Wire the model provider credential for Go only                                  | decision 6                                                                                                                                                                                                                       |
| SH-14 | M0   | A    | each recorded contract owner with Implementer 2 (application API); Implementer 5 (data and integration) for schema changes                                           | S, 1-4 h     | Run a quick shared review for every contract, schema and event-name change      | nothing                                                                                                                                                                                                                          |
| SH-37 | M0   | A    | Implementer 5 (data and integration), infrastructure role, with Implementer 3 (agent runtime)                                                                        | S, 0.5-1.5 h | Wire the non-secret Go variables, if the Go owners add any                      | decision 6 (the model name and pricing only)                                                                                                                                                                                     |
| SH-39 | M1   | A    | Implementer 2 (application API) with the Go internal API owner (SH-07)                                                                                               | S, 0.5-1 h   | Freeze and land the authenticated operator context contract                     | contract owners, decision 4, decision 7                                                                                                                                                                                          |
| SH-15 | M1   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-3 h     | Create the `app` schema and the app-schema migration                            | decision 7 for user records                                                                                                                                                                                                      |
| SH-16 | M1   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 2-5 h     | Write the first runtime-schema migration                                        | read path (the views only)                                                                                                                                                                                                       |
| SH-17 | M1   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-2 h     | Write the first demo-schema migration                                           | record versions                                                                                                                                                                                                                  |
| SH-18 | M1   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-3 h     | Seed the policy fixture and the minimal synthetic records                       | app-schema seed ownership                                                                                                                                                                                                        |
| SH-19 | M1   | A    | Implementer 5 (data and integration) with Implementer 2 (application API)                                                                                            | S, 1-2 h     | Seed the demonstration operator, organization and membership                    | decision 7, app-schema seed ownership                                                                                                                                                                                            |
| SH-20 | M1   | A    | Implementer 5 (data and integration), infrastructure role, with Implementer 2 and the Go internal API owner                                                          | S, 1-2 h     | Wire the authentication and operator-context secrets                            | decision 4, decision 7                                                                                                                                                                                                           |
| SH-21 | M1   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-3 h     | Provide a database-backed test command                                          | nothing                                                                                                                                                                                                                          |
| SH-22 | M1   | A    | Implementer 5 (data and integration) with Implementers 1 to 4                                                                                                        | M, 4-12 h    | Integrate the vertical path across the services                                 | decision 7, decision 3, decision 4, read path                                                                                                                                                                                    |
| SH-23 | M1   | A    | Implementer 5 (data and integration), integration role with infrastructure                                                                                           | S, 2-5 h     | Extend the smoke and leak checks                                                | smoke under login                                                                                                                                                                                                                |
| SH-24 | M2   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-3 h     | Write the demo-schema migration for reports and the simulated outbox            | record versions, stored report read (the report view only)                                                                                                                                                                       |
| SH-25 | M2   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 2-4 h     | Complete the synthetic fixture set                                              | decision 7 (the second organization's sign-in)                                                                                                                                                                                   |
| SH-26 | M2   | B    | Implementer 5 (data and integration), infrastructure role                                                                                                            | M, 2-9 h     | Create the service database roles and grants                                    | decision 2, read path, form options (the form-option read grant only), stored report read (the report read grant only), decision 4 and decision 7 (the Go read grant on the records behind operator and reviewer authority only) |
| SH-36 | M2   | A    | Implementer 5 (data and integration) with Implementers 3 and 4                                                                                                       | S, 1-3 h     | Observe the M2 exit across the services                                         | nothing                                                                                                                                                                                                                          |
| SH-27 | M3   | A    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-3 h     | Write the runtime-schema migration for approvals, reservations and usage        | dispatched attempts, read path and review payload read (the review view only)                                                                                                                                                    |
| SH-28 | M3   | A    | Implementer 5 (data and integration) with the researcher                                                                                                             | M, 3-6 h     | Capture evidence for the vertical-slice checks                                  | demonstration baseline                                                                                                                                                                                                           |
| SH-38 | M3   | B    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-2 h     | Write the app-schema migration for the revocation records                       | revocation reads, decision 2 (the grant only)                                                                                                                                                                                    |
| SH-29 | M4   | B    | Implementer 5 (data and integration)                                                                                                                                 | S, 1-4 h     | Write and document the reset command                                            | nothing                                                                                                                                                                                                                          |
| SH-30 | M4   | B    | Implementer 5 (data and integration)                                                                                                                                 | S, 2-6 h     | Write the deployment procedure for the demonstration environment                | nothing                                                                                                                                                                                                                          |
| SH-31 | M4   | B    | Implementer 5 (data and integration) with the researcher                                                                                                             | M, 3-8 h     | Record outcomes for the remaining critical checks                               | nothing                                                                                                                                                                                                                          |
| SH-32 | M5   | B    | Implementer 5 (data and integration) with the researcher                                                                                                             | S, 1-3 h     | Recapture the evidence from the final build                                     | nothing                                                                                                                                                                                                                          |
| SH-33 | M5   | B    | Researcher, document owner, and presenter, with all implementers                                                                                                     | M, 3-12 h    | Rehearse the storyboard end to end                                              | nothing                                                                                                                                                                                                                          |
| SH-34 | M6   | B    | the whole team; Implementer 5 (data and integration) coordinates                                                                                                     | S, 1-3 h     | Freeze features and fix critical faults                                         | nothing                                                                                                                                                                                                                          |
| SH-35 | M6   | B    | Implementer 5 (data and integration) with all implementers                                                                                                           | S, 1-3 h     | Assemble the technical handoff                                                  | nothing                                                                                                                                                                                                                          |

### Researcher track

| ID    | When | Tier | Owner                                     | Size     | Task                                                                            | Blocked by |
| ----- | ---- | ---- | ----------------------------------------- | -------- | ------------------------------------------------------------------------------- | ---------- |
| RS-01 | P    | A    | Researcher, document owner, and presenter | S, 1-3 h | Ask the organizers and sponsor mentors the report's questions, decision 8 first | nothing    |
| RS-02 | P    | C    | Researcher, document owner, and presenter | S, 0.5 h | Ask for cross-track rules only if another track is considered                   | nothing    |
| RS-03 | M0   | A    | Researcher, document owner, and presenter | S, 1-3 h | Write the official requirements sheet                                           | nothing    |
| RS-04 | M1   | A    | Researcher, document owner, and presenter | M, 3-6 h | Write the demo specification and storyboard                                     | nothing    |
| RS-05 | M1   | A    | Researcher, document owner, and presenter | S, 1-3 h | Keep the claim-to-proof list                                                    | nothing    |
| RS-06 | M1   | B    | Researcher, document owner, and presenter | S, 1-2 h | Keep the source register                                                        | nothing    |
| RS-07 | M2   | B    | Researcher, document owner, and presenter | S, 2-4 h | Compare existing controls from primary documentation                            | nothing    |
| RS-08 | M5   | A    | Researcher, document owner, and presenter | M, 3-6 h | Prepare the presentation from the final build                                   | nothing    |
| RS-09 | M6   | A    | Researcher, document owner, and presenter | S, 2-5 h | Complete the submission checklist and submit                                    | nothing    |

## Shared track

Owners are Implementer 5 (data and integration) and the researcher, plus whole-team sessions. Where
one exists, a size comes from the shared rows of the four estimates; the others are this roadmap's
estimates. Every size is a planning estimate, never a schedule.

### P (before the coding window)

- [ ] **SH-01 · Decide: authentication mechanism (decision 7)**
  - Owner: Implementer 2 (application API) · Tier: A · Size: S (estimate 0.5-2 h)
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
    lockfile change through integration. Owner in `docs/product/README.md`: nestjs (Implementer 2).
  - Done when: the outcome is recorded as decision 7 in `docs/product/README.md` by the researcher
    (document owner).
  - Tests: none (a decision).
  - Report: "Technical architecture and service ownership"; "Architecture and chart reading guide"
    (Interpreting the full architecture); "Functional requirements MVP boundary and deferred scope"
    (Deferred features)
  - Blocked by: `decision 7 in docs/product/README.md`: the authentication design is on hold by the
    user's decision of 2026-10-03; this task waits until the hold is lifted.

- [ ] **SH-02 · Decide: browser to API path (decision 3)**
  - Owner: Implementer 1 (interface) with Implementer 2 (application API) · Tier: A ·
    Size: S (estimate 0.5-2 h)
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
  - Done when: the outcome is recorded as decision 3 in `docs/product/README.md` by the researcher.
  - Tests: none (a decision).
  - Report: "Architecture and chart reading guide" ("The browser must never receive provider
    credentials, tool credentials, internal service secrets, or an unrestricted runtime command
    interface"); "Relative implementation milestones and critical dependencies" (authenticated
    polling before SSE)
  - Blocked by: `decision 7 in docs/product/README.md` (what the browser carries depends on it).

- [ ] **SH-03 · Decide: operator context to Go (decision 4)**
  - Owner: Implementer 2 (application API) with the Go internal API owner (SH-07) · Tier: A ·
    Size: S (estimate 1-2 h)
  - Depends on: SH-01 · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/internal/httpserver/middleware.go`,
    `apps/api/src/gateway-client/gateway-client.service.ts`
  - Work: The requirement is settled: Go verifies service identity and the authenticated operator
    context, authorizes each command against its organization and run, and checks reviewer authority
    itself. Options from `docs/product/README.md`: (1) extend the starter's service token with an
    operator context Go can verify; (2) replace the service token with another service-identity
    mechanism that carries the operator context. No proposal is recorded; the outcome also names the
    trusted records Go reads for reviewer authority. Owner in `docs/product/README.md`: nestjs with go.
  - Done when: the outcome is recorded as decision 4 in `docs/product/README.md` by the researcher.
  - Tests: none (a decision).
  - Report: "Technical architecture and service ownership" (Interfaces and repository strategy);
    "Threat model limits and unresolved design choices" ("The service token in the starter requires
    replacement or extension for authenticated operator context")
  - Blocked by: `decision 7 in docs/product/README.md`

- [ ] **SH-04 · Decide: model provider, model and accounting rule (decision 6)**
  - Owner: Implementer 3 (agent runtime) with Implementer 5 (infrastructure role) · Tier: A ·
    Size: S (estimate 0.5-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `.env.example`, `services/gateway/internal/config/config.go`
  - Work: Options: any one provider and model the owner proposes; no proposal is recorded. Settled
    constraints: one provider, called only by Go, which holds the credentials; one new variable in
    `.env.example` (name only), read by Go alone; a provider library beyond `net/http`, `slog` and pgx
    is a team decision and a `go.sum` change coordinated with integration. The outcome includes the
    documented accounting rule for estimated cost. Owner in `docs/product/README.md`: go (Implementer 3) with infrastructure.
  - Done when: the provider, the model and the accounting rule are recorded as decision 6 in
    `docs/product/README.md` by the researcher.
  - Tests: none (a decision).
  - Report: "Report purpose and design status" (one model provider); "Atomic allowances hard limits
    and estimated cost" ("The selected provider and model should have a documented accounting rule");
    "Relative implementation milestones and critical dependencies" (provider connection in hours 0-2)
  - Blocked by: nothing

- [ ] **SH-05 · Decide: read path for the run, usage, event and review views**
  - Owner: Researcher, document owner, and presenter, with Implementer 2 and the Go owners · Tier: A ·
    Size: S (estimate 0.5-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: Options: (1) NestJS reads sanitized runtime views through a read-only grant, created by the
    migrations (Figure 1, "Read authorized event view"; operations table, "Read authorized run and
    usage view"); (2) NestJS calls private Go endpoints for the run, usage, event and review reads
    ("NestJS would expose these operations and call corresponding private Go endpoints"); the
    gateway's 30 s write timeout ends any long-lived stream from Go. No proposal is recorded, and this
    roadmap does not resolve it. Owner: the document owner (session record of 2026-10-03), with nestjs
    and go.
  - Done when: the outcome is recorded in `docs/product/README.md` by the researcher.
  - Tests: none (a decision).
  - Report: "Architecture and chart reading guide" (Figure 1 and Figure 2); "Technical architecture
    and service ownership"; "Data ownership and the transition from starter to product"; "Illustrative
    passport and interface contracts" (Proposed browser and runtime operations)
  - Blocked by: nothing

- [ ] **SH-06 · Decide: single Go executor connection (decision 2 detail)**
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 0.5-1 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/internal/database/database.go`
  - Work: Options: (1) the report's recommendation, not adopted: "one Go executor connection with
    explicit permissions for the necessary runtime writes and limited demo report and outbox
    operations", so the effect, its completion record and its event share one transaction; (2)
    separate connections with different credentials, which "would not provide that atomicity
    automatically". The direction of decision 2 (explicit, narrow privileges per service; NestJS
    keeps separate application privileges) is settled. Owner in `docs/product/README.md`: integration
    (Implementer 5).
  - Done when: the outcome is recorded under decision 2 in `docs/product/README.md` by the researcher.
  - Tests: none (a decision).
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Data ownership and the
    transition from starter to product"
  - Blocked by: nothing

- [ ] **SH-07 · Record one owner per contract and per Go module**
  - Owner: Implementer 2 (application API) for the contracts; Implementers 3, 4 and 5 for the Go
    modules · Tier: A · Size: S (estimate 0.5-1 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/README.md`
  - Work: Agree one owner for each of the seven contracts and one owner for each planned Go module,
    including the parts the report's team table does not name (the internal API that verifies
    service identity and operator context, the runtime repository and events, data minimization, the
    labelled action replay, the Go DTO mirrors), the starter's existing packages (`cmd/gateway`,
    `internal/config`, `internal/logging`, `internal/database`, `internal/health`,
    `internal/httpserver`), the Go endpoints for X-25 and X-64 if `form options` and
    `stored report read` choose Go, and the placement of Implementer 5's transactional effects.
    Implementer 2 gives the contract owners and Implementers 3, 4 and 5 give the Go owners to the
    researcher, who records both in `docs/product/README.md` (Go owners by responsibility, without
    package names); each Go owner still records their package in `services/gateway/README.md` when
    it is created.
  - Done when: the contracts table in `docs/product/README.md` names an owner on every row, and
    `docs/product/README.md` names the agreed owner of every planned Go module, existing package and
    conditional Go endpoint listed in Work ("Record one owner per contract and per Go module").
  - Tests: none (a record).
  - Report: "Delivery scope and six person ownership"
  - Blocked by: nothing

- [ ] **SH-08 · Prepare every implementer machine**
  - Owner: Implementer 5 (data and integration) coordinates; each implementer runs it · Tier: A ·
    Size: S (estimate 0.5-1.5 h per person)
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
  - Owner: Implementer 5 (data and integration), infrastructure role · Tier: B ·
    Size: S (estimate 2-4 h)
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
  - Blocked by: nothing; a fix it needs before the coding window waits on
    `decision 8 in docs/product/README.md`.

### M0 (hours 0-2)

- [ ] **SH-10 · Freeze the task, contracts, tool arguments, policy fixture and schema ownership**
  - Owner: the whole team; Implementer 2 (application API) coordinates the contracts; the researcher
    records the outcomes · Tier: A · Size: M (estimate 3-10 h, person-hours summed over attendees)
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
    `docs/product/README.md` by the researcher.
  - Tests: none at the freeze; SH-11 runs the contract tests.
  - Report: "Relative implementation milestones and critical dependencies" (Proposed 24-hour
    implementation sequence, Hours 0-2; Critical path and sensible reductions); "Delivery scope and six
    person ownership"; "Illustrative passport and interface contracts"
  - Blocked by: `contract owners`; the names of the read operations also `read path`.

- [ ] **SH-11 · Land the frozen contracts in `packages/contracts`**
  - Owner: Implementer 2 (application API) · Tier: A · Size: M (estimate 2-7 h)
  - Depends on: SH-10 · Needs: X-01 · Provides: X-07, X-08, X-09, X-10, X-11, X-12, X-13
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
  - Owner: Implementer 5 (data and integration) with each implementer · Tier: A ·
    Size: S (estimate 2.5-5 h, one estimate's total for every machine and the demonstration
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
  - Owner: Implementer 5 (data and integration), infrastructure role, with Implementer 3 (agent
    runtime) and, for the API's own `.env` loading, Implementer 2 (application API) · Tier: A ·
    Size: S (estimate 1-2 h)
  - Depends on: SH-04 · Needs: X-01 · Provides: X-04
  - Paths: `.env.example`, `scripts/setup.mjs`, `scripts/dev.mjs`, `infra/compose.yaml`,
    `scripts/smoke.mjs`, `README.md`, `services/gateway/README.md`,
    `apps/api/src/config/app-config.module.ts` (Implementer 2), `scripts/with-env.mjs`,
    `docs/setup.md`
  - Work: In one change, add the variable Implementer 3 names to `.env.example` (name only), the
    gateway's Compose environment map only and the README table; keep it out of the web child and
    the API child in `scripts/dev.mjs` (today the API child receives every `.env` variable); add it
    to the smoke leak list. Setup appends it empty and never prints it; how Go reads it is Go-side
    work. The runner filter alone does not keep it out of the API on the host:
    `apps/api/src/config/app-config.module.ts` loads the root `.env` into `process.env` for every
    key not already set (the container image has no `.env`). Implementer 2 and the infrastructure
    owner record how the API stops holding it, for example by loading only the keys
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
  - Owner: each recorded contract owner with Implementer 2 (application API); Implementer 5 (data and
    integration) for schema changes · Tier: A · Size: S (estimate 1-4 h, person-hours over the window)
  - Depends on: SH-11 · Needs: X-15 · Provides: nothing
  - Paths: `packages/contracts`, `services/gateway/internal/health/dto.go`,
    `apps/api/src/database/migrations`, `docs/team-workflow.md`
  - Work: From M0 to M6, agree every change to a shared shape, table or event name with the affected
    owners before anyone builds on it, and land it through "Changing a shared contract", so the type,
    schema, fixtures, Go DTO and consumers merge together or in direct succession. The worker readiness
    change (X-32) goes through this review. So does `revocation reads`: by M3, Implementers 2, 4
    and 5 agree the app-schema revocation records and how Go reads them before dispatch and before
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
  - Owner: Implementer 5 (data and integration), infrastructure role, with Implementer 3 (agent
    runtime) · Tier: A · Size: S (estimate 0.5-1.5 h over M0 and M1, this roadmap's estimate)
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
  - Owner: Implementer 2 (application API) with the Go internal API owner (SH-07) · Tier: A ·
    Size: S (estimate 0.5-1 h, split from SH-11's estimate)
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
  - Blocked by: `contract owners`; `decision 4 in docs/product/README.md`;
    `decision 7 in docs/product/README.md`

- [ ] **SH-15 · Create the `app` schema and the app-schema migration**
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: SH-10 · Needs: X-17 · Provides: X-18
  - Paths: `apps/api/src/database/migrations`, `apps/api/src/database/typeorm-options.ts`,
    `db/migrations/README.md`, `docs/team-workflow.md`, `AGENTS.md`, `CLAUDE.md`,
    `.claude/agents/nestjs.md`, `.claude/agents/integration.md`
  - Work: Generate the app-schema migration from Implementer 2's entities, add
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
  - Blocked by: `decision 7 in docs/product/README.md` for user records.

- [ ] **SH-16 · Write the first runtime-schema migration**
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 2-5 h)
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
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 1-2 h)
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
  - Blocked by: `record versions`

- [ ] **SH-18 · Seed the policy fixture and the minimal synthetic records**
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 1-3 h)
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
  - Blocked by: `app-schema seed ownership`

- [ ] **SH-19 · Seed the demonstration operator, organization and membership**
  - Owner: Implementer 5 (data and integration) with Implementer 2 (application API) · Tier: A ·
    Size: S (estimate 1-2 h)
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
  - Owner: Implementer 5 (data and integration), infrastructure role, with Implementer 2 and the Go
    internal API owner · Tier: A · Size: S (estimate 1-2 h)
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
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 1-3 h)
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
  - Owner: Implementer 5 (data and integration) with Implementers 1 to 4 · Tier: A ·
    Size: M (estimate 4-12 h)
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
  - Blocked by: `decision 7 in docs/product/README.md`; `decision 3 in docs/product/README.md`;
    `decision 4 in docs/product/README.md`; `read path`

- [ ] **SH-23 · Extend the smoke and leak checks**
  - Owner: Implementer 5 (data and integration), integration role with infrastructure · Tier: A ·
    Size: S (estimate 2-5 h over M1 to M3)
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
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 1-3 h)
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
  - Blocked by: `record versions`; `stored report read` (the report view only)

- [ ] **SH-25 · Complete the synthetic fixture set**
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: SH-18, SH-24 · Needs: X-06 · Provides: X-34
  - Paths: `package.json`, `README.md`
  - Work: Extend the seed command with permitted invoices that carry a seeded discrepancy, the vendor's
    registered demonstration reporting address as the only allowed recipient, an invoice outside the
    task, a synthetic invoice note with the hostile instruction, a second organization's records for the
    organization-access check, and the small configured allowance beat 8 uses, all within the limits
    and content frozen in X-06. Unless SH-10 records the registered report template as Go code,
    also seed it where SH-10 stores it, with fields within the X-06 field rules. All data is
    synthetic and labelled.
  - Done when: one documented seed command yields fixtures that support the eight demo beats and the
    critical checks, and the document owner confirms the content matches X-06.
  - Tests: run the seed on an empty database and query the expected records (discrepancy, out-of-scope
    invoice, hostile note, second organization and, if it is a stored record, the registered
    report template).
  - Report: "Illustrative invoice scenario and future domain adaptations" (Scene 2); "Live
    demonstration storyboard and proof checks" (Proposed demo sequence); "Validation plan and evidence
    matrix" (critical check Organization access)
  - Blocked by: `decision 7 in docs/product/README.md` (the second organization's sign-in)

- [ ] **SH-26 · Create the service database roles and grants**
  - Owner: Implementer 5 (data and integration), infrastructure role · Tier: B ·
    Size: M (estimate 2-9 h)
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
  - Blocked by: `decision 2 in docs/product/README.md`; `read path`; `form options` (the form-option
    read grant only); `stored report read` (the report read grant only);
    `decision 4 in docs/product/README.md` and `decision 7 in docs/product/README.md` (the Go read
    grant on the records behind operator and reviewer authority only)

- [ ] **SH-36 · Observe the M2 exit across the services**
  - Owner: Implementer 5 (data and integration) with Implementers 3 and 4 · Tier: A ·
    Size: S (estimate 1-3 h, this roadmap's estimate)
  - Depends on: SH-25 · Needs: X-37, X-38, X-63 · Provides: nothing
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

### M3 (hours 10-14)

- [ ] **SH-27 · Write the runtime-schema migration for approvals, reservations and usage**
  - Owner: Implementer 5 (data and integration) · Tier: A · Size: S (estimate 1-3 h)
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
  - Owner: Implementer 5 (data and integration) with the researcher · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: RS-04, SH-21, SH-25 · Needs: X-13, X-16, X-31, X-36, X-37, X-38,
    X-43 (approval part), X-44, X-45, X-46, X-67 · Provides: nothing
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
  - Owner: Implementer 5 (data and integration) · Tier: B ·
    Size: S (estimate 1-2 h, this roadmap's estimate)
  - Depends on: SH-15, SH-26 · Needs: X-24, X-66 ·
    Provides: X-47 (part: the table and the Go read grant)
  - Paths: `apps/api/src/database/migrations`
  - Work: Generate the migration from Implementer 2's revocation entity (X-66) with
    `pnpm db:migration:generate <Name>`, review the SQL with Implementer 2 and apply it. The `app`
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
  - Owner: Implementer 5 (data and integration) · Tier: B · Size: S (estimate 1-4 h)
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
  - Owner: Implementer 5 (data and integration) · Tier: B · Size: S (estimate 2-6 h)
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
  - Blocked by: nothing

- [ ] **SH-31 · Record outcomes for the remaining critical checks**
  - Owner: Implementer 5 (data and integration) with the researcher · Tier: B · Size: M (estimate 3-8 h)
  - Depends on: SH-28 · Needs: X-24, X-50, X-51, X-52, X-53, X-54, X-55, X-56, X-57 ·
    Provides: nothing
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
  - Owner: Implementer 5 (data and integration) with the researcher · Tier: B · Size: S (estimate 1-3 h)
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
  - Owner: Researcher, document owner, and presenter, with all implementers · Tier: B ·
    Size: M (estimate 3-12 h, person-hours)
  - Depends on: RS-04, SH-29, SH-30 · Needs: X-16, X-36, X-43, X-48, X-49, X-58 · Provides: nothing
  - Paths: `docs/product`
  - Work: Run the eight beats on the final build in the demonstration environment, reset between runs,
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
  - Owner: the whole team; Implementer 5 (data and integration) coordinates · Tier: B ·
    Size: S (estimate 1-3 h)
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
  - Owner: Implementer 5 (data and integration) with all implementers · Tier: B ·
    Size: S (estimate 1-3 h)
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

The researcher writes no product code. The four estimates exclude the researcher's work, so these
sizes are this roadmap's estimates.

### P (before the coding window)

- [ ] **RS-01 · Ask the organizers and sponsor mentors the report's questions, decision 8 first**
  - Owner: Researcher, document owner, and presenter · Tier: A · Size: S (estimate 1-3 h)
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
  - Owner: Researcher, document owner, and presenter · Tier: C · Size: S (estimate 0.5 h)
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
  - Owner: Researcher, document owner, and presenter · Tier: A · Size: S (estimate 1-3 h)
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
  - Owner: Researcher, document owner, and presenter · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: RS-03, SH-10 · Needs: X-06 · Provides: X-16
  - Paths: `docs/product` (new file, named by the researcher)
  - Work: Specify the eight beats of "Proposed demo sequence" with the observable proof each needs, the
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
  - Owner: Researcher, document owner, and presenter · Tier: A · Size: S (estimate 1-3 h over the
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
  - Owner: Researcher, document owner, and presenter · Tier: B · Size: S (estimate 1-2 h)
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
  - Owner: Researcher, document owner, and presenter · Tier: B · Size: S (estimate 2-4 h)
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
  - Owner: Researcher, document owner, and presenter · Tier: A · Size: M (estimate 3-6 h)
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
  - Owner: Researcher, document owner, and presenter · Tier: A · Size: S (estimate 2-5 h)
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
those in `docs/product/README.md`, where the researcher records each outcome. "Settled through" names
what settles or carries the item; "Blocks" lists the IDs in this file that wait on it, and the side
files add theirs.

| Cite as                                | Item                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Status                                                                                             | Owner                                                             | Settled through                                                              | Blocks (this file)                                                                                                                       | Settle by                 |
| -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ------------------------- |
| `decision 1 in docs/product/README.md` | Schemas `app`, `runtime` and `demo` in one PostgreSQL instance                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Settled by the report                                                                              | integration with nestjs                                           | the report                                                                   | nothing                                                                                                                                  | settled                   |
| `decision 2 in docs/product/README.md` | Database roles; the single Go executor connection                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Direction settled; the executor connection is a report recommendation, not adopted                 | integration (Implementer 5)                                       | SH-06                                                                        | SH-26, SH-38; X-35, X-47, X-57                                                                                                           | P (M1 at the latest)      |
| `decision 3 in docs/product/README.md` | Browser to API path                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Open; a proposal is recorded, proposed, not decided                                                | frontend with nestjs                                              | SH-02                                                                        | SH-22; X-31, X-43, X-60                                                                                                                  | M0                        |
| `decision 4 in docs/product/README.md` | Operator context to Go                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Requirement settled, mechanism open                                                                | nestjs with go                                                    | SH-03                                                                        | SH-20, SH-22, SH-26 (Go authority-record read grant), SH-39; X-14, X-23, X-26, X-27                                                      | M0                        |
| `decision 5 in docs/product/README.md` | Background worker in Go: PostgreSQL jobs with leases, no broker                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Settled by the report; repository rule: graceful shutdown and the readiness check cover the worker | go (Implementer 3)                                                | the report                                                                   | nothing; how readiness covers the worker is `worker readiness`                                                                           | settled                   |
| `decision 6 in docs/product/README.md` | Model provider, model and accounting rule                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Credentials in Go settled; provider, model and accounting rule open                                | go (Implementer 3) with infrastructure                            | SH-04                                                                        | SH-13, SH-37 (the model name and pricing only); X-04, X-53                                                                               | P                         |
| `decision 7 in docs/product/README.md` | Authentication mechanism                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open; the design is on hold by the user's decision of 2026-10-03, and nothing is decided           | nestjs (Implementer 2)                                            | SH-01, which waits for the hold to be lifted                                 | SH-02, SH-03, SH-15, SH-19, SH-20, SH-22, SH-25, SH-26 (Go authority-record read grant), SH-39; X-14, X-17, X-22, X-23, X-31, X-34, X-43 | M0 (the M1 exit needs it) |
| `decision 8 in docs/product/README.md` | Reuse of pre-event work and its disclosure                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Open                                                                                               | researcher                                                        | RS-01                                                                        | SH-09 (fixes before the window), SH-12; X-01                                                                                             | P                         |
| `read path`                            | Whether NestJS reads sanitized runtime views or calls private Go endpoints for the run, usage, event and review reads                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Open; a report inconsistency                                                                       | document owner, with nestjs and go                                | SH-05                                                                        | SH-10 (read operation names), SH-16 (views), SH-22, SH-26, SH-27 (review view); X-29, X-30, X-35, X-41, X-60                             | M0                        |
| `contract owners`                      | One owner for each of the seven contracts                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Not recorded                                                                                       | nestjs (Implementer 2) gives them to the researcher               | SH-07                                                                        | SH-10, SH-11, SH-39; X-07 to X-14                                                                                                        | P                         |
| `Go package owners`                    | One owner per Go module, including the internal API, the runtime repository and events, data minimization, the replay and the Go DTO mirrors, which the report's team table does not name; the starter's existing packages; and the Go endpoints for X-25 and X-64 if their open items choose Go                                                                                                                                                                                                                                                                                                                           | Not recorded                                                                                       | go role (Implementers 3, 4 and 5)                                 | SH-07                                                                        | X-15, X-27, X-36, X-50, X-65                                                                                                             | P                         |
| `final result format`                  | "Final-output validation requires an output format and a data rule"; the report recommends "a structured status with authorized report references", proposed, not decided                                                                                                                                                                                                                                                                                                                                                                                                                                                  | Open                                                                                               | run state contract owner (not recorded) with Implementer 3        | SH-10 brings it to the document owner                                        | X-11, X-64                                                                                                                               | M0                        |
| `record versions`                      | "Record-version rechecks require versions and comparison semantics"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Open                                                                                               | action proposal contract owner (not recorded) with Implementer 5  | SH-10                                                                        | SH-17, SH-24; X-09, X-20, X-33                                                                                                           | M0                        |
| `exact reviewed material`              | "Freeze the payload, or bind its source records to versions and require a new proposal when they change"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open                                                                                               | Implementer 4 with the action proposal contract owner             | SH-10                                                                        | X-09, X-45                                                                                                                               | M0 (M3 at the latest)     |
| `dispatched attempts`                  | "Durable worker recovery requires identifiable dispatched attempts"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | Open                                                                                               | Implementer 3                                                     | the Go side (`docs/roadmap/go.md`); the tables follow in SH-27               | SH-27; X-39, X-54                                                                                                                        | M3                        |
| `revocation reads`                     | "Current revocation requires a single owner and reliable reads"; the report names NestJS as the owner of app-schema revocations and Go as the enforcer, and leaves the read undefined                                                                                                                                                                                                                                                                                                                                                                                                                                      | Open                                                                                               | Implementer 2 with Implementer 4, and Implementer 5 for the grant | SH-14                                                                        | SH-38; X-47                                                                                                                              | M3                        |
| `multiple-action responses`            | A model response with several actions "should be rejected or handled by an explicitly defined policy"                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Open                                                                                               | Implementer 3                                                     | the Go side (`docs/roadmap/go.md`)                                           | nothing in this file (Go-internal)                                                                                                       | M1                        |
| `model call retries`                   | Whether a failed model call is retried. Figure 5 goes from a failed call (Diagram 2: MOK -> MFAIL -> FAILED, "Record failure and known usage / Retain uncertain cost reservation") straight to "Run failed", while "Atomic allowances hard limits and estimated cost" says "Retries are separate attempts and consume allowance", model_call_limit reads "Illustrative hard dispatch count; retries consume allowance.", and "Risk register and scope controls" answers "Rate limits, malformed proposals, or intermittent calls interrupt the loop." with "Use bounded validation/retries and transparent failure states" | Open; a report inconsistency                                                                       | document owner, with Implementer 3                                | the document owner                                                           | X-46 (the retry part)                                                                                                                    | M0 (M1 at the latest)     |
| `canonical arguments`                  | "Canonicalization must be defined deliberately": the canonical representation of the supported argument types, the stored fields the action digest covers and how a change is detected; Go is "the authority for action canonicalization"                                                                                                                                                                                                                                                                                                                                                                                  | Open; no proposal is recorded                                                                      | Implementer 4                                                     | the Go side (`docs/roadmap/go.md`); X-09 carries it at the M0 freeze (SH-10) | X-09                                                                                                                                     | P (before the M0 freeze)  |
| `replay entry`                         | How the labelled adversarial action replay enters a run, so beat 6 completes "within the same run and allowance", and how it is marked in the stored action and its events; one option needs the conditional X-65                                                                                                                                                                                                                                                                                                                                                                                                          | Open; no proposal is recorded                                                                      | the replay owner recorded in SH-07 (not recorded)                 | the Go side (`docs/roadmap/go.md`)                                           | X-12 (replay label), X-36, X-65                                                                                                          | P (before the M0 freeze)  |
| `form options`                         | No listed operation supplies the task form's options (template, vendor, invoice set, destination, approval requirement, limits)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Open                                                                                               | document owner                                                    | SH-10                                                                        | SH-26 (form-option read grant); X-25                                                                                                     | M0                        |
| `passport in the run view`             | Whether "Read authorized run and usage view" carries the passport representation the summary shows                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Open                                                                                               | document owner, with the passport and run state contract owners   | SH-10                                                                        | X-08, X-29                                                                                                                               | M0                        |
| `review payload read`                  | No listed operation delivers the exact stored action to an authorized reviewer                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open                                                                                               | document owner                                                    | SH-10                                                                        | SH-27 (review view); X-41                                                                                                                | M0 (M3 at the latest)     |
| `stored report read`                   | No listed operation delivers the stored report and its registered template to the interface, which "renders its substantive content from the stored report and registered template" (part of the recommended final result, proposed, not decided); Figure 1 gives NestJS no read of `demo`                                                                                                                                                                                                                                                                                                                                 | Open                                                                                               | document owner                                                    | SH-10                                                                        | SH-24 (report view), SH-26 (report read grant); X-64                                                                                     | M0 (M2 at the latest)     |
| `demonstration baseline`               | Beat 2 needs the starting record versions, report count and outbox count; the report names no surface for them                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open; SH-28 uses documented read-only queries until the owner names another surface                | document owner                                                    | the document owner                                                           | SH-28                                                                                                                                    | M2                        |
| `app-schema seed ownership`            | Who writes the seed for the operator, memberships, task template, policy version and tool catalog                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Not assigned                                                                                       | integration with nestjs                                           | SH-10, before SH-18                                                          | SH-18, SH-19; X-21, X-22                                                                                                                 | M0                        |
| `command timeout budget`               | The web proxy allows 10 s, `GATEWAY_TIMEOUT_MS` up to 20 s and the gateway's write timeout is 30 s; no budget exists for commands                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Open                                                                                               | nestjs with frontend and go                                       | SH-10                                                                        | X-28                                                                                                                                     | M0                        |
| `smoke under login`                    | Health and diagnostics stay public; how smoke reaches product pages behind a login depends on decision 7                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open                                                                                               | integration with infrastructure                                   | the owners, after decision 7                                                 | SH-23                                                                                                                                    | M1                        |
| `worker readiness`                     | The readiness `checks` object is closed and holds only the database, so covering the worker is a shared contract change unless the team meets the rule another way                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Open                                                                                               | go with nestjs                                                    | SH-14                                                                        | X-32                                                                                                                                     | M1                        |
| `app-schema record list`               | The report's table lists revocations and no users; Figure 1's `app` node lists users, and its edge GATE -> APPDB reads current revocations from `app`                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Not open: both lists are partial; the `docs/product/README.md` schema table records the union      | document owner                                                    | `docs/product/README.md` (schema table)                                      | nothing                                                                                                                                  | settled                   |

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
  - "General natural-language policy compilation, comprehensive content-loss prevention, and automated
    classification of every possible leak."
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
5. **A critical check has its evidence.** Per the validation plan: "Maintain status, build identifier,
   fixture, observed outcome, and evidence reference for each check", and "For each acceptance check,
   record the build identifier, fixture, observed outcome, timestamp, and evidence location." For
   every denied action, "Compare business state and execution records before and after every denied
   action": record versions, report count, outbox count and execution records. "Happy-path screenshots
   alone are insufficient." "Keep failed or unverified checks visible."
