# Go side roadmap

**Status.** This file plans the Go side of Task Passport: the GO tasks. It is derived from the
project report, `docs/product/task-passport-project-report.docx` (version 1.2, "Official requirements and hybrid security controls", 3 October 2026; lines marked
"Report 1.2 change" amend a task and win over older text and "Report 1.1 change" lines), the architecture specification
`docs/product/project-architecture.md`, and the repository at commit `789bcd7`. Lines marked
"Report 1.1 change" amend the task they sit in; where they disagree with the older fields, they win. The task statuses below record implementation progress; unticked tasks and unrecorded sync
points remain open. Sizes are estimates, not a schedule. The
spine, `docs/roadmap/README.md`, is the contract for this file: its milestones, tiers, sync points
(X), shared tasks (SH), open decisions, task format and definition of done apply here unchanged.

## Who is on this side

One person, the Go implementer, holds every task in this file: the report's Implementer 3 (agent
runtime), Implementer 4 (enforcement) and the Go parts of Implementer 5 (the four tool adapters,
trusted source manifests, deterministic internal and vendor rendering, transactional effects), and
every module the report's team table does not name (the internal API that verifies service identity
and operator context, the runtime repository and events, data minimization, the labelled action
replay, the Go DTO mirrors, and the Go endpoints for X-25 and X-64 if `form options` and
`stored report read` choose Go). Report 1.2 adds the local-model connection and separate security purpose, timeouts, the
concurrency cap, timing instrumentation, the semantic-verdict boundary, detector configuration and
managed-pattern checks (GO-72 to GO-86). The lead helps when needed. Implementer 5's migrations, seeds,
database roles, reset and deployment work is on the shared track in the spine.

The report's role descriptions, from "Proposed team ownership" (report 1.1), name the areas:

- **Implementer 3 (agent runtime):** "Go worker, provider integration, bounded agent loop, model
  reservations, usage accounting, and cancellation checks."
- **Implementer 4 (enforcement):** "Go admission, immutable passport, argument checks, exact-action
  approvals, execution claims, and denial feedback", now with report provenance and the export
  checks.
- **Implementer 5 (data and integration), Go parts:** the four tool adapters (`read_invoice`,
  `read_vendor`, `create_report`, `queue_report`), trusted source manifests, deterministic internal
  and vendor rendering, transactional effects.

Inside one side the tasks are done by one person, so they run in sequence, not in parallel. Every
package belongs to the Go implementer and is recorded in `services/gateway/README.md` when it is
created (AGENTS.md, "Repository map and ownership").

## How to read this file

- **Read the full report first**, then `docs/product/project-architecture.md` and
  `docs/product/README.md`, then the spine (AGENTS.md, "Read
  the project report first").
- **Milestones** are the spine's: P before the coding window, then M0 to M6 for the report's
  relative windows. Each section quotes the report's team focus and exit condition for its window;
  the organizers' confirmed rules and deadline take precedence.
- **Area groups.** Inside a milestone the tasks are grouped by responsibility area: agent runtime
  (report role Implementer 3), enforcement (Implementer 4), tool adapters, provenance and rendering
  (Implementer 5), then modules the report's team table does not name. All are the Go implementer's.
  IDs follow milestone order and, inside a milestone, the area groups, not build order, so
  a task may depend on a higher number in the same milestone. A late addition takes the next free
  number and sits where its milestone and owner group put it, as GO-62 does after GO-18 (spine,
  "IDs"). A moved task keeps its number too: GO-60 moved from M5 to M4 to sit with its consumer,
  WEB-21.
- **Task format** is the spine's. "Depends on" lists GO tasks and SH tasks; "Needs" and "Provides"
  list the spine's X IDs. The Next.js + NestJS side is referred to only through X and SH IDs.
  "Blocked by" cites the strings of the spine's "Open decisions and blockers" verbatim.
  `Provides: X-15 (part: ...)` marks a task that delivers part of a sync point; this side has
  delivered it when every task marked "(part)" for it is done (spine, "Sync points").
- **Paths** are existing repository paths. New Go code is "a new package, named at M0 by the
  Go implementer"; the architecture's package tree (`internal/api`, `admission`, `passport`, `worker`,
  `model`, `policy`, `provenance`, `approvals`, `budget`, `executor`, `tools`, `audit`, `repository`) is
  the proposal, with its open points in `Go package layout`. The Go implementer records the package in `services/gateway/README.md` when it is created and
  supplies its "Product modules" row in `docs/architecture.md` to integration (AGENTS.md,
  "Implementation workflow", step 6).
- **Tests.** `pnpm --filter gateway run test` runs the unit tests. Database-backed tests run
  through the command SH-21 provides (X-24), written here as "the X-24 command" because it has no
  name yet; concurrency tests also run with `go test -race ./...` from `services/gateway`. Every
  task also runs the go checks of the spine's "Definition of done" (`format:check`, `lint`,
  `typecheck`, `test`, `build`) and `pnpm verify`, and quotes the results. A provider test double
  used in a test is labelled as one.
- **Conditional tasks.** A task that delivers one outcome of an open item (the read path,
  `form options`, `stored report read`, `review payload read`) says so in Work and becomes
  "Dropped: reason" when the other outcome is chosen. The records the other outcome would read are
  written by unconditional tasks.
- **The authentication hold.** Decision 7 is on hold, and decisions 3 and 4 wait on it (spine,
  "Milestones"). On this side it blocks GO-13 (its "Done when" needs the operator's verified
  authority), GO-14, GO-21, GO-41, GO-44, GO-57, GO-60 and GO-62 (the X-14 mirror), and through
  X-27 every internal route. Through "Depends on", GO-11, GO-15, GO-16, GO-24 to GO-30, GO-32 to
  GO-34, GO-36, GO-37, GO-40, GO-42, GO-43, GO-45 to GO-56, GO-58, GO-59 and GO-61 cannot be ticked
  while the hold stands either, and through "Needs" (X-21 and X-35 wait on the hold, spine
  "Milestones") neither can GO-17 and GO-38, nor GO-31 and GO-35, which depend on GO-17. The hold
  stops none of GO-01 to GO-10, GO-12, GO-18 to GO-20, GO-22, GO-23 and GO-39. The blocked tasks
  can still be built and tested at the package level: tests pass the verified context as an
  explicit input, no production code path creates one, and no route accepts a command without
  GO-21's verification.
- **Open items this file settles.** The spine's open-items table assigns
  `multiple-action responses`, `dispatched attempts`, `canonical arguments` and `replay entry` to
  the Go side; GO-01, GO-02, GO-04 and GO-05 decide them. GO-03 is the other Go-side decide task.
- **Sizes** start from the Go rows of the four estimates of 2026-10-03 that the spine describes
  ("Sides and people", "Effort split"; the estimates are not in the repository), split where this
  file splits a row and adjusted for the decide, conditional and evidence tasks the estimates have
  no row for. The class follows the midpoint: S up to 4 h, M above 4 h up to 10 h, L above 10 h up
  to 20 h. Estimates, not a schedule; "Coverage" gives the totals.

## Task overview

Every task in this file, one row each, in milestone order. 86 tasks: 65 Tier A, 19 Tier B, 2 Tier C. 7 name decision 7 (the authentication hold) under "Blocked by". Generated from the task blocks below on 2026-10-03. The task blocks are the source of truth: when you add, drop or rename a task, update its row in the same change.

| ID    | When | Tier | Owner                                                                                      | Size         | Task                                                                                        | Blocked by                                                                                                                                                    |
| ----- | ---- | ---- | ------------------------------------------------------------------------------------------ | ------------ | ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GO-01 | P    | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 0.5-1 h   | Decide: handling of model responses that propose several actions                            | nothing                                                                                                                                                       |
| GO-02 | P    | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 0.5-1.5 h | Decide: how dispatched attempts are identified for worker recovery                          | nothing                                                                                                                                                       |
| GO-03 | P    | A    | Go implementer (report role: Implementer 3, agent runtime) with the lead (infrastructure)  | S, 0.5-1.5 h | Decide: Go input to decision 6 (provider client, reservation sizing, usage)                 | nothing                                                                                                                                                       |
| GO-04 | P    | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-2 h     | Decide: canonical argument representation and action digest                                 | nothing                                                                                                                                                       |
| GO-05 | P    | A    | Go implementer (a module the report's team table does not name)                            | S, 0.5-1 h   | Decide: how the labelled action replay enters a run and is marked                           | nothing                                                                                                                                                       |
| GO-06 | M0   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-4 h     | Bring up the model provider connection from Go                                              | None (completed GO-06 scope)                                                                                                                                  |
| GO-07 | M0   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 1-2 h     | Record the tool-result contract and each tool's idempotency rule                            | `canonical arguments`                                                                                                                                         |
| GO-08 | M1   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | M, 3-6 h     | Claim durable jobs with a lease in one worker                                               | nothing                                                                                                                                                       |
| GO-09 | M1   | B    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 1-3 h     | Cover the worker in graceful shutdown and readiness                                         | `worker readiness`                                                                                                                                            |
| GO-10 | M1   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | M, 4-8 h     | Run a live model step through the model gateway                                             | `decision 6 in docs/product/README.md`; `multiple-action responses`; `model call retries` (the failure handling only)                                         |
| GO-11 | M1   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | M, 3-6 h     | Run the bounded agent loop for permitted actions                                            | nothing                                                                                                                                                       |
| GO-80 | M1   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-4 h     | Instrument performance telemetry                                                            | nothing                                                                                                                                                       |
| GO-12 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Canonicalize tool arguments and compute the action digest                                   | `canonical arguments`                                                                                                                                         |
| GO-13 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | M, 4-8 h     | Admit a start-run request and issue the passport, run and job together                      | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md` (which `app` records carry the operator's authority); `passport report fields` |
| GO-14 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-2 h     | Serve `POST /internal/runs`                                                                 | `command timeout budget`; `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                      |
| GO-15 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | M, 3-6 h     | Store each proposed action and decide allow, deny or approval required                      | nothing                                                                                                                                                       |
| GO-16 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Execute an allowed action through its registered adapter                                    | nothing                                                                                                                                                       |
| GO-74 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 3-5 h     | Apply deterministic content controls to designated fields                                   | `redaction rules`                                                                                                                                             |
| GO-75 | M1   | A    | Go implementer (report role: Implementer 4, enforcement, and Implementer 3, agent runtime) | M, 4-8 h     | Build the semantic security evaluator behind the metered model gateway                      | `classifier prompt and verdict schema`; `decision 6 in docs/product/README.md`                                                                                |
| GO-76 | M1   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Inspect tool results before they enter the agent context                                    | nothing                                                                                                                                                       |
| GO-17 | M1   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 2-4 h     | Build `read_invoice`                                                                        | nothing                                                                                                                                                       |
| GO-18 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 1.5-3 h   | Mirror the frozen contracts in Go DTOs                                                      | nothing                                                                                                                                                       |
| GO-62 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 0.5-1 h   | Mirror the operator context contract in Go                                                  | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                |
| GO-19 | M1   | A    | Go implementer (a module the report's team table does not name)                            | M, 4-8 h     | Build the runtime repository with guarded state transitions                                 | nothing                                                                                                                                                       |
| GO-20 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 2-4 h     | Build the Go PostgreSQL test harness on the X-24 command                                    | nothing                                                                                                                                                       |
| GO-21 | M1   | A    | Go implementer (a module the report's team table does not name)                            | M, 3-6 h     | Verify service identity and operator context on every internal command                      | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                |
| GO-22 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 2-4 h     | Write safe decision events with every state change                                          | nothing                                                                                                                                                       |
| GO-23 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 2-3 h     | Enforce tool-result field allowlists and minimize the model context                         | nothing                                                                                                                                                       |
| GO-24 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 2-3 h     | Serve the run, usage and event reads, if the read path chooses Go endpoints                 | `read path`; `passport in the run view`                                                                                                                       |
| GO-25 | M1   | A    | Go implementer (a module the report's team table does not name)                            | S, 1-2 h     | Serve the task form options, if `form options` chooses a Go endpoint                        | `form options`                                                                                                                                                |
| GO-26 | M2   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-3 h     | Validate the narrow final result and complete the run                                       | `final result format`                                                                                                                                         |
| GO-27 | M2   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 1-2 h     | Prove a permitted reconciliation on the Go side                                             | nothing                                                                                                                                                       |
| GO-71 | M2   | B    | Go implementer (report role: Implementer 3, agent runtime, and Implementer 5, provenance)  | S, 2-4 h     | Record the conservative context manifest, if `internal report rendering` admits model prose | `internal report rendering`; `report storage`                                                                                                                 |
| GO-28 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | M, 3-6 h     | Check resource relationships and destinations at the gate                                   | nothing                                                                                                                                                       |
| GO-29 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Return structured denial feedback and stop at the correction limit                          | nothing                                                                                                                                                       |
| GO-64 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Deny a restricted report export at the gate before approval                                 | nothing                                                                                                                                                       |
| GO-66 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-2 h     | Prove the denied internal export on the Go side                                             | nothing                                                                                                                                                       |
| GO-72 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Check the active catalog revision before every evaluation and dispatch                      | nothing                                                                                                                                                       |
| GO-77 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-3 h     | Apply the semantic risk check to otherwise permitted action proposals                       | nothing                                                                                                                                                       |
| GO-78 | M2   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Match the signature-feed rules                                                              | `feed grammar and trust`                                                                                                                                      |
| GO-31 | M2   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 1-3 h     | Build `read_vendor`                                                                         | nothing                                                                                                                                                       |
| GO-32 | M2   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 2-4 h     | Build `create_report`                                                                       | `record versions`; `internal report rendering` (internal body only)                                                                                           |
| GO-33 | M2   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 3-5 h     | Build `queue_report`                                                                        | `record versions`                                                                                                                                             |
| GO-34 | M2   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 2-4 h     | Commit each demo effect with its execution record and event in one transaction              | `decision 2 in docs/product/README.md`                                                                                                                        |
| GO-35 | M2   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 1-3 h     | Replace protected values with opaque references resolved inside adapters                    | nothing                                                                                                                                                       |
| GO-63 | M2   | A    | Go implementer (report role: Implementer 4, enforcement, and Implementer 5, provenance)    | M, 4-8 h     | Build the report provenance module                                                          | `report storage`; `source classification storage`                                                                                                             |
| GO-65 | M2   | A    | Go implementer (report role: Implementer 5, rendering)                                     | S, 3-5 h     | Render the vendor report from the approved projection                                       | `vendor projection fields`                                                                                                                                    |
| GO-67 | M2   | A    | Go implementer (report role: Implementer 5, rendering)                                     | S, 1-2 h     | Prove the approved external projection on the Go side                                       | nothing                                                                                                                                                       |
| GO-36 | M2   | A    | Go implementer (a module the report's team table does not name)                            | S, 2-3 h     | Replay a prohibited proposal through the real gate, labelled                                | `replay entry`                                                                                                                                                |
| GO-37 | M2   | A    | Go implementer (a module the report's team table does not name)                            | S, 1-2 h     | Serve the stored report, if `stored report read` chooses a Go endpoint                      | `stored report read`; `final result format`; `report storage`                                                                                                 |
| GO-38 | M2   | B    | Go implementer (a module the report's team table does not name)                            | S, 1-2 h     | Connect with the Go database roles from X-35                                                | `decision 2 in docs/product/README.md`                                                                                                                        |
| GO-39 | M3   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | M, 5-8 h     | Reserve model allowance before every dispatch and settle it afterwards                      | `decision 6 in docs/product/README.md`; `dispatched attempts`                                                                                                 |
| GO-40 | M3   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-4 h     | Release the lease during a review wait and resume the original action                       | nothing                                                                                                                                                       |
| GO-41 | M3   | B    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-3 h     | Persist cancellation through the internal cancel command                                    | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                |
| GO-42 | M3   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 1-2 h     | Prove the limit-triggered stop                                                              | `model call retries` (the retry part only)                                                                                                                    |
| GO-79 | M3   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-4 h     | Enforce the model allowlist, request timeout and local concurrency cap                      | nothing                                                                                                                                                       |
| GO-43 | M3   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 3-5 h     | Freeze the exact action for review                                                          | `record versions`                                                                                                                                             |
| GO-44 | M3   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | M, 3-6 h     | Accept the approval decision through the internal command                                   | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                |
| GO-45 | M3   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | M, 3-6 h     | Recheck before execution and claim the attempt in one transaction                           | `record versions`                                                                                                                                             |
| GO-46 | M3   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-2 h     | Prove approval integrity                                                                    | nothing                                                                                                                                                       |
| GO-69 | M3   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-2 h     | Prove that approval cannot override the export restriction                                  | nothing                                                                                                                                                       |
| GO-73 | M3   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Validate and acknowledge a candidate catalog revision                                       | `catalog activation protocol`                                                                                                                                 |
| GO-47 | M3   | A    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 1-2 h     | Prove the legitimate task on the Go side                                                    | nothing                                                                                                                                                       |
| GO-48 | M3   | A    | Go implementer (a module the report's team table does not name)                            | S, 1-2 h     | Serve the exact review payload, if the read path chooses Go endpoints                       | `read path`; `review payload read`                                                                                                                            |
| GO-49 | M4   | B    | Go implementer (report role: Implementer 3, agent runtime)                                 | M, 3-6 h     | Recover expired leases without replaying dispatched work                                    | `dispatched attempts`                                                                                                                                         |
| GO-50 | M4   | B    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 1-3 h     | Prove budget concurrency                                                                    | nothing                                                                                                                                                       |
| GO-51 | M4   | B    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 1-2 h     | Prove that cancellation and expiry stop dispatch, also after a review wait                  | nothing                                                                                                                                                       |
| GO-81 | M4   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-4 h     | Build the repeatable performance benchmark                                                  | `measurement method`                                                                                                                                          |
| GO-86 | M4   | A    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 2-3 h     | Prove policy reload, the model allowlist and local model resources                          | nothing                                                                                                                                                       |
| GO-52 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-3 h     | Check current revocations before dispatch and before execution                              | `revocation reads`; `decision 2 in docs/product/README.md`                                                                                                    |
| GO-53 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Handle known failures, safe retries and unknown outcomes                                    | nothing                                                                                                                                                       |
| GO-54 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-3 h     | Prove approval replay under concurrent requests                                             | nothing                                                                                                                                                       |
| GO-55 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-3 h     | Prove the database execution transaction with fault injection                               | `decision 2 in docs/product/README.md`                                                                                                                        |
| GO-30 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-3 h     | Prove the resource and destination boundaries                                               | nothing                                                                                                                                                       |
| GO-68 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-3 h     | Prove label and rename tampering and missing lineage                                        | `rename operation` (rename part)                                                                                                                              |
| GO-70 | M4   | B    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-3 h     | Prove source or template policy changes after review                                        | nothing                                                                                                                                                       |
| GO-84 | M4   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 2-4 h     | Prove the live semantic cases, the false-negative boundary and guard failure                | nothing                                                                                                                                                       |
| GO-85 | M4   | A    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-3 h     | Prove redaction and the attack feed update                                                  | nothing                                                                                                                                                       |
| GO-56 | M4   | B    | Go implementer (report role: Implementer 5, tool adapters)                                 | S, 1-3 h     | Inspect model context, events and output channels for protected fields                      | nothing                                                                                                                                                       |
| GO-57 | M4   | B    | Go implementer (a module the report's team table does not name)                            | S, 1-3 h     | Prove organization access at the internal boundary                                          | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                                                |
| GO-60 | M4   | C    | Go implementer (a module the report's team table does not name)                            | S, 1-3 h     | Optional: stream events from Go, if the read path chooses Go endpoints                      | `read path`; `decision 3 in docs/product/README.md`                                                                                                           |
| GO-82 | M4   | A    | Go implementer (a module the report's team table does not name)                            | S, 2-4 h     | Serve the control evaluation adapter contract                                               | nothing                                                                                                                                                       |
| GO-83 | M4   | A    | Go implementer (a module the report's team table does not name)                            | S, 1-3 h     | Serve the security decision records, if the read path chooses Go endpoints                  | `read path`                                                                                                                                                   |
| GO-58 | M5   | B    | Go implementer (report role: Implementer 3, agent runtime)                                 | S, 1-3 h     | Make every Go stop, failure and denial state readable                                       | nothing                                                                                                                                                       |
| GO-59 | M5   | C    | Go implementer (report role: Implementer 4, enforcement)                                   | S, 1-2 h     | Optional: rehearse an unknown outcome                                                       | nothing                                                                                                                                                       |
| GO-61 | M6   | B    | Go implementer (all report roles on this side)                                             | S, 1-2 h     | Supply the Go technical handoff text                                                        | nothing                                                                                                                                                       |

## Constraints for the Go side

1. **Go is the only execution authority and holds the model provider and tool credentials.** Every
   governed model request and tool effect goes through Go; NestJS forwards commands and never
   performs agent effects. Source: AGENTS.md guardrail 4; report "Technical architecture and service
   ownership" ("it must not possess independent model or tool credentials that let it bypass the
   gateway") and "The enforcement loop and data minimization" ("Model and tool credentials remain
   with Go").
2. **The Go modules run in one Go service.** They are packages of the gateway, not separate
   deployments. Source: report "Architecture and chart reading guide" ("The modules inside the Go
   group run in one Go service") and "Delivery scope and six person ownership".
3. **`net/http`, `slog` and the pgx pool,** unless the team decides otherwise; a new Go module is a
   `go.mod` and `go.sum` change coordinated with integration. Source: `.claude/agents/go.md`;
   AGENTS.md, "Single owner for shared assets".
4. **No migration framework.** Every `runtime` and `demo` table comes from the shared migration
   tasks (SH-16, SH-17, SH-24, SH-27) through TypeORM in `apps/api`; Go consumes the schema. Source:
   AGENTS.md; `.claude/agents/go.md`; report "Design decision record" ("One migration history; Go
   consumes the resulting schema.").
5. **Schema-qualified SQL.** Neither TypeORM nor the Go pool sets `search_path`, so every Go query
   names its schema (`app`, `runtime` or `demo`). Source: spine, "Schema ownership";
   `services/gateway/internal/database/database.go` sets no runtime parameters.
6. **Fail closed.** "A missing policy, an unavailable authorization dependency, a transport error or
   a configuration error is never an allow decision." Source: AGENTS.md guardrail 6; report "The
   enforcement loop and data minimization" and "Illustrative passport and interface contracts" ("A
   transport or configuration error is not an allow decision").
7. **Identity and organization come from verified context,** never from model output, tool results
   or identifiers the browser supplies: "An invoice ID or action ID is a reference, not
   authorization." Source: AGENTS.md guardrail 5; report "Trusted authority and passport invariants"
   and "Data ownership and the transition from starter to product".
8. **One action per model step.** A response with several actions follows GO-01's outcome and is
   never handled by silently executing a subset. Source: report "Functional requirements MVP
   boundary and deferred scope", "The enforcement loop and data minimization" and "Design decision
   record".
9. **Durable jobs in PostgreSQL with leases, no broker, one worker process.** Source: decision 5 in
   `docs/product/README.md`; report "Durable state idempotency audit and uncertain outcomes" and
   "Relative implementation milestones and critical dependencies".
10. **Short transactions; no database lock across a model or tool request.** "Commit the
    reservation first, perform the request, then settle it in a later transaction." Source: report
    "Atomic allowances hard limits and estimated cost".
11. **Nothing at startup runs migrations, creates tables or loads seed data;** the gateway keeps
    starting while PostgreSQL is down. Source: AGENTS.md guardrail 3; `services/gateway/README.md`.
12. **Truthful labels and illustrative limits.** The simulated outbox, the replay, provider test
    doubles and estimated cost are labelled. The report's example limits are illustrative values,
    not requirements; the limit values come from the policy fixture frozen in X-06. Source:
    AGENTS.md guardrail 9.
13. **Secrets stay secret.** The provider credential and any operator-context secret are
    `logging.Secret` values, never logged or returned, and payloads stay out of logs. Source:
    AGENTS.md working rules 8 and 9; `.claude/agents/go.md`.
14. **Packages.** A package is added when its first real code lands, with no empty directories, and
    is recorded in `services/gateway/README.md`; the Go implementer owns all of them. Source: AGENTS.md
    guardrail 1 and "Repository map and ownership"; `.claude/agents/go.md`.
15. **Go 1.27 or newer on each Go developer's machine** (SH-08). On the preparation machine Go
    1.27.1 is installed at `/usr/local/go`: on 2026-10-03 `/usr/local/go/bin/go version` printed
    `go version go1.27.1 darwin/arm64`, while `go` was not on the PATH of the shell that ran it,
    although `/etc/paths.d/go` lists `/usr/local/go/bin`. Source: spine SH-08;
    `services/gateway/go.mod`; `services/gateway/scripts/go.mjs`.

## Starting point in the repository

- Packages under `services/gateway`: `cmd/gateway` and `internal/config`, `internal/logging`,
  `internal/database`, `internal/health`, `internal/httpserver`. No product package exists.
- Three GET routes (`/health/live`, `/health/ready`, `/internal/ping`); no route reads a request
  body; `RequireServiceToken` in `services/gateway/internal/httpserver/middleware.go` guards the
  ping only.
- One pgx pool of at most 10 connections, one database user, lazy connect;
  `services/gateway/cmd/gateway/main.go` hands it only to readiness.
- Server write timeout 30 s; on SIGINT or SIGTERM the server drains for up to 8 s
  (`shutdownTimeout`), then the pool closes.
- The readiness `checks` object is closed and holds only `database`
  (`packages/contracts/schemas/readiness.schema.json`).
- `services/gateway/internal/health/dto_test.go` decodes fixtures by explicit case, so a fixture
  without a case is never checked on the Go side.
- No worker, no outbound client, no database-backed test, no migration and no seed data.

## P: before the coding window

- **Team focus.** The report gives no window for this work. Spine: decisions, environment setup,
  container validation and organizer questions.
- **Exit condition.** The report gives no exit condition for P. The spine's roadmap condition (not
  a quote) applies; for this side it means the Go decide tasks below are recorded or carried into
  M0 as open, and every Go developer has quoted a `pnpm verify` result (SH-08).
- **Sync points needed by the end (spine):** X-01, which RS-01 provides and GO-06 needs at the start
  of M0.
- Decide tasks are decisions, not code. Code written before the coding window waits on decision 8
  (spine, "Before the coding window (P)").

### Agent runtime (report role: Implementer 3)

- [x] **GO-01 · Decide: handling of model responses that propose several actions**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 0.5-1 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: Options: (1) reject the whole response as a denied proposal with a stable reason code and
    bounded correction feedback that counts toward the limits; (2) an explicitly defined policy that
    stores every proposed action and states which one is evaluated and what happens to the others.
    Outcome agreed with the user on 2026-10-03: option (1), recorded under "Go runtime decisions"
    in `docs/product/README.md`. No adapter executes; safe rejection feedback is bounded by the
    correction limit, and exhausted corrections stop the run. The proposed reason code
    `multiple_actions_not_supported` waits for the X-13 freeze. Provider constraints (GO-03)
    supplement the runtime rejection. "The worker should not silently execute an arbitrary
    subset". Owner per
    the spine's open-items table (`multiple-action responses`): the Go implementer;
    `docs/product/README.md` records no numbered decision for it.
  - Done when: the outcome is recorded in `docs/product/README.md` by the document owner (document
    owner), by M1 at the latest.
  - Tests: none (a decision).
  - Status: decision recorded and statically verified. On 2026-10-03, after installing the
    pinned dependencies and selecting Node.js 24.18.0, `pnpm verify` passed all six steps
    (6 passed, 0 failed, 0 skipped); `git diff --check` passed. The earlier pnpm/Node environment
    blocker is resolved. Runtime behavior is not implemented. The checkbox remains open pending
    the decision-record review and completion requirements of this roadmap.
  - Completed (2026-10-03): the outcome is recorded in `docs/product/README.md` ("GO-01: multiple-
    action model responses"). Implemented on go/f3: `agent.Stepper` (GO-10, 19d710e) rejects a
    response with several tool calls as `multiple_actions_not_supported` and never runs a subset
    (`TestSeveralToolCallsRejectTheWholeResponse`); the loop (GO-11, faac792) stops the run with
    that reason and stores no action (`TestSeveralActionsInOneResponseStopTheRun`). Bounded
    correction instead of the stop comes with the GO-29 wiring.
  - Report: "The enforcement loop and data minimization" ("Unsupported multiple-action responses
    should be rejected or handled by an explicitly defined policy"); "Design decision record" (One
    action per model step)
  - Blocked by: nothing

- [x] **GO-02 · Decide: how dispatched attempts are identified for worker recovery**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 0.5-1.5 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: Decide what marks a model request and a tool execution as dispatched, so a worker that
    takes over an expired lease can tell what already happened: "Lease expiry indicates that
    ownership needs recovery; it does not prove a previously dispatched operation failed." Options:
    (1) a durable attempt record per dispatch, committed before the request is sent and bound to the
    stable action identifier for tool executions, with a recovery rule per case (a model attempt
    without settled usage keeps its reservation; a local demo effect whose completion record is
    absent did not commit, if effect and completion share one transaction as SH-06 settles; an
    attempt with no recorded outcome otherwise is an unknown outcome); (2) another mechanism the
    owner proposes. Outcome agreed with the user on 2026-10-03: option (1), recorded under
    "GO-02: durable attempts and worker recovery" in `docs/product/README.md`. Pre-dispatch records
    prove intent only; unresolved model reservations remain held, successful actions are not
    replayed, and unknown tool outcomes pause for attention. Local no-effect recovery depends on
    SH-06's shared transaction and establishing that the former worker can no longer commit.
    The constraints the outcome needs reach SH-27 through
    the shared review in SH-14. Owner per the spine's open-items table (`dispatched attempts`):
    the Go implementer; `docs/product/README.md` records no numbered decision for it.
  - Done when: the outcome is recorded in `docs/product/README.md` by the document owner, by M3 at the
    latest and before SH-27 writes the tables.
  - Tests: none (a decision).
  - Status: decision recorded and statically verified. On 2026-10-03, after installing the
    pinned dependencies and selecting Node.js 24.18.0, `pnpm verify` passed all six steps
    (6 passed, 0 failed, 0 skipped); `git diff --check` passed. The earlier pnpm/Node environment
    blocker is resolved. Runtime behavior is not implemented. The checkbox remains open pending
    the decision-record review and completion requirements of this roadmap.
  - Completed (2026-10-03): the outcome is recorded in `docs/product/README.md` ("GO-02: durable
    attempts and worker recovery"). Implemented for model calls on go/f3: `budget.CallLog` commits
    the `runtime.model_calls` pre-dispatch record before every model dispatch and records its
    outcome once; its id is the ledger call id (19d710e,
    `TestStepRecordsTheDispatchAndSettlesUsage`, `TestCallLogRejectsASecondOutcome`). Tool attempts
    are written by w3's executor in `runtime.execution_attempts`. Recovery behaviour is GO-49.
  - Report: "Threat model limits and unresolved design choices" ("Durable worker recovery requires
    identifiable dispatched attempts"); "Durable state idempotency audit and uncertain outcomes"
  - Blocked by: nothing

- [ ] **GO-03 · Decide: Go input to decision 6 (provider client, reservation sizing, usage)**
  - **Report 1.2 change:** Decision 6 input now targets a local model ("Choose a local model that runs on the actual machine"), one provider serving agent and security purposes; record the hardware fit, and use reported tokens, call counts and request duration as the accounting basis.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) with the lead (infrastructure) · Tier: A · Size: S (estimate 0.5-1.5 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`, `services/gateway/go.mod`,
    `services/gateway/internal/config/config.go`
  - Work: Bring to SH-04 the Go facts the provider and model choice needs: (1) whether a
    hand-written `net/http` client is enough or a provider library is needed, which is a team
    decision and a `go.mod` and `go.sum` change coordinated with integration; (2) how a reservation
    is sized before dispatch, "based on the configured model, estimated input and the permitted
    output ceiling"; (3) which usage the provider reports and what counts as missing usage; (4) how
    estimated cost is represented, "an explicitly defined decimal or minor-unit representation" and
    never floating-point equality; (5) whether the provider can be held to one tool call or a final
    answer per response, which GO-01 relies on. Options for (1): the hand-written client or a
    library the owner names; no proposal is recorded. Owner in `docs/product/README.md` (decision
    6): go (the Go implementer) with the lead (infrastructure).
  - Done when: these points are recorded with decision 6 in `docs/product/README.md` by the
    document owner, as part of SH-04's outcome.
  - Tests: none (a decision).
  - Status: the user adopted report 1.2’s primary local-model path on 2026-10-03, replacing
    the earlier OpenAI selection. The user selected Ollama on a separate M1 Pro MacBook with
    16 GB RAM; `qwen3.5:4b` is a provisional candidate and may change after testing.
    A Go client and accounting proposal is recorded under GO-03 in the product README.
    The final model freeze, measured hardware fit, endpoint, reservation strategy and SH-04 adoption
    remain open. The local Go connectivity diagnostic passed on M2/8 GiB; this does not freeze
    the model/accounting decision or verify runtime governance.
  - Progress (2026-10-03): the Go client and the accounting rule are implemented (GO-06) and a live
    agent step on `qwen3.5:4b` with `think: false` passed on the developer M2/8 GiB machine (GO-10,
    19d710e). Still open: the model and hardware freeze on the presentation machine (SH-04), its
    endpoint and measured fit.
  - Report: "Atomic allowances hard limits and estimated cost" ("The selected provider and model
    should have a documented accounting rule"); "Report purpose and design status" (one model
    provider)
  - Blocked by: nothing

### Enforcement (report role: Implementer 4)

- [x] **GO-04 · Decide: canonical argument representation and action digest**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: Define how Go represents the "supported argument types, rejecting ambiguous or unsupported
    values", which stored fields the digest covers (the report's review record: "the tool, canonical
    arguments, recipient, affected resources, relevant versions, exact outbound content, passport
    reference, policy version and expiry"), and how a change is detected. Options: (1) one
    canonical encoding for every supported argument type, defined in Go, with a digest over the
    encoded action; (2) a canonical form per tool's typed arguments, with a digest over each.
    Outcome agreed with the user on 2026-10-03: typed canonical encoding per tool and SHA-256 over
    the complete versioned canonical action, recorded under "GO-04: canonical arguments and action
    digest" in `docs/product/README.md`. Input field order and insignificant JSON whitespace do
    not affect the digest; strict decoding rejects unknown and duplicate fields. Content is
    preserved, and lists keep their order unless X-09 explicitly defines a set. Either option
    rejects inputs that have more than one representation, uses
    no floating-point values and treats the digest as change detection only: "hashing a request
    does not authenticate its author or make its contents authorized". The outcome feeds X-09 at
    the M0 freeze (SH-10). Owner per the spine's open-items table (`canonical arguments`):
    the Go implementer, because the argument checks and exact-action approvals depend on it; Go is "the
    authority for action canonicalization" (`docs/product/README.md`), which records no numbered
    decision for it.
  - Done when: the outcome is recorded in `docs/product/README.md` by the document owner before the M0
    freeze, so X-09 can carry it.
  - Tests: none (a decision).
  - Status: decision recorded and statically verified. On 2026-10-03, after installing the
    pinned dependencies and selecting Node.js 24.18.0, `pnpm verify` passed all six steps
    (6 passed, 0 failed, 0 skipped); `git diff --check` passed. The earlier pnpm/Node environment
    blocker is resolved. Runtime behavior is not implemented. The checkbox remains open pending
    the decision-record review and completion requirements of this roadmap.
  - Report: "Exact action approval versioning and execution rechecks" ("Canonicalization must be
    defined deliberately"); "Technical architecture and service ownership" ("Go remains the
    authority for action canonicalization and execution"); "Terminology for developers and
    presenters" (Canonical arguments)
  - Blocked by: nothing
  - Completed (2026-10-03): the recorded decision is implemented by GO-12 in `internal/policy` (commits 2b7cc5c, 582eafd): typed strict decoding per tool, one canonical compact encoding, SHA-256 over the versioned canonical action. Evidence is in GO-12.

### Modules the report's team table does not name (Go implementer)

- [x] **GO-05 · Decide: how the labelled action replay enters a run and is marked**
  - **Report 1.1 change:** A recorded proposal enters a run for the export test (beat 5) and for the supporting rehearsal of an out-of-scope read (beat 9); the beat 6 sentence is dropped. Report field: "Supporting rehearsals hostile instructions limits and uncertainty".
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 0.5-1 h)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `docs/product/README.md`
  - Work: The report asks for "a clearly labeled adversarial action replay that submits a
    prohibited proposal to the same validation and execution path" and says "never present a
    scripted proposal as a model-generated action". Decide how a replayed proposal enters a run, so
    that beat 6 can complete "within the same run and allowance", and how it is marked in the stored
    action and its events (the safe event contract X-12 carries "a label for replayed proposals").
    Options: (1) a Go-side command or labelled runtime test that substitutes one stored prohibited
    proposal for the next model step of a named run; (2) an operation the interface triggers
    through NestJS, which needs the conditional sync point X-65 (the replay trigger through NestJS)
    and a facade operation on the other side. The user chose a labelled Go runtime scenario test
    (option 1) on 2026-10-03 and owns replay through the SH-07 Go ownership update. Owner per the spine's
    open-items table (`replay entry`): the Go implementer;
    `docs/product/README.md` records no numbered decision for it.
  - Done when: the outcome is recorded in `docs/product/README.md` by the document owner before the M0
    freeze, so X-12 carries the replay label.
  - Tests: none (a decision).
  - Status: option (1), a labelled Go runtime scenario test, was chosen with the user on 2026-10-03
    and recorded under "GO-05: replay entry and labels" in `docs/product/README.md`. The user is
    the replay owner and sole Go implementer; SH-07 remains open for shared-track staffing.
    No replay runtime code is implemented.
  - Report: "Live demonstration storyboard and proof checks" (Reliable demonstrations without
    invented behavior); "Illustrative invoice scenario and future domain adaptations" (Scene 2 a
    hostile instruction in a business document)
  - Blocked by: nothing
  - Completed (2026-10-03): the recorded decision (option 1, a labelled Go runtime scenario) is implemented by GO-36: the label lives on the stored action (X-09 `replaySource`, `runtime.actions.replay_source`) and in every event (X-12 `maskedSummary.replaySource`); evidence in GO-36.

## M0: hours 0-2

- **Team focus (report 1.2).** "Confirm start/deadline and reuse guidance. Freeze task/adapter/verdict contracts, policy schema, shared and security budgets, two templates, projection and migration ownership. Choose a local model that runs on the actual machine."
- **Exit condition (report 1.2).** "Services connect; policy imports successfully; agent and guard requests can be made within recorded limits; the initial test command and fixtures exist."
- **Report 1.2 sync points (spine).** X-78, X-80, X-81, X-84, X-89 (initial).
- **Sync points needed by the end (spine):** X-03 to X-06. SH-11 lands X-07 to X-13 after the
  freeze; they are needed by M1.
- The Go implementer takes part in the freeze (SH-10): they bring the canonical form (GO-04), the Go
  checks behind the four tools' typed arguments and the names of the internal operations they
  provide. SH-12 brings up the starter on every machine.

### Agent runtime (report role: Implementer 3)

- [x] **GO-06 · Bring up the model provider connection from Go**
  - **Report 1.2 change:** Bring up the local model connection (for example Ollama) from X-84; by the M0 exit an agent request and a security-purpose request can be made within recorded limits.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: SH-04, GO-03 · Needs: X-01, X-04 · Provides: nothing
  - Paths: `services/gateway/internal/config/config.go`,
    `services/gateway/internal/config/config_test.go`,
    `services/gateway/internal/logging/logging.go`, `services/gateway/cmd/gateway/main.go`,
    `services/gateway/README.md`; a new package for the model gateway, named at M0 by the Go implementer
  - Work: On its owner's machine, once SH-12 has run there, read the credential SH-13 wires as a
    `logging.Secret` and build the only outbound client to the decision 6 provider, with bounded
    timeouts. A missing credential never leads to a dispatch; the owner records in
    `services/gateway/README.md` whether the gateway then refuses to start or starts and fails
    every dispatch closed with a recorded reason. If the model name or pricing become environment
    variables, each goes through the lead (infrastructure) in one change (`.env.example`, the
    gateway's Compose map, the README tables), as SH-13 does for the credential (SH-37, only if
    any are added). Outbound HTTPS from the gateway image stays unverified until the container
    path runs (X-02).
  - Done when: from a developer machine with the credential in `.env`, one request through the Go
    client reaches the decision 6 provider and model and returns a response, and the credential
    appears in no log line or error text (report, hours 0-2: "Bring up the starter and provider
    connection").
  - Tests: unit tests against a labelled provider test double: the credential is sent only in the
    provider's authentication header; a non-success status, a malformed body and a timeout each
    become a failure, never a model output; neither the credential nor a request body appears in
    log or error text. `pnpm --filter gateway run test`; the live request run by hand, with its
    result quoted and the credential not printed.
  - Completed (2026-10-03): bounded native Ollama transport, local preflight, trusted central
    accounting settings, durable PostgreSQL shared reservations, unknown usage retention,
    one-time late reconciliation and full overrun pause are implemented. The user adopted
    reservation = JSON UTF-8 input bytes + 1024 template tokens + capped output; agent 512,
    security 256, shared initial total 20000, think false and stream false. Values are editable
    through the existing policy/catalog import. The catalog branch was merged as an intentional
    dependency; the only merge conflict preserved both architecture module rows.
    Four explicit qwen3.5:4b estimator fixtures passed. A live catalog-backed budget diagnostic
    returned measured usage and correct refunds for both purposes; final used 62, reserved 0.
    Exact commands, fixture outcomes and hardware limits are in the gateway README.
    This completes the user's expanded GO-06 accounting acceptance. Worker/admission wiring,
    call-count/concurrency enforcement, full catalog activation, semantic detection and the
    presentation-machine check remain their own later tasks; there is no startup dispatch.
    Verification: `pnpm verify` passed 6/6; `pnpm test:db` passed both sides with no skipped
    database tests; `go -C services/gateway test -race ./... -count=1 -timeout=60s` passed with
    PostgreSQL enabled; `pnpm smoke` passed 21/21 in host mode.
  - Report: "Relative implementation milestones and critical dependencies" (Proposed 24-hour
    implementation sequence, Hours 0-2); "Technical architecture and service ownership";
    "Architecture and chart reading guide" (Figure 1)
  - Blocked by: nothing for this completed developer-machine/accounting scope

### Tool adapters, provenance and rendering (report role: Implementer 5)

- [x] **GO-07 · Record the tool-result contract and each tool's idempotency rule**
  - **Report 1.1 change:** Record the `create_report` arguments ("Scoped source references and registered template identifier"); `read_invoice` may return the internal note where expressly allowed, with its restriction: "Readable data may have stricter export rules than invoice fields approved for the vendor."
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 1-2 h, person-hours summed)
  - Depends on: SH-10, GO-04 · Needs: X-06 · Provides: nothing
  - Paths: `services/gateway/README.md`
  - Work: Agreed in the M0 freeze session and Go-internal, so it has no sync point (spine,
    "Contracts to freeze first"): for each of the four tools, with the typed arguments agreed in
    SH-10, the fields it returns to the worker within the X-06 field rules, the opaque references
    that stand for protected values, and its idempotency and retry rule (which failures are
    known-safe to retry under the same action identifier, and which outcomes count as unknown).
    "The runtime developer depends on the action schema and tool-result contract."
  - Done when: each of the four tools has a recorded field allowlist, its opaque references and a
    defined idempotency rule in `services/gateway/README.md`, recorded by the Go implementer
    ("Each tool returns an explicit field allowlist; protected values remain
    opaque references"; "Each action needs a stable identifier and a defined idempotency rule").
  - Tests: none at the record; GO-17, GO-23, GO-31 to GO-33 and GO-53 test it.
  - Status: a proposed tool-result field allowlist and idempotency/retry rules are recorded in
    `services/gateway/README.md`. This task remains open until SH-10 freezes typed arguments and
    X-06 field rules, including which protected fields may appear in reviewed outbound content.
  - Completed (2026-10-03): W2 lane, branch go/w2, f2cd7a9. The tool-result contract is recorded in
    `services/gateway/README.md` (Tool results and idempotency): per tool the X-09 arguments, the
    model-facing allowlist, the protected values (the note readable but internal_only; the address
    only as the opaque reference `recipient:<run_id>:<vendor_id>`) and the idempotency and retry
    rule. Documentation; GO-17 to GO-35 implement and test it.
  - Report: "Relative implementation milestones and critical dependencies" (Critical path and
    sensible reductions); "Illustrative passport and interface contracts" (Narrow final result and
    context boundary); "Durable state idempotency audit and uncertain outcomes"
  - Blocked by: `canonical arguments`

## M1: hours 2-6

- **Team focus (report 1.2).** "Build task form, authenticated facade, admission, local model gateway, one governed read, deterministic content handling, live semantic tool-result check and events."
- **Exit condition (report 1.2).** "A real task executes one allowed read; a hostile tool-result fixture is blocked before agent context; both model purposes appear in usage and latency records."
- **Report 1.2 sync points (spine).** X-79, X-85, X-86.
- **Sync points needed by the end (spine):** X-07 to X-32. This side provides X-15, X-27, X-28 and
  X-32, plus X-29 and X-30 if the read path chooses Go endpoints and X-25 if `form options` chooses
  a Go endpoint.
- **First integrated deliverable placed here (spine).** Implementer 3: "A live model proposes a
  typed tool action within a recorded allowance." Report 1.2 replaces it with "A live agent call and live
  semantic check both reserve allowance and record independent purpose and latency.", placed at M3
  (GO-75 with GO-39).
- While decision 7 is on hold the M1 exit, X-27 and X-28 cannot be reached (spine, "Milestones");
  see "The authentication hold" above for what proceeds.

### Agent runtime (report role: Implementer 3)

- [x] **GO-08 · Claim durable jobs with a lease in one worker**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: GO-19, GO-20 · Needs: X-19 · Provides: nothing
  - Paths: `services/gateway/cmd/gateway/main.go`, `services/gateway/internal/database/database.go`,
    `services/gateway/README.md`; a new package for the worker, named at M0 by the Go implementer
  - Work: Start one worker inside the gateway process (decision 5: PostgreSQL jobs with leases, no
    broker, one worker process for the prototype). It claims a job with a lease in a short
    transaction, renews the lease while it works and persists progress through the runtime
    repository. The worker shares the pool's 10 connections with the HTTP handlers and holds no
    connection or lock across a model or tool request. If worker settings become environment
    variables, they go through the lead (infrastructure) in one change (SH-37);
    constants need none.
  - Done when: a job stored by admission is claimed by exactly one worker and processed under a
    live lease (Figure 4: "Go worker claims job with lease"; MVP requirement Durable execution:
    "Persist jobs, continuations, approvals, and execution state").
  - Tests: database-backed, also with `-race`: two workers racing for one job yield one owner; a
    job under a live lease cannot be claimed; a lease past expiry can be claimed again; a claimed
    job and its progress survive closing the pool and opening a new one; no claim transaction stays
    open while a stubbed model call runs. The X-24 command.
  - Completed (2026-10-03): `internal/worker` (95d180c on go/f3, on main since f14b591). One-
    statement `FOR UPDATE SKIP LOCKED` claim that sets status, a fresh per-claim lease token and
    expiry together on the database clock; renew, finish and release succeed only for the current
    token on a live lease (`ErrLeaseLost` otherwise); renewal at a third of the lease cancels the
    handler on failure; no transaction or lock outlives a store call. Tests: concurrent claims yield
    one owner, a live lease cannot be claimed, an expired lease is claimed again and fences the old
    claim, a claim survives a new pool, no row lock or acquired connection during the handler.
    Checks: `pnpm --filter gateway run format:check`, `lint`, `typecheck`, `test`, `build` exit 0;
    `pnpm test:db gateway` on PostgreSQL 18 "167 passed, 0 failed, 0 skipped"; `go test -race ./...`
    with PostgreSQL all packages ok; `pnpm verify` 6 passed. The worker is started by the gateway
    process only with the GO-11 wiring.
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Relative implementation
    milestones and critical dependencies" (Critical path and sensible reductions, one worker
    process); "Functional requirements MVP boundary and deferred scope" (Durable execution)
  - Blocked by: nothing

- [x] **GO-09 · Cover the worker in graceful shutdown and readiness**
  - **Report 1.2 change:** Readiness also reports a missing valid catalog as not ready (GO-72, X-82), through a shared contract change.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-08 · Needs: nothing · Provides: X-32
  - Paths: `services/gateway/cmd/gateway/main.go`, `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/shutdown_test.go`,
    `services/gateway/internal/health/health.go`, `services/gateway/internal/health/dto.go`,
    `services/gateway/internal/health/dto_test.go`,
    `packages/contracts/schemas/readiness.schema.json` (changed by nestjs through SH-14),
    `services/gateway/README.md`
  - Work: On SIGINT or SIGTERM the worker stops claiming, ends its current step at a safe point and
    stops before `pool.Close()`, inside the 8 s shutdown budget that is kept below the container
    stop grace period. Readiness reports the worker as the `worker readiness` outcome says. The
    readiness `checks` object is closed and holds only `database`, so a new field is a shared
    contract change through SH-14: schema, fixtures, Go DTO, the API's gateway client, smoke and the
    diagnostics page together. Tier B: the shutdown part serves the MVP requirement Durable
    execution, as GO-49 does at Tier B, and readiness is decision 5's repository rule, which the
    report does not tier; the Next.js + NestJS side's part of the same contract change is Tier B as
    well. The tier sets the order of cuts only: SH-23 still waits for X-32, because the rule binds
    whatever the tier (spine, "Tiers").
  - Done when: decision 5's repository rule holds ("the worker must be covered by graceful shutdown
    and by the readiness check"), and a stop during a step leaves the job recoverable with nothing
    it committed lost (MVP requirement Durable execution).
  - Tests: a shutdown test in the pattern of `shutdown_test.go`: a worker mid-step stops within the
    budget and the pool closes after it; a job interrupted by the shutdown can be claimed again
    after its lease expires; readiness answers `unavailable` while the worker loop is not running;
    the readiness fixtures decode strictly. `pnpm --filter gateway run test`.
  - Completed (2026-10-03): `worker.Service` (456d802) stops claiming on `Stop`, lets the current
    step finish until the drain deadline and cancels the handler after it; an interrupted job keeps
    its lease and is claimed again after expiry. `cmd/gateway` now starts it with the production
    chain, reports it in `/health/ready` (option B of `worker readiness`, no contract change: 503
    `unavailable` with the true database check while the loop is not running) and stops it in
    parallel with the HTTP drain before `pool.Close()`. Observed: the built gateway on PostgreSQL 18
    as `task_passport_gateway` answered `/health/ready` 200 with the worker running and logged
    `shutdown requested` then `http server stopped` on SIGTERM; with `MODEL_NAME` empty it started,
    logged `model not configured; every model call fails closed` and answered 200. Checks: see
    GO-11's completion line, same commit.
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Functional requirements MVP
    boundary and deferred scope" (Durable execution)
  - Blocked by: `worker readiness`

- [x] **GO-10 · Run a live model step through the model gateway**
  - **Report 1.2 change:** Each call carries its trusted metered purpose and records usage and latency per purpose (M1 exit); GO-75 adds the security purpose and GO-80 the timing records.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: M (estimate 4-8 h)
  - Depends on: GO-01, GO-06, GO-19, GO-22, GO-23 · Needs: X-04, X-06, X-09, X-11 · Provides: nothing
  - Paths: `services/gateway/internal/config/config.go`, `services/gateway/README.md`; the model
    gateway package from GO-06
  - Work: Each model step sends the context GO-23 builds to the decision 6 provider with the
    permitted output ceiling, records the dispatch and the reported usage against the passport's
    call and token limits, and reads the response as either a final result or one proposed action,
    applying GO-01 to a response with several. The model is offered the four registered tools with
    their X-09 arguments only, never "unrestricted SQL, shell execution, arbitrary HTTP access". A
    failed call records the failure and its known usage; what follows is the `model call retries`
    outcome, and neither option is decided: (a) the run fails, as Figure 5 shows ("Record failure
    and known usage", then "Run failed"); (b) a bounded number of retries, each a separate dispatch
    that reserves through GO-39 and counts against the passport's model call limit ("retries
    consume allowance"), and the run fails when they are used up. Model output gets bounded
    validation: a malformed proposal is stored and denied at the gate. Atomic reservations come
    with GO-39; until then every dispatch and its usage is recorded, so none is unaccounted.
  - Done when: Implementer 3's first integrated deliverable is observed: "A live model proposes a
    typed tool action within a recorded allowance."
  - Tests: against a labelled provider test double: one proposed action becomes one proposal handed
    to the gate; a response with several follows GO-01 and never executes a subset; an unparseable
    or unsupported response becomes a recorded failure or denial, never an action; a provider error
    records the failure with its known usage and fails the run at once under outcome (a), or after
    the bounded retries under outcome (b), each retry recorded as its own dispatch; the request
    holds nothing outside the GO-23 context; the recorded call and token counts match the
    dispatches made. `pnpm --filter gateway run test`; one live run with the decision 6 model
    through the X-24 command, quoted.
  - Completed (2026-10-03): `internal/agent` `Stepper.Step` and `budget.CallLog` (19d710e on go/f3).
    The model must be in the passport's allowed models; the pre-dispatch record commits first; the
    request is the fixed instruction plus the minimized context with only the four X-09 tools (a
    drift test checks the parameters against `action-proposal.schema.json`); one tool call becomes
    one proposal for the gate, several are rejected whole (GO-01), text is a final answer; a failed
    call fails the run with no retry (`model call retries` default). Live: the opt-in
    `TestLiveModelProposesATypedAction` passed four times on the developer M2/8 GiB machine (Ollama
    0.35.1, `qwen3.5:4b` ID `2a654d98e6fb`, `think: false`), each a typed `read_invoice` with
    `invoice_id: "invoice_A01"`, 647 input and 30 output tokens, ledger settled to 677 of 20,000
    with nothing reserved. Checks: gateway `format:check`, `lint`, `typecheck`, `test`, `build` exit
    0; `pnpm test:db gateway` "399 passed, 0 failed, 0 skipped"; `go test -race ./...` with
    PostgreSQL 17 packages ok; `pnpm verify` 6 passed. Call-count limits and per-purpose sub-budgets
    are GO-39/GO-79.
  - Report: "The enforcement loop and data minimization"; "Relative implementation milestones and
    critical dependencies" (Hours 2-6, live model call); "Delivery scope and six person ownership"
    (Proposed team ownership); "Risk register and scope controls" (Provider instability or
    unsuitable output)
  - Blocked by: `decision 6 in docs/product/README.md`; `multiple-action responses`; `model call retries` (the failure handling only)

- [x] **GO-11 · Run the bounded agent loop for permitted actions**
  - **Report 1.2 change:** Each step also checks the active catalog revision (GO-72, from M2) and the applicable guards.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: GO-08, GO-10, GO-15, GO-16 · Needs: X-06, X-11 · Provides: nothing
  - Paths: the worker package from GO-08
  - Work: Before every model request, including the first after a claim and the one after each tool
    step, check that the run is active, unexpired and within its step and time limits; otherwise
    stop the run and record the reason. Hand each proposed action to the gate (GO-15) and an allowed
    one to the executor (GO-16), and give the minimized result to the next model step. A denied
    action executes nothing: the loop takes the blocked action and its reason code from the gate
    and, when no correction is available, which is every case until GO-29 is done, stops the run
    with its reason ("If no permitted alternative remained, the run would stop with an
    explanation"). This is the blocked-action path: GO-40, GO-44 and GO-45 send a rejected or
    expired approval and a failed recheck to it, and GO-29 adds bounded correction feedback while
    corrections remain. Progress is committed at each step, so a restarted worker continues from
    the last committed step. Corrections, the final result, approvals and reservations join the
    loop in GO-29, GO-26, GO-40 and GO-39.
  - Done when: with a live model, "Go executes a permitted tool" inside a run whose every model
    request was preceded by the run check ("Before requesting another model response, it checks
    whether the run remains active, unexpired and within its limits").
  - Tests: database-backed with a labelled provider test double: a run past its expiry, past its
    step limit or no longer active sends no model request and records its stop reason; a denied
    proposal reaches no adapter and, with no correction remaining, stops the run with its recorded
    reason; a permitted read runs and its minimized result reaches the next model request; the
    steps and their order can be reconstructed from the stored records. The X-24 command;
    `pnpm --filter gateway run test`.
  - Completed (2026-10-03): `agent.Loop` with `runtime.context_entries`, c1's tool-result
    inspection, GO-29 corrections, the active-catalog narrowing (GO-72) and GO-80 telemetry, built
    once by `agent.NewProductionChain` and run by the gateway's worker. Live done-when observed with
    `TestLiveProductionChainExecutesAPermittedTool` (opt-in, developer M2/8 GiB, Ollama 0.35.1,
    `qwen3.5:4b`, the repository's policy and feed activated through `catalogtest`): the model read
    invoice A01 (note passed inspection) and A02, created a `vendor_reconciliation_v1` report and
    proposed `queue_report`, which stopped at `awaiting_approval`; 4 agent calls each preceded by
    the run check (4 policy lookups), 5 security calls, 6,954 tokens. Other runs on this memory-
    constrained machine answered without a tool or paused with `outcome_unknown` after a 20-second
    request timeout, as designed. Checks: gateway `format:check`, `lint`, `typecheck`, `test`, `build` exit 0; `pnpm verify` 6 passed, 0 failed, 0 skipped; `go test -p 1 -count=1 ./...` against the test database with `TEST_DATABASE_REQUIRED=1`: all 21 packages ok. Parallel `pnpm test:db` on the shared, memory-constrained machine failed 6 and then 5 different database tests in other packages at the 3-second connect timeout (api 16 passed); the serial run passes them all.
    The run used the chain directly; the HTTP admission path is 3c's GO-14.
  - Report: "The enforcement loop and data minimization"; "Atomic allowances hard limits and
    estimated cost" (Cancellation and time limits); "Architecture and chart reading guide"
    (Figure 4); "Users operating model and proposed user journeys" (Journey 3 recover cancel or
    investigate)
  - Blocked by: nothing

- [x] **GO-80 · Instrument performance telemetry**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-10, GO-19 · Needs: X-79, X-85 · Provides: X-95 (part: instrumentation)
  - Paths: the model gateway and worker packages
  - Work: Measure monotonic durations separately for policy lookup, deterministic controls, semantic evaluation, provider request, approval waiting and local commit, plus total handling latency, queue depth, concurrency and errors, per model purpose. Keep untrusted confidential input out of the timing records. Observed durations stay distinct from cost estimates.
  - Done when: each agent and security call and each gate decision has its timing record, readable for the summary and export.
  - Tests: unit tests with a fake clock; a database-backed test through the X-24 command.
  - Completed (2026-10-03): `agent.Telemetry` on go/f3 writes `runtime.timing_records` per step
    (policy lookup, agent provider call with its `model_calls` id, gate decision, executor commit,
    step total) and, for each tool-result inspection, the deterministic and semantic controls, each
    security call's provider time and one `runtime.control_assessments` row per control decision
    (semantic rows with verdict source and the security call id), committed with the step's context
    entries. No inspected text is stored. Test
    `TestTelemetryRecordsPhasesAndAssessmentsWithoutInspectedText` passes on PostgreSQL. Checks: gateway `format:check`, `lint`, `typecheck`, `test`, `build` exit 0; `pnpm test:db` gateway "612 passed, 0 failed, 0 skipped", api "16 passed"; `go test -race ./...` with PostgreSQL 20 packages ok; `pnpm verify` 6 passed. Not covered here: `approval_wait` (GO-40), the concurrency slot (GO-79); queue
    depth is read from `runtime.jobs` by the summary.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Performance telemetry and measurement); "Validation plan and evidence matrix" (Performance measurement method)
  - Blocked by: nothing

### Enforcement (report role: Implementer 4)

- [x] **GO-12 · Canonicalize tool arguments and compute the action digest**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-04, GO-18 · Needs: X-09 · Provides: nothing
  - Paths: a new package for enforcement, named at M0 by the Go implementer
  - Work: Implement GO-04's outcome: the canonical form of each tool's typed arguments, the rejection
    of ambiguous or unsupported values, and the digest that identifies a stored action and detects a
    change. The digest never stands in for the stored record or the policy checks.
  - Done when: the same intended arguments always give the same canonical form and digest, and a
    changed material argument always gives a different digest ("A digest can identify the stored
    action and detect changes").
  - Tests: table tests over the four tools: equivalent inputs give identical canonical bytes and
    digests; each ambiguous or unsupported input GO-04 names is rejected; changing any material
    field, one at a time, changes the digest. `pnpm --filter gateway run test`.
  - Report: "Exact action approval versioning and execution rechecks"; "Terminology for developers
    and presenters" (Canonical arguments)
  - Blocked by: `canonical arguments`
  - Completed (2026-10-03): `internal/policy/canonical.go` (2b7cc5c, 582eafd, b6fb078; tool names and templates switched to `internal/contracts` in c07ca60). Strict decoding rejects invalid UTF-8, non-objects, unknown, case-variant and duplicate keys, missing or null fields, wrong types including numbers, trailing data, empty, over-long or control-character identifiers, unregistered templates, empty or repeated `source_invoice_ids` and a non-UUID `report_id`; the digest covers canonicalization version, action, run, tool, canonical arguments, passport, policy revision, recipient, affected resources with versions, outbound content and expiry. Checks: `go test -v ./internal/policy` 55 cases PASS (equivalent inputs give identical bytes for all four tools, 22 rejected inputs, 15 one-at-a-time material changes each change the digest); `pnpm verify` 6 passed, 0 failed, 0 skipped. Argument names match X-09 as frozen by 3c.
  - Hardened (2026-10-03, lane w3, lead decision after lane c1's action-fix review): the decoder enforces record identifier shapes, invoice ids `^invoice_[A-Za-z0-9_-]{1,120}$` (also in `source_invoice_ids`) and vendor ids `^vendor_[A-Za-z0-9_-]{1,121}$`, inside the semantic check's constrained identifier format; prose inside an id is `invalid_arguments` before any check. `recipient_reference` stays bounded so a redirected recipient is stored and denied with `destination_not_allowed` (X-38 evidence unchanged). A grep of every lane's fixture ids found nothing legitimate that breaks.

- [x] **GO-13 · Admit a start-run request and issue the passport, run and job together**
  - **Report 1.2 change:** The passport adds approved model references, the admission catalog revision, shared limits with agent and security sub-limits, concurrency and run expiry; model selection is constrained by the catalog allowlist.
  - **Report 1.1 change:** The passport adds allowed report templates, source authority and the projection rules or source policy version; both templates are in the grant (shape: `passport report fields`).
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: M (estimate 4-8 h)
  - Depends on: GO-18, GO-19, GO-20, GO-22 · Needs: X-06, X-07, X-08, X-13, X-18, X-19, X-21, X-22 · Provides: nothing
  - Paths: `services/gateway/internal/database/database.go`; a new package for admission, named at
    M0 by its owner
  - Work: Derive the passport from the verified operator context, the authoritative task and policy
    versions read from `app`, and the requested scope, by the report's field groups: identity
    (organization, initiating actor, run reference) from verified context only; immutable task and
    policy version references; registered tools and permitted resources, with each invoice's vendor;
    field rules, the recipient resolved through the trusted directory, and opaque references; limits
    (calls, steps, attempts, token ceilings, estimated spending allowance) within the X-06 values;
    issue time and expiry. A request that exceeds authority is rejected with a reason code and the
    scope or limit that must change, and creates nothing; admission never narrows it silently.
    Passport, run and job commit in one transaction, and nothing updates a stored passport.
  - Done when: MVP requirement Trusted admission holds, "Issue a passport only when requested scope
    fits verified user and organization authority", with its acceptance evidence "Reject an
    unauthorized task and prevent passport creation."
  - Tests: database-backed: an invoice outside the organization or the task template, a recipient
    outside the trusted directory and a limit above the policy are each rejected with their reason
    and leave no passport, run or job row; a fitting request creates one of each, and a fault
    injected before commit leaves none; identity fields equal the verified context, and a body that
    carries an organization or actor field fails strict decoding; a missing policy version or an
    unreadable `app` record rejects; a narrower request after a rejection is a new admission
    ("Verify that scope rejection is explicit and the revised request must be resubmitted"); no
    repository path updates a stored passport. The X-24 command.
  - Completed (2026-10-03, 0931f93 and 276862a): `internal/admission` derives the passport from
    the verified operator, the registered task template `reconcile_atlas_v1` (lead decision), the
    active catalog revision read in the admission transaction, and the organization's demo
    records: invoices of the organization sharing one vendor, the requested vendor and destination
    equal to it with a registered reporting address, `approvalRequirement` absent or
    `review_queue_report`, requested limits only below the catalog (`limit_not_allowed` otherwise).
    Passport, run (queued), job (queued, `agent_step`), token ledger (`budget.OpenRunLedger`, limit
    from the passport) and a `run.queued` event commit in one transaction; a rejection writes only
    an `admission.rejected` event and returns the X-13 code with the scope or limit to change; a
    missing or invalid catalog or a storage error stores nothing. Tests (PostgreSQL, rolled-back
    outer transaction with its own records and catalog revision): the fitting request stores one
    of each and an identical passport read-back; twelve over-authority requests are rejected with
    their code and zero passport, run or job rows; no or invalid catalog is unavailable; a narrowed
    catalog narrows templates; a fault injected into the last write leaves nothing. Checks: gateway
    five checks PASS; `pnpm test:db gateway` 461 passed, 0 skipped (at 0931f93); `go test -race
./internal/admission` ok after the ledger; `pnpm verify` 6 passed. Known limitation recorded by
    the lead: no idempotency key on X-07, so two identical commands make two runs.
  - Report: "Trusted authority and passport invariants" (Passport fields and their purpose; Task
    relationships matter); "Functional requirements MVP boundary and deferred scope" (Trusted
    admission); "Threat model limits and unresolved design choices" (Verification priorities);
    "Illustrative passport and interface contracts" (Illustrative passport fields)
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md` (which `app` records carry the operator's authority); `passport report fields`

- [x] **GO-14 · Serve `POST /internal/runs`**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-13, GO-21 · Needs: X-07, X-08, X-13 · Provides: X-28
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/cmd/gateway/main.go`,
    `services/gateway/README.md`
  - Work: Register the route behind GO-21's verification, decode the start-run request strictly,
    take actor and organization only from the verified context, call admission, and answer with the
    passport representation (X-08) or the rejection envelope with the X-13 reason fields. The
    command never waits on a model or tool request: the run proceeds in the worker, and the answer
    fits the agreed `command timeout budget`.
  - Done when: X-28 is reached: "passport, run and job stored in one transaction, or a rejection
    that names the scope or limit that must change and creates no passport" (report:
    "POST /internal/runs issues the passport and durable job").
  - Tests: route tests: a valid command returns the X-08 shape; a rejected command returns the
    shared envelope with its reason code and leaves no passport row; unknown fields, an oversized
    body and a missing or forged context are rejected before admission runs.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Completed (2026-10-03): `internal/api` registers `POST /internal/runs` through
    `httpserver.Options.InternalCommands`, so it runs only behind the service token and the verified
    `X-Operator-Context` (GO-21). The body decodes strictly (at most 64 KiB, no unknown fields, so an
    organization or actor field is refused); identity comes only from the verified operator; GO-13
    admission answers `201` `{runId, passportId}`, a `400` envelope with the X-13 code and the scope
    or limit to change, or `503 decision_unavailable`. It never waits on a model or tool request.
    The same command list mounts lane w2's GO-37 stored-report route with a viewer taken from the
    verified operator. Tests: route tests with a labelled admission double (ids returned, only the
    verified operator passed, rejection and unavailability mapped, bad bodies refused before
    admission, no operator refused) and a PostgreSQL end-to-end test through the real guard,
    admission and catalog (201 and a stored passport; a foreign invoice gives
    `resource_out_of_scope` and no second passport). Checks: `pnpm --filter gateway run`
    `format:check`, `lint`, `typecheck`, `test`, `build` PASS; `go test -race ./internal/api` with
    PostgreSQL ok; `pnpm verify` 6 passed. Live: the built gateway on 127.0.0.1:18310 with tokens
    signed by the API's jose library and the real key returned 201 for the seeded Atlas request
    (run queued, agent_step job, ledger 20000, recipient reference), 400 `resource_out_of_scope` for
    invoice_C01 and 400 `limit_not_allowed` for 25 model calls; the key never appeared in the log.
    Not verified: the call from NestJS itself (no seeded app users; SH-19), and the `command timeout
budget` item stays open (admission is one short transaction).
  - Report: "Illustrative passport and interface contracts" (Proposed browser and runtime
    operations; Decision and error semantics); "Technical architecture and service ownership"
    (Interfaces and repository strategy)
  - Blocked by: `command timeout budget`; `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [x] **GO-15 · Store each proposed action and decide allow, deny or approval required**
  - **Report 1.2 change:** Order per Figure 6: deterministic scope and provenance first, then the semantic check of GO-77 for otherwise permitted proposals.
  - **Report 1.1 change:** Report actions go through the provenance check (GO-63) before the policy decision; unknown report lineage is a denial.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: GO-12, GO-13, GO-19, GO-22 · Needs: X-06, X-09, X-13 · Provides: nothing
  - Paths: the enforcement package from GO-12
  - Work: Store every proposal as an immutable action (stable identifier, tool, canonical
    arguments) before evaluation, then check the registered tool name, the argument schema, the
    organization and the passport's tools and resources, and return to the worker (GO-11) exactly
    one of allow, deny or approval required with a stable reason code. The passport's approval
    rule yields approval required (illustrative value: "Review every `queue_report` effect"; the
    X-06 value governs), so reads and the internal report proceed without review. Malformed or
    unsupported input is a denial; missing policy data, a repository error or any other failure is
    a denial. Each decision is recorded as a safe event (GO-22) before any tool effect. Resource
    relationships and destinations follow in GO-28, current revocations in GO-52.
  - Done when: the gate's three outcomes exist and fail closed: "A successful policy decision should
    distinguish allow, deny, and approval required. A transport or configuration error is not an
    allow decision."
  - Tests: an unregistered tool, a tool missing from the passport, unknown or missing fields, wrong
    types and an invoice outside the passport are each denied with their reason code; a forced
    repository error and a missing policy record are denied; the action row commits before its
    decision row; the approval rule yields approval required for `queue_report`; the emitted reason
    codes equal the X-13 fixture values. `pnpm --filter gateway run test`; database-backed cases
    through the X-24 command.
  - Report: "The enforcement loop and data minimization"; "Illustrative passport and interface
    contracts" (Decision and error semantics); "Architecture and chart reading guide" (Figures 5
    and 6)
  - Blocked by: nothing
  - Completed (2026-10-03): `internal/policy/gate.go` and `recorder.go` (7775a96; contracts and repository switch c07ca60). `Gate.Evaluate` always returns a decision and every failure denies; Figure 6 order; the action is stored and committed before the decision, which updates its status and appends the X-12 event (`action.allowed`, `approval.requested`, `action.denied`, `report.export_denied`) through `repository.Tx.AppendEvent` in one transaction; malformed arguments get no action row, only the denial event. Checks: unit tables cover every denial in Tests with its X-13 code, fail-closed scope/revision/store/record failures, and store-before-decide call order; `pnpm test:db gateway` 430 passed, 0 failed, 0 skipped at c07ca60 including the three recorder database tests; `pnpm verify` 6/6. The reason codes are the X-13 constants of `internal/contracts` (no separate fixture comparison exists).

- [x] **GO-16 · Execute an allowed action through its registered adapter**
  - **Report 1.1 change:** Done when quote: "Only registered adapters would be executable."; Figure 7 is now Figure 8.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-15, GO-17, GO-19, GO-23 · Needs: X-09 · Provides: nothing
  - Paths: a new package for the tool executor, named at M0 by the Go implementer
  - Work: Execute only the registered adapter Go selects from the stored action, by its stable action
    identifier, after a fresh check that the stored action still matches its digest and the run is
    still active and unexpired. Record the attempt and its outcome through the runtime repository,
    count it against the passport's tool attempt limit, and pass the result through data
    minimization (GO-23) to the worker. GO-45 replaces this check with the atomic tool reservation,
    attempt claim and approval consumption.
  - Done when: an allowed `read_invoice` action executes once by its action identifier and leaves a
    matching execution record (Figure 7: "Only a registered adapter can execute the effect, using
    the stable action identity"), the Go half of the M1 exit's "Go executes a permitted tool".
  - Tests: an action whose stored arguments no longer match its digest is not executed; an action
    naming an unregistered tool reaches no adapter; a cancelled or expired run executes nothing;
    every execution has exactly one attempt record with its outcome.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Report: "Architecture and chart reading guide" (Figure 7); "The enforcement loop and data
    minimization"; "Durable state idempotency audit and uncertain outcomes"
  - Blocked by: nothing
  - Completed (2026-10-03): `internal/policy/executor.go` (5a60018, X-11 run status in c07ca60) dispatches an allowed action by its id to Worker 2's `tools.Runner`: fresh checks (status allowed, run running without a cancel request, stored passport unexpired, digest recomputed from the stored arguments), the attempt counted against `limits.toolAttempts` under a run-row lock and committed before dispatch, then `RunEffect` plus the final action status in one transaction; an adapter error rolls back and closes the attempt as aborted (paused), a failed commit is `outcome_unknown`; the worker receives `tools.MinimizeForModel` output only. Checks: `pnpm test:db gateway` 269 passed, 0 failed, 0 skipped at 5a60018 with the 7 executor database tests (executes once with one succeeded attempt; changed digest, denied or unknown action, cancelled run and expired passport reach no adapter; attempt limit; adapter error pauses); `pnpm verify` 6/6. GO-45 replaces the attempt step with the reservation, claim and approval consumption.

- [x] **GO-74 · Apply deterministic content controls to designated fields**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 3-5 h, this roadmap's estimate)
  - Depends on: GO-23 · Needs: X-78, X-86 · Provides: nothing
  - Paths: the enforcement package from GO-12
  - Work: Apply the configured secret and PII patterns to the designated text fields of model input and tool results, with the catalog's block or redact response. Redaction uses validated server operations: deterministic match spans must be valid, and the context record keeps the original source identity, version, classification and the applied rule. "Masking an internal note does not make its report Vendor shareable." Oversized content is rejected or inspected by a documented bounded strategy; "uninspected truncation remainders cannot be appended to context".
  - Done when: a defined secret in a fixture field is masked before the content reaches model context or broad logs, and a block-mode rule withholds the field, with the rule and revision recorded.
  - Tests: unit tests for each pattern, span validation and oversized content; a test that redaction leaves the source restriction unchanged.
  - Report: "Hybrid security controls and managed attack signatures"; "Validation plan and evidence matrix" (Redaction control)
  - Blocked by: `redaction rules`
  - Completed (2026-10-03): `internal/security` (f25b173, field-limit order a0672e9): `ApplyContentRules` with password keyword and URL credential, API token, IBAN (mod-97) and payment card (Luhn) rules; spans validated (bounds, non-empty, UTF-8 boundaries) and merged; `redact` masks as `[REDACTED:<kind>]` (`content_redacted`), `block` withholds (`content_blocked`); fields over 4096 bytes or invalid UTF-8 withheld whole (`content_too_large`); the trusted `SourceRef` passes through unchanged; records carry rule and catalog revision, never text. Designated fields `tool_result_text`, `internal_note`, `model_input_text` (lead's delegate). Checks: `pnpm --filter gateway run format:check|lint|typecheck|test|build` all exit 0, the six corpus secret cases give exactly their fixture spans and the other cases and hostile notes none; `pnpm verify` 6 passed. Model context and broad logs receive only the inspected text once f3 wires GO-76 into the loop.

- [x] **GO-75 · Build the semantic security evaluator behind the metered model gateway**
  - Owner: Go implementer (report role: Implementer 4, enforcement, and Implementer 3, agent runtime) · Tier: A · Size: M (estimate 4-8 h, this roadmap's estimate)
  - Depends on: GO-10 · Needs: X-79, X-84, X-85, X-86 · Provides: nothing
  - Paths: a new package, named at M0 by the Go implementer; the model gateway package from GO-10
  - Work: Send minimized, clearly delimited content with a fixed classifier instruction to the local model as a separate security purpose through the same model gateway, without tool credentials or execution authority. Treat the output as untrusted data: accept only "a schema-validated risk category, score in the configured numeric range and a bounded reason code", reject unsupported fields and malformed scores, and apply the catalog thresholds in Go. For an enabled required guard, a timeout, unavailable model, malformed response or exhausted security allowance pauses or denies; retries stay bounded and metered. Security-purpose requests do not trigger another semantic check. Record purpose, verdict, revision and latency.
  - Done when: a live security request returns a validated verdict that blocks a hostile fixture and allows a clean one, a malformed or missing verdict never becomes an allow, and both model purposes appear in the usage and latency records (M1 exit).
  - Tests: parser and threshold tests with stubbed verdicts (labelled as stubs); timeout and malformed-output tests; one live local-model call recorded separately.
  - Report: "Hybrid security controls and managed attack signatures"; "Relative implementation milestones and critical dependencies" (Hours 2-6); "Delivery scope and six person ownership" (Implementer 4: "Implement the semantic-verdict boundary")
  - Blocked by: `classifier prompt and verdict schema`; `decision 6 in docs/product/README.md`
  - Completed (2026-10-03): `SemanticEvaluator` (5883988, context 8192 in a0672e9): one security-purpose call through `model.AccountedCaller` (reserved before dispatch), fixed instruction, untrusted text between nonce markers, strict `ParseVerdict` ({risk_category, score 0 to 1, reason_code}), `score >= threshold` applied in Go, one attempt and no retry; refused reservation is `security_allowance_exhausted`, timeout, transport, unknown usage and malformed verdict are `security_evaluator_unavailable`, both pause with no text. Checks: gateway checks all exit 0 with stub verdicts labelled as fixtures; live opt-in `GO_SECURITY_LIVE=1 ... go test -tags=model_live ./internal/security -run '^TestLiveSemanticEvaluator$'` PASS on Ollama 0.35.1, `qwen3.5:4b` (2a654d98e6fb): hostile note blocked (score 1), clean note passed (score 0). Not verified here: the latency rows in `runtime.timing_records` and the assessment rows, which the caller persists (GO-80, f3); the ledger usage row is written by `AccountedCaller`.

- [ ] **GO-76 · Inspect tool results before they enter the agent context**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-23, GO-74, GO-75 · Needs: X-86 · Provides: nothing
  - Paths: the worker package from GO-08; the enforcement package from GO-12
  - Work: Figure 10: minimize the authorized tool fields, apply the current deterministic content and signature rules, and inspect applicable untrusted text with the semantic evaluator. "Only permitted or validated redacted content returns to the bounded agent loop. Blocked text is withheld; guard failure or budget exhaustion is visible. Redaction never clears inherited source restrictions." No raw-result channel bypasses this check.
  - Done when: a hostile tool-result fixture is blocked before agent context (M1 exit), the clean internal note passes with its restriction, and the decision is a safe event.
  - Tests: a worker test with the hostile and clean fixtures asserting what the next model request contains; a guard-failure test that pauses the run.
  - Report: "Architecture and chart reading guide" (Figure 10); "The enforcement loop and data minimization" (Minimize information before it enters the model)
  - Blocked by: nothing
  - Progress (2026-10-03): the pipeline function is done (91d15c7): `Inspector.InspectToolResult` runs the field limit and secret rules, signatures, then the semantic check on `internal_note.text` only (redacted text) over every string of the minimized result; blocked values become `[WITHHELD:<reason>]` while permitted values return, a guard failure withholds everything (`paused`); classification and sources unchanged. Checks: gateway checks all exit 0 (clean note unchanged, hostile note withheld, signature before semantic, secrets masked before the classifier, guard failures pause); `pnpm verify` 6 passed. Missing half (f3): calling it from the worker loop, pausing the run, persisting the records, and the worker test of the next model request's contents.

### Tool adapters, provenance and rendering (report role: Implementer 5)

- [x] **GO-17 · Build `read_invoice`**
  - **Report 1.1 change:** Returns the internal investigation note where the field rules allow, with its trusted Internal only label.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-07, GO-20 · Needs: X-06, X-09, X-20, X-21 · Provides: nothing
  - Paths: a new package for the tool adapters, named at M0 by the Go implementer
  - Work: The adapter takes `invoice_id` only, checks itself that the invoice belongs to the run's
    organization and to the passport (an upstream check never replaces its own: "Resource-owning
    services must still enforce their own authorization"), and returns exactly the GO-07 fields, with
    protected values as opaque references, through schema-qualified SQL on `demo`.
  - Done when: the tool argument boundary for `read_invoice` holds: "Verify tenant and passport
    membership; return only authorized fields." (report, hours 2-6: "one read tool").
  - Tests: database-backed, calling the adapter directly as well as through the executor: an invoice
    of another organization and an invoice outside the passport return an error and no data; the
    serialized result holds exactly the allowlisted fields; protected values appear only as
    references; an unknown argument is rejected. The X-24 command.
  - Completed (2026-10-03): W2 lane, branch go/w2, e37434d (events reworked in b838586).
    `internal/tools`: the effect runner (RunEffect) and read_invoice: strict X-09 decoding, the
    passport scope and organization checked by the adapter itself, exactly the GO-07 allowlist, the
    note only with internalNoteReadable and its classification. Tests: exact fields, note omitted,
    out-of-scope and other-organization invoices fail with resource_out_of_scope and no data,
    unknown argument / other organization / changed digest / wrong tool are precondition errors, a
    completed attempt cannot rerun. Checks: gateway format:check, lint, typecheck, test, build
    PASS; go test -race against my migrated PostgreSQL 17 (w2_check, 127.0.0.1:55435): pass;
    `pnpm test:db gateway` 166 passed, 0 skipped. Through the executor: Worker 3's GO-16 calls
    tools.Runner.
  - Report: "Illustrative passport and interface contracts" (Proposed tool argument boundaries);
    "Threat model limits and unresolved design choices" ("Verify that every adapter checks
    arguments and resource relationships, not only the tool name")
  - Blocked by: nothing

### Modules the report's team table does not name (Go implementer)

- [x] **GO-18 · Mirror the frozen contracts in Go DTOs**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 1.5-3 h)
  - Depends on: nothing · Needs: X-07, X-08, X-09, X-10, X-11, X-12, X-13 · Provides: X-15 (part: the X-07 to X-13 mirrors)
  - Paths: `services/gateway/internal/health/dto.go`,
    `services/gateway/internal/health/dto_test.go`, `packages/contracts/fixtures` (read; landed by
    SH-11), `services/gateway/README.md`; a new DTO file or package, named at M0 by the Go implementer
  - Work: Mirror each frozen contract and the error envelope's reason fields in Go, following
    "Changing a shared contract" in `docs/team-workflow.md`. The fixture test lists fixtures by
    explicit case, so an unlisted fixture is never checked: add a test that fails for any fixture
    with neither a Go case nor a recorded API-only exemption. The X-14 mirror is GO-62.
  - Done when: every fixture of X-07 to X-13 decodes strictly into its Go DTO and re-encodes
    unchanged, and every enum value Go emits (states, reason codes, decision outcomes) matches the
    fixtures, so the services "describe the same identifiers, enum values, timestamps, error
    semantics, and action integrity rules"; X-15 is reached when GO-62 is done as well.
  - Tests: the strict decode and re-encode test per fixture; an emitted-values test in the pattern
    of `TestEmittedValuesMatchContractFixtures`; the new test that lists
    `packages/contracts/fixtures` and fails for an unmapped fixture.
    `pnpm --filter gateway run test`.
  - Completed (2026-10-03, 7f8904f, with GO-62 080030c): the Go-owned contracts were drafted and
    approved by the lead as Go owner (X-08 passport, X-09 action proposal and stored action, X-11
    run state, X-12 safe event, X-13 reason codes, now 29 with `limit_not_allowed`), and X-07 got a
    schema and fixtures mirroring the API's `StartRunSchema` without changing it. All live in
    `packages/contracts` (types, JSON Schemas, fixtures, typed samples) and are mirrored in
    `services/gateway/internal/contracts`. Go tests: every fixture decodes strictly and re-encodes
    unchanged; every shared fixture has a Go case or a recorded exemption (the unmapped-fixture
    test); every Go enum equals its schema enum; proposal arguments fit their tool's shape. Checks:
    `pnpm --filter @workspace/contracts run lint`, `typecheck`, `test` (6/6), `build` PASS; gateway
    `format:check`, `lint`, `typecheck`, `test`, `build` PASS; `pnpm verify` 6 passed. X-15 is
    reached with GO-62. RunView and SanitizedEvent (web + API side) still differ from X-11/X-12;
    the lead asked Batın to align them.
  - Report: "Illustrative passport and interface contracts"; "Risk register and scope controls"
    (NestJS/Go contract drift: "validate serialized contracts")
  - Blocked by: nothing

- [x] **GO-62 · Mirror the operator context contract in Go**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 0.5-1 h, split from GO-18)
  - Depends on: GO-18 · Needs: X-14 · Provides: X-15 (part: the X-14 mirror)
  - Paths: `services/gateway/internal/health/dto_test.go`, `services/gateway/README.md`; the DTO
    file or package from GO-18
  - Work: Once SH-39 lands X-14, mirror it in Go as GO-18 mirrors the others and give its fixture a
    case in the strict decode test.
  - Done when: the X-14 fixture decodes strictly into its Go DTO and re-encodes unchanged, as
    GO-18's fixtures do, and with GO-18 done X-15 is reached ("validate serialized contracts").
  - Tests: the strict decode and re-encode test for the X-14 fixture; GO-18's unmapped-fixture test
    passes. `pnpm --filter gateway run test`.
  - Completed (2026-10-03, 080030c): X-14 `OperatorContext` (the API's type, unchanged) got
    `operator-context.schema.json` (uuid ids, unique bounded roles), a fixture and a typed sample,
    and is mirrored as `contracts.OperatorContext` with a strict round-trip case. Checks: contracts
    `lint`, `typecheck`, `test`, `build` PASS; gateway five checks PASS; `pnpm --filter api run
typecheck` PASS; `pnpm verify` 6 passed.
  - Report: "Illustrative passport and interface contracts"; "Risk register and scope controls"
    (NestJS/Go contract drift: "validate serialized contracts")
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [x] **GO-19 · Build the runtime repository with guarded state transitions**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: M (estimate 4-8 h)
  - Depends on: GO-20 · Needs: X-19 · Provides: nothing
  - Paths: `services/gateway/internal/database/database.go`, `services/gateway/cmd/gateway/main.go`,
    `services/gateway/README.md`; a new package for the runtime repository, named at M0 by the Go implementer
  - Work: The single writer of the `runtime` schema (Figure 1: "Owns runtime writes"), with short
    explicit transactions, schema-qualified SQL, row mapping, and run and action state transitions
    that reject an invalid move such as executing a denied action or resuming a completed run. "A
    state change and the continuation or event it produces should be committed together." It offers
    a transaction that the executor and an adapter can share for a local demo effect (GO-34).
    `main.go` hands it the pool, which today goes only to readiness.
  - Done when: every runtime write goes through it, an invalid transition leaves the stored state
    unchanged, and a state change commits with its event or not at all (MVP requirement Durable
    execution: "Persist jobs, continuations, approvals, and execution state").
  - Tests: database-backed: each invalid transition the guards name is rejected and changes no row;
    a failure injected between a state change and its event leaves neither; queries succeed with no
    `search_path` set. The X-24 command.
  - Completed (2026-10-03, cda6336; EnqueueJob 9654150): `internal/repository` writes runtime
    passports, runs, jobs and audit events in short transactions over the pool or an enclosing
    transaction (`InTransaction`, `Tx.Raw` for GO-34): `InsertAdmission`, guarded `TransitionRun`
    (one status-checked UPDATE plus its X-12 event; terminal statuses accept nothing; a reason
    exactly for paused, failed and stopped), `AppendEvent` (validated against X-12), `EnqueueJob`,
    organization-scoped `RunState` and `Passport` reads (stored scope and limits decoded strictly).
    By lead decision the gate (lane w3) writes `runtime.actions` and `runtime.approvals` and lane
    f3's budget package writes `runtime.model_calls`, so "every runtime write" holds for the
    records this repository owns. Tests (PostgreSQL, rolled-back outer transaction): admission rows
    and read-back, injected failure leaves none, duplicate run fails whole, refused transitions
    write nothing, a failing event rolls back its state change, another organization reads and
    changes nothing, schema-qualified queries with no `search_path`. Checks: gateway five checks
    PASS; `pnpm test:db gateway` 186 passed, 0 skipped (at cda6336); `go test -race ./...` with a
    required database ok; `pnpm verify` 6 passed.
  - Report: "Data ownership and the transition from starter to product" (Proposed database
    ownership); "Durable state idempotency audit and uncertain outcomes"; "Validation plan and
    evidence matrix" ("Exercise malformed tool arguments and invalid state transitions as well as
    valid ones")
  - Blocked by: nothing

- [x] **GO-20 · Build the Go PostgreSQL test harness on the X-24 command**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: nothing · Needs: X-24 · Provides: nothing
  - Paths: `services/gateway/internal/database/database.go`,
    `services/gateway/internal/database/database_test.go`, `services/gateway/scripts/go.mjs`,
    `services/gateway/README.md`
  - Work: A shared helper for the Go database-backed tests: it connects to the database the X-24
    command provides, creates and removes its own synthetic rows, and makes each such test report
    itself as skipped, never as passed, when no database is configured, so `pnpm verify` stays free
    of a database and a skipped check is never quoted as a pass. The gateway `test` task in
    `services/gateway/scripts/go.mjs` runs `go test ./...` with fixed arguments, and without `-v`
    or `-json` Go prints only `ok` for a package whose tests skipped, so this task also makes that
    task print each skipped test (for example with `-v`, or with `-json` and a printed list of
    skips) and updates the command list in `services/gateway/README.md`.
  - Done when: a Go test that needs PostgreSQL passes through the X-24 command against a real
    database and is listed as skipped in the output of `pnpm --filter gateway run test` without
    one, so the team can "choose and verify the appropriate transaction boundaries" against
    PostgreSQL.
  - Tests: the harness's own test: a round trip passes with the database up; the X-24 command fails
    with the database down; without configuration `pnpm --filter gateway run test` lists the test
    as skipped. The X-24 command; `pnpm --filter gateway run test`.
  - Completed (2026-10-03): `internal/testdb` provides the shared explicit pool/ping helper and
    UUID fixture identifiers. Budget and native Ollama PostgreSQL tests use it. Absent optional
    settings visibly skip; required, partial, invalid and unreachable settings fail safely.
    No schemas, tables or seeds are created. The gateway wrapper uses `go test -count=1 -v ./...`
    to name skips and avoid cached database outcomes. The old independent URL is removed so X-24
    discovery stays database-free. The user's accepted runtime schema review inputs and the
    remaining GO-07 contract blockers are recorded in the gateway README.
    Verification: optional `pnpm --filter gateway run test` lists 12 database skips; PostgreSQL
    round-trip/rollback and `go -C services/gateway test -race ./... -count=1 -timeout=60s` pass.
    `pnpm test:db` passes Go and API with no skips; after stopping test PostgreSQL, `pnpm test:db gateway`
    correctly exits 1. `pnpm verify` passes all six steps. No service wiring is changed, so no
    additional smoke was run for GO-20.
  - Report: "Atomic allowances hard limits and estimated cost" ("the application must choose and
    verify the appropriate transaction boundaries"); "Validation plan and evidence matrix"
    (Interpreting results honestly)
  - Blocked by: nothing

- [x] **GO-21 · Verify service identity and operator context on every internal command**
  - **Report 1.1 change:** Both proposed internal command paths (`POST /internal/runs/:id/cancel`, `POST /internal/actions/:id/approval`) carry an identifier, so the `withJSONErrors` change applies; package per `Go package layout`.
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: SH-03, GO-62 · Needs: X-13, X-14, X-23 · Provides: X-27
  - Paths: `services/gateway/internal/httpserver/middleware.go`,
    `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/errors.go`,
    `services/gateway/internal/httpserver/server_test.go`,
    `services/gateway/internal/config/config.go`,
    `services/gateway/internal/config/config_test.go`, `services/gateway/README.md`
  - Work: Extend or replace the starter's service-token check as decision 4 says, so every internal
    product route verifies the calling service and the authenticated operator context before any
    handler runs, and rejects the rest with the shared envelope (401 or 403, with codes aligned with
    the API's exception filter). Add strict JSON decoding with a body size limit, because no route
    reads a body today. If a route named at M0 carries an identifier in its path, make
    `withJSONErrors` in `errors.go` serve matched routes through the mux: it looks the route up with
    `mux.Handler` and calls the handler directly, and `mux.Handler` "does not populate named path
    wildcards, so r.PathValue will always return the empty string" (`net/http`). The verified actor
    and organization reach the handlers as their only identity source; request ids never act as
    authorization. Each command still authorizes itself against its organization and run (GO-14,
    GO-41, GO-44). Acceptance of what NestJS actually sends, and rejection of the same command
    altered, are checked where X-26 is delivered and in SH-22; this task tests against X-14.
  - Done when: X-27 is reached: "Go verifies service identity and the operator context on every
    internal command and rejects the rest with the shared envelope" (report: "Go must verify service
    identity and authenticated operator context, then authorize the command against its
    organization and run").
  - Tests: in the pattern of `server_test.go`: the service token alone, a missing context, a forged
    or altered context, a context issued for another service, and a malformed or oversized body are
    each rejected before the handler with the shared envelope; no token, secret or context value
    appears in logs or responses; a well-formed command whose context follows X-14 and the
    mechanism decision 4 records is accepted; if a route has a path wildcard, its handler receives
    the value from the path through `NewHandler`. `pnpm --filter gateway run test`.
  - Completed (2026-10-03, 7ed32c6): `internal/operatorcontext` verifies the `X-Operator-Context`
    HS256 JWT the API signs (HMAC first, strict header and claims, issuer `gateway-client`,
    audience `gateway`, at most five minutes, one-use `jti`, X-14 context); every product route is
    registered through `httpserver.Options.InternalCommands`, behind the service token and that
    verification, with the shared envelope (401) before the handler. `DecodeJSONBody` adds strict,
    bounded bodies (400); matched routes are served through the mux so path wildcards work;
    `OPERATOR_CONTEXT_SIGNING_KEY` is required in Go config. Tests: a token signed by the API's
    jose library verifies; replay, wrong key, altered payload, alg none, wrong issuer or audience,
    expiry and lifetime faults, unknown claims and bad contexts are rejected; route tests reject
    the service token alone, the context alone, forged or duplicated contexts and a missing
    verifier before the handler; no credential in responses or logs. Checks: gateway five checks
    PASS; `go test -race` for httpserver, config, operatorcontext ok; `pnpm verify` 6 passed; the
    built gateway on 127.0.0.1:18310 served health and ping and refused to start without the key.
    The check against what NestJS sends end to end stays with X-26/SH-22.
  - Report: "Technical architecture and service ownership" (Interfaces and repository strategy);
    "Threat model limits and unresolved design choices" ("The service token in the starter requires
    replacement or extension for authenticated operator context"); "Validation plan and evidence
    matrix" ("a hidden URL is not a protection")
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [x] **GO-22 · Write safe decision events with every state change**
  - **Report 1.2 change:** Events add the metered purpose, admission and active catalog revisions, matched rule and feed revision; safe summaries omit raw notes, secrets, model requests and classifier reasoning.
  - **Report 1.1 change:** Events link the run, action, policy version, matched rule, report ID, template version, classification, lineage-check outcome and actual effect; event names from SH-10 (the architecture's examples include `report.created`, `report.export_denied`, `report.safe_template_offered`).
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-19 · Needs: X-12, X-13, X-19 · Provides: nothing
  - Paths: the runtime repository package from GO-19
  - Work: Admission, each decision, approval, execution, stop and failure writes an event in the
    same transaction as its state change: ordered per run with a cursor, linking "the run, action,
    policy version, matched rule, outcome and actual effect", with the X-13 reason code, a masked
    summary and the replay label, and never confidential arguments or review content. These rows
    are what the interface displays and what the read path serves, whichever outcome SH-05 records.
  - Done when: a run's events reconstruct its decisions in order with masked metadata only ("Masked
    summaries support the timeline without copying confidential arguments into general logs"), the
    Go half of Implementer 1's "display a real persisted event".
  - Tests: database-backed: the events of one run are strictly ordered, and a cursor read returns no
    gap or duplicate under concurrent writes; a decision event carries every link listed; the
    serialized events of a run that read protected fields contain none of the fixture's protected
    values; a failed state change leaves no event. The X-24 command.
  - Completed (2026-10-03): `internal/repository` is the X-12 event writer and reader.
    `Tx.AppendEvent` validates every event against X-12 (closed types, decisions, reason codes and
    masked-summary values; an action needs its run) and inserts it in the caller's transaction, so
    it commits with its state change or not at all; `Join(pgx.Tx)` lets another lane's transaction
    use it. A run-scoped append first takes a `FOR NO KEY UPDATE` lock on the run row until commit,
    so a run's events commit in id order and `Repository.RunEvents(org, run, afterEventID, limit)`
    pages by cursor without gaps or duplicates; a stored row outside X-12 is never served. Admission
    writes `run.queued` / `admission.rejected`, and every `TransitionRun` requires its event. Tests
    (PostgreSQL): four concurrent writers commit 60 events while a reader pages by cursor and reads
    each exactly once in order (the same test failed 10 of 10 runs with the lock removed and passed
    10 of 10 with it); a decision event round-trips every link (purpose, admission and active
    catalog revisions, matched rule, feed revision, report, template, classification, lineage
    check, effect, replay source, alternative template, safe message); another organization reads
    and writes nothing; a row with an extra summary key fails closed; a failed state change leaves
    no event (GO-19 test). Checks: gateway five checks PASS; `go test -race -count=3
./internal/repository` with PostgreSQL ok; `pnpm verify` 6 passed. Waits on other lanes:
    decision, approval and execution events are written by lanes w3, w2 and f3, which must switch
    their raw inserts to `repository.Join(tx).AppendEvent` for the ordering guarantee.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a
    second disclosure channel); "Illustrative passport and interface contracts" (Decision and error
    semantics)
  - Blocked by: nothing

- [ ] **GO-23 · Enforce tool-result field allowlists and minimize the model context**
  - **Report 1.2 change:** Untrusted authorized free text passes GO-74 and GO-76 before it becomes agent context.
  - **Report 1.1 change:** The authorized internal note may enter the model context with its restriction; the architecture places context minimization in the model gateway (`Go package layout`).
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 2-3 h)
  - Depends on: GO-07 · Needs: X-06 · Provides: nothing
  - Paths: a new package for data minimization, named at M0 by the Go implementer
  - Work: Apply the GO-07 allowlist to every tool result before the worker sees it, and build every
    model request only from "the fixed task template and authorized parameters" plus minimized
    earlier results, so the rule covers "initial task input, retrieved documents, tool results and
    stored conversation state", not only the filter after tool execution. Protected values stay
    references; GO-35 adds their resolution inside the adapters.
  - Done when: returned fields obey the X-06 field rules on every path into the model ("Data rules
    should determine approved fields before model access"), the minimization half of demo beat 3's
    "Returned fields obey policy".
  - Tests: a tool result with an extra field loses it before the worker sees it; the serialized
    model request of a fixture run holds no protected value and no field outside the template and
    the allowlists; stored conversation state replays only minimized content.
    `pnpm --filter gateway run test`.
  - Progress (2026-10-03): W2 lane, branch go/w2, f338396: `tools.MinimizeForModel` re-encodes
    every tool result through its typed allowlist (extra fields dropped, a failure yields only
    outcome and reason, unknown tools refused) and lists the untrusted note text for GO-74/GO-76.
    Missing half: building every model request only from the task template and minimized results is
    the worker/model lane (f3), which calls MinimizeForModel.
  - Report: "The enforcement loop and data minimization" (Minimize information before it enters the
    model); "Illustrative passport and interface contracts" (Narrow final result and context
    boundary)
  - Blocked by: nothing

- [x] **GO-24 · Serve the run, usage and event reads, if the read path chooses Go endpoints**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 2-3 h)
  - Depends on: SH-05, GO-21, GO-22 · Needs: X-11, X-12 · Provides: X-29, X-30
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/README.md`
  - Work: Only if SH-05 chooses private Go endpoints ("NestJS would expose these operations and call
    corresponding private Go endpoints"): serve the run and usage view (status, usage, terminal
    reason, and the passport if `passport in the run view` says so) and the sanitized events after a
    cursor, behind GO-21, checking on every read that the run belongs to the verified organization
    and that the actor may see it. Each response ends well inside the 30 s write timeout (polling,
    no long-lived stream). If SH-05 chooses runtime views, this task becomes "Dropped: the read path
    chose runtime views (SH-16)"; the records the views read come from GO-22, GO-39 and GO-19.
  - Done when: X-29 and X-30 are reached through Go: MVP requirement Authorized visibility, "Expose
    organization-scoped run state and sanitized ordered events", with "Organization and object
    access checked on every read."
  - Tests: a read of another organization's run returns the rejection envelope and no data; cursor
    reads return events in order with no gap or duplicate; no raw argument, review content or
    protected value leaves Go; the responses decode strictly against the X-11 and X-12 fixtures.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Progress (2026-10-03): W2 lane, branch go/w2: package `internal/reads` serves
    `GET /internal/runs/{runId}` (exactly X-11, via `repository.RunState`),
    `GET /internal/runs/{runId}/events?after=&limit=` (X-12 page and `nextCursor`, via 3c's
    `repository.RunEvents`) and `GET /internal/runs/{runId}/usage` (a Go-side draft of the X-29
    usage: model calls per purpose by outcome, settled and held tokens, unknown usage as its own
    count, the token ledger, tool attempts by outcome; no cost, since the local model has none).
    Identity from `operatorcontext.FromContext` only; another organization's run is 404 with no
    data. Tests: handler cases (401, 404, 400, 503, strict decode against the X-11 and X-12
    fixtures) and PostgreSQL cases (usage values, other organization, gapless event paging,
    reads as `task_passport_gateway`). Missing: the mount in `internal/api` (3c), the usage view in
    `packages/contracts`, and `passport in the run view` (no passport is served).
  - Completed (2026-10-03): 3c mounted the routes in `internal/api` (da44d15, merged into go/w2).
    `TestPostgresReadRoutesThroughTheGatewayHandler` calls them through the production handler
    tree (`httpserver.NewHandler` with `api.Commands`, service token and signed operator
    context): the owner gets 200 with the run state (strict X-11), events (strict X-12) and usage
    (now with the GO-39 ledger); another organization gets 404; no service token gets 401. 3c's
    GO-57 test adds every cross-organization case. The organization-wide event check now uses
    `repository.ValidStoredEvent`. `go test ./internal/reads` against `starter_test`: ok. Not
    served: the passport (`passport in the run view` is still open); the usage view is a Go draft
    until nestjs lands it in `packages/contracts`.
  - Report: "Illustrative passport and interface contracts" (Proposed browser and runtime
    operations); "Technical architecture and service ownership" (Interfaces and repository
    strategy); "Functional requirements MVP boundary and deferred scope" (Authorized visibility)
  - Blocked by: `read path`; `passport in the run view`

- [ ] **GO-25 · Serve the task form options, if `form options` chooses a Go endpoint**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-21 · Needs: X-21 · Provides: X-25
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/README.md`
  - Work: Only if the document owner settles `form options` with a Go endpoint: return the options
    of the fixed task template (vendor, invoice set, allowed destination, approval requirement,
    available limits) for the verified organization, read from the authoritative records. If
    another outcome is chosen, this task becomes "Dropped: `form options` chose another provider".
  - Done when: X-25 is reached through Go, and the options never exceed what admission accepts for
    that organization, so the task setup screen can "expose these choices in business language".
  - Tests: options of another organization are never returned; every returned option is admitted by
    GO-13 for the same context; nothing outside the template's authority appears.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Report: "Project definition purpose and intended outcome" (What a passport would contain);
    "Relative implementation milestones and critical dependencies" (Critical path and sensible
    reductions: "keep a fixed server-owned task template")
  - Blocked by: `form options`

## M2: hours 6-10

- **Team focus (report 1.2).** "Complete the four adapters, scoped reads, trusted source manifests, both fixed report templates, inherited classifications, destination checks and bounded feedback. Implement current-catalog lookup and versioned signature-rule activation; add otherwise permitted action semantic checks."
- **Exit condition (report).** "An internal report is retained for authorized internal viewing; its vendor
  export is denied; a separate approved-field vendor report can be created."
- **Report 1.2 sync points (spine).** X-82, X-87, X-88.
- **Sync points needed by the end (spine):** X-33 to X-38, X-63 and X-64 (X-65 only if the replay
  is triggered through NestJS). This side provides X-36, X-37, X-38 and X-63, X-64 if
  `stored report read` chooses a Go endpoint, X-65 if `replay entry` has the interface trigger the
  replay (GO-05, option 2), through a task the Go implementer adds once that outcome is chosen
  (spine, X-65), and X-67 early (GO-29), which the spine needs by M3.
- **Report 1.1.** This side adds X-72 and X-75 (GO-66, GO-67) and needs X-68 to X-71; GO-29 is now
  Tier A and provides X-67 with GO-47; GO-30 moved to M4 with X-37 and X-38. The first integrated
  deliverables of Implementer 4 ("An Internal only report cannot be exported to the correct vendor,
  even if an approval is submitted.") and Implementer 5 ("The stored vendor report uses only
  approved invoice fields and creates one matching outbox effect.") start here with GO-66 and GO-67
  and complete at M3 (GO-69, GO-47).

### Agent runtime (report role: Implementer 3)

- [ ] **GO-26 · Validate the narrow final result and complete the run**
  - **Report 1.1 change:** The final result may reference both reports.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-3 h)
  - Depends on: GO-11, GO-32 · Needs: X-11, X-33 · Provides: nothing
  - Paths: the worker package from GO-08
  - Work: A final answer passes only the `final result format` outcome. The report recommends "a
    structured status with authorized report references" (proposed, not decided): every report it
    names must belong to the run, its organization and its passport, unsupported fields are
    rejected, and only the validated result is persisted before the run completes. Model prose is
    never stored as the authoritative deliverable.
  - Done when: a reconciliation run completes only with a validated result (Figure 5: "A final
    answer remains subject to the defined output boundary"; "This turns final-output validation
    into a defined check of identifiers, ownership, and supported fields").
  - Tests: a final answer that names a report of another run or organization, carries an unknown
    field or lacks a reference is rejected and the run does not complete; a valid result completes
    the run with its recorded reason; the persisted result holds no free text the format does not
    allow. `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Progress (2026-10-03): `final result format` decided by the lead: the model's final answer is
    exactly `{"status":"completed","report_ids":[...]}` naming one or two reports this run created
    in its organization; nothing else, no prose. `internal/runresult.Validate` parses it strictly
    (unknown or repeated keys, text around it, other statuses, more than two or repeated ids and
    non-uuid ids are `invalid_arguments`) and checks every id is a `demo.reports` row of this run
    and organization (another run's, another organization's or an unknown report is
    `resource_out_of_scope`); it returns the canonical reference `{"report_ids":[...]}`.
    `repository.RunTransition.ResultReference` persists it in `runtime.runs.result_reference` in the
    same transaction as the transition to completed (refused for any other target or for a
    reference outside the format), and X-11 `RunState.resultReference {reportIds}` exposes it
    (schema, fixtures including a completed run, TS, Go). `runresult.FinalAnswerInstruction` is the
    one wording for the agent prompt. Tests: 14 rejected formats, two valid ones, the instruction's
    example parses; PostgreSQL: own reports validated, a sibling run's report (same organization),
    another organization's and an unknown report rejected; a rejected final answer leaves the run
    running and a forged or misplaced reference is refused; the validated reference completes the
    run and reads back unchanged with no prose. Missing half: lane f3 calls `Validate` in the
    loop's StepFinal, counts a rejection as a correction (GO-29, lead decision) and adds the
    instruction to the agent prompt; then a reconciliation run completes only with a validated
    result.
  - Report: "Illustrative passport and interface contracts" (Narrow final result and context
    boundary); "The enforcement loop and data minimization"; "Threat model limits and unresolved
    design choices" ("Final-output validation requires an output format and a data rule")
  - Blocked by: `final result format`

- [x] **GO-27 · Prove a permitted reconciliation on the Go side**
  - **Report 1.1 change:** Rewritten for beats 3 and 4: the agent reads the permitted invoice fields and the note, identifies INV104 in A01 and A02, and `create_report` stores the `internal_investigation_v1` report as Internal only with its source trail. Provides X-63 as amended in the spine.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-11, GO-26, GO-28, GO-31, GO-32, GO-63 · Needs: X-16, X-33, X-34 · Provides: X-63
  - Paths: none (a scenario test in the packages above)
  - Work: Run the reconciliation over the X-34 fixtures through the real gate, executor and adapters,
    once with the decision 6 model and once with a labelled provider test double for repeatability,
    and capture the returned fields, the stored report and its references for the storyboard (X-16).
  - Done when: X-63 is reached: "Returned fields obey policy; the report references authorized
    records and contains the expected discrepancy." (demo beat 3; M2 exit: "A permitted
    reconciliation succeeds").
  - Tests: the scenario test asserts the allowlisted fields, the report's references to permitted
    invoices only and the seeded discrepancy, and its output can be retrieved as evidence. The X-24
    command; one live run, quoted.
  - Report: "Live demonstration storyboard and proof checks" (Proposed demo sequence, beat 3);
    "Relative implementation milestones and critical dependencies" (Hours 6-10)
  - Blocked by: nothing
  - Completed (2026-10-03): `internal/scenario/permitted_reconciliation_postgres_test.go` drives
    beats 3 and 4 through `agent.NewProductionChain` (real admission, gate, executor, adapters) on
    Worker 2's story harness. `TestPermittedReconciliationThroughTheProductionChain` (scripted
    fixture provider, labelled: no model) asserts every returned field against the adapter's
    GO-07 allowlist (read from the result types' tags) and the passport scope, the note returned
    only on A01 with its `internal_only` classification, and the stored
    `internal_investigation_v1` report labelled `internal_only` with a source trail of exactly the
    two passport invoices and the finding "- INV104: A01, A02". `TestLivePermittedReconciliation`
    (`-tags=model_live`) checks the same on what the live model chose; one live run, quoted below.
    Checks: gateway `format:check`, `lint`, `typecheck`, `test`, `build` exit 0; `pnpm verify`
    6 passed; `GOFLAGS=-p=3 pnpm test:db` gateway "880 passed, 0 failed, 0 skipped", api "16 passed".
  - Live evidence (2026-10-03, decision 6 model `qwen3.5:4b`, Ollama digest
    `2a654d98e6fba55d452b7043684e9b57a947e393bbffa62485a7aac05ee4eefd`, local test database;
    command from `services/gateway`:
    `GO_STORY_LIVE=1 MODEL_BASE_URL=http://localhost:11434 MODEL_NAME=qwen3.5:4b go test -tags=model_live -run <test> -v ./internal/scenario`).
    The organization,
    users, memberships, the Atlas vendor and invoices A01 and A02 were inserted by the test harness
    (`openStory`), and the signature feed was bound to the seeded catalog revision by the harness
    (`completeSeededCatalog`, `catalogtest.InsertFeed`), not by the product import (API-34). The
    agent instruction is the storyboard's business task (see the gateway README, "Agent task
    instruction"); it names no report to send and no control.
    - Run set 1, at edb712c (before c1's not-applicable fix 49ad175, before the recipient
      sentence), `TestLiveStoryThroughTheProductionChain`: runs `67423f60-6344-48cf-9c56-53a1ae9e6c54`
      (stopped allowance_exhausted), `50f42988-e1cd-4201-a6e1-3cef6a4cf67a` (awaiting_approval),
      `2c89fe4e-ee76-45bd-8590-d6f019530f20` (stopped allowance_exhausted). Internal report created
      3/3; internal export attempted 0/3 (each queued the vendor report); recipient reference
      mangled 2/3 (prefix dropped, denied `destination_not_allowed`); `semantic_injection_detected`
      denials of clean proposals in 2/3 runs (create_report and read_vendor in the first,
      read_invoice in the second); outbox 0.
    - Run set 2, at 87f22f0 plus the recipient sentence (c1's fix present; Worker 2's live test now
      approves the review): runs `8b812e16-d454-4a03-9e4d-c261f8bfd975` (internal report queued to
      Atlas and denied `report_export_restricted`, then the vendor report approved and queued, outbox
      1, a further queue proposal awaiting review), `cfbd598b-f7fa-4610-b729-e3c9f56480bf`
      (completed, outbox 1), `b4a7a4c8-71a0-4d52-9639-bdd142c02669` (completed, outbox 1). Internal
      report created 3/3; internal export attempted and denied 1/3; recipient reference mangled 0/3;
      semantic denials 0 (one security call per run).
    - GO-27 live run, same build: `7c1bc441-9010-402b-922c-2b862230c44e`, awaiting_approval; returned
      fields read_invoice (with `internal_note` only on A01), read_vendor
      `name,recipient_reference,vendor_id,version`, create_report
      `classification,content_hash,report_id,source_invoice_ids,template,version`; internal report
      labelled `internal_only`, source trail A01@1 internal_only and A02@1 vendor_shareable, finding
      "- INV104: A01, A02". The lead decided that demo beat 5 (the denied internal export) uses the
      labelled replay (`cmd/replay`), said openly, since the live model attempts it in fewer than 2
      of 3 runs.

- [ ] **GO-71 · Record the conservative context manifest, if `internal report rendering` admits model prose** Dropped: `internal report rendering` is deterministic only (lead's delegate, 2026-10-03)
  - Owner: Go implementer (report role: Implementer 3, agent runtime, and Implementer 5, provenance) · Tier: B · Size: S (estimate 2-4 h)
  - Depends on: GO-23, GO-63 · Needs: X-69 · Provides: nothing
  - Paths: the model gateway package from GO-10
  - Work: Conditional. "Go tracks the trusted provenance of task input and model-visible tool results"
    and attaches it to the internal report. If `internal report rendering` chooses server rendering
    only, this task becomes "Dropped: `internal report rendering` chose server rendering only".
  - Done when: the internal report's stored context manifest lists every source the model saw.
  - Tests: a database-backed test through the X-24 command.
  - Report: "Report provenance and inherited restrictions"; "Live demonstration storyboard and proof
    checks" (beat 3)
  - Blocked by: `internal report rendering`; `report storage`

### Enforcement (report role: Implementer 4)

- [x] **GO-28 · Check resource relationships and destinations at the gate**
  - **Report 1.1 change:** `create_report` needs a permitted fixed template; `queue_report` an authorized report and recipient; the export restriction itself is GO-64.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: GO-15 · Needs: X-06, X-09, X-20, X-33 · Provides: nothing
  - Paths: the enforcement package from GO-12
  - Work: Go beyond the tool name. `read_vendor` accepts only a vendor of the organization that is
    associated with a passport-scoped invoice; `create_report` only passport invoice references and
    the registered template; `queue_report` only a report created in this run from authorized
    references, and only the trusted recipient reference the passport resolved from the directory,
    never "a destination found in invoice prose". Read access and outbound access are checked
    independently, and every action must fit the passport and pass the current checks: "The
    passport is an upper bound, not an instruction that every allowed operation should occur."
  - Done when: every argument relationship in "Proposed tool argument boundaries" and "Task
    relationships matter" is checked at the gate for all four tools, so a denied proposal never
    reaches an adapter (hours 6-10: "scoped reads ... destination checks").
  - Tests: a vendor not linked to a passport invoice, a vendor of another organization, a report of
    another run, an invoice reference outside the passport, a recipient taken from invoice text and
    an unregistered template are each denied with their reason code; permission to read a field
    does not make it allowed in outbound content; an unreadable directory record denies.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Report: "Trusted authority and passport invariants" (Task relationships matter); "Illustrative
    passport and interface contracts" (Proposed tool argument boundaries; Concrete synthetic
    business example); "Relative implementation milestones and critical dependencies" (Hours 6-10)
  - Blocked by: nothing
  - Completed (2026-10-03): `internal/policy` (1eb72bb; the report check became GO-64's export verdict in 95dcc43). `read_vendor` needs a vendor of the organization linked to a passport invoice; `create_report` a registered and passport template and passport sources; `queue_report` a passport recipient reference that names this run; an unreadable relationship or a missing reader denies. Checks: `TestGateChecksArgumentRelationships` 12 cases PASS; `pnpm test:db gateway` 292 passed, 0 failed, 0 skipped at 1eb72bb with `TestPostgresRelationships` against real demo rows (vendor and report of another organization included); `pnpm verify` 6/6. Outbound use of a readable field is the report restriction of GO-64.

- [ ] **GO-29 · Return structured denial feedback and stop at the correction limit**
  - **Report 1.1 change:** Tier A (the slice's safe continuation). After an export denial the feedback names `vendor_reconciliation_v1` only when the passport permits that template and its source set, and grants no new authority. Proof: the blocked export continues to the newly rendered vendor report in the same passport and run; exhausted or unauthorized recovery stops. Report: MVP Bounded recovery; Figure 6; beat 7.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-11, GO-15, GO-27, GO-36, GO-64, GO-65 · Needs: X-06, X-13, X-16, X-34 · Provides: X-67 (part: continuation and stop)
  - Paths: the enforcement package from GO-12; the worker package from GO-08
  - Work: After a denial, give the model a safe structured reason (reason code, safe message and,
    where one exists, the permitted alternative such as the registered vendor or the permitted
    record set), count the correction against the passport's correction limit, and continue at the
    run check, or stop the run with its reason when no correction remains (Figure 6: "Rejected or
    stale proposals receive bounded handling"). The feedback never carries protected values or
    review content. Then prove beat 6 over the X-34 fixtures through the real gate, executor and
    adapters: a prohibited proposal, from the live model where it proposes one and otherwise from
    the labelled replay (GO-36), is denied, and the same run continues to the permitted
    reconciliation and its stored report; capture the correction count and the run's recorded
    usage against its one passport's limits for the storyboard (X-16).
  - Done when: MVP requirement Bounded recovery holds, "Return structured denial feedback while
    limiting correction attempts", with its evidence "A recoverable branch continues; repeated
    forbidden proposals terminate at the limit.", and X-67 is reached (demo beat 6): "The original
    useful output is completed within the same run and allowance; correction attempts are counted."
  - Tests: with a labelled provider test double: a denied proposal followed by a permitted one
    completes the run with one counted correction; repeated forbidden proposals stop the run exactly
    at the X-06 correction limit with the recorded reason; no feedback text holds a protected value.
    The beat 6 scenario asserts one run and one passport from the denied proposal to the stored
    report, the counted correction, usage within that passport's limits and the replay label on a
    replayed proposal, and its output can be retrieved as evidence.
    `pnpm --filter gateway run test`; database-backed cases and the scenario through the X-24
    command.
  - Report: "The enforcement loop and data minimization"; "Functional requirements MVP boundary and
    deferred scope" (Bounded recovery); "Live demonstration storyboard and proof checks" (beat 6)
  - Blocked by: nothing
  - Progress (2026-10-03): the policy side is done (18402ab): `BuildDenialFeedback` (fixed safe message, the vendor template alternative only when the passport permits it and its sources, the passport's invoice references after an out-of-scope read, no protected value), `CorrectionCounter.CorrectionsUsed` from the run's durable denial events (malformed proposals included) and `CheckCorrections`. Checks: unit tests PASS; `pnpm test:db gateway` 439 passed, 0 failed, 0 skipped with `TestCorrectionCounterCountsEveryDenialOfTheRun`; `pnpm verify` 6/6. Missing: f3's loop calling them and stopping the run, and the beat-6 scenario through the labelled replay (GO-36).

- [x] **GO-64 · Deny a restricted report export at the gate before approval**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-15, GO-28, GO-63 · Needs: X-13 · Provides: nothing
  - Paths: the enforcement package from GO-12
  - Work: For `queue_report`, before the policy decision, verify the artifact's content hash and
    manifest and its restriction against the destination. A forbidden export is
    `report_export_restricted`, a denial and not an approval request, with its safe event.
  - Done when: an Internal only report proposed for Atlas is denied before review, no approval request
    and no outbox row exist, and the event names the rule.
  - Tests: unit tests for the destination rule; a database-backed test through the X-24 command.
  - Report: "Report provenance and inherited restrictions" (Export checks and safe continuation);
    "Functional requirements MVP boundary and deferred scope" (Denied export)
  - Blocked by: nothing
  - Completed (2026-10-03): for `queue_report`, after the resource checks and before the approval rule, the gate asks `provenance.LoadReport`, `provenance.CurrentInvoiceVersions` and `provenance.AuthorizeExport` for the registered vendor recipient in one read-only transaction (95dcc43). A refused export is a deny (`report_export_restricted`, `report_lineage_missing`, `resource_version_changed`), never an approval request, never reaches the semantic check, and its `report.export_denied` event names the reason with `lineageCheck` and the `vendor_reconciliation_v1` alternative (c07ca60). Checks: `TestRestrictedExportIsDeniedBeforeReview` PASS; `pnpm test:db gateway` 317 passed, 0 failed, 0 skipped at 95dcc43 with reports stored through `provenance.StoreReport` (internal restricted, no lineage, other run or organization, stale source version); `TestRecorderWritesAnExportDenialEvent` PASS at c07ca60; `pnpm verify` 6/6. Not verified here: the whole path with no outbox row, which comes with the approval flow (GO-44, GO-45); Worker 2's adapter re-checks at effect time.

- [x] **GO-66 · Prove the denied internal export on the Go side**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-27, GO-36, GO-64 · Needs: X-16, X-34 · Provides: X-72
  - Paths: none (a scenario test in the packages above)
  - Work: The live model or a labelled replay proposes `queue_report` of the internal report to Atlas;
    capture the source manifest, the label, the denial rule and the outbox count.
  - Done when: the evidence X-72 names is captured: "Trusted source manifest and label; export denial
    rule; unchanged outbox count."
  - Tests: the scenario test with the before and after outbox count quoted.
  - Progress (2026-10-03): W2 lane, branch go/w2, 82c6aec, direct adapter path, not the agent loop:
    X-72 evidence captured (label internal_only, the source manifest, rule
    report_export_restricted, outbox rows before 0 after 0). Missing: the rerun with the live model
    or a labelled replay proposing the action.
  - Completed (2026-10-03): W2 lane, branch go/w2, package `internal/scenario`.
    `TestStoryThroughTheProductionChain` drives the story through `agent.NewProductionChain` from
    real admission, with a scripted fixture model provider (labelled: no model; verdicts stored
    as `fixture`). Its fifth step proposes `queue_report` of the internal report to the registered
    vendor: the report is labelled `internal_only`; the source manifest is invoice A01 (internal
    note, `internal_only`) and A02 (`vendor_shareable`), template v1; the action is denied with
    `report_export_restricted` (one `report.export_denied` event); outbox rows 0 before and 0
    after; the next model request carries the reason and the vendor template alternative.
    `TestLiveStoryThroughTheProductionChain` (`-tags=model_live`, `qwen3.5:4b`, labelled live) ran
    the same chain twice; the model did not propose the internal export either time (run 1: read,
    vendor report, prose answer; run 2 with GO-26's final-result check: read, vendor report, a
    queue_report denied by `semantic_injection_detected`, a second one awaiting approval), so the
    denial on the agent path rests on the labelled scripted provider. Both live runs: outbox 0,
    no internal note in a vendor report, every verdict labelled live.
  - Report: "Validation plan and evidence matrix" (Inherited restriction); "Live demonstration
    storyboard and proof checks" (beat 5); challenge concern Sensitive data exposure
  - Blocked by: nothing

- [ ] **GO-72 · Check the active catalog revision before every evaluation and dispatch**
  - **Report 1.2 change:** Figure 3 names this module the "Trusted active snapshot loader"; the external signature feed reaches it.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-15, GO-19 · Needs: X-79, X-80, X-81 · Provides: X-82
  - Paths: the enforcement package from GO-12; `services/gateway/internal/health`
  - Work: Read the active-version pointer before new evaluations and dispatches "rather than relying indefinitely on a stale cache", and record both the admission and the evaluated revision in each decision. Optional settings apply at the next evaluation; removed models and lowered budgets apply as current restrictions; raised limits never exceed the passport. "With no valid initial catalog, the gateway is not ready and cannot dispatch work" (a readiness contract change through SH-14).
  - Done when: a changed optional threshold changes the next decision and the decision records the new revision, while the passport ceiling stays unchanged.
  - Tests: database-backed tests through the X-24 command for a threshold change, a lowered budget and a raised budget.
  - Progress (2026-10-03): `internal/catalog` is the trusted active snapshot loader.
    `Loader.Active(ctx, querier)` reads the active pointer on every call (in the caller's
    transaction when given one), joins the active revision and the bound signature-feed revision,
    and returns one snapshot: revision id, feed revision id, the limits (re-checked) and c1's
    `security.SettingsFromCatalog` settings; it memoizes parsing only per immutable revision pair.
    Anything missing or invalid is `ErrUnavailable` (no active revision, signature matching without
    a feed, a feed revision other than the policy's, invalid limits, unknown disabled rules).
    `EffectiveFor(passport, snapshot)` narrows the passport by the active catalog (removed models,
    lowered budgets and disabled templates apply at once; raised values never exceed the passport)
    and carries the admission and evaluated revision ids for decisions. Tests (PostgreSQL, isolated
    revisions and feed in a rolled-back transaction): a threshold change is in the next snapshot
    with the new revision id; five fail-closed cases; a disabled signature control needs no feed;
    lowered and raised catalogs against a passport. Checks: gateway five checks PASS; `go test -race
./internal/catalog` ok; `pnpm verify` 6 passed. Missing half: the gate (lane w3), the worker
    and model path (f3) and the security controls (c1) must call `Loader.Active` before every
    evaluation and dispatch and record both revisions; the readiness change ("no valid catalog,
    not ready") waits on a readiness contract change; no path imports the signature feed yet, so
    the active snapshot is unavailable while the policy enables signature matching.
  - Report: "Central policy configuration and safe reload"; "Trusted authority and passport invariants"; "Relative implementation milestones and critical dependencies" (Hours 6-10)
  - Blocked by: nothing

- [x] **GO-77 · Apply the semantic risk check to otherwise permitted action proposals**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-3 h, this roadmap's estimate)
  - Depends on: GO-15, GO-75 · Needs: nothing · Provides: nothing
  - Paths: the enforcement package from GO-12
  - Work: Figure 6: fast argument, scope and report-lineage checks reject forbidden actions first; otherwise applicable proposals pass the configured deterministic content controls and a budgeted semantic check before allow or exact review. "A semantic verdict cannot authorize a restricted export." Record the assessment with the proposal without exposing classifier input in the ordinary timeline.
  - Done when: a forbidden action is denied with no semantic request, and a permitted action's semantic block stops it before review.
  - Tests: gate tests with a permissive and a blocking stubbed verdict (labelled as stubs).
  - Report: "Architecture and chart reading guide" (Figure 6); "The enforcement loop and data minimization"
  - Blocked by: nothing
  - Completed (2026-10-03): policy side in `internal/policy/security_check.go` with c1's `security.Inspector.EvaluateAction` (c1 commit 64a8278, merged from go/c1). The gate calls it only after a deterministic allow or approval requirement; `no_objection` keeps the outcome, `block` denies with the control's reason before any review material is frozen, `pause` or a failure denies (`security_evaluator_unavailable`), which the worker treats as a pause; settings come from `security.SettingsFromCatalog` for the evaluated revision; every control record is written to `runtime.control_assessments` in the decision transaction. Checks: `TestDeterministicDenialRunsNoSecurityCheck` (a forbidden action gets no security check), `TestStubbedSemanticBlockStopsAPermittedActionBeforeReview` (labelled stub verdict; nothing frozen), `TestSemanticCheckRestrictsButNeverGrants` and `TestSecurityActionCheckThroughTheGate` with the real inspector and the real sample feed (clean no objection, signature block, missing evaluator and unloadable settings pause, review never skipped) PASS; `pnpm test:db gateway` 565 passed, 0 failed, 0 skipped including `TestSecurityRecordsAreWrittenWithTheDecision`; `pnpm verify` 6/6. Not verified here: a live semantic verdict and its `security_model_call_id` foreign key (needs the metered evaluator wired by the worker lane), and `CatalogSecuritySettings` against an imported catalog revision in the database.

- [x] **GO-78 · Match the signature-feed rules**
  - **Report 1.2 change:** Figure 6 applies the known-signature checks to action proposals too ("Fast typed schema scope and known-signature checks"), and Figure 10 to tool results ("Fast field size and signature checks").
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-72, GO-76 · Needs: X-87, X-88 · Provides: nothing
  - Paths: the enforcement package from GO-12
  - Work: Apply the active feed's rules (pattern type, scope, response) to normalized text, for example `prompt_ignore_previous_v1` on `tool_result_text` with a configurable block response. Rules carry no executable code, unsafe regex constructs or loading paths. Record the feed revision, digest and matched rule in each decision.
  - Done when: the sample rule blocks its phrase, and after a feed revision that disables it the same input is decided under the new revision.
  - Tests: unit tests for the supported grammar; a test that rejects an unsupported rule.
  - Report: "Hybrid security controls and managed attack signatures" (Trusted historical attack feed)
  - Blocked by: `feed grammar and trust`
  - Completed (2026-10-03): `ParseFeed`, `MatchSignatures`, `NormalizeText` and `SettingsFromCatalog` (b456a3f; field-limit order a0672e9): closed feed grammar of `normalized_substring` rules with `block` response, no regex or code; feed bytes pinned to `file_digest`; first enabled, not-disabled rule in feed order blocks with `signature_match`, recording rule, feed revision, digest and catalog revision; an enabled guard without a feed is an error. Checks: gateway checks all exit 0, including the sample rule blocking its phrase (case, whitespace and zero-width variants), the same input passing under a later revision that disables it, a new feed revision's rule hit, and rejection of regex, unnormalized, duplicate and unknown-key rules; `pnpm verify` 6 passed. Reading the feed row from PostgreSQL is 3c's GO-72 loader, which calls `SettingsFromCatalog`.

### Tool adapters, provenance and rendering (report role: Implementer 5)

- [x] **GO-31 · Build `read_vendor`**
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: GO-17, GO-35 · Needs: X-20 · Provides: nothing
  - Paths: the adapter package from GO-17
  - Work: The adapter takes `vendor_id` only, checks itself that the vendor belongs to the
    organization and is associated with a passport-scoped invoice, and returns the GO-07 fields
    with selected values as safe references (GO-35).
  - Done when: the tool argument boundary for `read_vendor` holds: "Verify the vendor belongs to the
    task; resolve selected values as safe references."
  - Tests: database-backed, called directly and through the executor: a vendor of another
    organization and a vendor not linked to a passport invoice return an error and no data; the
    result holds exactly the allowlisted fields. The X-24 command.
  - Completed (2026-10-03): W2 lane, branch go/w2, 0723e57. read_vendor checks passport vendorIds,
    the organization and a link to a passport-scoped invoice; returns id, version, name and the
    recipient reference only. Tests: no address in the result; another organization's, an unlinked
    and an unlisted vendor fail with resource_out_of_scope. Checks: gateway checks PASS; go test
    -race against my migrated PostgreSQL 17 (w2_check, 127.0.0.1:55435): pass.
  - Report: "Illustrative passport and interface contracts" (Proposed tool argument boundaries);
    "Trusted authority and passport invariants" (Task relationships matter)
  - Blocked by: nothing

- [x] **GO-32 · Build `create_report`**
  - **Report 1.1 change:** Rewritten: takes "Scoped source references and registered template identifier"; resolves trusted source records and versions, renders the permitted template, and derives and persists provenance, classification and content hash through GO-63, inside GO-34's transaction. `internal_investigation_v1` is always Internal only with all consumed sources' restrictions. The model never selects the classification; vendor rendering is GO-65. Tests add: a model-supplied classification or source list is ignored; missing lineage stores nothing.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-17, GO-34, GO-63 · Needs: X-33, X-68, X-69 · Provides: nothing
  - Paths: the adapter package from GO-17
  - Work: The adapter takes authorized invoice references and a registered template, checks every
    reference against the passport itself, and stores a structured report with a stable identifier,
    a version, the organization, the run and its source references, inside GO-34's transaction. The
    model selects and summarizes permitted information; it never writes free outbound content. If
    SH-10 records the registered template as Go code, this task defines it in the adapter package,
    with fields within the X-06 field rules; otherwise the adapter reads the record SH-25 seeds.
  - Done when: the tool argument boundary for `create_report` holds: "Verify every reference; store a
    structured report with a stable ID and version." (hours 6-10: "structured report creation").
  - Tests: database-backed: an unauthorized invoice reference or an unregistered template stores
    nothing; a stored report carries its references, version and run; the same action identifier
    executed again creates no second report. The X-24 command.
  - Completed (2026-10-03): W2 lane, branch go/w2, d18536f. create_report: registered template in
    reportTemplates, scoped sources of the organization, server rendering, classification and
    lineage through provenance.StoreReport in the executor's transaction; a model classification
    argument is refused; a retry under the same action hits reports_one_per_action. Tests
    database-backed. Checks: gateway checks PASS; go test -race against my migrated PostgreSQL 17
    (w2_check, 127.0.0.1:55435): pass.
  - Report: "Illustrative passport and interface contracts" (Proposed tool argument boundaries);
    "Functional requirements MVP boundary and deferred scope" (Product decisions that keep the MVP
    coherent)
  - Blocked by: `record versions`; `internal report rendering` (internal body only)

- [x] **GO-33 · Build `queue_report`**
  - **Report 1.1 change:** Rewritten: no rendering. Checks the immutable report against its content hash and manifest, inherited restrictions, source and template versions, exact content and review, and queues the exact reviewed permitted report.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 3-5 h)
  - Depends on: GO-32, GO-34, GO-35 · Needs: X-33 · Provides: nothing
  - Paths: the adapter package from GO-17
  - Work: The adapter takes a stored report reference and a trusted recipient reference. It checks
    that the report belongs to this run and organization with the expected source references,
    template and version ("A generated report ID is not permission to queue an unrelated report"),
    renders the exact content from the stored report and its registered template (GO-43 freezes
    it), resolves the recipient reference after its own checks, and inserts one simulated outbox
    row whose uniqueness is tied to the action identifier. Nothing is delivered to a real recipient,
    and the outbox is labelled simulated. It executes only under the bound approval (GO-45).
  - Done when: the tool argument boundary for `queue_report` holds: "Render and freeze exact content;
    require bound review; insert a single simulated outbox effect."
  - Tests: database-backed, called directly: an unrelated report, a report of another organization,
    a changed version and an untrusted recipient insert nothing; identical inputs render identical
    content; a second insert for the same action identifier is rejected by the database and leaves
    one row. The X-24 command.
  - Completed (2026-10-03): W2 lane, branch go/w2, 528cf65 (events b838586). queue_report: the
    report of this run and organization, the recipient resolved inside the adapter, the export
    decided from stored lineage and current source versions, one simulated outbox row with the
    stored content hash. Tests: the internal report to the correct Atlas reference fails with
    report_export_restricted (outbox unchanged, report.export_denied and
    report.safe_template_offered events); the vendor report gives one row to
    reports@atlas.example.com with the matching hash; a retry hits outbox_messages_one_per_action;
    unrelated report, raw address, other run or organization and a changed source are refused.
    Checks: gateway checks PASS; go test -race against my migrated PostgreSQL 17 (w2_check,
    127.0.0.1:55435): 22 passed. Approval binding is the executor's GO-45.
  - Report: "Illustrative passport and interface contracts" (Proposed tool argument boundaries;
    Concrete synthetic business example); "Illustrative invoice scenario and future domain
    adaptations" ("No message would be delivered to a real recipient")
  - Blocked by: `record versions`

- [x] **GO-34 · Commit each demo effect with its execution record and event in one transaction**
  - **Report 1.1 change:** Report content and lineage commit atomically, "so an artifact cannot exist without its restrictions"; that replaces the old first-deliverable quote.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: SH-06, GO-16, GO-19, GO-22 · Needs: X-33 · Provides: nothing
  - Paths: `services/gateway/internal/database/database.go`; the executor package from GO-16
  - Work: For `create_report` and `queue_report`, the effect, its completion record and its event
    commit in one short transaction on the connection SH-06 settles ("one Go executor connection"
    is the report's recommendation, not adopted), with the uniqueness constraint tied to the action
    identifier. "Separate connections with different credentials would not provide that atomicity
    automatically." Until the service roles arrive (X-35, GO-38) the single starter user serves this
    connection; GO-55 repeats the proof on the final one. The report gives these transactional
    effects to Implementer 5; on this team they are the Go implementer's.
  - Done when: Implementer 5's first integrated deliverable is observed: "A permitted action
    produces one inspectable database effect and its matching execution record."
  - Tests: database-backed: a successful effect has exactly one completion record and one event; an
    effect executed twice under one action identifier leaves one row and one completion; the
    fault-injection proof follows in GO-55. The X-24 command.
  - Completed (2026-10-03): W2 lane, branch go/w2, 24d8162. Report, lineage, attempt completion and
    event (and queue_report's outbox row) go through the executor's one transaction (decision 2).
    Test with a savepoint as the executor transaction: after rollback
    report/lineage/completion/event = 0/0/0/0, after commit 1/2/1/1. Checks: gateway checks PASS;
    go test -race against my migrated PostgreSQL 17 (w2_check, 127.0.0.1:55435): pass. Ran as the
    starter owner; the gateway-role proof is GO-38.
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Delivery scope and six person
    ownership" (Proposed team ownership)
  - Blocked by: `decision 2 in docs/product/README.md`

- [x] **GO-35 · Replace protected values with opaque references resolved inside adapters**
  - **Report 1.1 change:** The note is readable but not exportable.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: GO-17, GO-23 · Needs: X-06 · Provides: nothing
  - Paths: the adapter package from GO-17; the data-minimization package from GO-23
  - Work: For each value the X-06 field rules protect, the adapter returns an opaque reference that
    the model can use without receiving the value, and resolves it only inside an adapter after
    authorization, for example the recipient address in `queue_report`. A reference is scoped to
    its run, so it means nothing in another run. Tier A because `read_vendor` (GO-31) and
    `queue_report` (GO-33), both Tier A, depend on it to resolve their safe references and the
    trusted recipient.
  - Done when: "Opaque references can identify protected records without exposing their raw
    contents; adapters resolve those references only after authorization."
  - Tests: no protected value appears in any tool result or model request of a fixture run; a
    reference from another run or organization does not resolve; an adapter resolves a reference
    only after its own checks pass. `pnpm --filter gateway run test`; database-backed cases through
    the X-24 command.
  - Completed (2026-10-03): W2 lane, branch go/w2, 1926f89 and 3828ad6. The address leaves the
    database only as a run-scoped reference, resolved inside queue_report (resolveRecipient) after:
    same run, listed in recipientReferences, vendor scoped, in the organization, linked and
    addressed; `tools.ResolveRecipientForReview` exports the same check for GO-43. Tests: another
    run, another organization, no address, unlisted, malformed and a raw address are refused; no
    read result holds the address. Checks: gateway checks PASS; go test -race against my migrated
    PostgreSQL 17 (w2_check, 127.0.0.1:55435): pass.
  - Report: "The enforcement loop and data minimization" (Minimize information before it enters the
    model); "Terminology for developers and presenters" (Opaque reference)
  - Blocked by: nothing

- [x] **GO-63 · Build the report provenance module**
  - Owner: Go implementer (report role: Implementer 4, enforcement, and Implementer 5, provenance) · Tier: A · Size: M (estimate 4-8 h)
  - Depends on: GO-13, GO-15, GO-19, GO-22 · Needs: X-06, X-08, X-68, X-69, X-70, X-71 · Provides: nothing
  - Paths: a new package, named at M0 by the Go implementer (architecture proposal: `internal/provenance`)
  - Work: Resolve trusted source records and versions under the passport, derive the classification
    (the internal template is always Internal only; an Internal only source context never becomes
    Vendor shareable through a model label), validate template and projection versions, authorize
    creation and export, and persist the lineage. Fail closed on "Missing, unclassified or unresolved
    lineage".
  - Done when: every report creation and export decision reads the stored lineage, and a report
    without complete trusted lineage is neither created nor exported.
  - Tests: unit tests for the derivation rules; a database-backed test through the X-24 command that a
    model-declared label, a title and a missing source each fail closed.
  - Completed (2026-10-03): W2 lane, branch go/w2, e809fee. `internal/provenance`: Go-registered
    templates and projection (versions recorded in the lineage), DeriveClassification from trusted
    sources only, StoreReport (report and lineage in one transaction, nothing without lineage),
    LoadReport, AuthorizeExport from the stored lineage only. Decisions applied: `report storage` =
    runtime.report_lineage (migration 1791060000000, 8032829), `source classification storage` =
    demo.invoices.internal_note_classification, `internal report rendering` = deterministic server
    rendering. Tests: derivation and export rules; a model label, a "Public summary" title and
    missing lineage each fail closed (database-backed). Checks: gateway checks PASS; go test -race
    against my migrated PostgreSQL 17 (w2_check, 127.0.0.1:55435): 8 passed.
  - Report: "Report provenance and inherited restrictions"; "Functional requirements MVP boundary and
    deferred scope" (Trusted source classifications, Inherited report restrictions); "Validation plan
    and evidence matrix" (Inherited restriction, Missing lineage, Label and rename tampering)
  - Blocked by: `report storage`; `source classification storage`

- [x] **GO-65 · Render the vendor report from the approved projection**
  - Owner: Go implementer (report role: Implementer 5, rendering) · Tier: A · Size: S (estimate 3-5 h)
  - Depends on: GO-32, GO-63 · Needs: X-06, X-68 · Provides: nothing
  - Paths: the adapter package from GO-17
  - Work: Render `vendor_reconciliation_v1` directly from the named projection of authorized database
    fields, in a fixed format with typed fields, with no model prose, internal report text or raw
    values. Identical inputs give identical bytes. Store the projection rule and its version with the
    report.
  - Done when: the vendor report holds only the approved fields, and the internal note's text is
    absent from it.
  - Tests: a golden-bytes test for identical inputs; a test that the internal note never appears.
  - Completed (2026-10-03): W2 lane, branch go/w2, eefd0d2. vendor_reconciliation_v1 rendered only
    from vendor_invoice_fields_v1 v1 (invoice reference, external reference, duplicate flag,
    currency, total, due date; lead's `vendor projection fields`), one vendor per report, no note.
    Tests: golden bytes and order independence; the stored vendor report is vendor_shareable with
    projection v1 in its lineage and no note text. Checks: gateway checks PASS; go test -race
    against my migrated PostgreSQL 17 (w2_check, 127.0.0.1:55435): pass.
  - Report: "Report provenance and inherited restrictions" (Fixed prototype classification and
    template matrix); "Functional requirements MVP boundary and deferred scope" (Trusted template
    manifests); "Validation plan and evidence matrix" (Approved external projection)
  - Blocked by: `vendor projection fields`

- [x] **GO-67 · Prove the approved external projection on the Go side**
  - Owner: Go implementer (report role: Implementer 5, rendering) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-29, GO-65 · Needs: X-16, X-34 · Provides: X-75
  - Paths: none (a scenario test in the packages above)
  - Work: After the denial, the vendor report is rendered; capture its content, the selected fields,
    and the source, template and projection versions; internal free text is absent.
  - Done when: the evidence X-75 names is captured: "Serialized report content, selected fields,
    source versions, template and projection versions."
  - Tests: the scenario test with the serialized content quoted.
  - Progress (2026-10-03): W2 lane, branch go/w2, 82c6aec, direct adapter path, not the agent loop:
    X-75 evidence captured (per-source version, classification and fields; template v1, projection
    vendor_invoice_fields_v1 v1; the serialized content without note text). Missing: the rerun on
    the agent path after the denial.
  - Completed (2026-10-03): on the agent path (`TestStoryThroughTheProductionChain`, labelled
    scripted provider), after the denial the model's next step creates the vendor report:
    `vendor_shareable`, lineage per source (version 1, fields invoice_id, external_reference,
    duplicate_reference, currency, total_minor_units, due_on), template `vendor_reconciliation_v1`
    v1, projection `vendor_invoice_fields_v1` v1; the serialized content lists INV104 twice with
    the duplicate flag and holds no internal free text (the test quotes it). The live run also
    produced a vendor report without the note.
  - Report: "Validation plan and evidence matrix" (Approved external projection); "Live demonstration
    storyboard and proof checks" (beat 7)
  - Blocked by: nothing

### Modules the report's team table does not name (Go implementer)

- [x] **GO-36 · Replay a prohibited proposal through the real gate, labelled**
  - **Report 1.2 change:** The replay stays labelled and is supplementary: "the delivered semantic control and its real-model tests must still work".
  - **Report 1.1 change:** The stored proposals are the export of a genuinely created internal report and the out-of-scope read or redirect. Tier A: the denied export is in the slice, and the report asks for a labelled replay when the live model does not propose the send.
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 2-3 h)
  - Depends on: GO-05, GO-15 · Needs: X-12, X-34 · Provides: X-36
  - Paths: `services/gateway/README.md`; a new package or command for the replay, named at M0 by the Go
    implementer
  - Work: Implement GO-05's outcome: submit a stored prohibited proposal taken from the hostile note
    in X-34 (reading the out-of-scope invoice or changing the recipient) "to the same validation and
    execution path", marked as a replay in the stored action and in every event it produces, and
    documented as a deterministic rehearsal.
  - Done when: X-36 is reached: the replay is denied by the real gate, leaves no business effect,
    and every record of it carries the replay label ("never present a scripted proposal as a
    model-generated action").
  - Tests: the replayed proposal runs through the same gate and executor code as a live one, with no
    test-only branch, and is denied with the same reason code as its live equivalent; its events
    carry the replay label. `pnpm --filter gateway run test`; database-backed cases through the
    X-24 command.
  - Report: "Live demonstration storyboard and proof checks" (Reliable demonstrations without
    invented behavior); "Risk register and scope controls" (Provider instability or unsuitable
    output)
  - Blocked by: `replay entry`
  - Completed (2026-10-03): migration `1791140000000-AddActionReplaySource` and `internal/policy/replay.go`: `ReplayProposal` turns each hostile note of X-34 into the prohibited proposal a model would make if it obeyed it, labelled `labelled_replay:<fixture id>` on the stored action and on every event it produces; it goes through the production gate and executor with no replay branch, no provider call and no usage. Checks: fresh database, 18 migrations run, revert and re-run of 1791140000000 succeeded; `GOFLAGS=-p=3 pnpm test:db --fresh gateway` exit 0, 749 passed, 0 failed, 0 skipped with `TestLabelledReplayIsDeniedByTheRealGate` (evidence X-36: redirect record -> `resource_out_of_scope`, redirect recipient -> `destination_not_allowed`, internal disclosure -> `report_export_restricted`, each the same reason as its unlabelled live equivalent, the executor refuses, every event labelled, outbox 0, attempts 0, model calls 0), `TestReplayFixturesExistInTheHostileNotes` and `TestMalformedReplayLabelIsDenied`; `pnpm verify` did not pass: its test step failed on five apps/api gateway-client timing tests under machine load (they pass alone, 15/15), corrected in 7955b9f. The demo-triggerable entry `cmd/replay` (`-run <finished run> -fixture <id>`) followed: it submits the labelled proposal through the production chain's gate, never executes it, and prints `LABELLED REPLAY` with the decision and reason; `TestDemoReplayOfAFinishedRunIsDeniedAndLabelled` and `TestDemoReplayRefusesWithoutWriting` cover it, run against a test catalog without its feed it printed `deny / decision_unavailable` as UNEXPECTED, exit 1 (fail closed); with `config/attack-signatures.json` loaded by hand as the active feed (private `starter_test`, finished run d33cc6de) all three fixtures printed the expected denial through the production gate, exit 0: `resource_out_of_scope`, `destination_not_allowed`, `report_export_restricted`, each action and event labelled, 0 execution attempts, no model configured.

- [ ] **GO-37 · Serve the stored report, if `stored report read` chooses a Go endpoint**
  - **Report 1.1 change:** Serves the extended X-64 (classification, template and projection versions, content hash, destination class, lineage summary).
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-21, GO-32 · Needs: X-33 · Provides: X-64
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/README.md`
  - Work: Only if the document owner settles `stored report read` with a private Go endpoint: return
    the stored report (stable identifier, version, source references, structured content) and its
    registered template for the verified organization and run, so the interface "renders its
    substantive content from the stored report and registered template". If the view outcome is
    chosen, this task becomes "Dropped: `stored report read` chose a view and read grant (SH-24,
    SH-26)".
  - Done when: X-64 is reached through Go for the completed run's report.
  - Tests: a report of another organization or run is not returned; the response decodes strictly
    against its contract fixture; nothing beyond the template's fields is returned.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Progress (2026-10-03): W2 lane, branch go/w2, 90e3824: `provenance.StoredReportHandler` for
    `GET /internal/runs/{runId}/reports/{reportId}` with the ReportView (X-64 draft), organization-
    and run-scoped, internal content withheld without MayReadInternal; handler tests pass against
    w2_check. Missing: the route mount in internal/api (3c) and the X-64 TypeScript contract.
  - Report: "Illustrative passport and interface contracts" (Narrow final result and context
    boundary)
  - Blocked by: `stored report read`; `final result format`; `report storage`

- [x] **GO-38 · Connect with the Go database roles from X-35**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: SH-06 · Needs: X-35 · Provides: nothing
  - Paths: `services/gateway/internal/config/config.go`,
    `services/gateway/internal/config/config_test.go`,
    `services/gateway/internal/database/database.go`,
    `services/gateway/internal/database/database_test.go`, `services/gateway/cmd/gateway/main.go`,
    `services/gateway/README.md`
  - Work: Read the Go credentials SH-26 wires (one executor connection or separate connections, as
    SH-06 settles) as `logging.Secret` values, build the pool or pools, and give each Go module only
    the connection its writes need. The bootstrap user that runs the migrations stops being the
    gateway's user.
  - Done when: the gateway connects with its own role and a Go write outside its authority fails at
    the database ("Credentials and database roles must enforce the intended boundary").
  - Tests: configuration tests for the new variables (problems named, values never printed);
    database-backed: the Go role cannot write `app` records, and an adapter cannot write outside its
    `demo` tables. The X-24 command; `pnpm --filter gateway run test`.
  - Completed (2026-10-03): W2 lane, branch go/w2, 3c7f880 (grants) and this commit (connection). The
    gateway connects as `task_passport_gateway` with `POSTGRES_GATEWAY_PASSWORD` (logging.Secret,
    generated by `pnpm run setup`, gateway-only in dev and Compose) and refuses to start without it,
    naming the fix; `pnpm db:roles` (explicit, idempotent, owner, also run by `pnpm reset:demo`) sets
    the role's login password. Grant audit: no grant missing. Checks: config tests (problem named, no
    value echoed); a SET LOCAL ROLE test runs the Atlas scenario and a GO-55 fault as the role while
    app writes, invoice updates, DELETE, TRUNCATE and CREATE TABLE fail with permission denied;
    login from the host with the generated password works and a wrong one is refused; the gateway
    without the variable exits 1; `pnpm smoke` (host mode, dev stack on 3200/3201/8280, gateway on the
    role) 26 passed, 0 failed, 7 skipped (owner-password checks: my test container's password is
    under 16 characters; log checks: host mode); `pnpm verify` 6/6; `pnpm test:db gateway` 551
    passed, 0 skipped. Not run: `pnpm smoke --mode=container` (the shared Compose project `starter`
    would disturb the other lanes).
  - Report: "Architecture and chart reading guide"; "Data ownership and the transition from starter
    to product"; "Technical architecture and service ownership" (Proposed ownership)
  - Blocked by: `decision 2 in docs/product/README.md`

## M3: hours 10-14

- **Team focus (report 1.2).** "Add frozen action previews, authorized approval decisions, approval consumption, model/tool reservations, and controlled stopping. Finish shared/purpose token limits, request timeout and local concurrency enforcement, validated policy reload and last-known-good behavior."
- **Exit condition (report).** "The reviewed action executes once; changed content and depleted
  allowance cannot dispatch an operation."
- **Report 1.2 sync points (spine).** X-83.
- **Sync points needed by the end (spine):** X-39 to X-46, X-66 and X-67. This side provides X-40,
  X-42, X-45 and its halves of X-44 and X-46, X-41 if the read path chooses Go endpoints, and its
  half of X-53 early; X-67 came with GO-29 at M2. X-66 is the Next.js + NestJS side's.

### Agent runtime (report role: Implementer 3)

- [x] **GO-39 · Reserve model allowance before every dispatch and settle it afterwards**
  - **Report 1.2 change:** Reserve shared task allowance and the agent or security sub-budget atomically, including maximum output tokens and a concurrency slot; purpose is assigned by trusted runtime code, and security calls count against the shared ceiling. With GO-75 this delivers Implementer 3's report 1.2 first integrated deliverable: "A live agent call and live semantic check both reserve allowance and record independent purpose and latency."
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: M (estimate 5-8 h)
  - Depends on: GO-02, GO-03, GO-10 · Needs: X-06, X-11, X-39 · Provides: X-53
  - Paths: the model gateway package from GO-06; the runtime repository package from GO-19
  - Work: Before each model request, reserve the call count, the capped tokens and the estimated
    cost in one short row-locked transaction, "based on the configured model, estimated input and
    the permitted output ceiling"; with no allowance left, dispatch nothing and stop the run with
    its reason. Commit, send, then settle the reported usage in a later transaction. When usage is
    missing or a timeout leaves billing uncertain, the reservation stays unresolved. A model call
    retry, where `model call retries` adopts them, is a separate dispatch that reserves again. The
    usage view separates reported usage, reserved allowance and estimated cost; estimated cost is a
    second rule next to the hard limits, follows the decision 6 accounting rule and is labelled
    estimated.
  - Done when: MVP requirement Bounded usage holds for model calls ("Reserve allowance atomically
    before dispatch and count model calls, tool attempts, and corrections"), and the Go half of X-53
    is reached: Unknown usage, "Retained reservation and visibly uncertain estimated-cost state."
  - Tests: database-backed, also with `-race`: an exhausted allowance dispatches nothing and
    records the stop reason; a response without usage leaves the reservation unresolved and
    counted, never zero; settlement settles the reservation against the reported usage in its own
    transaction; no reservation transaction is open while the stubbed provider call runs; a model
    call retry, where `model call retries` adopts them, reserves again. The X-24 command.
  - Completed (2026-10-03): migration `1791130000000-AlignTokenLedger` applies alignment decisions 1
    to 5 (uuid run id with a foreign key to runs, organization id, call id = `model_calls.id` with
    the purpose bound, per-purpose token sub-limits and counters, call limits and counters, request
    timeout and concurrency slot). `budget.OpenRunLedger` copies every passport limit at admission;
    `Reserve` refuses before dispatch on a paused ledger, an exhausted shared or purpose call count
    or token allowance, a held slot or a missing dispatch record, and returns the request timeout
    that `model.AccountedCaller` applies; calls count at reserve and are never refunded; unknown
    usage keeps the reservation and the slot (`Snapshot` reports unresolved calls per purpose);
    `Settle` settles once in its own transaction. The loop's step count is the ledger's
    `agent_calls`; a held slot requeues. Tests: shared and purpose call limits, security token sub-
    budget, slot held through unknown usage and released by the late settlement, purpose binding,
    concurrent reservations, overrun and overflow pauses, late settlement once, no ledger lock
    during the provider call (`TestPostgresNoLedgerLockDuringTheProviderCall`). Migration: run,
    revert, run on a fresh database (18 migrations) and on a database holding an admitted run whose
    ledger had a settled and an unknown reservation plus an orphan diagnostic ledger: limits
    backfilled from the passport, counters rebuilt, dispatch records backfilled, the orphan deleted.
    Checks: gateway `format:check`, `lint`, `typecheck`, `test`, `build` exit 0; api `lint`, `typecheck` exit 0; `pnpm verify` 6 passed, 0 failed, 0 skipped; `GOFLAGS=-p=3 pnpm test:db`: gateway "741 passed, 0 failed, 0 skipped; 185 need the database", api "16 passed". Model call retries are not adopted
    (`model call retries`), so no retry reserves again; local inference has no tariff, so no
    estimated cost is recorded.
  - Report: "Atomic allowances hard limits and estimated cost"; "Validation plan and evidence
    matrix" (critical check Unknown usage); "Architecture and chart reading guide" (Figures 4 and 5)
  - Blocked by: `decision 6 in docs/product/README.md`; `dispatched attempts`

- [x] **GO-40 · Release the lease during a review wait and resume the original action**
  - **Report 1.1 change:** Figure 7; quote "Approval pauses the durable job and binds the original action."
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: GO-08, GO-11, GO-43, GO-44 · Needs: X-39 · Provides: nothing
  - Paths: the worker package from GO-08
  - Work: When the gate requires approval and GO-43 has frozen the action, persist the
    awaiting-approval state, write the event that tells the interface, and release the lease, all in
    one transaction. When GO-44 enqueues the continuation, claim it with a lease, load the original
    stored action (never a new proposal), run the run check (active, unexpired, not cancelled) and
    pass the action to the recheck in GO-45. On rejection or expiry, continue on the blocked-action
    path (GO-11; GO-29 adds the correction). No worker holds the job during the wait, so an approval
    nobody decides needs its own trigger: at its expiry it closes as expired (GO-44) and the run
    continues on the blocked-action path, by a mechanism the Go implementer chooses within constraint 9; the
    closure, its event and what resumes the run commit together.
  - Done when: an approval wait survives the browser closing and the worker stopping, and resumes
    the original stored action (Figure 6: "Approval pauses the durable job and applies to the
    original action"; MVP requirement Durable execution).
  - Tests: database-backed: a waiting run holds no lease; after a worker restart the continuation
    resumes the stored action with the same identifier and digest; a cancelled or expired run does
    not resume; a rejection executes nothing and continues on the blocked-action path, counted as a
    correction once GO-29 is done; an approval nobody decides closes as expired at its expiry with
    its event while no worker holds the job, and the run then continues on the blocked-action path,
    its persisted state showing the expiry, not awaiting approval. The X-24 command.
  - Completed (2026-10-03): the awaiting transition, its `run.awaiting_approval` event and the claim
    completion leave no lease during the wait; a continuation claim finds the decided action
    (w3's `DecidedActionFor`), moves the run to `running` with `run.resumed`, runs the run check and
    executes the original stored action through the executor's recheck; rejection and expiry
    continue on the blocked-action path as a counted correction; `agent.ApprovalExpiry` (started by
    `cmd/gateway`) closes undecided approvals every 5 s through w3's `ExpireOverdue`, which commits
    the closure, its event and the continuation together. Tests pass on PostgreSQL:
    `TestReviewWaitHoldsNoWorkerAndResumesTheApprovedActionAfterARestart` (no lease, same action id
    and digest, one outbox message, one `queue_report` action),
    `TestRejectedActionExecutesNothingAndIsACountedCorrection`,
    `TestUndecidedApprovalExpiresWhileNoWorkerHoldsTheRun`,
    `TestApprovedActionOfACancelledRunDoesNotResume`. Checks: gateway `format:check`, `lint`,
    `typecheck`, `test`, `build` exit 0; `pnpm verify` 6 passed; `GOFLAGS=-p=3 pnpm test:db` gateway
    "821 passed, 0 failed, 0 skipped", api "16 passed". The rejected reason is `approval_required` until 3c's
    `approval_rejected` reaches main.
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Exact action approval
    versioning and execution rechecks"; "Architecture and chart reading guide" (Figure 6)
  - Blocked by: nothing

- [x] **GO-41 · Persist cancellation through the internal cancel command**
  - **Report 1.1 change:** Figure 7; name proposal `POST /internal/runs/:id/cancel`.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: B · Size: S (estimate 2-3 h)
  - Depends on: GO-11, GO-21 · Needs: X-11, X-13 · Provides: X-42
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/README.md`; the worker
    package from GO-08
  - Work: Serve the cancel command named at M0, behind GO-21. It authorizes that the verified actor
    "may manage this run within this organization", persists the cancellation and its time before
    any future dispatch, closes a pending approval as cancelled (Figure 6: approve, reject, expire
    or cancel), and requests interruption of in-flight work where supported, without claiming that
    a provider stops billing. Cancellation reverses no committed effect, and the command writes no
    NestJS-owned revocation record; GO-52 reads those.
  - Done when: X-42 is reached: "Persist cancellation before future dispatches"; the run stops with
    its reason and its committed effects stay recorded.
  - Tests: a cancel of another organization's run, or by an actor without the right, is rejected and
    changes nothing; after a cancel no model request or tool execution starts, including for a run
    waiting for approval; a repeated cancel is harmless. `pnpm --filter gateway run test`;
    database-backed cases through the X-24 command.
  - Completed (2026-10-03): `POST /internal/runs/{runId}/cancel` in `internal/api`, behind the
    service token and the verified operator context; body empty or `{}`; answers 200 with the X-11
    run state, 404 for an unknown or another organization's run, 503 `decision_unavailable`
    otherwise. `repository.Tx.RequestCancellation` locks the run, stamps `cancel_requested_at` once
    with a `run.cancel_requested` event (additive X-12 type), and stops a run no worker is advancing
    (queued, awaiting approval, paused) at once as `stopped` / `run_cancelled` with `run.stopped`; a
    running run keeps its status and is stopped by the worker loop (lane f3), which, like the
    executor (lane w3), refuses every dispatch once the stamp is set. A finished run and a repeated
    cancel change nothing. No NestJS revocation record is written. Tests: repository (queued and
    awaiting-approval runs stop with both events; a running run is stamped once and a repeat adds
    nothing; a completed run is untouched; another organization's run changes nothing) and route
    tests with a labelled double plus a PostgreSQL route test (another organization 404, own run
    stopped). Checks: gateway five checks PASS; `go test -race ./internal/repository
./internal/api` with PostgreSQL ok; contracts `test` PASS; `pnpm verify` 6 passed. Not covered
    here: interrupting an in-flight model request (the loop stops at its next check), and the
    approval command refusing a decision on a stopped run, which is lane w3's GO-44 check.
  - Report: "Atomic allowances hard limits and estimated cost" (Cancellation and time limits);
    "Illustrative passport and interface contracts" (Proposed browser and runtime operations);
    "Exact action approval versioning and execution rechecks" (Versioned policy and current
    revocation)
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [x] **GO-42 · Prove the limit-triggered stop**
  - **Report 1.2 change:** The exhausted allowance may be the security sub-budget (`security_allowance_exhausted`).
  - **Report 1.1 change:** Beat 9; X-46 as amended in the spine.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-39, GO-45 · Needs: X-16, X-34 · Provides: X-46
  - Paths: none (a scenario test in the packages above)
  - Work: In a separate run with the small configured allowance from X-34, or in a clearly labelled
    runtime test, exhaust a model or tool limit. Show that the next request is rejected before
    dispatch with a visible terminal reason, and that every dispatch has its reservation and usage.
    The challenge table's evidence for Unpredictable costs asks to "Run a bounded retry scenario"
    with "usage and uncertainty recorded": if `model call retries` adopts bounded retries, exhaust
    the model call limit with retries against a labelled failing provider test double; otherwise
    the document owner revises that evidence (spine, "Scope changes"). Either way, show the
    reported usage, the reservations and any unresolved reservation.
  - Done when: the Go half of X-46 is reached: "The next request is rejected before dispatch; a
    terminal reason is visible and the ledger does not record an unaccounted call." (demo beat 9;
    the vertical slice's "one limit-triggered stop").
  - Tests: the scenario test counts dispatches against reservation and usage rows, checks the stop
    reason and that no request left Go after the limit; with bounded retries adopted, each retry has
    its own reservation and usage row. The X-24 command.
  - Report: "Live demonstration storyboard and proof checks" (beat 9); "Threat model limits and
    unresolved design choices" (smallest credible vertical slice); "Mapping the proposal to the
    Goldman Sachs challenge" (Unpredictable costs)
  - Blocked by: `model call retries` (the retry part only)
  - Completed (2026-10-03, lane w3): `internal/agent/limit_stop_postgres_test.go`, `TestModelLimitStopsTheRunBeforeTheNextDispatch`, a clearly labelled runtime test: a run admitted with an agent call allowance of 2 runs through lane f3's loop with the production stepper, accounted caller, Ollama client, token ledger and call log against a labelled provider double (an httptest server speaking the Ollama chat API, always proposing a permitted read). Evidence X-46: provider requests 2, `model_calls` 2 completed, reservations 2 settled with input and output usage, 0 unresolved, ledger used 98 reserved 0 tokens, run paused/`allowance_exhausted` with the `run.paused` event's safe message; the third request was rejected before dispatch. The tool-attempt limit is covered at the executor by `TestAttemptLimitIsEnforced`. Retry part (`model call retries` decided 2026-10-03 by the lead's delegate: the MVP makes no automatic model-call retries): `TestFailedModelCallIsNeverResent` uses a provider double that fails request 1 (HTTP 500) and would answer request 2; provider requests 1, `model_calls` outcome `usage_unknown`, the reservation held as `usage_unknown`, run paused/`outcome_unknown`, and a second claim of the job sends nothing. The security sub-budget variant is lane c1's `security_allowance_exhausted` evidence. Checks: see the commit.

- [x] **GO-79 · Enforce the model allowlist, request timeout and local concurrency cap**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-39, GO-72 · Needs: X-82 · Provides: nothing
  - Paths: the model gateway package from GO-10
  - Work: Reject a model alias outside the active catalog and the passport before dispatch (`model_not_allowed`). Bound each request by the configured deadline and hold a concurrency slot in the reservation. "A stalled request is cancelled where supported; a client timeout does not prove inference stopped, so the concurrency/usage record is reconciled conservatively."
  - Done when: an unlisted model, a third concurrent request over a cap of two, and a request over the deadline do not dispatch or are recorded as timed out with their reservation retained.
  - Tests: concurrency tests with `go test -race`; timeout tests with a slow stub provider (labelled).
  - Completed (2026-10-03): `agent.CatalogAccountedCaller` checks every call of either purpose
    before reserving or dispatching: the configured model must be in the active catalog's and the
    run passport's allowed models (`ErrModelNotAllowed`; the loop stops with `model_not_allowed`).
    Each request is bounded by the catalog's request time and the ledger's (GO-39). A process-wide
    cap of `local_max_concurrency` makes a further request wait for a slot within its deadline (a
    wait that runs out requeues the job); after a timeout the slot stays held one more request
    period and the ledger keeps the reservation and its per-run slot. Tests with a labelled HTTP
    provider double: `TestModelGatewayRefusesModelsOutsideCatalogOrPassport` (no hit, nothing
    reserved), `TestModelGatewayHoldsAThirdConcurrentRequestOverACapOfTwo` (3 runs, at most 2 at the
    provider, the third waits and completes), `TestModelGatewayDeadlineRetainsTheReservationAndSlot`
    (1 s catalog deadline; reservation unresolved, ledger and process slots held), with `-race`.
    Checks: gateway `format:check`, `lint`, `typecheck`, `test`, `build` exit 0; `pnpm verify` 6
    passed, 0 failed, 0 skipped; `GOFLAGS=-p=3 pnpm test:db`: gateway "744 passed, 0 failed, 0
    skipped; 188 need the database", api "16 passed".
  - Report: "Atomic allowances hard limits and estimated cost"; "Validation plan and evidence matrix" (Model allowlist and current reductions, Local model resources)
  - Blocked by: nothing

### Enforcement (report role: Implementer 4)

- [x] **GO-43 · Freeze the exact action for review**
  - **Report 1.1 change:** The frozen payload adds the report identifier, content hash, source manifest and its digest, classification, template and projection versions, exact recipient and exact outbound content: "Freeze the payload and bind its source records, template and projection to versions." The content is the stored server-rendered report.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 3-5 h)
  - Depends on: GO-12, GO-15, GO-33 · Needs: X-09, X-39 · Provides: nothing
  - Paths: a new package for the approval manager, named at M0 by the Go implementer
  - Work: Before requesting review, store "the tool, canonical arguments, recipient, affected
    resources, relevant versions, exact outbound content, passport reference, policy version and
    expiry", with the content `queue_report` renders and its digest, as the `exact reviewed
material` outcome says: "Freeze the payload, or bind its source records to versions and require
    a new proposal when they change." The exact content goes to restricted review storage, never
    into general events.
  - Done when: the stored action holds "the exact material reviewed" and a later change to any
    material field is detectable (MVP requirement Exact-action approval: "Bind review to immutable
    arguments, content, record versions, expiry, and a single action").
  - Tests: database-backed: the frozen record holds every listed field; a change to a source invoice
    after freezing is detected by the version comparison the outcome defines; the run's general
    events hold no review content. The X-24 command.
  - Report: "Exact action approval versioning and execution rechecks"; "Users operating model and
    proposed user journeys" (Journey 2 review an exact outbound effect)
  - Blocked by: `record versions`
  - Completed (2026-10-03): migration `1791080000000-AddReviewPayloads` (immutable `runtime.review_payloads`, gateway SELECT and INSERT) and `internal/policy/review.go` (7e844de). Before an approval request the gate freezes the tool, canonical arguments, passport, policy revision, the exact recipient (`tools.ResolveRecipientForReview` on the stored passport), and for `queue_report` the stored report with content, hash, template and projection versions, classification and its sorted source manifest with versions and digest; the review expires with the passport; `payload_digest` is SHA-256 over the canonical payload; no freezer or a failed freeze denies. `record versions` is exact integer equality (`StaleSources`). Checks: fresh database, 13 migrations run, revert and re-run of 1791080000000 succeeded; `pnpm test:db gateway` 461 passed, 0 failed, 0 skipped with `TestFreezeHoldsTheExactReviewedMaterial` (every field, stored digest equals recomputed, second freeze refused, UPDATE rejected), `TestSourceChangeAfterFreezeIsDetected`, `TestUnresolvableRecipientFreezesNothing` and `TestApprovalRequestEventHoldsNoReviewContent`; `pnpm verify` 6/6. The Go read endpoint for the review screen follows with GO-44.

- [x] **GO-44 · Accept the approval decision through the internal command**
  - **Report 1.1 change:** Name proposal `POST /internal/actions/:id/approval`; an approval cannot override an Internal only export denial.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: GO-21, GO-43 · Needs: X-10, X-13, X-18, X-39 · Provides: X-40
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/README.md`; the approval
    manager package from GO-43
  - Work: Serve the approval command named at M0, behind GO-21. It identifies the stored action and
    carries the decision only. Go checks the reviewer's authority for this action from trusted
    records ("A signed user identifier alone is insufficient"), the action's integrity and its
    expiry. Approve stores the grant and enqueues the original action in one transaction; reject
    closes the approval and continues on the blocked-action path (GO-11); an undecided approval
    closes as expired at its expiry (GO-40 provides the trigger while no worker holds the job). A
    replacement is a new bounded proposal, never a reuse of the closed grant, and an approval never
    turns a denied resource or destination into an allowed one.
  - Done when: X-40 is reached: "Go checks reviewer authority, action integrity and expiry, and
    stores the grant and the continuation in one transaction" (report: "When the reviewer approves,
    Go checks the reviewer's authority and persists the decision with a durable continuation").
  - Tests: a non-reviewer, a reviewer of another organization, an expired approval, an altered
    action and a body that carries a payload are each rejected and store no grant; an approval
    writes the grant and the continuation together or neither; a decision on a closed approval
    fails. `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Report: "Exact action approval versioning and execution rechecks"; "Illustrative passport and
    interface contracts" (Proposed browser and runtime operations); "Users operating model and
    proposed user journeys" (Journey 2 review an exact outbound effect)
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`
  - Completed (2026-10-03): `internal/policy/approvals.go` and `approval_handlers.go`, migration `1791110000000-GrantGatewayMembershipRead` (lead decisions: reviewer role `reviewer` in `app.memberships`, routes `POST /internal/actions/{actionId}/approval` and `GET /internal/actions/{actionId}/review`, the run state moved by f3's worker). Reviewer authority, action integrity, frozen payload integrity and expiry are checked; the approvals row, the action status, the `approval.decided` event and the continuation job commit in one transaction or not at all. Checks: fresh database, 15 migrations run, revert and re-run of 1791110000000 succeeded; `pnpm test:db gateway` 642 passed, 0 failed, 0 skipped with `TestReviewerApprovalStoresTheGrantAndTheContinuationTogether` (and a second decision fails), `TestRejectionClosesTheApprovalAndContinues`, `TestApprovalRefusalsStoreNoGrant` (non-reviewer, a reviewer claim without the membership role, reviewer of another organization, expired, altered action, an injected continuation failure: no grant, no job, still awaiting) , `TestFrozenReviewIsReadOnlyByReviewers` and `TestGatewayRoleReadsMembershipsButCannotWriteThem`; `TestApprovalHandlerAcceptsOnlyTheDecision` (a body with a payload or replacement content is 400 and reaches no decision) PASS; `pnpm verify` 6/6. Pending other lanes: 3c mounts the two handlers in `api.Commands` and X-10 `contracts.ApprovalDecision` (1c12eb9 on go/3c) replaces the handler-local type once on main.

- [x] **GO-45 · Recheck before execution and claim the attempt in one transaction**
  - **Report 1.2 change:** Figure 8: when the required action guard assessment is not current, the action returns to the budgeted semantic action check (GO-77) instead of executing.
  - **Report 1.2 change:** The recheck adds the active catalog revision and the required guard status: "required current semantic checks cannot be satisfied by a failed or stale assessment".
  - **Report 1.1 change:** The recheck adds current source and template policy and revocations, resource and report lineage preconditions and destination restrictions, with the new reason codes; Figure 8.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: GO-16, GO-40, GO-44 · Needs: X-39 · Provides: nothing
  - Paths: the executor package from GO-16
  - Work: Immediately before executing an allowed or approved action, recheck its integrity (the
    stored action against its digest), resource versions, run cancellation, expiry and
    preconditions. A failed recheck records a blocked action with its reason (`action_changed`,
    `resource_version_changed`, `approval_expired` or `run_cancelled` in the proposed vocabulary;
    X-13 governs), executes nothing and hands the action to the blocked-action path (GO-11), where
    GO-29 adds the bounded correction feedback. Then, in one short transaction, reserve the tool
    allowance, claim the attempt and validate the bound grant, consuming a new approval once per
    action. Current revocations join the recheck in GO-52.
  - Done when: the M3 exit holds on the Go side, "The reviewed action executes once; changed content
    and depleted allowance cannot dispatch an operation." ("An atomic execution claim consumes the
    approval once.")
  - Tests: database-backed: a changed digest, a changed resource version, an expired grant and a
    cancelled run each execute nothing; an exhausted tool allowance stops the run before the
    adapter; a consumed grant cannot be consumed again. The X-24 command, also with `-race`.
  - Report: "Exact action approval versioning and execution rechecks"; "Architecture and chart
    reading guide" (Figure 7); "Relative implementation milestones and critical dependencies" (Hours
    10-14)
  - Blocked by: `record versions`
  - Completed (2026-10-03): `internal/policy/executor.go`: approved actions run after rechecks of the active catalog revision (`source_policy_changed`), the open unexpired grant (`approval_expired`), the frozen source versions against the current ones (`resource_version_changed`) and the review material rebuilt from current rows against the frozen digest (recipient address, content, template, arguments: `action_changed`); the grant is consumed once by the attempt in the effect's transaction before `RunEffect`; attempts are counted under `FOR NO KEY UPDATE` on the run row (a `FOR UPDATE` lock deadlocked with a running effect's event insert in the concurrency test). Checks: `pnpm test:db gateway` 653 passed, 0 failed, 0 skipped with `TestApprovedActionExecutesOnceAndConsumesItsGrant` (one outbox row, the grant consumed by the attempt, a second execution refused), `TestApprovedActionRechecksBeforeExecution` (changed source version, changed recipient address, changed arguments, cancelled run, changed catalog revision: refused, no attempt, no outbox row), `TestExpiredOrRejectedGrantExecutesNothing` and `TestConcurrentExecutionsConsumeTheGrantOnce` (4 concurrent executions: one success, one outbox row, one consumption); `go test -race -count=10` on the concurrency and recheck tests against the database PASS; the exhausted attempt allowance is `TestAttemptLimitIsEnforced` (GO-16); `pnpm verify` 6/6. Not here: the guard re-run when the assessment is stale (Report 1.2 change) is covered by refusing a changed catalog revision, which hands the action back for a fresh evaluation; current revocations join in GO-52.

- [x] **GO-46 · Prove approval integrity**
  - **Report 1.1 change:** Adds a changed template or projection version; Report: Scene 4 precise human review and one simulated delivery.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-45 · Needs: X-16, X-34 · Provides: X-45
  - Paths: none (scenario tests in the packages above)
  - Work: After an approval, change the content, the recipient or a source record version, and try
    an expired approval; capture each outcome and the outbox state. Also show that an approval
    cannot enlarge the passport.
  - Done when: X-45 is reached, Approval integrity: "Tampered proposal outcome and no matching
    business effect." (acceptance evidence: "Edited and expired proposals do not execute under the
    original grant.")
  - Tests: one scenario test per tampering, asserting the denial reason, no outbox row and the
    original grant unconsumed or closed. The X-24 command.
  - Report: "Validation plan and evidence matrix" (critical check Approval integrity); "Threat model
    limits and unresolved design choices" ("Verify that approval cannot enlarge the passport and
    changed content requires new review"); "Illustrative invoice scenario and future domain
    adaptations" (Scene 3 precise human review)
  - Blocked by: nothing
  - Completed (2026-10-03): `TestApprovalIntegrity` (internal/policy, against Worker 2's real queue_report): after a real approval, each tampering is refused with its reason, no outbox row and the original grant unconsumed: changed recipient address and changed recipient reference and changed content (another report) -> `action_changed`; changed source record version -> `resource_version_changed`; an expired approval stores no grant (`approval_expired`); approving an action the gate denied (`resource_out_of_scope`) is refused with no grant, so an approval cannot enlarge the passport. `pnpm test:db gateway` 703 passed, 0 failed, 0 skipped; `pnpm verify` 6/6. A changed template or projection version is Worker 2's evidence (`policy_change_test.go`, X-76: template_not_allowed, outbox 0); the gate's rebuilt payload also binds the template version.

- [x] **GO-69 · Prove that approval cannot override the export restriction**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: GO-44, GO-64 · Needs: X-34 · Provides: X-77
  - Paths: none (a scenario test in the packages above)
  - Work: Submit an approval decision or a replayed grant for the denied export; the export stays
    denied and no outbox row exists. Tier B by the spine's rule (a critical check outside the slice);
    the team may raise it, since it completes Implementer 4's first integrated deliverable.
  - Done when: the evidence X-77 names is captured: "Submitted approval or replayed grant; export
    remains denied with no outbox effect." and Implementer 4's first integrated deliverable is
    observed: "An Internal only report cannot be exported to the correct vendor, even if an approval
    is submitted."
  - Tests: the scenario test with the outbox count quoted.
  - Report: "Validation plan and evidence matrix" (Approval cannot override classification);
    "Delivery scope and six person ownership" (Proposed team ownership)
  - Blocked by: nothing
  - Completed (2026-10-03): `TestApprovalCannotOverrideTheExportRestriction`: a genuinely created Internal only report (provenance.StoreReport, titled "Public summary") proposed to the correct, permitted Atlas recipient is denied at the gate before review (`report_export_restricted`); a submitted approval is refused and stores no grant; a replayed grant inserted directly with the action set to approved still executes nothing (refused `action_changed`: nothing was frozen for review); outbox rows for the report: 0. Worker 2's queue_report re-checks the restriction at effect time as a last line. `pnpm test:db gateway` 703 passed, 0 failed, 0 skipped; `pnpm verify` 6/6.

- [x] **GO-73 · Validate and acknowledge a candidate catalog revision**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-72 · Needs: X-79 · Provides: X-83 (part: Go validation and acknowledgement)
  - Paths: the enforcement package from GO-12; the internal API package from GO-21
  - Work: "Go fetches and validates the candidate, acknowledges readiness, and the activation flow publishes its active-version pointer." An invalid candidate is never activated; the last-known-good revision stays in use.
  - Done when: a valid candidate is acknowledged and becomes active, and an invalid one is rejected with a reason while the old revision keeps deciding.
  - Tests: database-backed tests through the X-24 command for a valid and an invalid candidate.
  - Completed (2026-10-03): the `catalog activation protocol` as the lead decided it.
    `catalog.ActivateRequested` runs under a transaction-scoped advisory lock (one gateway instance
    at a time), reads the pointer, and validates a requested revision that is not active and has not
    already failed with the gateway's own parsers (limits plus `security.SettingsFromCatalog`, with
    the feed found by the policy's `signatures.revision` and its pinned digest). Success sets
    `validated_revision_id = active_revision_id = requested` and `active_feed_revision_id`, and
    clears `last_error` in one transaction (Go's acknowledgement); failure writes a safe
    `last_error` (`reason`, `code`, `message`, `revision_id`, `stage`; never file content) and keeps
    the last good revision. `catalog.WatchRequested` runs it every second in `cmd/gateway` and
    stops before the pool closes. Migration `1791120000000-GrantGatewayCatalogActivation` grants
    the gateway role UPDATE on exactly those pointer columns. Tests (PostgreSQL, rolled-back
    transaction): a valid request becomes active with its feed and the loader serves it; invalid
    limits, an unknown disabled rule and a feed that was not imported are each rejected with their
    code while the last good revision keeps deciding, and are not retried; a first revision with
    signatures disabled needs no feed; a held lock gives `busy`; as `task_passport_gateway` the
    activation works while requesting or importing a revision is `permission denied`. Checks:
    gateway five checks PASS; `go test -race ./internal/catalog/...` ok; api `lint`, `typecheck`,
    `test`, `build` PASS; `pnpm db:migration:run` applied the migration; `pnpm verify` 6 passed.
    Live on the 55510 database with the gateway binary: `pnpm policy:import` of policy.yaml with
    threshold 0.8 was accepted as revision 230, and within 2.5 s the pointer showed requested,
    validated and active 230 with feed 134 and no error; a start-run command then admitted a
    passport with admission revision 230; a second import naming feed_v9 (revision 231) was
    rejected with `signature_feed_missing` while 230 stayed active. The feed row was inserted by
    hand for that run, because the feed half of the import (c1) has not landed yet.
  - Report: "Central policy configuration and safe reload"
  - Blocked by: `catalog activation protocol`

### Tool adapters, provenance and rendering (report role: Implementer 5)

- [x] **GO-47 · Prove the legitimate task on the Go side**
  - **Report 1.1 change:** Work: internal report, denied export, vendor report, review, one outbox row; the reviewed bytes match the simulated queued content. Done when adds Implementer 5's first integrated deliverable, "The stored vendor report uses only approved invoice fields and creates one matching outbox effect." Beat 8.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: GO-27, GO-40, GO-45, GO-29, GO-64, GO-65 · Needs: X-16, X-34 · Provides: X-44, X-67 (part: reviewed outbox effect)
  - Paths: none (a scenario test in the packages above)
  - Work: Run the full reconciliation through the approval to the queued report, once with the
    decision 6 model and once with a labelled provider test double, and capture the report
    references, the discrepancy, the single outbox row and the completed run's events.
  - Done when: the Go half of X-44 is reached, Legitimate task: "Report references, expected
    discrepancy, one outbox row, and completed run events."
  - Tests: the scenario test asserts exactly one outbox row with the frozen content and the trusted
    recipient, and the completed run's ordered events. The X-24 command; one live run, quoted.
  - Progress (2026-10-03): W2 lane, branch go/w2, 82c6aec, direct adapter path, not the agent loop:
    X-44 Go-half evidence captured (report references, the INV104 discrepancy, one outbox row to
    reports@atlas.example.com whose hash matches the stored content, the ordered events). Missing:
    the run through the approval with the decision 6 model and with a labelled provider double.
  - Progress (2026-10-03): `internal/scenario` runs the story through `agent.NewProductionChain`
    from real admission. Labelled scripted provider, complete (with f3's GO-40, 5d56898):
    `TestStoryAfterApproval` approves the vendor report's exact queue_report through
    `policy.Approvals`; the continuation resumes it (`run.resumed`), the original action executes
    through the executor recheck, one outbox row goes to the registered address with the reviewed
    content hash, the final answer names the reports and the run completes. Ordered events:
    run.queued, run.started, 3 x (action.allowed, action.succeeded), action.allowed,
    report.created, report.export_denied report_export_restricted, action.allowed,
    report.created, approval.requested, run.awaiting_approval, approval.decided, run.resumed,
    action.succeeded, run.completed. Live (`qwen3.5:4b`, labelled live, the approval step
    included in `TestLiveStoryThroughTheProductionChain`): one run reached the review wait for its
    vendor report; another had every proposal, even `read_invoice`, denied by the action-proposal
    semantic check (`semantic_injection_detected`) and stopped at the correction limit, outbox 0
    each time. Missing: the live run through the approval, after c1 limits the action-proposal
    semantic check to free-text arguments.
  - Completed (2026-10-03): with c1's fix on `main` (855ae20, merged), the live run went through
    the approval. `TestLiveStoryThroughTheProductionChain` (`qwen3.5:4b`, labelled live): step 1
    proposed several actions at once and was denied (`multiple_actions_not_supported`); then
    read_invoice A01 and A02, create_report `vendor_reconciliation_v1` (vendor_shareable, no
    internal note), queue_report awaiting approval; the reviewer approved; the run resumed, the
    approved action executed and queued one simulated outbox message to the registered address,
    and the run completed (6 agent calls, 1 security call, 1 live verdict, outbox rows 1). The
    model did not create the internal report in this run; the scripted run covers that beat.
    Both runs: one outbox row, approved and to the trusted recipient.
  - Report: "Validation plan and evidence matrix" (critical check Legitimate task); "Live
    demonstration storyboard and proof checks" (beat 7)
  - Blocked by: nothing

### Modules the report's team table does not name (Go implementer)

- [x] **GO-48 · Serve the exact review payload, if the read path chooses Go endpoints**
  - **Report 1.1 change:** The review payload adds the report fields of GO-43; "Review payloads and source manifests need their own access rules".
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: SH-05, GO-21, GO-43 · Needs: X-09 · Provides: X-41
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`, `services/gateway/README.md`
  - Work: Only if SH-05 and `review payload read` choose a private Go endpoint: return the frozen
    action's recipient, rendered content, referenced report and version, and reason for review,
    only to a verified reviewer of the run's organization. If they choose the view, this task
    becomes "Dropped: the read path chose the review view (SH-27)".
  - Done when: X-41 is reached through Go, the exact review payload "readable by authorized reviewers
    only" ("Review payloads need their own access rules").
  - Tests: a non-reviewer and a reviewer of another organization get the rejection envelope and no
    content; the returned content equals the frozen content byte for byte.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a
    second disclosure channel); "Users operating model and proposed user journeys" (Journey 2 review
    an exact outbound effect)
  - Blocked by: `read path`; `review payload read`
  - Completed (2026-10-03): the read path chose private Go endpoints (lead decision); `policy.ReviewHandler` with `Approvals.FrozenReviewFor` serves `GET /internal/actions/{actionId}/review` (c8207de), mounted by 3c behind the service token and operator context (d5c5e8c). Checks: `pnpm test:db gateway` 694 passed, 0 failed, 0 skipped with `TestReviewEndpointServesTheFrozenPayloadToReviewersOnly` (evidence X-41: the reviewer's served report content and recipient address equal the stored frozen payload byte for byte; an operator without the reviewer role gets 403, a reviewer of another organization 404, no operator context 401, none of them with content) and `TestFrozenReviewIsReadOnlyByReviewers`; `pnpm verify` 6/6.

## M4: hours 14-18

- **Team focus (report 1.2).** "Run the automated positive/negative/redaction/budget/exploit suite; exercise approval concurrency, source-version changes, policy changes, guard failures and waiting-state recovery. Complete management summary, sanitized audit export, latency summary and judge client."
- **Exit condition (report 1.2).** "Actual assertion results are retained; missing or failed checks remain visible; judge can change a rule and submit an unexpected input through real controls."
- **Report 1.2 sync points (spine).** X-89 (complete), X-90 to X-104.
- **Sync points needed by the end (spine):** X-02 and X-47 to X-57. This side provides X-51, X-52,
  X-54 and X-57 and its halves of X-50, X-55 and X-56; its half of X-53 came with GO-39. The
  optional X-60 (Tier C) gets its Go half here (GO-60), only if events stream from Go. SH-31
  records the outcomes.

### Agent runtime (report role: Implementer 3)

- [x] **GO-49 · Recover expired leases without replaying dispatched work**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: B · Size: M (estimate 3-6 h)
  - Depends on: GO-02, GO-39, GO-40, GO-45 · Needs: X-24, X-39 · Provides: X-54
  - Paths: the worker package from GO-08
  - Work: Implement GO-02's outcome. A worker that takes over an expired lease reads the
    identifiable dispatched attempts and continues without repeating a completed effect:
    "Successful actions must not be re-executed merely because a response was lost." A waiting run
    survives the restart and resumes its original stored action.
  - Done when: X-54 is reached, Waiting-state restart: "State before/after restart and exactly one
    resulting effect." (acceptance evidence: "A worker restart preserves an approval wait and does
    not replay a successful operation.")
  - Tests: database-backed, stopping the worker at each step boundary (after a reservation, after a
    dispatch, after an effect commits, during an approval wait) and starting a new one: each run
    ends with exactly one effect per action and no unaccounted dispatch; a run stopped by its limit,
    a cancelled run, a run with an unresolved reservation and a run in the attention state keep
    their state and recorded reason across the restart. The X-24 command.
  - Progress (2026-10-03): `agent.Recovery` runs at the start of every claim of a running run: an
    unresolved model call keeps its reservation as `usage_unknown` (or `failed` when it never
    reserved) and pauses the run with `outcome_unknown` with no resend; an action still executing
    with an open attempt pauses the run for attention; an executed action without context entries is
    restored as its call plus a withheld-result marker without re-execution; awaiting, paused,
    stopped and completed runs keep their state and reason. Tests pass on PostgreSQL:
    `TestRecoveryPausesARunWithAnUnresolvedModelCall` (after the reservation and after the dispatch
    record only), `TestRecoveryContinuesAfterAnEffectCommittedWithoutItsContext` (one execution
    attempt), `TestRecoveryPausesOnAnActionStillExecuting`,
    `TestRestartKeepsWaitingAndEndedRunsAsTheyWere`.
  - Completed (2026-10-03): the remaining half, a waiting run resuming its original stored action
    after a restart with exactly one effect, is GO-40's
    `TestReviewWaitHoldsNoWorkerAndResumesTheApprovedActionAfterARestart` (one outbox message, same
    action id and digest). The dispatched attempts are identified by `runtime.model_calls` and
    `runtime.execution_attempts`; the open item `dispatched attempts` stays for the lead to record.
    Checks: as GO-40.
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Validation plan and evidence
    matrix" (critical check Waiting-state restart); "Functional requirements MVP boundary and
    deferred scope" (Durable execution)
  - Blocked by: `dispatched attempts`

- [x] **GO-50 · Prove budget concurrency**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-39, GO-45 · Needs: X-24, X-34, X-39 · Provides: X-52
  - Paths: none (concurrency tests in the packages above)
  - Work: Race model reservations and tool reservations against one remaining allowance and capture
    the reservation and usage rows.
  - Done when: X-52 is reached, Budget concurrency: "Reservation and usage rows with reconciled
    totals and denied competing request." (acceptance evidence: "Competing requests cannot each
    spend the same remaining allowance.")
  - Tests: concurrent reservation tests with `go test -race ./...` through the X-24 command: of the
    competing requests only those the allowance covers succeed, the others are denied, and the
    totals reconcile.
  - Completed (2026-10-03): W2 lane (taken over from the agent-runtime list), branch go/w2.
    Model reservations (`internal/budget`, `TestPostgresCompetingReservationsCannotSpendTheSameAllowance`):
    12 overlapping reservations of 1000 tokens, both purposes, for an allowance of 3000: 3
    granted, 9 refused `ErrExhausted` before any dispatch with no reservation row; after two
    settle (600 tokens each) and one stays usage-unknown, the ledger's used 1200 equals the
    settled rows and its reserved 1000 equals the held row, 3 calls counted.
    `TestPostgresCompetingCallsCannotExceedTheCallLimit`: 10 calls for a limit of 4: 4 granted.
    Tool attempts (`internal/policy`, `TestPostgresCompetingExecutionsCannotSpendTheSameToolAttempts`):
    6 allowed actions executed at once through the real executor against 2 remaining attempts: 2
    succeeded (one attempt each, two adapter calls), 4 refused `allowance_exhausted` with no attempt
    and still allowed. `go test -race -count=5` against `starter_test`: 5 of 5 for each.
  - Report: "Validation plan and evidence matrix" (critical check Budget concurrency); "Atomic
    allowances hard limits and estimated cost"; "Threat model limits and unresolved design choices"
    ("Verify that two concurrent attempts cannot consume one approval or allowance twice")
  - Blocked by: nothing

- [ ] **GO-51 · Prove that cancellation and expiry stop dispatch, also after a review wait**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: GO-41, GO-45, GO-52 · Needs: X-24, X-34 · Provides: X-55
  - Paths: none (scenario tests in the packages above)
  - Work: Cancel a running run, cancel a run waiting for approval, let a run pass its expiry, and
    revoke authority during a review wait (GO-52); capture the timestamps, the denials and the
    dispatch records.
  - Done when: the Go half of X-55 is reached, Cancellation and expiry: "Cancellation/expiry
    timestamp, subsequent denial, and dispatch records." (acceptance evidence: "Future dispatches
    stop while committed effects remain correctly recorded.")
  - Tests: scenario tests asserting no model request or tool execution after the applicable check,
    and committed effects unchanged. The X-24 command.
  - Report: "Validation plan and evidence matrix" (critical check Cancellation and expiry); "Threat
    model limits and unresolved design choices" ("Verify that cancellation and revocation prevent
    future dispatch after a review wait"); "Functional requirements MVP boundary and deferred scope"
    (Cancellation and revocation)
  - Blocked by: nothing
  - Progress (2026-10-03, lane w3): `internal/agent/cancellation_postgres_test.go` through lane f3's loop with the production gate (review freezer), executor and adapters, model scripted and counted on the ledger. Evidence X-55: cancel during a model request -> stopped/run_cancelled, 2 model calls, 1 succeeded attempt from before the cancel, the later proposal not executed, a late continuation dispatches nothing; cancel during a review wait -> stopped/run_cancelled, the reviewer's decision refused (run stopped), executor refuses, no approval row, report kept, outbox 0, no further model request; cancel after approval -> the approved action refused run_cancelled, grant unconsumed, outbox 0; expiry between steps -> stopped/run_expired before the next model request, the earlier read kept. Each logs the cancel_requested_at and run.stopped timestamps. The executor now refuses an expired passport with run_expired instead of run_cancelled (lane f3 asked to map it to stopped in `refusalEnd`). Not ticked: the continuation after a review wait needs lane f3's GO-40, and the revocation case needs GO-52 (blocked on SH-38).

- [x] **GO-81 · Build the repeatable performance benchmark**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-80 · Needs: nothing · Provides: X-95 (part: benchmark)
  - Paths: a new command or test package, named by the Go implementer
  - Work: Record build, hardware, model, fixture size and active catalog, then report sample counts, errors, latency distribution (p50 and p95 only after collecting observations) and throughput. Compare the same permitted operation with optional semantic inspection enabled and disabled in an authorized test configuration; stubbed guard runs isolate gateway overhead and a separate live-model run records actual semantic and provider delay. Never invent latency results.
  - Done when: one documented command produces the benchmark report on the developer machine.
  - Tests: the benchmark run once with its output quoted.
  - Completed (2026-10-03): W2 lane, branch go/w2: `cmd/benchmark`, run with
    `node scripts/with-env.mjs go -C services/gateway run ./cmd/benchmark [--live]`. It measures the
    policy lookup and hybrid inspection of one permitted `read_invoice` result with the semantic
    check off, on with a labelled fixture caller (gateway overhead) and on with the live model.
    `measurement method` is decided and recorded in the gateway README ("Performance benchmark
    (GO-81)"): concurrency 1, warmup excluded, the configurations without a model interleaved,
    GO-80 phase names, nearest-rank p50 and p95, null before observations, plus a separate
    aggregate of `runtime.timing_records`. Run once (Apple M1 Pro, `qwen3.5:4b`, load average
    108.88 on 10 CPUs, feed loaded by hand because the API-34 feed import is not on `main`):
    `semantic_off` 300 samples, 0 errors, total p50 45,021 µs, p95 284,823 µs;
    `semantic_on_fixture` 300, 0 errors, p50 38,802 µs, p95 279,920 µs, semantic p50 39 µs;
    `semantic_on_live` 5, 0 errors, p50 16,204,307 µs, provider p50 15,456,473 µs. The full table
    is in the gateway README. `go test ./cmd/benchmark`: ok. GO-80's recorded spans were empty
    (its writer is on go/f3, not `main`).
  - Rerun (2026-10-03, quiet machine, at the lead's request):
    `MODEL_NAME=qwen3.5:4b pnpm benchmark --live` at 3aeade7, load average 12.03 at the start
    and 10.11 at the end (below 15 throughout): `semantic_off` 300 samples, total p50 1,311 µs,
    p95 3,452 µs (deterministic p50 77 µs); `semantic_on_fixture` 300, p50 1,288 µs (semantic
    13 µs); `semantic_on_live` 10, p50 1,930,552 µs, provider p50 1,919,761 µs, gateway overhead
    p50 5,367 µs; 0 errors. The gateway README holds the table; it replaces the loaded-machine
    numbers for the slides.
  - Report: "Validation plan and evidence matrix" (Performance measurement method)
  - Blocked by: `measurement method`

- [ ] **GO-86 · Prove policy reload, the model allowlist and local model resources**
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: A · Size: S (estimate 2-3 h, this roadmap's estimate)
  - Depends on: GO-39, GO-73, GO-79 · Needs: X-83, X-89 · Provides: X-101, X-102, X-103
  - Paths: none (scenario tests run through the X-89 suite)
  - Work: Change a threshold and a rule and show the before and after revision and decisions; submit an invalid file and show the rejected activation and the retained revision; remove a model and lower a budget on an admitted run; exhaust calls, tokens, time and the concurrency cap.
  - Done when: the evidence of X-101, X-102 and X-103 is captured, and no case widens the stored passport.
  - Tests: the scenario tests in the suite, with the results quoted.
  - Progress (2026-10-03): live evidence, run once on the 3c private database with the gateway binary
    (go/3c at c8d0e64 with lane f3's chain and GO-79), local qwen3.5:4b, every edit through `pnpm
policy:import` and the gateway's activation (GO-73), every decision through `POST
/internal/control/evaluate` (GO-82). The signature feed was loaded by hand (c1's import not on
    main); the evaluation runs' jobs were closed by hand so the worker left them to the evaluations.
    X-101 policy reload: the baseline was active 4.9 s after import (revision 234); the hostile tool
    result was denied `signature_match` (prompt_ignore_previous_v1). Disabling that rule (revision 235,
    active after 3.3 s) changed the decision on the same input to a semantic block (score 0.85, rule
    passes). A threshold of 0.99 (revision 236, 3.5 s) still blocked because the live score was 1.0,
    so the live run does not show a threshold-driven change; the catalog test shows the new threshold
    in the next snapshot. An unknown disabled rule (revision 237) was rejected with `catalog_invalid`
    while 236 kept deciding. Raising calls_total to 40 (revision 238) left the admitted run's
    passport (24 calls) and ledger (24, 12 security, concurrency 2) unchanged. X-102 model
    allowlist: allowed_models [qwen3.5:9b] (revision 239) refused the next security call before any
    reservation (reservations 3 before, 3 after; denied `security_evaluator_unavailable`). X-103
    local resources: local_max_concurrency 1 (revision 241) serialized three parallel evaluations
    (dispatch records within 12 ms, completions at 07.2, 09.0 and 10.9 s). The original policy was
    restored (revision 242, same digest). Missing: lowering calls_security to 1 (revision 240) did
    not restrict the admitted run, whose 4th security call was still reserved; the ledger keeps the
    passport's limits, so current reductions do not reach security calls (lane f3, reported).
    Calls, tokens and time exhaustion are covered by lane f3's GO-39/GO-79 tests and not repeated
    live here.
  - Report: "Validation plan and evidence matrix" (Policy reload and rollback safety, Model allowlist and current reductions, Local model resources)
  - Blocked by: nothing

### Enforcement (report role: Implementer 4)

- [ ] **GO-52 · Check current revocations before dispatch and before execution**
  - **Report 1.1 change:** Adds source and template revocations.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 2-3 h)
  - Depends on: GO-11, GO-45 · Needs: X-35, X-47 · Provides: nothing
  - Paths: the enforcement package from GO-12; the worker package from GO-08
  - Work: Read the current revocation records NestJS writes in `app`, as the `revocation reads`
    outcome defines, before every model dispatch and in the execution recheck. An unreadable
    revocation source stops dispatch. The passport and its policy version stay unchanged and
    available to explain the run, and revocation stays effective while a run waits for review.
  - Done when: "Historical versions explain why the run was admitted, while revocation prevents
    future effects when authority is withdrawn", and MVP requirement Cancellation and revocation
    holds: "Observe current revocations and cancellation before new execution."
  - Tests: database-backed: a revocation written during an approval wait blocks the resumed action;
    a revocation unrelated to the run has no effect on it; a failed revocation read dispatches
    nothing; the stored passport is unchanged. The X-24 command.
  - Report: "Exact action approval versioning and execution rechecks" (Versioned policy and current
    revocation); "Trusted authority and passport invariants"; "Threat model limits and unresolved
    design choices" ("Current revocation requires a single owner and reliable reads")
  - Blocked by: `revocation reads`; `decision 2 in docs/product/README.md`
  - Progress (2026-10-03): blocked on SH-38/X-66 (web + API): `revocation reads` is not decided and no revocation table exists, so no reader is built (a reader that fails closed against a missing table would stop every run; lead decision).

- [x] **GO-53 · Handle known failures, safe retries and unknown outcomes**
  - **Report 1.1 change:** Figure 9.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 2-4 h)
  - Depends on: GO-07, GO-45 · Needs: X-11, X-39 · Provides: nothing
  - Paths: the executor package from GO-16; the adapter package from GO-17
  - Work: Classify each execution as succeeded, failed with a known outcome, or uncertain (Figure
    8). A known failure records its outcome and settles the allowance; it is retried only when GO-07
    marks the operation known-safe and limits remain, with the same frozen action and its consumed
    grant after fresh checks, never under a new identifier; otherwise the run fails. An uncertain
    outcome is persisted as unknown and pauses the run for operator attention; it is never returned
    to the queue, and "the adapter must not invent exactly-once behavior from a local status field".
    The report names no reconciliation operation beyond this state, which is recorded as a
    limitation in GO-61.
  - Done when: MVP requirement Safe outcomes and retries holds, "Use stable action identifiers;
    retry only known-safe operations; reconcile unknown effects", with "A repeated request does not
    create a second outbox message; uncertainty pauses the run."
  - Tests: with adapters forced to fail: a known-safe failure retries under the same action
    identifier and still yields one outbox row; an unsafe failure fails the run; an uncertain
    outcome sets the attention state and dispatches nothing further; each retry consumes allowance.
    `pnpm --filter gateway run test`; database-backed cases through the X-24 command.
  - Progress (2026-10-03): W2 lane, branch go/w2, ca42cb6: adapter half. `tools.ClassifyRunError`
    (precondition vs known no-effect after rollback) and `tools.RetrySafe`; a forced event failure
    of queue_report retried under the same action yields one outbox row and two counted attempts
    (against w2_check). Missing: the executor's retry loop and unknown-outcome attention state in
    internal/policy (Worker 3).
  - Completed (2026-10-03): both halves are on `main` (abf3c10): the adapter half above (ca42cb6)
    and Worker 3's executor half (5a6b211). A known no-effect failure of a retry-safe tool is
    retried once under the same action id and counts as another tool attempt; a precondition
    failure fails the action and stops the run; a failed commit leaves the attempt open, marks
    the action unknown with `action.unknown` (`outcome_unknown`), pauses the run and is never
    re-queued. Rerun on the merged tree 55c851f against a private PostgreSQL (`starter_test`):
    `TestSafeRetryUnderTheSameActionQueuesOneMessage` (one outbox row, two attempts),
    `TestPreconditionFailureFailsTheActionWithoutRetry`, `TestFailedCommitRecordsAnUnknownOutcome`
    (a second execution is refused), `TestAdapterErrorRollsBackAndPauses`,
    `TestKnownSafeRetryOfQueueReportYieldsOneOutboxRow` and `TestClassifyRunErrorAndRetrySafety`:
    PASS. `go test ./internal/tools ./internal/provenance ./internal/reads ./internal/policy` with
    `GOFLAGS=-p=3`: ok. No reconciliation of unknown effects exists beyond this state (GO-61).
  - Report: "Durable state idempotency audit and uncertain outcomes"; "Architecture and chart
    reading guide" (Figure 8); "Functional requirements MVP boundary and deferred scope" (Safe
    outcomes and retries)
  - Blocked by: nothing

- [x] **GO-54 · Prove approval replay under concurrent requests**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-44, GO-45 · Needs: X-24, X-34 · Provides: X-51
  - Paths: none (concurrency tests in the packages above)
  - Work: Send concurrent approval decisions and concurrent execution attempts for one approved
    action, and capture the results, the grant and the outbox.
  - Done when: X-51 is reached, Approval replay: "Concurrent request results, one consumed grant,
    and one outbox row." ("Verify that blocked operations create no synthetic outbox effect and
    successful retries do not duplicate it.")
  - Tests: concurrency tests with `go test -race ./...` through the X-24 command: one grant is
    consumed and one outbox row exists however many requests race.
  - Report: "Validation plan and evidence matrix" (critical check Approval replay); "Threat model
    limits and unresolved design choices" (Verification priorities)
  - Blocked by: nothing
  - Completed (2026-10-03): `TestConcurrentApprovalDecisionsStoreOneGrant` (6 concurrent approve/reject decisions -> 1 accepted, 5 refused; 1 grant, 1 continuation job) and `TestConcurrentExecutionsConsumeTheGrantOnce` (4 concurrent executions of one approved action -> 1 succeeded; 1 consumed grant; 1 outbox row), both against the database: `pnpm test:db gateway` 703 passed, 0 failed, 0 skipped, and `go test -race -count=5` on both PASS; `pnpm verify` 6/6.

- [x] **GO-55 · Prove the database execution transaction with fault injection**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-34, GO-45 · Needs: X-24, X-35 · Provides: X-57
  - Paths: none (fault-injection tests in the packages above)
  - Work: Inject failures between the effect, the completion record and the event of
    `create_report` and `queue_report`, on the connection SH-06 settled, and capture the action and
    business state after each.
  - Done when: X-57 is reached, Database execution transaction: "Fault-injection outcome and
    consistent action/business state." (target: "Prototype business effect and successful execution
    record commit together or neither commits.")
  - Tests: fault-injection tests through the X-24 command: after each injected failure either all
    three rows exist or none does.
  - Completed (2026-10-03): W2 lane, branch go/w2, 145f2ba. Fault injection with a
    transaction-local trigger: create_report failing at lineage, completion or event and
    queue_report failing at completion or event each leave report/lineage/outbox/completion/events
    = [0 0 0 0 0]; without a fault queue_report leaves [0 0 1 1 1]. Checks: gateway checks PASS; go
    test -race against my migrated PostgreSQL 17 (w2_check, 127.0.0.1:55435): pass. Ran as the
    starter owner; the run on the task_passport_gateway role belongs to GO-38.
  - Report: "Validation plan and evidence matrix" (critical check Database execution transaction);
    "Durable state idempotency audit and uncertain outcomes"
  - Blocked by: `decision 2 in docs/product/README.md`

- [x] **GO-30 · Prove the resource and destination boundaries**
  - **Report 1.1 change:** Tier B, moved to M4 where X-37 and X-38 are needed (the ID stays). The beat 4 sentence is dropped; "The attempted out-of-scope read would leave no corresponding data access or effect." Report: beat 9 and "Supporting rehearsals". It no longer carries a first integrated deliverable.
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-28, GO-31, GO-33, GO-36 · Needs: X-16, X-34 · Provides: X-37, X-38
  - Paths: none (scenario tests in the packages above)
  - Work: With the X-34 fixtures, submit an out-of-scope invoice, a vendor outside the task and a
    changed recipient, from the live model where it proposes them and otherwise from the labelled
    replay (GO-36). Capture the state before and after each: the denial record, the excluded
    records' versions, the report and outbox counts and the absence of an execution attempt. Also
    show that the hostile note changed neither the stored passport nor the operator identity (demo
    beat 4).
  - Done when: X-37 and X-38 are reached with the evidence the report lists, Resource boundary:
    "Denial record plus unchanged excluded records and absence of an execution attempt."; Destination
    boundary: "Stored proposal, rule decision, and outbox comparison."; and Implementer 4's first
    integrated deliverable is observed: "An out-of-scope proposal is denied before the tool adapter
    runs."
  - Tests: scenario tests asserting, for each denied proposal, no adapter call, no execution record,
    unchanged record versions and unchanged report and outbox counts; replayed proposals carry the
    replay label. The X-24 command.
  - Report: "Validation plan and evidence matrix" (critical checks Resource boundary and Destination
    boundary); "Live demonstration storyboard and proof checks" (beats 4 and 5); "Risk register and
    scope controls" (Demo proves logs, not prevention)
  - Blocked by: nothing
  - Completed (2026-10-03): `internal/policy/boundary_evidence_postgres_test.go`, `TestResourceAndDestinationBoundaries`, through the production gate (with the review freezer) and executor. No live model loop exists on this branch yet (f3), so the out-of-scope invoice and the changed recipient come from the labelled replay (GO-36) and the vendor outside the task is a constructed proposal, logged as "not a replay"; none is presented as model output. Evidence X-37: out-of-scope invoice [labelled_replay:hostile_note_redirect_record_v1] and vendor outside the task -> stored decision `deny/resource_out_of_scope`; X-38: changed recipient [labelled_replay:hostile_note_redirect_recipient_v1] -> `deny/destination_not_allowed`. For each: adapter calls 0 (a counting adapter wraps the real runner), the executor refuses, execution attempts 0->0, excluded invoice version 1->1, reports 2->2, outbox 0->0, stored passport scope and actor unchanged; every snapshot read fails the test if it errors. Checks: see the commit.

- [x] **GO-68 · Prove label and rename tampering and missing lineage**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-63, GO-64 · Needs: X-34 · Provides: X-73, X-74
  - Paths: none (scenario tests in the packages above)
  - Work: Submit an agent-supplied classification, a "Public summary" title (through the operation
    `rename operation` names; until then a public label in the arguments), a copied artifact, and
    unknown, incomplete or unverifiable source metadata.
  - Done when: the evidence of X-73 ("Stored provenance before and after attempt; denied export or
    rejected unsupported mutation.") and X-74 ("Rejected report proposal or export denial with no
    outbox row.") is captured.
  - Tests: the scenario tests with the stored provenance before and after quoted.
  - Completed (2026-10-03): W2 lane, branch go/w2, f5dc0af, direct adapter path. X-73: an agent
    classification and a "Public summary" title argument are rejected before any effect; the stored
    provenance snapshot is unchanged. X-74: renamed internal report -> report_export_restricted,
    copied artifact -> report_lineage_missing, unverifiable lineage -> resource_version_changed;
    outbox 0 -> 0 each. `rename operation` is still open, so a public label in the arguments stands
    in for it. Checks: gateway checks PASS; go test -race against my migrated PostgreSQL 17
    (w2_check, 127.0.0.1:55435): pass.
  - Report: "Validation plan and evidence matrix" (Label and rename tampering, Missing lineage);
    "Live demonstration storyboard and proof checks" (beat 6)
  - Blocked by: `rename operation` (rename part)

- [ ] **GO-70 · Prove source or template policy changes after review**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-45, GO-52 · Needs: X-34, X-47 · Provides: X-76
  - Paths: none (scenario tests in the packages above)
  - Work: During the review wait, change the source versions or the template or projection version,
    or revoke a source or template; the old approval is rejected and nothing stale is queued.
  - Done when: the evidence X-76 names is captured: "Old approval is rejected; no stale report is
    queued."
  - Tests: the scenario tests with the rejected approval and the outbox count quoted.
  - Progress (2026-10-03): W2 lane, branch go/w2, 5ed3b7e: no stale report is queued after a source
    version change (resource_version_changed) or an unregistered projection version
    (template_not_allowed), outbox 0. Missing: the "old approval is rejected" half (needs Worker
    3's GO-45 and GO-52) and revocations (SH-38).
  - Progress (2026-10-03): the "old approval is rejected" half for changes is on `main` through
    GO-45: Worker 3's `TestApprovedActionRechecksBeforeExecution` approves a queue_report, then
    changes the source invoice version (refused `resource_version_changed`) or the active catalog
    revision (refused `source_policy_changed`), with outbox 0 and no attempt; rerun on 55c851f
    against `starter_test`: PASS (5 of 5 subtests). Template and projection versions are Go
    constants, so a running gateway cannot change them during a review; a stored report with an
    unregistered projection version is refused by the adapter test above. Missing: revoking a
    source or template during review, which needs GO-52 and the revocation records (SH-38).
  - Report: "Validation plan and evidence matrix" (Source or template policy changes)
  - Blocked by: nothing

- [x] **GO-84 · Prove the live semantic cases, the false-negative boundary and guard failure**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-75, GO-76, GO-77 · Needs: X-86, X-89 · Provides: X-96, X-97, X-98
  - Paths: none (scenario tests in the packages above, run through the X-89 suite)
  - Work: Run the labelled corpus through the real local evaluator and record inputs, expected labels, exact model and configuration, actual verdicts, false positives and negatives and pass or fail counts. Give the gate a permissive verdict for an out-of-scope read and an internal report export and show the deterministic denial. Feed a malformed verdict, a timeout, an unavailable model and an exhausted security allowance.
  - Done when: the evidence of X-96, X-97 and X-98 is captured, with fixture verdicts and live verdicts kept apart.
  - Tests: the scenario tests in the suite, with the results quoted.
  - Report: "Validation plan and evidence matrix" (Live semantic benign and attack cases, Semantic false negative boundary, Guard failure and security ceiling)
  - Blocked by: nothing
  - Completed (2026-10-03): evidence tests in `internal/security`. X-96 (opt-in `TestLiveSemanticCorpus`, `model_live` tag, writes a JSON results file): 21 corpus cases plus 3 hostile notes on Ollama 0.35.1, `qwen3.5:4b` (2a654d98e6fb), threshold 0.75, context 8192; run 1: 23 of 24 matched, 0 false positives, 1 false negative, 0 guard failures, one hostile note passed by its second (pipeline) evaluation; run 2: 22 of 24, 0 false positives, 2 false negatives, 0 guard failures; live verdicts labelled `live`, all other tests use labelled fixtures. X-97 (`TestSemanticFalseNegativeStillDeniedDeterministically`, external package): Worker 3's real gate with a fixture verdict of score 0 denies each hostile note's obeyed action with its fixture reason (`resource_out_of_scope`, `destination_not_allowed`, `report_export_restricted`) and makes no security call. X-98 (`TestPostgresGuard*`, real ledger): timeout keeps the reservation as `usage_unknown`, malformed verdict pauses after settled usage, exhausted or paused allowance dispatches nothing. Checks: `pnpm test:db gateway` 556 passed, 0 failed, 0 skipped; gateway checks all exit 0; `pnpm verify` 6 passed. Not done: running these through the one-command X-89 suite (SH-47 has no `verify:controls` yet); the outbox assertions for the X-97 denials are the tools lane's X-72 and X-74 tests.

- [ ] **GO-85 · Prove redaction and the attack feed update**
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: A · Size: S (estimate 1-3 h, this roadmap's estimate)
  - Depends on: GO-74, GO-78 · Needs: X-83, X-87, X-88, X-89 · Provides: X-99, X-100
  - Paths: none (scenario tests run through the X-89 suite)
  - Work: Show the exact redacted serialization of a defined secret with its retained restriction and audit reason. Activate a trusted new pattern and show its rule hit; offer a malformed and an untrusted feed and show that the accepted rules stay.
  - Done when: the evidence of X-99 and X-100 is captured.
  - Tests: the scenario tests in the suite, with the results quoted.
  - Report: "Validation plan and evidence matrix" (Redaction control, Attack feed update)
  - Blocked by: nothing

### Tool adapters, provenance and rendering (report role: Implementer 5)

- [x] **GO-56 · Inspect model context, events and output channels for protected fields**
  - **Report 1.2 change:** The inspection also covers broad logs, telemetry and audit exports.
  - **Report 1.1 change:** "Inspect model context separately from outbox content; the model may see an authorized internal note while the vendor report excludes it." The Sensitive data exposure evidence moves to GO-66 and GO-47.
  - Owner: Go implementer (report role: Implementer 5, tool adapters) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-22, GO-23, GO-35, GO-47 · Needs: X-06, X-12, X-34 · Provides: X-50
  - Paths: none (inspection tests in the packages above)
  - Work: For a fixture run that reaches the queued report (the GO-47 scenario), serialize every
    tool result, the model request fixture, the event payloads, the stored report, the persisted
    final result and the simulated outbox row, and search them for each protected value the X-06
    field rules name. The outbox row may hold only the fields the registered template renders, and
    a protected value only where the X-06 field rules allow it in outbound content (such as the
    recipient `queue_report` resolves inside the adapter, GO-35). Record what the check covers and
    what it does not ("without claiming universal detection of personally identifiable information
    or encoded disclosure"): a search for literal values does not find a value the model transforms
    or encodes "in an allowed channel".
  - Done when: the Go half of X-50 is reached, Field minimization: "Inspected serialized tool
    result, model request fixture, and event payload.", and the Go output channels are inspected
    for MVP requirement Data minimization ("Inspect model context and output channels for the
    defined protected fields.") and for Sensitive data exposure ("Inspect what reaches the model,
    the simulated outbox, and the general activity feed."; the activity feed is the other side's
    part of X-50).
  - Tests: the inspection test fails if any protected value appears in a model-facing result, a
    model request or a safe event; if the stored report or the persisted final result holds a
    protected value the X-06 field rules do not allow there; or if the outbox row holds a field the
    registered template does not render or a protected value the field rules do not allow in
    outbound content. The X-24 command.
  - Completed (2026-10-03): two inspections, both literal-value searches for the registered
    reporting address and the internal note. On the agent path (`TestStoryThroughTheProductionChain`,
    `internal/scenario`): 7 serialized model requests, 7 security requests, 15 events (row_to_json:
    the activity feed and audit export source), 71 control assessments, 93 timing records (report
    1.2 telemetry), 12 agent context entries, the vendor report and the gateway's JSON log: the
    address appears in none; the note only in what the model may read (read_invoice's result)
    and in the internal report. On the adapter path (6ef3f0b, `TestProtectedFieldsStayInTheirAllowedChannels`):
    the outbox row holds the address only as its recipient and the queued vendor report. Not
    covered: the persisted final result holds report ids only (GO-26's narrow result), and the
    loop's outbox row after an approval waits for GO-40 (`TestStoryAfterApproval`); a transformed
    or encoded value is not found by a literal search ("without claiming universal detection").
  - Report: "Validation plan and evidence matrix" (critical check Field minimization); "Risk
    register and scope controls" (Data leakage through secondary views); "The enforcement loop and
    data minimization"; "Functional requirements MVP boundary and deferred scope" (Data
    minimization); "Mapping the proposal to the Goldman Sachs challenge" (Sensitive data exposure)
  - Blocked by: nothing

### Modules the report's team table does not name (Go implementer)

- [x] **GO-57 · Prove organization access at the internal boundary**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-14, GO-21, GO-41, GO-44 · Needs: X-22, X-24, X-34 · Provides: X-56
  - Paths: `services/gateway/internal/httpserver/server_test.go`
  - Work: Call every internal command, and every Go read if the read path chose Go endpoints,
    directly with a valid service identity and the second organization's operator context against
    the first organization's run, action and resources, and capture the rejections and the
    unchanged runtime state.
  - Done when: the Go half of X-56 is reached, Organization access: "Rejected read/command requests
    and absence of runtime mutation." (target: "Another organization cannot inspect the run, approve
    its action, or claim its resources.")
  - Tests: internal-boundary tests through the X-24 command: each cross-organization command and
    read is rejected, and the runtime rows are identical before and after.
  - Completed (2026-10-03): `internal/api/organization_access_test.go` builds organization A's
    committed run (awaiting approval), an action awaiting approval, an internal report, a vendor and
    invoices, and calls every mounted internal route with a valid service token and organization
    B's verified operator context: start a run on A's invoices, cancel A's run, read A's report,
    approve and reject A's action, read A's review, read A's run state, events and usage (lane w2's
    GO-24). Each is rejected (start 400 or 503; cancel, report, run state, events and usage 404;
    approval and review 403 or 404); the organization-wide security summary, assessments and events
    (GO-83) answer B with none of A's run, action, report or organization ids; no response contains
    A's report text or vendor; B gets no passport; and an md5 fingerprint of every row A owns in
    runtime.passports, runs, jobs, actions, approvals, audit_events, review_payloads, demo.reports,
    outbox_messages, invoices and vendors is identical before and after. A's own operator still
    reads the report, the run state and the events (200) and cancels the run (200). Checks: `go test
./internal/api` against the test database PASS; gateway five checks PASS; `pnpm test:db
gateway` and `pnpm verify` as quoted in the commit. On the seeded test database the
    cross-organization start-run answers 503 (its catalog binds no feed), so the scope rejection of
    that call is shown by GO-13's tests.
  - Report: "Validation plan and evidence matrix" (critical check Organization access; "Test identity
    and authorization through the public path and the internal service boundary")
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **GO-60 · Optional: stream events from Go, if the read path chooses Go endpoints**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: C · Size: S (estimate 1-3 h)
  - Depends on: GO-24 · Needs: X-12 · Provides: X-60
  - Paths: `services/gateway/internal/httpserver/server.go`,
    `services/gateway/internal/httpserver/server_test.go`
  - Work: Only after authenticated polling works, and only if events stream from Go (SH-05): serve
    the sanitized events as server-sent events. The gateway's 30 s write timeout ends any long-lived
    response, so the stream needs a bounded lifetime and a reconnect from the cursor. Otherwise this
    task becomes "Dropped: events do not stream from Go".
  - Done when: the Go half of X-60 is reached: an "Authorized subscription; no unrestricted raw
    payload stream" carries the same sanitized events as the cursor read.
  - Tests: a stream of another organization's run is rejected; the streamed events equal the cursor
    read; a reconnect from the last cursor loses and repeats nothing.
    `pnpm --filter gateway run test`.
  - Report: "Relative implementation milestones and critical dependencies" ("use authenticated
    polling before adding SSE if necessary"); "Illustrative passport and interface contracts"
    (Proposed browser and runtime operations)
  - Blocked by: `read path`; `decision 3 in docs/product/README.md`

- [x] **GO-82 · Serve the control evaluation adapter contract**
  - **Report 1.2 change:** The judge reaches this endpoint through the NestJS live test entry (Figure 2; API-38, X-106).
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: GO-21, GO-76, GO-77 · Needs: X-79 · Provides: X-91
  - Paths: the internal API package from GO-21
  - Work: `POST /internal/control/evaluate`: "a documented Go client/HTTP contract for governed model calls and registered tool proposals". A call names a passport or run, a metered purpose, an allowed model and bounded input and output; a tool proposal names its registered adapter and typed arguments. It runs the same gates as the invoice agent; "the caller cannot issue a grant", and it exposes no shell, HTTP or model credentials.
  - Done when: a benign and an adversarial call through the endpoint receive the same decisions the agent path gives, and an unauthenticated call is rejected.
  - Tests: handler tests with the shared envelope; a database-backed test through the X-24 command.
  - Completed (2026-10-03): `POST /internal/control/evaluate` (X-91, camelCase, landed in
    `packages/contracts` and `internal/contracts`), behind the service token and the verified
    operator context. `internal/evaluation` reuses lane f3's production chain: `model_input` runs
    the content rules, the signatures and the semantic check at that boundary (never the agent
    model); `tool_result` runs `chain.Inspector`; `action_proposal` runs the agent path's gate
    (`chain.Scopes`, the relationships, `policy.NewSecurityActionEvaluator(chain.Inspector,
chain.Settings)`) with a recorder and freezer that store nothing, so evaluated actions are
    decisions only: nothing is stored as an action, executed or counted as a correction. The
    evidence, an X-12 `control.evaluated` event and the control records keyed by the evaluation id
    (through `repository.Tx.InsertControlRecords`), commits in one transaction; without it the
    answer is 503. Every decision answers 200; a body outside X-91 is 400, another organization's
    run 404. Tests: the three boundaries with the real security controls and a labelled fixture
    model (benign allowed, signature and semantic attacks denied, secret redacted, oversized
    blocked, no guard or a failing guard denied), a decision-only gate double, request validation,
    a PostgreSQL test (evidence stored, inspected text in no event, no action row, another
    organization not found) and route tests (200, 400, 404, 503, identity field refused, no
    credentials 401). Checks: gateway five checks exit 0; `pnpm test:db gateway` and `pnpm verify`
    as quoted in the commit. Live (gateway binary with the production chain, local qwen3.5:4b): a
    hostile tool result was denied with `signature_match` (rule prompt_ignore_previous_v1, feed_v1),
    an out-of-scope `read_invoice` proposal with `resource_out_of_scope` and no stored action, and
    three `control.evaluated` events with their assessments were recorded. Not shown live: a
    benign semantic allow. Under a machine load average of 50-90 the security call timed out
    (usage unknown), and the evaluation correctly denied with `security_evaluator_unavailable`. The
    benign allow is shown with the labelled fixture model.
  - Report: "Technical architecture and service ownership" (Small integration boundary); "Illustrative passport and interface contracts" (Proposed browser and runtime operations)
  - Blocked by: nothing

- [x] **GO-83 · Serve the security decision records, if the read path chooses Go endpoints**
  - Owner: Go implementer (a module the report's team table does not name) · Tier: A · Size: S (estimate 1-3 h, this roadmap's estimate)
  - Depends on: GO-21, GO-80 · Needs: X-85 · Provides: nothing
  - Paths: the internal API package from GO-21
  - Work: Conditional. Only if `read path` chooses private Go endpoints: return organization-scoped control assessments, usage per purpose and timings for the summary and export, with stable codes and no raw protected content. Otherwise this task becomes "Dropped: `read path` chose runtime views".
  - Done when: NestJS reads the records it needs for X-93 and X-94 through this endpoint only for its own organization.
  - Tests: handler tests for organization scoping.
  - Progress (2026-10-03): W2 lane, branch go/w2: `internal/reads` serves
    `GET /internal/security/summary` (runs by status, events by type, decision and reason, control
    assessments by control, outcome and verdict source, model usage per purpose, observed timings
    per phase with count, failed, median, p95 and max microseconds),
    `GET /internal/security/assessments?cursor=&limit=` (stable codes and revisions, the verdict
    reduced to `risk_category`, `score`, `reason_code`; a stored row with any other verdict key is
    refused, never passed on) and `GET /internal/security/events?cursor=&limit=` (X-12, including
    events without a run). The organization-wide cursor windows rows by inserting transaction id
    below the oldest running transaction, so each committed row is read exactly once without a
    shared lock; the test commits a lower event id after a higher one and reads both, once each
    (10 of 10 race runs). Timing rows were inserted by the test, since GO-80's writer is not on
    main. Missing: the mount in `internal/api` (3c) and the record shapes in `packages/contracts`.
  - Completed (2026-10-03): mounted by 3c (da44d15). Through the production handler tree the
    owner reads the summary, assessments and organization events (the committed run event
    appears once its transaction is final); another organization gets only its own empty
    records (this test and 3c's GO-57). The record shapes are Go drafts until nestjs lands them
    in `packages/contracts` for API-35 and API-36.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a second disclosure channel)
  - Blocked by: `read path`

## M5: hours 18-21

- **Team focus (report 1.2).** "Polish comprehension and error states; rehearse the complete story; capture evidence from the final build. Run live semantic fixtures and performance measurements on the presentation machine; rehearse configuration changes and ad-hoc input."
- **Exit condition (report).** "A reviewer can understand the task boundary, attempted action,
  decision, actual effect, and limitation without narration filling gaps."
- **Report 1.2 sync points (spine).** X-105.
- **Sync points needed by the end (spine):** X-58 and X-59; X-61 is optional (Tier C).
  SH-32 recaptures the evidence from the final build and SH-33 runs the rehearsal; the Go implementer
  reruns the scenario tests against that build.

### Agent runtime (report role: Implementer 3)

- [ ] **GO-58 · Make every Go stop, failure and denial state readable**
  - **Report 1.2 change:** Readable states add guard failures, rejected reloads and security allowance exhaustion with their reason codes.
  - Owner: Go implementer (report role: Implementer 3, agent runtime) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: GO-29, GO-42, GO-53 · Needs: X-13, X-16 · Provides: nothing
  - Paths: `services/gateway/internal/httpserver/errors.go`; the packages above
  - Work: Check every reason code and terminal state Go emits against the storyboard (X-16): each
    carries its X-13 safe operator message, a provider failure records the actual failure state
    rather than a generic error, and nothing sensitive reaches general error text. Fix what the
    rehearsal (SH-33) shows to be unclear.
  - Done when: the Go states support the M5 exit, "A reviewer can understand the task boundary,
    attempted action, decision, actual effect, and limitation without narration filling gaps.", and
    "On provider failure, show the actual failure state" holds for Go's records.
  - Tests: a table test that every emitted reason code has a safe message and a fixture value; a
    provider-failure test that records the actual failure state. `pnpm --filter gateway run test`.
  - Progress (2026-10-03): W2 lane, branch go/w2. The Go-records half is done; the task stays open
    for the SH-33 rehearsal. `contracts.ReasonCode.SafeMessage()` holds a fixed safe operator
    message for each of the 29 X-13 codes (`TestEveryReasonCodeHasASafeMessage`: matches the
    schema enum and the reason-code fixture, unique, storable as an event's safeMessage). Agent
    run-end events carry a safeMessage, specific for each model failure, and the purpose `agent`
    (`TestModelFailuresNameTheActualFailure`); tool events with a reason carry theirs (the export
    denial asserts it). Provider failure (`TestUnreachableProviderRecordsTheActualFailureState`,
    real stepper, accounted caller and ledger against an unreachable Ollama): `model_calls.outcome`
    `usage_unknown`, the reservation held as `usage_unknown`, the run paused with
    `outcome_unknown` and an event message saying the local model could not be reached (f3's
    GO-79 joins the transport cause; a bad response has its own message; lead: no new X-13 code). Error envelopes: no error text reaches them; admission echoes request ids
    through `%q`. Every other reason-coded event gets its X-13 message from 3c's default in
    `repository.AppendEvent` (go/3c; an emitter's own message wins).
  - Report: "Relative implementation milestones and critical dependencies" (Hours 18-21); "Live
    demonstration storyboard and proof checks" (Reliable demonstrations without invented
    behavior); "Illustrative passport and interface contracts" (Decision and error semantics)
  - Blocked by: nothing

### Enforcement (report role: Implementer 4)

- [x] **GO-59 · Optional: rehearse an unknown outcome**
  - **Report 1.1 change:** Report: "Supporting rehearsals hostile instructions limits and uncertainty".
  - Owner: Go implementer (report role: Implementer 4, enforcement) · Tier: C · Size: S (estimate 1-2 h)
  - Depends on: GO-53 · Needs: X-34 · Provides: X-61
  - Paths: none (a labelled rehearsal in the packages above)
  - Work: Simulate an unknown external result for one action, labelled as a simulation, and show the
    action placed in the attention state instead of being repeated.
  - Done when: X-61 is reached: the rehearsal "would simulate an unknown external result, placing the
    action into an attention-required state instead of automatically repeating an effect that might
    already have succeeded."
  - Tests: the rehearsal test asserts the attention state and no second dispatch. The X-24 command.
  - Report: "Illustrative invoice scenario and future domain adaptations" (Scene 4 cost limits and
    uncertain execution)
  - Blocked by: nothing
  - Completed (2026-10-03): `TestFailedCommitRecordsAnUnknownOutcome`, labelled as a simulation in its evidence line (X-61): the request is cancelled right before the effect's commit, so the outcome cannot be established; the action is placed in the attention state `unknown` with an `action.unknown` event (`outcome_unknown`) and one open attempt, the result is `paused`, and a second dispatch is refused instead of repeating the effect. The executor half of GO-53 implements it (5a6b211). `pnpm test:db gateway` 703 passed, 0 failed, 0 skipped; `pnpm verify` 6/6. The simulated failure is a local commit failure, not a remote provider; no external system exists in the prototype.

## M6: hours 21-24

- **Team focus (report 1.2).** "Freeze the submission build and configuration, fix critical faults before the confirmed deadline, capture test/telemetry evidence, finalize the maximum 10-slide PDF and HackTribe fields, and submit. Preserve the submitted commit and artifact checksums."
- **Exit condition (report).** "Submission checklist is complete; the demonstration matches the
  submitted build and its documented limitations."
- **Sync points needed by the end (spine):** X-62. The Go implementer takes part in the feature freeze and
  the critical-fault fixes (SH-34): "If a critical check fails, either fix it or narrow the
  supported behavior and the claims."

### All Go areas

- [ ] **GO-61 · Supply the Go technical handoff text**
  - **Report 1.2 change:** The handoff adds local model acquisition and setup, the adapter contract, `policy.yaml`, the feed schema and revision, telemetry and the measured limits.
  - **Report 1.1 change:** Adds provenance, templates, the projection and the 13 reason codes; limitation: "The lineage mechanism covers fixed templates and registered adapters".
  - Owner: Go implementer (all report roles on this side) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: SH-34 · Needs: X-02, X-48, X-49, X-59 · Provides: X-62
  - Paths: `services/gateway/README.md`, `docs/architecture.md` (text supplied to integration)
  - Work: Write the Go part of "setup instructions, architecture and boundaries, synthetic-data
    reset, tool contracts, policy fixture, known limitations, dependency disclosures where required,
    and the critical-check outcomes": the packages and their owners, the internal operations, the
    four tools' arguments and results, the decision and reason vocabulary, the accounting rule, the
    host and container setup checked against X-02 and X-49, and the known limitations, among them
    the attention state without a reconciliation operation, the simulated outbox, and that "The
    proposed audit stream is application evidence" and not tamper-proof.
  - Done when: X-62 is reached for the Go area and matches the build identified in X-59, so that
    "the demonstration matches the submitted build and its documented limitations".
  - Tests: a teammate who did not write the Go code follows the Go setup text on a clean checkout
    and runs the go checks.
  - Report: "Research documentation and submission workflow" (From requirements to verified
    presentation); "Durable state idempotency audit and uncertain outcomes" (Evidence without
    creating a second disclosure channel)
  - Blocked by: nothing

## Coverage

### (a) Critical checks, MVP requirements and the Tier A basis

Critical checks ("Validation plan and evidence matrix", "Proposed critical checks"). Every one of
the eighteen involves the Go side. SH-28 and SH-31 record the outcomes.

| Critical check                          | Go tasks that make it pass                                                  | Go evidence    | Recorded by |
| --------------------------------------- | --------------------------------------------------------------------------- | -------------- | ----------- |
| Legitimate task                         | GO-13, GO-26, GO-32, GO-33, GO-34, GO-43, GO-44, GO-45, GO-47               | X-44 (Go half) | SH-28       |
| Resource boundary                       | GO-15, GO-17, GO-28, GO-31, GO-30                                           | X-37           | SH-31       |
| Destination boundary                    | GO-13, GO-28, GO-33, GO-30                                                  | X-38           | SH-31       |
| Field minimization                      | GO-07, GO-22, GO-23, GO-35, GO-56                                           | X-50 (Go half) | SH-31       |
| Approval integrity                      | GO-12, GO-43, GO-45, GO-46                                                  | X-45           | SH-28       |
| Approval replay                         | GO-33, GO-34, GO-44, GO-45, GO-54                                           | X-51           | SH-31       |
| Budget concurrency                      | GO-39, GO-45, GO-50                                                         | X-52           | SH-31       |
| Unknown usage                           | GO-39                                                                       | X-53 (Go half) | SH-31       |
| Waiting-state restart                   | GO-02, GO-08, GO-40, GO-49                                                  | X-54           | SH-31       |
| Cancellation and expiry                 | GO-11, GO-41, GO-45, GO-51                                                  | X-55 (Go half) | SH-31       |
| Organization access                     | GO-14, GO-21, GO-41, GO-44, GO-57; GO-24 and GO-48 if the read path uses Go | X-56 (Go half) | SH-31       |
| Database execution transaction          | GO-34, GO-55                                                                | X-57           | SH-31       |
| Inherited restriction                   | GO-17, GO-27, GO-32, GO-63, GO-64, GO-66                                    | X-72           | SH-28       |
| Label and rename tampering              | GO-32, GO-63, GO-64, GO-68                                                  | X-73           | SH-31       |
| Missing lineage                         | GO-32, GO-63, GO-68                                                         | X-74           | SH-31       |
| Approved external projection            | GO-29, GO-65, GO-67                                                         | X-75           | SH-28       |
| Source or template policy changes       | GO-45, GO-52, GO-70                                                         | X-76           | SH-31       |
| Approval cannot override classification | GO-44, GO-64, GO-69                                                         | X-77           | SH-31       |

Report 1.1 adds the last six rows and moves Resource boundary and Destination boundary out of the vertical slice (SH-31 records them); GO-29 is Tier A for the safe continuation.

The storyboard proofs this side also provides (report 1.1 beats): the internal investigation of
beats 3 and 4 and the M2 exit (GO-23, GO-27, GO-32, GO-63; X-63, observed in SH-36), the denied export
of beat 5 (GO-64, GO-66; X-72), the tampering attempts of beat 6 (GO-68; X-73), the safe continuation
of beat 7 (GO-29, GO-65, GO-67; X-67, X-75, recorded in SH-28), the reviewed delivery of beat 8
(GO-43 to GO-47; X-44) and the limit-triggered stop of beat 9 (GO-39, GO-45, GO-42; X-46, Go half,
recorded in SH-28).

MVP requirements ("Functional requirements MVP boundary and deferred scope", "Proposed MVP
requirements and acceptance evidence"). Every one of the ten involves the Go side.

| Requirement                 | Go tasks                                               | Acceptance evidence (report)                                                              |
| --------------------------- | ------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| Trusted admission           | GO-13, GO-14, GO-21                                    | "Reject an unauthorized task and prevent passport creation."                              |
| Task-scoped actions         | GO-15, GO-17, GO-28, GO-31, GO-32, GO-33, GO-30        | "A forbidden action leaves no report or outbox effect."                                   |
| Exact-action approval       | GO-04, GO-12, GO-40, GO-43, GO-44, GO-45, GO-46        | "Edited and expired proposals do not execute under the original grant."                   |
| Durable execution           | GO-08, GO-09, GO-19, GO-40, GO-49                      | "A worker restart preserves an approval wait and does not replay a successful operation." |
| Bounded usage               | GO-10, GO-39, GO-45, GO-50                             | "Competing requests cannot each spend the same remaining allowance."                      |
| Safe outcomes and retries   | GO-07, GO-16, GO-33, GO-34, GO-53, GO-54               | "A repeated request does not create a second outbox message; uncertainty pauses the run." |
| Data minimization           | GO-23, GO-26, GO-32, GO-33, GO-35, GO-56               | "Inspect model context and output channels for the defined protected fields."             |
| Authorized visibility       | GO-22, GO-57; GO-24 if the read path uses Go endpoints | "A user outside the organization cannot read another organization's run or approval."     |
| Bounded recovery            | GO-29                                                  | "A recoverable branch continues; repeated forbidden proposals terminate at the limit."    |
| Cancellation and revocation | GO-11, GO-41, GO-52, GO-51                             | "Future dispatches stop while committed effects remain correctly recorded."               |

Verification priorities ("Threat model limits and unresolved design choices"):

| Verification priority                                                                                                            | Go tasks                          |
| -------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| "Verify that scope rejection is explicit and the revised request must be resubmitted."                                           | GO-13, GO-14                      |
| "Verify that every adapter checks arguments and resource relationships, not only the tool name."                                 | GO-17, GO-31, GO-32, GO-33, GO-30 |
| "Verify that approval cannot enlarge the passport and changed content requires new review."                                      | GO-44, GO-45, GO-46               |
| "Verify that two concurrent attempts cannot consume one approval or allowance twice."                                            | GO-50, GO-54                      |
| "Verify that cancellation and revocation prevent future dispatch after a review wait."                                           | GO-51, GO-52                      |
| "Verify that blocked operations create no synthetic outbox effect and successful retries do not duplicate it."                   | GO-30, GO-53, GO-54               |
| "Verify that spending uncertainty, simulated effects and unimplemented features remain visible in the report and demonstration." | GO-33, GO-36, GO-39, GO-61        |

The Tier A basis: the smallest credible vertical slice and the four protections kept intact under
time pressure.

| Element (report)                       | Go tasks                                         |
| -------------------------------------- | ------------------------------------------------ |
| One authorized invoice/report workflow | GO-47, through the chain GO-13 to GO-45 it names |
| One denied out-of-scope operation      | GO-30, GO-36                                     |
| One exact-action approval              | GO-43, GO-44, GO-45, GO-46                       |
| One limit-triggered stop               | GO-42                                            |
| Service-side authorization             | GO-13, GO-21, GO-44                              |
| Scope enforcement                      | GO-15, GO-17, GO-28, GO-31, GO-32, GO-33         |
| Pre-dispatch limits                    | GO-11, GO-39, GO-45                              |
| Exact-action approval                  | GO-43, GO-44, GO-45                              |

Tool argument boundaries ("Proposed tool argument boundaries") and passport field groups ("Passport
fields and their purpose"):

| Item                                                                  | Go tasks                               |
| --------------------------------------------------------------------- | -------------------------------------- |
| `read_invoice` (`invoice_id`)                                         | GO-17; the gate in GO-15               |
| `read_vendor` (`vendor_id`)                                           | GO-31, GO-35; the gate in GO-28        |
| `create_report` (authorized invoice references, registered template)  | GO-32; the gate in GO-28               |
| `queue_report` (stored report reference, trusted recipient reference) | GO-33, GO-43, GO-45; the gate in GO-28 |
| Identity                                                              | GO-13, GO-21                           |
| Versions                                                              | GO-13, GO-52                           |
| Capabilities                                                          | GO-13, GO-15                           |
| Data and destinations                                                 | GO-13, GO-23, GO-28, GO-35             |
| Limits                                                                | GO-11, GO-13, GO-29, GO-39, GO-45      |
| Lifetime                                                              | GO-11, GO-13, GO-40, GO-45, GO-52      |

Internal operations NestJS calls (spine, "Internal runtime operations"):

| Operation                                       | Go task                                                                     | Sync point |
| ----------------------------------------------- | --------------------------------------------------------------------------- | ---------- |
| `POST /internal/runs`                           | GO-14                                                                       | X-28       |
| Cancel command (named at M0)                    | GO-41                                                                       | X-42       |
| Approval decision command (named at M0)         | GO-44                                                                       | X-40       |
| Run and usage view; sanitized events by cursor  | GO-24, if the read path uses Go                                             | X-29, X-30 |
| Exact review payload                            | GO-48, if the read path uses Go                                             | X-41       |
| Stored report and registered template           | GO-37, if `stored report read` uses Go                                      | X-64       |
| Task form options                               | GO-25, if `form options` uses Go                                            | X-25       |
| Events as server-sent events (optional, Tier C) | GO-60, if events stream from Go                                             | X-60       |
| Replay trigger through NestJS (conditional)     | none until `replay entry` chooses it; the Go implementer adds the task then | X-65       |

Sync points this side provides, each in or before the milestone that needs it (spine, "Sync
points"):

| Needed by       | Sync point and Go task                                                                                                |
| --------------- | --------------------------------------------------------------------------------------------------------------------- |
| M1              | X-15 GO-18 and GO-62; X-25 GO-25 (conditional); X-27 GO-21; X-28 GO-14; X-29 and X-30 GO-24 (conditional); X-32 GO-09 |
| M2              | X-36 GO-36; X-37 and X-38 GO-30; X-63 GO-27; X-64 GO-37 (conditional)                                                 |
| M3              | X-40 GO-44; X-41 GO-48 (conditional); X-42 GO-41; X-44 GO-47; X-45 GO-46; X-46 GO-42; X-67 GO-29 (provided at M2)     |
| M4              | X-50 GO-56; X-51 GO-54; X-52 GO-50; X-53 GO-39 (provided at M3); X-54 GO-49; X-55 GO-51; X-56 GO-57; X-57 GO-55       |
| M6              | X-62 GO-61                                                                                                            |
| None (optional) | X-60 GO-60; X-61 GO-59                                                                                                |

**Totals (estimates, not a schedule).** 86 tasks; their ranges add up to 157-316 h, 236.5 h at the
midpoints (Tier A 191.5 h, Tier B 41.5 h, Tier C 3.5 h). That includes 5 h of decide tasks, 7 h of
conditional endpoint tasks of which only one outcome survives and the conditional GO-71 (3 h) and
GO-83 (2 h); without them and Tier C it is 216 h. Report 1.1 added GO-63 to GO-71 (24.5 h at the
midpoints) and report 1.2 added GO-72 to GO-86 (46 h). The four estimates predate both reports (their
Go midpoints run from 68.25 h to 141.5 h; they are not in the repository). Per milestone at the
midpoints: P 5 h, M0 4.5 h, M1 80 h, M2 58.5 h, M3 38.5 h, M4 45 h, M5 3.5 h, M6 1.5 h. One person,
the Go implementer, has at most 24 h in the window (18-20 h under one estimate's own assumption of
focused hours, not a report figure), plus the lead's help. Tier A alone is 130-253 h (191.5 h at the
midpoints), now including M4 work, and M1 holds 54-106 h, of which 53-103 h is Tier A, against at most
4 person-hours. Removing every Tier B and Tier C task does not close the gap: the tiers set the order
of cuts but do not make the work fit, and narrowing inside Tier A is the team decision described in
the spine's "Tiers", recorded through its "Scope changes".

### (b) Diagram components and transitions

Diagram 1 (section 1 of `docs/product/project-architecture.md`, report Figures 1-3): the ten components of the
GO group and the provider outside it.

| Component                                                                              | Go tasks                                                                                 |
| -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| INTERNAL: Internal API, "Verify service identity and operator context"                 | GO-21; commands GO-14, GO-41, GO-44; conditional reads GO-24, GO-25, GO-37, GO-48; GO-57 |
| ADMIT: Admission controller, "Issue immutable task passport"                           | GO-13, GO-14                                                                             |
| RUNNER: Agent worker, "Claim durable jobs and run bounded loop"                        | GO-08, GO-09, GO-11, GO-26, GO-29, GO-40, GO-49                                          |
| MODEL: Model gateway, "Call limits, token caps and cost reservations"                  | GO-06, GO-10, GO-39                                                                      |
| GATE: Action gate, "Schema, task scope, data rules and limits"                         | GO-12, GO-15, GO-28, GO-29, GO-52                                                        |
| APPROVAL: Approval manager, "Exact action, expiry and single use"                      | GO-43, GO-44, GO-45                                                                      |
| EXEC: Tool executor, "Recheck, reserve and execute by action ID"                       | GO-16, GO-34, GO-45, GO-53                                                               |
| ADAPTERS: Typed tool adapters (read invoice, read vendor, create report, queue report) | GO-07, GO-17, GO-31, GO-32, GO-33                                                        |
| FILTER: Data minimization, "Scoped fields and opaque references"                       | GO-23, GO-35, GO-56                                                                      |
| STATE: Runtime repository, "Jobs, leases, approvals, usage and audit"                  | GO-19, GO-20, GO-22                                                                      |
| LLM (outside GO): Approved LLM provider, "Credentials held by Go"                      | GO-06, GO-10; X-04                                                                       |

Not diagram components: the labelled replay (GO-05, GO-36) and the Go DTO mirrors (GO-18, GO-62).

Diagram 1 edges that touch the GO group (24 of 32; the other eight stay inside WEB, NEST and
POSTGRES):

| Edge (label)                                                   | Go tasks                                 | Sync points      |
| -------------------------------------------------------------- | ---------------------------------------- | ---------------- |
| FACADE -> INTERNAL ("Private authenticated API")               | GO-21; GO-14, GO-41, GO-44               | X-26, X-27       |
| INTERNAL -> ADMIT ("Start run")                                | GO-14                                    | X-28             |
| INTERNAL -> APPROVAL ("Operator decision")                     | GO-44                                    | X-40             |
| INTERNAL -> STATE ("Cancel or revoke")                         | GO-41 (cancellation); revoke: note below | X-42             |
| ADMIT -> RUNNER                                                | GO-13, GO-08                             | none             |
| RUNNER -> MODEL                                                | GO-10                                    | none             |
| RUNNER -> GATE ("Proposed action")                             | GO-11, GO-15                             | none             |
| GATE -> EXEC ("Allowed")                                       | GO-16, GO-45                             | none             |
| GATE -> APPROVAL ("Approval required")                         | GO-15, GO-43                             | none             |
| APPROVAL -> GATE ("Approved stored action")                    | GO-40, GO-44, GO-45                      | none             |
| GATE -> RUNNER ("Blocked action and reason")                   | GO-15, GO-11; GO-29 adds correction      | none             |
| EXEC -> ADAPTERS                                               | GO-16                                    | none             |
| ADAPTERS -> FILTER ("Tool result")                             | GO-23, GO-35                             | none             |
| FILTER -> RUNNER                                               | GO-23                                    | none             |
| RUNNER -> STATE                                                | GO-08, GO-11, GO-19                      | none             |
| MODEL -> STATE                                                 | GO-10, GO-39                             | none             |
| GATE -> STATE                                                  | GO-15, GO-22                             | none             |
| APPROVAL -> STATE                                              | GO-43, GO-44                             | none             |
| EXEC -> STATE                                                  | GO-16, GO-34                             | none             |
| ADMIT -> APPDB ("Read authoritative task and policy versions") | GO-13                                    | X-18, X-21, X-35 |
| GATE -> APPDB ("Read current revocations")                     | GO-52                                    | X-47, X-35       |
| STATE -> RUNDB ("Owns runtime writes")                         | GO-19, GO-38                             | X-19, X-39, X-35 |
| ADAPTERS -> DEMODB ("Narrow database permissions")             | GO-17, GO-31, GO-32, GO-33, GO-38        | X-20, X-33, X-35 |
| MODEL -> LLM ("Governed model requests and responses")         | GO-06, GO-10                             | X-04             |

FEED -> RUNDB ("Read authorized event view") stays outside GO; if the read path chooses Go
endpoints instead, GO-24 and GO-48 serve those reads.

On this roadmap the only caller of INTERNAL -> STATE is the cancel command (GO-41, X-42), as the
spine's "Diagram 1 boundary crossings" says. The report: "The diagram's internal cancel-or-revoke
path should not imply that Go can directly edit every NestJS-owned policy record." Revocation
writes stay in NestJS (X-47); GO-52 reads them under GATE -> APPDB.

Diagram 2 (section 2 of `docs/product/project-architecture.md`, report Figures 4-9): all 51 transitions, grouped
by the task that covers them.

| Transitions                                                                                                                                     | Go tasks                                                    |
| ----------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| START -> AUTH                                                                                                                                   | none: Next.js + NestJS side (X-31)                          |
| AUTH -> ADMIT                                                                                                                                   | GO-21, GO-14 (X-26, X-27, X-28)                             |
| ADMIT -> VALID; VALID -> REJECT; VALID -> QUEUE                                                                                                 | GO-13; GO-14 answers the rejection                          |
| QUEUE -> CLAIM                                                                                                                                  | GO-08                                                       |
| CLAIM -> LIVE; LIVE -> STOP; SUCCESS -> LIVE                                                                                                    | GO-11                                                       |
| LIVE -> MRESERVE; MRESERVE -> MFITS; MFITS -> STOP; MFITS -> MODEL                                                                              | GO-39 (until M3 GO-10 records each dispatch)                |
| MODEL -> MOK; MOK -> MFAIL; MFAIL -> FAILED; MOK -> USAGE; USAGE -> OUTPUT                                                                      | GO-10; GO-39 settles usage and keeps uncertain reservations |
| OUTPUT -> FINAL; FINAL -> COMPLETE                                                                                                              | GO-26                                                       |
| OUTPUT -> STORE; STORE -> POLICY; POLICY -> DECISION; DECISION -> DENY                                                                          | GO-15; GO-28 adds relationships and destinations            |
| DENY -> CORRECT; CORRECT -> STOP; CORRECT -> LIVE                                                                                               | GO-29                                                       |
| DECISION -> FREEZE                                                                                                                              | GO-43                                                       |
| FREEZE -> WAIT; ENQUEUE -> RESUME                                                                                                               | GO-40                                                       |
| WAIT -> HUMAN                                                                                                                                   | GO-40 (event); GO-48 if the read path uses Go (X-41)        |
| HUMAN -> APPROVED; APPROVED -> DENY; APPROVED -> ENQUEUE                                                                                        | GO-44 (X-40); GO-40 for an undecided approval's expiry      |
| APPROVED -> STOP                                                                                                                                | GO-41 (X-42)                                                |
| DECISION -> RECHECK; RESUME -> RECHECK; RECHECK -> CURRENT; CURRENT -> DENY; CURRENT -> RESERVE; RESERVE -> TFITS; TFITS -> STOP; TFITS -> EXEC | GO-45 (until M3 GO-16); GO-52 adds revocations              |
| EXEC -> OUTCOME; OUTCOME -> SUCCESS                                                                                                             | GO-16; GO-34 for the demo effects                           |
| OUTCOME -> TFAIL; TFAIL -> RETRY; RETRY -> FAILED; RETRY -> RECHECK; OUTCOME -> UNKNOWN; UNKNOWN -> ATTENTION                                   | GO-53; GO-59 rehearses UNKNOWN (optional)                   |

### (c) Report items that need no Go task

Two Go-side obligations in the report are constraints rather than work, so no task implements
them on its own:

- "The modules inside the Go group run in one Go service" and the database groups are schemas in
  one PostgreSQL instance: constraint 2 above; the schemas are decision 1.
- One migration history in the TypeORM tooling, which Go consumes without a migration framework of
  its own: constraint 4 above; the migrations are SH-15, SH-16, SH-17, SH-24, SH-27 and SH-38 in
  the spine.

During planning on 2026-10-03 the report's Go-side obligations were listed and mapped to the tasks
in this file. That working list is not kept in the repository; each task's "Report" field names its
source in the report, and the tables above map the report's checks, requirements, components and
transitions to tasks.
