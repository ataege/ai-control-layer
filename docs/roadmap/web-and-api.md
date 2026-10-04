# Next.js and NestJS side roadmap

**Status.** Plan only: every task is open and nothing in it is implemented. Derived from the
project report, `docs/product/task-passport-project-report.docx` (version 1.2, "Official requirements and hybrid security controls", 3 October 2026; lines marked
"Report 1.2 change" amend a task and win over older text and "Report 1.1 change" lines), from the architecture specification
`docs/product/project-architecture.md`, from the spine `docs/roadmap/README.md` and from the
repository at commit `789bcd7`. Lines marked "Report 1.1 change" amend the task they sit in; where
they disagree with the older fields, they win. Sizes are estimates, not a schedule. The milestone windows are relative to the report's
24-hour coding window; the organizers' confirmed rules and deadline take precedence. The
authentication design is on hold (decision 7), so the M1 exit cannot be reached until the hold is
lifted (see "The authentication hold").

## Who is on this side

From the spine, "Sides and people", and the report's table "Proposed team ownership" ("Delivery
scope and six person ownership", report 1.1):

| Person (report roles)                                                            | Responsibility (report 1.1)                                                                                                                                                                                                                                                                                                                                                                        | Agents and paths (AGENTS.md)                                                  | IDs      |
| -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | -------- |
| Web + API implementer (Implementer 1, interface; Implementer 2, application API) | Interface: task form, passport summary, run timeline, approval preview, terminal states, report classification, authorized source trail, export denial and approved alternative template. Application API: NestJS authentication, membership checks, task configuration, runtime facade, authorized event feed, versioned source-policy and projection configuration; coordinates shared contracts | frontend: `apps/web`, `packages/ui`; nestjs: `apps/api`, `packages/contracts` | API, WEB |

Report 1.2 adds the policy-file and feed imports, catalog activation, the security summary and audit
export and the live test entry (API-31 to API-38), and the posture views (WEB-29 to WEB-32). One person does both, so the API
and WEB tasks run in sequence, not in parallel; the lead helps when
needed. By default the same person also writes the migrations and seeds on the shared track
(`shared-track assignment`).

First integrated deliverables (report; the milestone is the spine's placement):

- Implementer 1: "Start a run and display a real persisted event through the NestJS API." M1,
  WEB-06.
- Implementer 2: "An authenticated command reaches Go with verifiable actor and organization
  context." M1, API-10.

The Go side (the Go implementer) plans in `docs/roadmap/go.md`; the shared track (the web + API
implementer for migrations and seeds, the lead for roles, Compose, deployment, smoke and evidence, by
default) and the researcher track (not assigned) are in the spine.

## How to read this file

- **The spine is the contract.** Its milestones, tiers, sync points (X), shared tasks (SH), open
  decisions, task format and "Definition of done" bind this file. This file refers to the Go side
  only through X and SH IDs. Read the full report first, then `docs/product/project-architecture.md` and
  `docs/product/README.md` (AGENTS.md,
  "Read the project report first").
- **IDs.** `API-nn` are NestJS tasks and `WEB-nn` are Next.js tasks; one person, the web + API
  implementer, does both, in sequence.
  They are numbered in milestone order, and inside each milestone the API tasks come first. IDs are
  permanent: a dropped task keeps its ID with "Dropped: reason", and a new task takes the next free
  number.
- **Task fields.** "Depends on" lists tasks of this file or SH tasks that must be done before this
  task is done; it fixes the order in which tasks are ticked, not when work on them starts (spine,
  "IDs"), so work may begin earlier against the contract fixtures (see "Fixtures, not mocks");
  SH-14, RS-05 and RS-06 stay open until M6, and SH-23 until M3, so tasks refer to them in their
  text only. "Needs" and "Provides" use the spine's X IDs. "Paths" names existing paths; new code is
  "new module, named at M0 by its owner", because this roadmap names no module, route or table.
  "Blocked by" cites the spine's strings verbatim, backticks included.
- **Sync points delivered in parts.** `Provides: X-31 (part)` means the task delivers part of this
  side's share of a sync point (spine, "Sync points"). This side has delivered its share when every
  task marked "(part)" for it is done; a sync point with two providers in the spine is reached only
  when both have delivered. "Coverage" lists each set.
- **Decisions.** The decide tasks live in the spine: SH-01 (decision 7, authentication), SH-02
  (decision 3, browser to API path), SH-03 (decision 4, operator context to Go) and SH-05 (the read
  path). This file holds the build tasks they block and has no decide task of its own. Each open
  item is planned for every outcome the spine names.
- **Contracts.** SH-11 (the web + API implementer) lands the frozen contracts X-07 to X-13 in
  `packages/contracts`, and SH-39 lands X-14; this file adds no landing task. Later changes go
  through SH-14's quick shared review and "Changing a shared contract" in `docs/team-workflow.md`,
  with the Swagger DTO classes and the web consumers updated in the same change.
- **Tiers.** A: needed for the smallest credible vertical slice or for one of the four protections
  the report keeps intact. B: the rest of the MVP requirements, acceptance evidence and critical
  checks. C: optional breadth, cut first. Deferred features are not on this roadmap.
- **Sizes.** S up to 4 h, M above 4 h up to 10 h, L above 10 h up to 20 h, by the midpoint of the
  range. They start from the Next.js and NestJS rows of the four estimates of 2026-10-03 that the
  spine describes, split where a task covers part of a row; tasks that carry shared-row work (the
  M0 checks, the evidence runs) are this roadmap's estimates.
- **Tests.** Every task also runs the checks of its area from the spine's "Definition of done" and
  then `pnpm verify`, with results quoted. API tests use Vitest with `supertest` and stub servers
  (`pnpm --filter api run test`); database-backed tests use the command from SH-21 (X-24). Web tests
  are pure-function Vitest tests in the node environment (`pnpm --filter web run test`) plus a
  quoted browser check: component and browser end-to-end tests are an open item in
  `docs/preparation-record.md` ("Deferred at the baseline") and need a new dependency through
  integration.
- **Fixtures, not mocks.** Tests may use the contract fixtures in `packages/contracts/fixtures`. No
  page shows fixture or invented data as real: "A mocked response can unblock interface
  development, but it must be replaced or clearly labeled before any result is presented as a
  working capability" ("Delivery scope and six person ownership").

### The authentication hold

The authentication design (decision 7) is on hold by the user's decision of 2026-10-03, and nothing
about its mechanism is decided. Decisions 3 and 4 wait on it (SH-02, SH-03). The spine: "While the
hold stands, the M1 exit cannot be reached, and by the 'Depends on' column of 'Sync points' neither
can X-14, X-22, X-23, X-26, X-27, X-28, X-31, X-40, X-42, X-43, X-55 and X-56 (it needs the second
organization's sign-in from X-34), nor the optional X-60 and the conditional X-65. That column names
direct dependencies only; through their providing tasks' 'Depends on' and 'Needs', more sync points
wait, among them X-44 to X-47, X-50 to X-54, X-57 to X-59 and X-62, the optional X-61, and X-25,
X-29, X-30, X-41 and X-64 when a Go endpoint serves them (X-25 also when app records do)." This file
records that and does not resolve it. The spine also notes that the task, policy and tool tables
and their seed can be written and applied during the hold, while SH-15 and SH-18 stay unticked
until the user records land.

Two proposals are recorded and are proposed, not decided: the authentication proposal listed in
SH-01, and the same-origin forwarder listed in SH-02. No task here builds against either before its
decision is recorded.

Work that does not wait on decision 7:

- API-01 and WEB-01 (no product code), and API-02 except its operator context part, which waits on
  decisions 4 and 7; its "Done when" lets SH-10 record that part as a gap.
- API-03 (the deny-by-default guard; it reads no credential, so every protected route answers 501
  until API-06), API-04 (the task, policy and tool entities, whose tables can be migrated during the
  hold) and API-09 (the command client's transport, blocked only by `command timeout budget`).
- API-15, once `worker readiness` is settled, and WEB-07 (it depends on API-04, not on SH-15).
- The pure parts of WEB-03, WEB-05, WEB-06 and WEB-08 to WEB-16 (contract guards, view models,
  state mapping and labels), tested against the contract fixtures. Their "Done when" still needs the
  authenticated path.

## Task overview

Every task in this file, one row each, in milestone order. 70 tasks: 50 Tier A, 16 Tier B, 4 Tier C. 25 name decision 7 (the authentication hold) under "Blocked by". Generated from the task blocks below on 2026-10-03. The task blocks are the source of truth: when you add, drop or rename a task, update its row in the same change.

### NestJS (report role: Implementer 2)

| ID     | When | Tier | Owner                                                                                              | Size         | Task                                                                                                        | Blocked by                                                                                                                          |
| ------ | ---- | ---- | -------------------------------------------------------------------------------------------------- | ------------ | ----------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| API-01 | P    | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 0.5-1 h   | Confirm the migration tooling against the Compose PostgreSQL image                                          | a fix it needs before the coding window waits on `decision 8 in docs/product/README.md`                                             |
| API-02 | M0   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 0.5-1.5 h | Check the frozen examples against the checks the public API makes                                           | `contract owners`; the operator context part also `decision 4 in docs/product/README.md` and `decision 7 in docs/product/README.md` |
| API-31 | M0   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-3 h     | Add the control catalog, active revision and feed records                                                   | nothing                                                                                                                             |
| API-32 | M0   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 3-5 h     | Import and validate `policy.yaml` into an immutable catalog revision                                        | nothing                                                                                                                             |
| API-03 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2.5 h   | Deny every non-public route by default                                                                      | nothing                                                                                                                             |
| API-04 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Add the task template, policy version and tool definition entities                                          | nothing                                                                                                                             |
| API-05 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2.5 h   | Add the user and membership entities                                                                        | `decision 7 in docs/product/README.md`                                                                                              |
| API-06 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | M, 4-7 h     | Authenticate the operator through a real credential check                                                   | `decision 7 in docs/product/README.md`                                                                                              |
| API-07 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2 h     | Accept authenticated browser calls on the path chosen in decision 3                                         | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                      |
| API-08 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3 h   | Resolve the operator context and check organization membership                                              | `decision 7 in docs/product/README.md`                                                                                              |
| API-09 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3 h   | Add a fail-closed command client toward Go                                                                  | `command timeout budget`                                                                                                            |
| API-10 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3 h   | Carry the verified operator context on every runtime command                                                | `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`                                                      |
| API-11 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Start a run through `POST /api/runs`                                                                        | `decision 7 in docs/product/README.md`; `command timeout budget`                                                                    |
| API-12 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3.5 h | Serve the task form's options                                                                               | `form options`; `decision 7 in docs/product/README.md`                                                                              |
| API-13 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | M, 3-6 h     | Serve the run and usage view through `GET /api/runs/{id}`                                                   | `read path`; `passport in the run view`; `decision 7 in docs/product/README.md`                                                     |
| API-14 | M1   | A    | Web + API implementer (report role: Implementer 2, application API)                                | M, 3-6 h     | Serve sanitized events by cursor through `GET /api/runs/{id}/events`                                        | `read path`; `decision 7 in docs/product/README.md`                                                                                 |
| API-15 | M1   | B    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) | S, 1-2 h     | Carry the worker readiness change through the diagnostics route and page                                    | `worker readiness`                                                                                                                  |
| API-16 | M2   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Check role and object access on every read and command                                                      | `decision 7 in docs/product/README.md`                                                                                              |
| API-17 | M2   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2.5 h   | Connect the API with its own database role                                                                  | `decision 2 in docs/product/README.md`; `read path`                                                                                 |
| API-18 | M2   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3 h   | Serve the stored report and its registered template                                                         | `stored report read`; `final result format`; `decision 7 in docs/product/README.md`; `report storage`                               |
| API-28 | M2   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Add the report template and projection rule records                                                         | `vendor projection fields` (rule content)                                                                                           |
| API-29 | M2   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-3 h     | Record trusted source classifications and recipient rules, if `source classification storage` chooses `app` | `source classification storage`                                                                                                     |
| API-30 | M2   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Serve the report classification and trusted source trail to authorized users                                | `stored report read`; `report storage`; `read path`; `decision 7 in docs/product/README.md`                                         |
| API-34 | M2   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Validate and import the signature feed                                                                      | `feed grammar and trust`                                                                                                            |
| API-19 | M3   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Serve the exact review payload to authorized reviewers only                                                 | `read path`; `review payload read`; `decision 7 in docs/product/README.md`                                                          |
| API-20 | M3   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3 h   | Forward approval decisions through `POST /api/actions/{id}/approval`                                        | `decision 7 in docs/product/README.md`                                                                                              |
| API-21 | M3   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2 h     | Forward cancellation through `POST /api/runs/{id}/cancel`                                                   | `decision 7 in docs/product/README.md`                                                                                              |
| API-22 | M3   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3.5 h | Record revocations in the app schema                                                                        | `revocation reads`; `decision 7 in docs/product/README.md`                                                                          |
| API-33 | M3   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 3-5 h     | Serve the authenticated policy reload and activation                                                        | `catalog activation protocol`; `decision 7 in docs/product/README.md`                                                               |
| API-23 | M4   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1.5-3 h   | Record organization access evidence on the public path                                                      | nothing                                                                                                                             |
| API-24 | M4   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2.5 h   | Record field minimization evidence for the activity views                                                   | nothing                                                                                                                             |
| API-25 | M4   | C    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Optional: serve the activity feed over server-sent events                                                   | `read path`; `decision 3 in docs/product/README.md`                                                                                 |
| API-35 | M4   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Serve the security summary                                                                                  | `read path`                                                                                                                         |
| API-36 | M4   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-4 h     | Serve the sanitized audit export                                                                            | `read path`                                                                                                                         |
| API-37 | M4   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2 h     | Prove the audit export and security summary                                                                 | nothing                                                                                                                             |
| API-38 | M4   | A    | Web + API implementer (report role: Implementer 2, application API)                                | S, 2-3 h     | Serve the live test entry for judge input                                                                   | `decision 7 in docs/product/README.md`                                                                                              |
| API-26 | M5   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 1-2 h     | Clean up the API's error states                                                                             | nothing                                                                                                                             |
| API-27 | M6   | B    | Web + API implementer (report role: Implementer 2, application API)                                | S, 0.5-1.5 h | Supply the NestJS part of the technical handoff                                                             | nothing                                                                                                                             |

### Next.js (report role: Implementer 1)

| ID     | When | Tier | Owner                                                                                              | Size         | Task                                                                      | Blocked by                                                                                            |
| ------ | ---- | ---- | -------------------------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| WEB-01 | M0   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 0.5-1.5 h | Check the frozen examples against what the interface must show            | `contract owners`                                                                                     |
| WEB-02 | M1   | A    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) | M, 3-7 h     | Build the browser to API path chosen in decision 3                        | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                        |
| WEB-03 | M1   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1.5-4 h   | Add a typed client for the product operations                             | `decision 3 in docs/product/README.md` (the transport only)                                           |
| WEB-04 | M1   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-5 h     | Add sign-in, the operator display and the development demonstration label | `decision 7 in docs/product/README.md`; `smoke under login`                                           |
| WEB-05 | M1   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2.5-5 h   | Build the task form                                                       | `form options`; `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`        |
| WEB-06 | M1   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-4 h     | Show the run's persisted events on the run page                           | `read path`; `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`           |
| WEB-07 | M1   | B    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 0.5-1.5 h | Replace the starter texts that the product makes untrue                   | nothing                                                                                               |
| WEB-08 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1.5-4 h   | Show the passport summary beside the timeline                             | `passport in the run view`                                                                            |
| WEB-09 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | M, 3-6 h     | Build the run timeline with attempts apart from effects                   | nothing                                                                                               |
| WEB-10 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-4.5 h   | Show the run's waiting and terminal states with their reasons             | nothing                                                                                               |
| WEB-11 | M2   | B    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1-2.5 h   | Explain an admission rejection and require explicit resubmission          | nothing                                                                                               |
| WEB-12 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1-3 h     | Render the report from the stored report and its registered template      | `stored report read`; `final result format`                                                           |
| WEB-13 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1-2 h     | Label the simulated outbox, replays and the development demonstration     | nothing                                                                                               |
| WEB-27 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-4 h     | Show the report classification and source trail on the run page           | `stored report read`                                                                                  |
| WEB-28 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-4 h     | Explain the export denial and the safe continuation                       | nothing                                                                                               |
| WEB-32 | M2   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-3 h     | Show hybrid decisions in the run timeline                                 | nothing                                                                                               |
| WEB-14 | M3   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | M, 3.5-6.5 h | Build the approval preview of the stored action                           | `review payload read`; `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md` |
| WEB-15 | M3   | B    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 0.5-1.5 h | Add the cancel control that says cancellation is not a reversal           | `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`                        |
| WEB-16 | M3   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1.5-3.5 h | Show usage with reported, reserved and estimated amounts apart            | `decision 6 in docs/product/README.md` (the estimated cost display)                                   |
| WEB-17 | M3   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 0.5-1.5 h | Show the limit-triggered stop with its terminal reason                    | nothing                                                                                               |
| WEB-18 | M3   | A    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) | S, 1-3 h     | Run the legitimate task through the interface                             | nothing                                                                                               |
| WEB-29 | M3   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-3 h     | Show the active controls, policy revision and reload state                | nothing                                                                                               |
| WEB-19 | M4   | B    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 0.5-1.5 h | Show unknown usage as uncertain on a real run                             | `decision 6 in docs/product/README.md`                                                                |
| WEB-20 | M4   | B    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) | S, 0.5-1.5 h | Repeat the workflow through the interface after a reset                   | nothing                                                                                               |
| WEB-21 | M4   | C    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1-3 h     | Optional: receive the activity feed over server-sent events               | `decision 3 in docs/product/README.md`                                                                |
| WEB-22 | M4   | C    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) | M, 3-6 h     | Optional: show the demonstration baseline in the interface                | `demonstration baseline`; `read path`                                                                 |
| WEB-30 | M4   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 3-5 h     | Build the security posture dashboard                                      | nothing                                                                                               |
| WEB-31 | M4   | A    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1-2 h     | Offer the authorized audit export                                         | nothing                                                                                               |
| WEB-23 | M5   | B    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 2-4 h     | Polish the error states                                                   | nothing                                                                                               |
| WEB-24 | M5   | B    | Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) | S, 1-3 h     | Make every demonstration beat observable for the rehearsal                | nothing                                                                                               |
| WEB-25 | M5   | C    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 1-3 h     | Optional: polish comprehension beyond the exit condition                  | nothing                                                                                               |
| WEB-26 | M6   | B    | Web + API implementer (report role: Implementer 1, interface)                                      | S, 0.5-1.5 h | Supply the Next.js part of the technical handoff                          | nothing                                                                                               |

## Constraints for this side

- **NestJS forwards; Go decides.** NestJS authenticates and forwards commands. It never performs
  agent effects and never writes runtime decisions, approval grants, execution states or budget
  balances. Source: AGENTS.md guardrail 4; report "Technical architecture and service ownership"
  ("NestJS forwards authenticated commands rather than performing agent effects itself") and "Data
  ownership and the transition from starter to product" ("should not directly edit approval grants,
  execution states or budget balances").
- **Next.js presents.** It calls NestJS only and cannot grant scope or execute tools; the approval
  preview renders the stored action from the server, never a browser-edited payload. Source: report
  "Technical architecture and service ownership", table "Proposed ownership" ("Calls NestJS; cannot
  grant scope or execute tools"); `.claude/agents/frontend.md`.
- **Identity from verified context.** Organization, actor and reviewer authority come from the
  authenticated context and trusted records, never from browser-supplied identifiers, model output
  or tool results. Organization and object access are checked on every read and every command.
  Source: AGENTS.md guardrail 5; report "Users operating model and proposed user journeys" ("The
  organization boundary would be enforced in the application and runtime, not inferred from a
  record identifier supplied by the browser or agent"), "Data ownership and the transition from
  starter to product" ("An invoice ID or action ID is a reference, not authorization") and the table
  "Proposed browser and runtime operations" ("Organization and object access checked on every
  read").
- **Fail closed.** A missing policy, an unavailable dependency, a transport error, a timeout or a
  configuration error is never an allow and never a success. Source: AGENTS.md guardrail 6; report
  "Illustrative passport and interface contracts" ("A transport or configuration error is not an
  allow decision"). Roadmap rule, not a report quote: a command whose outcome is unknown is shown
  as unconfirmed and is not resubmitted automatically. Report basis: "Users operating model and
  proposed user journeys" (Journey 3 recover cancel or investigate: "the product would not quietly
  convert a timeout into permission to repeat a potentially completed action").
- **Real authentication only.** No allow-all guard, no fabricated identity and no fake login;
  `UnimplementedAuthProvider` answers 501 until a real provider replaces it. A seeded demo operator
  exists only through an explicit seed command (SH-19), passes a real credential check and is
  labelled as a development demonstration. Source: AGENTS.md guardrail 2; report "Architecture and
  chart reading guide" (Interpreting the full architecture).
- **No secrets in the browser.** The browser never receives `GATEWAY_SERVICE_TOKEN`, a provider
  credential or any other server secret: none in a `NEXT_PUBLIC_*` variable, a response, a log line
  or a web asset. Source: AGENTS.md working rules 8 and 9; report "Architecture and chart reading
  guide" ("The browser must never receive provider credentials, tool credentials, internal service
  secrets, or an unrestricted runtime command interface").
- **Request-time configuration.** Route handlers read server configuration at request time. Next.js
  rewrites resolve their destination at build time and do not fit this repository: the web image
  builds without `.env` and Compose passes `API_UPSTREAM_URL` only at runtime. Source: experiment
  of 2026-10-03 with Next.js 16.3.8 (rewrites resolve at build time), not recorded in the
  repository; `infra/docker/web.Dockerfile` ("The build needs no .env"); `infra/compose.yaml` (the
  web service's `API_UPSTREAM_URL`); `apps/web/src/server/upstream-proxy.ts` (reads
  `API_UPSTREAM_URL` per request); `.claude/agents/frontend.md`.
- **Until decision 3 is recorded**, the web app keeps its exact-path allowlist and forwards no
  caller-supplied URL or host and no cookie. Source: `.claude/agents/frontend.md`.
- **If decision 3 chooses a forwarding route handler** (proposed, not decided), it builds the
  upstream headers from an allowlist, rejects encoded slashes, backslashes and dot segments, relays
  every `Set-Cookie` with `headers.getSetCookie()` and sends `Cache-Control: no-cache, no-transform`
  for event streams; the API trusts no `x-forwarded-*` header it did not set. Three behaviours of
  today's `proxyUpstreamGet` would break that path if kept: it drops the caller's query string (its
  test asserts this), so a cursor sent in the query never reaches the API; it answers 502
  `upstream_invalid_response` for an empty body, so a 204 cannot pass; and its 10 s deadline also
  bounds reading the body, so a relayed stream ends after 10 s. Source: experiments of 2026-10-03
  with Next.js 16.3.8, not recorded in the repository; `apps/web/src/server/upstream-proxy.ts` and
  its test.
- **NestJS facts for a cookie-based mechanism** (only if decision 7 chooses cookies): Express 5 has
  no `req.cookies` without a parser, and `@Post` answers 201, so sign-in and sign-out need
  `@HttpCode(200)` or `@HttpCode(204)`; a 204 passes the web path only once the proxy and
  `fetchJson` accept an empty body (today they answer 502 and `invalid_json`). Source: experiments
  of 2026-10-03 with NestJS 12.1.2 on Express 5, not recorded in the repository;
  `apps/web/src/server/upstream-proxy.ts`; `apps/web/src/lib/fetch-json.ts`.
- **Deny by default.** A global guard denies every route without a public marker; unknown routes
  still answer 404; a guard that needs the database checks `dataSource.isInitialized` and answers
  503 while the database is down, because a repository used before initialization throws a plain
  error that the filter maps to 500. Source: AGENTS.md guardrails 2 and 6; report "Validation plan
  and evidence matrix" (Interpreting results honestly: "a hidden URL is not a protection");
  `apps/api/src/database/database-initializer.service.ts` (background initialization) and
  `apps/api/src/common/all-exceptions.filter.ts` (an error that is not an HTTP exception becomes
  500); the NestJS 12 behaviour of the guard and the marker was verified in an experiment of
  2026-10-03, not recorded in the repository.
- **No startup side effects.** `uuidExtension: "pgcrypto"` and `installExtensions: false` go into
  the shared options factory before the first entity; `synchronize`, `migrationsRun` and
  `dropSchema` stay false; nothing at application startup runs migrations, creates tables or loads
  seed data. Source: AGENTS.md guardrail 3; the spine, "Schema ownership" (verified by experiment
  on 2026-10-03); `apps/api/src/database/typeorm-options.ts`; report "Data ownership and the
  transition from starter to product".
- **Schemas.** App entities carry `schema: "app"`; runtime and demo tables are not NestJS entities;
  raw SQL is schema-qualified because TypeORM never sets `search_path`. Source:
  `.claude/agents/nestjs.md`; `docs/team-workflow.md` ("Adding the first entity and migration");
  the spine, "Schema ownership".
- **New variables.** A new API variable goes through `environmentSchema.extend` (not
  `databaseEnvironmentSchema`, which the TypeORM CLI also parses) and a typed getter in
  `AppConfigService`, then, through infrastructure, `.env.example`, the Compose environment map of
  each service that reads it and, for a secret, the dev runner's web filter; a generated secret also
  needs an entry in `scripts/setup.mjs`. Source: `apps/api/src/config/environment.ts`;
  `apps/api/src/config/app-config.service.ts`; `scripts/dev.mjs` (`WEB_FORBIDDEN_VARIABLE_PATTERN`);
  `scripts/setup.mjs`; `infra/compose.yaml`; `docs/team-workflow.md` ("Branching and integration
  habits").
- **Contracts.** A contract change lands the type, schema and fixtures together and keeps the fixture
  test registrations (`typedSamples` and `fixtureFileOfSample`); `packages/contracts` cannot use
  TypeScript enums; a response shape never changes on one side only. Source:
  `packages/contracts/test/fixtures.test.ts` (`typedSamples`, `fixtureFileOfSample`);
  `packages/config/tsconfig/node-library.json` (`erasableSyntaxOnly`, which rules out enums);
  `docs/team-workflow.md` ("Changing a shared contract"); the spine, "Contracts to freeze first".
- **Truthful labels.** The simulated outbox, replays, sample data, the development demonstration and
  estimated cost are labelled. Limits shown come from the persisted passport and policy; the
  report's example limits are illustrative values, not requirements. Source: AGENTS.md guardrail 9;
  report "Live demonstration storyboard and proof checks" and "Atomic allowances hard limits and
  estimated cost".
- **Health and diagnostics stay public.** Health and diagnostics stay public in local development
  unless the team decides otherwise. `pnpm smoke` loads the home, components and diagnostics pages
  without a session, so gating any of them breaks it; how smoke reaches product pages behind a
  sign-in is the open item `smoke under login`. No source decides whether the home and components
  pages need a sign-in; WEB-04's owner settles that after decision 7 and `smoke under login`.
  Source: AGENTS.md guardrail 2; `scripts/smoke.mjs`.
- **Request id and envelope.** Every new call propagates `x-request-id`, and every failure uses
  `ErrorResponse` with safe, fixed 5xx messages; calls to other services have bounded timeouts.
  Source: the spine, "Request id and error envelope (in the starter)"; `.claude/agents/nestjs.md`.
- **No pre-created structure.** A module, page or route lands with its first real code; names of new
  modules, routes and tables are decided by their owner at M0, not by this roadmap. Source: AGENTS.md
  guardrail 1; the spine, "Internal runtime operations" and "Schema ownership" (names decided at the
  M0 freeze, SH-10).

## P (before the coding window)

The report gives no window or exit condition for P. The spine's roadmap condition, not a report
quote: "the decisions the first contracts depend on are recorded or carried into M0 as open, every
implementer machine passes `pnpm verify`, and the organizers' answer on decision 8 is recorded or
its absence is stated." Code written before the coding window waits on decision 8 (RS-01, X-01), so
no task in this section writes code.

This side's person in the spine's P tasks: the web + API implementer owns SH-01 (decision 7, blocked
by the hold), SH-02 (decision 3) and SH-03 (decision 4, with the Go implementer), gives the contract
owners to the lead in SH-07, joins SH-05 (the read path) and prepares their machine in SH-08.

### NestJS (report role: Implementer 2)

- [ ] **API-01 · Confirm the migration tooling against the Compose PostgreSQL image**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 0.5-1 h, this roadmap's
    estimate)
  - Depends on: SH-08 · Needs: nothing · Provides: nothing
  - Paths: `apps/api/scripts/typeorm-cli.mjs`, `apps/api/src/database/typeorm-options.ts`,
    `apps/api/src/database/data-source.ts`, `db/migrations/README.md`
  - Work: The five migration commands were verified on 2026-10-03 against an embedded PostgreSQL,
    not the `postgres:18-alpine` image that Compose uses (`README.md`, "Verification status"). On a
    machine with Docker, run the zero-entity sequence of `db/migrations/README.md` against that
    image; this runs commands only and changes no file. Supply the results to integration for
    "Verification status".
  - Done when: against the image, "Configuration loading and an empty migration list should work,
    while generation with no entities may correctly report no changes": `pnpm db:migration:show`
    lists no migration and `pnpm db:migration:generate <Name>` exits non-zero with "No changes in
    database schema were found", both outputs quoted.
  - Tests: `pnpm infra:up`, `pnpm db:migration:show`, `pnpm db:migration:generate <Name>`,
    `pnpm db:migration:revert` (no migration to revert), a table listing that shows only the
    bookkeeping table `migrations` in `public`, then `pnpm infra:down`.
  - Report: "Data ownership and the transition from starter to product"; "Design decision record"
    (TypeORM migration toolchain)
  - Blocked by: a fix it needs before the coding window waits on `decision 8 in docs/product/README.md`

### Next.js (report role: Implementer 1)

The interface work has no task of its own in P: SH-02 (decision 3) and SH-08 cover it.

## M0 (hours 0-2)

- Team focus (report 1.2): "Confirm start/deadline and reuse guidance. Freeze task/adapter/verdict contracts, policy schema, shared and security budgets, two templates, projection and migration ownership. Choose a local model that runs on the actual machine."
- Exit condition (report 1.2): "Services connect; policy imports successfully; agent and guard requests can be made within recorded limits; the initial test command and fixtures exist."
- Report 1.2 sync points (spine): X-78, X-80, X-81, X-84, X-89 (initial).
- Sync points needed by the end (spine): X-03 to X-06. This side consumes X-03 and X-06 directly;
  X-05 reaches it through X-16. SH-11 lands X-07 to X-13 after the freeze; they are needed by M1.

This side's M0 work is mostly in the spine: SH-10 (the freeze; the web + API implementer coordinates the
contracts), SH-11 (the web + API implementer lands X-07 to X-13 after the freeze, finishing
in M1; SH-39 lands X-14 at M1, once decisions 4 and 7 are recorded) and SH-12 (each person brings up the starter). The two checks below make sure the agreed examples carry what this side
must enforce and show, so that any gap reaches the document owner in the same session.

### NestJS (report role: Implementer 2)

- [ ] **API-02 · Check the frozen examples against the checks the public API makes**
  - **Report 1.1 change:** Authority checks also cover the stored report and source trail reads.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 0.5-1.5 h, this roadmap's
    estimate)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `packages/contracts/src/index.ts`, `packages/contracts/schemas/error.schema.json`,
    `apps/api/src/main.ts`, `apps/api/src/config/environment.ts`,
    `apps/web/src/server/upstream-proxy.ts`, `apps/web/src/lib/fetch-json.ts`
  - Work: During the SH-10 session, walk each row of "Proposed browser and runtime operations" and
    confirm that NestJS can make its authority check from the verified operator context plus fields
    of an agreed example: the start-run request has no actor or organization field, the approval
    decision has no replacement payload, and the run, event and review examples carry the
    organization and run references an object check needs. Bring the facts for
    `command timeout budget` to the same session: the web proxy's fixed 10 s deadline, the browser
    helper's 15 s default, `GATEWAY_TIMEOUT_MS` (default 3000 ms, at most 20000 ms), the API's 30 s
    request timeout and the gateway's 30 s write timeout.
  - Done when: the authority of every public operation ("Verified actor and organization",
    "Actor may manage this run within this organization", "Authorized reviewer; exact action
    integrity and expiry", "Organization and object access checked on every read", "Authorized
    subscription; no unrestricted raw payload stream") can be checked from an agreed example, or the
    gap is recorded as an open item with the document owner in SH-10.
  - Tests: none (a review of the examples).
  - Report: "Illustrative passport and interface contracts" (Proposed browser and runtime
    operations); "Technical architecture and service ownership" (Interfaces and repository strategy)
  - Blocked by: `contract owners`; the operator context part also `decision 4 in docs/product/README.md` and `decision 7 in docs/product/README.md`

- [ ] **API-31 · Add the control catalog, active revision and feed records**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-3 h, this roadmap's estimate)
  - Depends on: SH-10 · Needs: X-78 · Provides: X-80 (part: entities)
  - Paths: `apps/api/src` (a new module; the architecture proposes PoliciesModule), `apps/api/src/database/typeorm-options.ts`
  - Work: `app` entities with `schema: "app"` for immutable catalog revisions (content, file digest, source file name, validation result, created by), the active-version pointer with its requested, validated and active states, and trusted feed revisions (issuer, revision, digest). Names are decided at the M0 freeze.
  - Done when: once SH-43 has migrated them, a revision and its pointer can be stored and Go can read the active one.
  - Tests: `pnpm --filter api run test`; `pnpm db:migration:generate <Name>` yields only `app` statements.
  - Report: "Data ownership and the transition from starter to product" (Proposed database ownership); "Central policy configuration and safe reload"
  - Blocked by: nothing

- [ ] **API-32 · Import and validate `policy.yaml` into an immutable catalog revision**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 3-5 h, this roadmap's estimate)
  - Depends on: API-31 · Needs: X-78 · Provides: X-81
  - Paths: `apps/api/src` (the module from API-31); `package.json` for an explicit import command (through integration)
  - Work: Parse a bounded schema, reject unknown executable plugins and arbitrary URLs, validate rule and model references and budget values, record the file digest and write a new immutable revision. The first valid import sets the active pointer; an invalid file never partially activates and leaves an error visible. The file is an import input, "not a second configuration authority". The authenticated reload is API-33.
  - Done when: the sample file imports successfully and an invalid copy is rejected with its reason (M0 exit: "policy imports successfully").
  - Tests: `pnpm --filter api run test` with valid and invalid files (unknown model, impossible limit, URL, unknown control).
  - Report: "Central policy configuration and safe reload"; "Illustrative passport and interface contracts" (Documented policy file and reload contract)
  - Blocked by: nothing

### Next.js (report role: Implementer 1)

- [x] **WEB-01 · Check the frozen examples against what the interface must show**
  - **Report 1.1 change:** The fields add the classification, source trail, export denial, alternative template, report and source versions in review, and internal evidence and report types in the task form.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 0.5-1.5 h, this roadmap's
    estimate)
  - Depends on: nothing · Needs: nothing · Provides: nothing
  - Paths: `packages/contracts/src/index.ts`, `packages/contracts/fixtures`
  - Work: During the SH-10 session, check that the agreed examples carry every field the report asks
    the interface to show. Task: the goal of the admitted task (Journey 1). Passport: permitted
    invoices, associated vendors, allowed recipient, approval requirement, expiry and remaining
    allowance. Run: approval waiting, rejection, expiry, execution success, an uncertain result,
    completion, failure and a stop with its reason. Events: the attempted operation apart from the
    completed effect, the matched rule, the correction count and the replay label. Usage: reported
    usage, reserved allowance and estimated cost with their uncertainty. Review: recipient, rendered
    content, referenced report and version, and the reason for review.
  - Done when: each field is in an agreed example or is recorded as an open item with the document
    owner in SH-10 (for example under `passport in the run view` or `review payload read`), so that
    the M0 exit's "agreed contract example for each command and event" covers the screens the report
    names.
  - Tests: none (a review of the examples).
  - Evidence (2026-10-04, branch `web/audit-01-02`): `docs/web-contract-audit.md`. A reading review,
    not a live run (the API does not compile on that day's merged `main`). Open items recorded there:
    task goal text, correction count, estimated cost, report types and internal evidence in the task
    form. The page-level mismatches (legacy `RunView` and `SanitizedEvent` shapes against the frozen
    `RunState` and `SafeEvent`) are listed there as blocking the demonstration; they are follow-up
    work, not part of this review.
  - Report: "Users operating model and proposed user journeys" (Journeys 1 to 3); "Live
    demonstration storyboard and proof checks"; "Atomic allowances hard limits and estimated cost"
  - Blocked by: `contract owners`

## M1 (hours 2-6)

- Team focus (report 1.2): "Build task form, authenticated facade, admission, local model gateway, one governed read, deterministic content handling, live semantic tool-result check and events."
- Exit condition (report 1.2): "A real task executes one allowed read; a hostile tool-result fixture is blocked before agent context; both model purposes appear in usage and latency records."
- Report 1.2 sync points (spine): X-79, X-85, X-86.
- Sync points needed by the end (spine): X-07 to X-32. This side provides X-17 (API-04 and API-05),
  X-26 (API-10), X-31 (API-07, API-11, API-13, API-14 and WEB-02) and, if `form options` chooses
  app records, X-25 (API-12).

The spine places authentication and the membership check on the start-run command here, because
the M1 focus names an "authenticated facade". This side also owns SH-39 (landing the operator context contract X-14 once decisions 4 and 7 are
recorded) and, by default, SH-15 (the `app` schema migration of X-17's entities) and SH-19 (the
seeded operator), and joins
SH-20 (the authentication and operator-context secrets), SH-22 (the vertical path across the
services) and SH-23 (smoke and leak checks). While the hold stands, the M1 exit cannot be reached.

### NestJS (report role: Implementer 2)

- [x] **API-03 · Deny every non-public route by default**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1-2.5 h)
  - Depends on: nothing · Needs: X-03 · Provides: nothing
  - Paths: `apps/api/src/auth/auth.module.ts`, `apps/api/src/auth/auth.types.ts`,
    `apps/api/src/auth/unimplemented-auth.provider.ts`, `apps/api/src/app.module.ts`,
    `apps/api/src/health/health.controller.ts`, `apps/api/src/diagnostics/diagnostics.controller.ts`,
    `apps/api/src/testing/create-test-app.ts`, `apps/api/src/common/all-exceptions.filter.spec.ts`
  - Work: Register a global guard that denies every route without a public marker, read over the
    handler and the class, and mark the health and diagnostics controllers public. The guard reads
    no credential and does not call `AUTH_PROVIDER`, because where a credential travels on a
    request (cookie, header or other) is decision 7's mechanism: until API-06, every protected
    route answers 501 through the shared envelope, as `UnimplementedAuthProvider` does, and never
    runs its handler. Keep the guard in the wiring the test helper builds, so every API test runs
    behind it, and give the test controller of `all-exceptions.filter.spec.ts` the public marker:
    its routes test the filter, not the guard.
  - Done when: the public API fails closed ("Keep service-side authorization, scope enforcement,
    pre-dispatch limits, and exact-action approval intact"): a protected route answers with the
    shared envelope and never runs its handler, the three starter routes stay public and
    `pnpm smoke` still passes.
  - Tests: specs with a test controller (the pattern of
    `apps/api/src/common/all-exceptions.filter.spec.ts`): a route without the marker answers with
    the shared envelope and its handler never runs; the health and diagnostics routes answer
    without a credential; `/api/docs-json` still answers; an unknown route still answers 404; a
    guard error passes through the global filter with a safe message; the existing specs built with
    `createTestApp` keep their assertions and pass: `pnpm --filter api run test`; `pnpm smoke`.
  - Completed (2026-10-04, lane 3c on api/3c): already in place on main 94e9a65 and verified here.
    `auth/default-deny.guard.ts` is the global `APP_GUARD` (`auth/auth.module.ts`, and the same wiring
    in `testing/create-test-app.ts`, so every spec built with it runs behind the guard); it reads
    `@Public()` over the handler and the class; health, diagnostics and the two auth routes are public.
    Since API-06 a protected route authenticates the session instead of answering 501; it never runs its
    handler without a valid session and organization membership. Tests in `default-deny.guard.spec.ts`:
    a missing or revoked session gets 401 in the shared envelope and the protected handler never runs
    (new); `/api/docs-json` still answers 200 with the guard installed while the product route of the
    same app stays 401 (new); a public route answers without a credential; an unknown route answers 404;
    a database or session-lookup failure answers 503 through the global filter without the internal
    error text. `pnpm --filter api run test` (see the commit).
  - Report: "Relative implementation milestones and critical dependencies" (Critical path and
    sensible reductions); "Architecture and chart reading guide" (Interpreting the full
    architecture: "Authentication placeholders in the starter must not be presented as implemented
    production identity controls")
  - Blocked by: nothing

- [ ] **API-04 · Add the task template, policy version and tool definition entities**
  - **Report 1.2 change:** The policy configuration now comes from `policy.yaml` imported into the catalog (API-31 to API-33); there is no policy editor screen.
  - **Report 1.1 change:** Module names: the architecture's proposal, TasksModule, PoliciesModule and ToolsModule.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: SH-10 · Needs: X-24 · Provides: X-17 (part: task, policy and tool entities)
  - Paths: `apps/api/src/database/typeorm-options.ts`,
    `apps/api/src/database/database-initializer.service.ts`, `apps/api/src` (new configuration
    module, named at M0 by its owner), `docs/team-workflow.md`
  - Work: First set `uuidExtension: "pgcrypto"` and `installExtensions: false` in the shared options
    factory, so that no startup creates an extension. Then add entity classes with `schema: "app"`
    for the records Go admission reads: the task template and its versions, the policy versions and
    the tool definitions, with the names and the organization reference agreed in SH-10, registered
    once in the factory. Tool definitions name only the four registered tools, and no record carries
    code or a URL. If SH-10 stores the registered report template or the trusted recipient
    directory in `app`, add their entity classes here too. SH-15 migrates them (the same person by default); hand the module's row in "Product modules" (`docs/architecture.md`) to integration.
  - Done when: once SH-15 has migrated them, Go admission can read the "authoritative task and
    policy versions" from these records, and starting the API on a fresh database creates no table
    and no extension (the starter settings "need to be preserved when the first product migrations
    are added").
  - Tests: a database-backed test through the command from SH-21 (X-24): initializing the API's data
    source on a fresh database leaves `pg_extension` and the table list unchanged;
    `pnpm --filter api run test`; `pnpm db:migration:generate <Name>` yields only `app` statements,
    reviewed before SH-15 applies it.
  - Report: "Data ownership and the transition from starter to product" (Proposed database
    ownership); "Architecture and chart reading guide" (Interpreting the full architecture: "a fixed
    reviewed policy and a small task form can stand in for a general policy editor"); "Functional
    requirements MVP boundary and deferred scope" (Product decisions that keep the MVP coherent)
  - Blocked by: nothing

- [ ] **API-05 · Add the user and membership entities**
  - **Report 1.1 change:** Adds `organizations` (architecture table proposal).
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1-2.5 h)
  - Depends on: SH-01, SH-10 · Needs: X-24 · Provides: X-17 (part: user and membership entities)
  - Paths: `apps/api/src/database/typeorm-options.ts`, `apps/api/src/auth/auth.types.ts`,
    `apps/api/src` (new module, named at M0 by its owner)
  - Work: Add the user and membership records with `schema: "app"` in the shape decision 7 records:
    each membership ties a user to one organization, and the roles of the report's table "Proposed
    user responsibilities" are stored where decision 7 puts them. The recorded proposal (roles held
    on the membership, more than one per membership; password hashes and sessions in tables
    separate from memberships, so that the Go role can read memberships without credentials) is
    proposed, not decided. SH-15 migrates them (the same person by default).
  - Done when: after SH-15's migration these records let NestJS "authenticate the user and verify
    organization membership" (Journey 1) without any fabricated identity, and startup still creates
    no table and no extension.
  - Tests: API-04's database-backed startup check repeated with these entities (X-24);
    `pnpm --filter api run test`.
  - Report: "Users operating model and proposed user journeys" (Journey 1 create and delegate a
    task; Proposed user responsibilities); "Data ownership and the transition from starter to
    product"
  - Blocked by: `decision 7 in docs/product/README.md`

- [x] **API-06 · Authenticate the operator through a real credential check**
  - **Done (2026-10-03, api-impl-n):** Completed the explicit development operator seed (SH-19) through `pnpm db:seed`, using the fixture organization and `.env` password, preserving credentials and operator/reviewer membership on reruns and refusing drift. Fixed the cookie provider to look up the stored SHA-256 session hash. PostgreSQL tests exercise idempotence, real sign-in, HttpOnly cookie access, wrong/missing credentials, logout and role drift. `pnpm --filter api run lint`, `typecheck`, `test` and `build`: exit 0 (`test`: "109 passed (109)"); `pnpm test:db api`: "18 passed, 0 failed, 0 skipped"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; `pnpm smoke`: "28 passed, 0 failed, 5 skipped" (host mode does not capture service logs). An isolated local PostgreSQL cluster was used; Docker/container checks were not run (Docker unavailable). No startup seed or migration.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: M (estimate 4-7 h)
  - Depends on: API-03, API-05, SH-01 · Needs: X-18, X-22, X-23, X-24 · Provides: nothing
  - Paths: `apps/api/src/auth/auth.module.ts`, `apps/api/src/auth/auth.types.ts`,
    `apps/api/src/auth/unimplemented-auth.provider.ts`,
    `apps/api/src/auth/unimplemented-auth.provider.spec.ts`, `apps/api/src/config/environment.ts`,
    `apps/api/src/config/app-config.service.ts`, `apps/api/src/openapi.ts`
  - Work: Replace `UnimplementedAuthProvider` with the mechanism decision 7 records, including
    sign-in and sign-out if it has them, so that the seeded demonstration operator (X-22, seeded in SH-19) authenticates through a real credential check. Extend the API-03 guard
    to read the credential where decision 7 puts it and to resolve the caller only through
    `AUTH_PROVIDER`. If the mechanism uses cookies, the constraints above apply (cookie parsing,
    `@HttpCode`, `addCookieAuth` in the Swagger document). A guard that needs the database answers
    503 while it is down. API-06 and SH-19 finish together: SH-19 provides X-22, and its
    "Done when" needs this task's credential check. New variables go through
    `environmentSchema.extend` and `AppConfigService`; their wiring is SH-20.
  - Done when: the seeded operator signs in through the real credential check, a wrong or missing
    credential is rejected, and nothing grants an identity without the check ("One seeded operator
    can replace enterprise onboarding only within a clearly labeled development demonstration").
  - Tests: specs: the correct credential is accepted, a wrong, expired or revoked one is rejected as
    decision 7 defines them, a protected route answers 503 while the database is down, every
    rejection message is a safe text; database-backed tests against the seeded records through X-24;
    the spec that asserted the 501 placeholder is replaced by these: `pnpm --filter api run test`.
  - Report: "Technical architecture and service ownership"; "Architecture and chart reading guide"
    (Interpreting the full architecture); "Users operating model and proposed user journeys"
    (Journey 1 create and delegate a task)
  - Blocked by: `decision 7 in docs/product/README.md`

- [ ] **API-07 · Accept authenticated browser calls on the path chosen in decision 3**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: API-06, SH-02 · Needs: nothing · Provides: X-31 (part)
  - Paths: `apps/api/src/app.setup.ts`, `apps/api/src/config/environment.ts`,
    `apps/api/src/common/request-id.middleware.ts`
  - Work: If decision 3 chooses a same-origin forwarder (proposed, not decided), the API reads the
    operator's credential from the forwarded request and trusts no `x-forwarded-*` header it did not
    set. If it chooses direct browser calls, the API allows the methods the product needs with
    credentials, for the explicit origins only and never `*`; today CORS allows GET, HEAD and OPTIONS
    without credentials.
  - Done when: an authenticated browser request reaches the product routes on the chosen path, an
    unauthenticated or other-origin call is refused, and "The browser must never receive provider
    credentials, tool credentials, internal service secrets, or an unrestricted runtime command
    interface" holds.
  - Tests: specs: forged `x-forwarded-for` and `x-forwarded-host` headers change nothing (forwarder
    option), or a credentialed preflight from an allowed origin passes while another origin gets no
    CORS headers (direct option): `pnpm --filter api run test`; `pnpm smoke`.
  - Report: "Architecture and chart reading guide"; "Technical architecture and service ownership"
  - Blocked by: `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **API-08 · Resolve the operator context and check organization membership**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1.5-3 h)
  - Depends on: API-06 · Needs: X-18, X-22, X-24 · Provides: nothing
  - Paths: `apps/api/src/auth/auth.types.ts`, `apps/api/src` (the module from API-05)
  - Work: From the authenticated principal, load the actor and the organization from the trusted
    membership records and the roles from the trusted records decision 7 defines, and refuse every
    command and read when the membership is missing. The organization never comes from a
    browser-supplied or record identifier, model output or a tool result. This resolved context is
    the only operator context API-10 sends to Go and the only basis for the object checks of API-13,
    API-14 and API-16.
  - Done when: NestJS "would authenticate the user and verify organization membership" before any
    command reaches Go (Journey 1), and an organization named in a request's body, query or path
    cannot change the resolved organization.
  - Tests: specs with a stub Go: a principal without a membership is refused and the stub receives
    nothing; an organization identifier in the body, query or path is ignored; database-backed tests
    against the seeded memberships through X-24: `pnpm --filter api run test`.
  - Report: "Users operating model and proposed user journeys" ("The organization boundary would be
    enforced in the application and runtime, not inferred from a record identifier supplied by the
    browser or agent"); "Data ownership and the transition from starter to product"
  - Blocked by: `decision 7 in docs/product/README.md`

- [x] **API-09 · Add a fail-closed command client toward Go**
  - Done (2026-10-03): shared authenticated GET/POST transport rejects missing or invalid X-14 context before dispatch, signs only verified context, preserves upstream HTTP status, validates responses and refuses redirects. Stub tests cover 404, 401, 500, 503, timeout, refused connection, malformed JSON and unknown fields; secrets and upstream messages are withheld. Checks: API lint, typecheck and build exited 0; API unit tests: "134 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". Database schema and records are unchanged by this task.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1.5-3 h)
  - Depends on: SH-11 · Needs: X-13 · Provides: nothing
  - Paths: `apps/api/src/gateway-client/gateway-client.service.ts`,
    `apps/api/src/gateway-client/gateway-client.module.ts`,
    `apps/api/src/gateway-client/gateway-client.service.spec.ts`,
    `apps/api/src/config/app-config.service.ts`
  - Work: The gateway client sends GET probes only. Add commands: POST with a JSON body, the deadline
    agreed under `command timeout budget`, the propagated request id, `redirect: "manual"` and the
    starter's service identity until decision 4 extends or replaces it, and validate every response
    against its frozen contract. A timeout, a transport error, a body that is not JSON, an unknown
    field or an unexpected status becomes a failure the facade reports as unconfirmed, never a
    success; the token and upstream bodies are never logged or returned.
  - Done when: the facade has a command path toward Go on which "A transport or configuration error
    is not an allow decision" holds for every failure kind.
  - Tests: stub-server specs (the pattern of `gateway-client.service.spec.ts`): a valid 2xx body, a
    4xx envelope with a reason code, a 5xx, a timeout, a refused connection, a body that is not JSON
    and an unknown field each map to their own outcome; a redirect is not followed; the token never
    appears in a result or a log line: `pnpm --filter api run test`.
  - Report: "Illustrative passport and interface contracts" (Decision and error semantics); "Risk
    register and scope controls" (NestJS/Go contract drift)
  - Blocked by: `command timeout budget`

- [x] **API-10 · Carry the verified operator context on every runtime command**
  - Done (2026-10-04): every runtime read/command signs validated X-14 context sourced only from the real session/current membership. Admission now propagates request.requestId instead of the nonexistent request.id, validates the shared StartRunResponse strictly and uses faithful gateway error mapping. Checks: API lint, typecheck and build exited 0; API unit tests: "333 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; host smoke: "28 passed, 0 failed, 5 skipped". Real Go returned 401 for missing and invalid operator JWTs; real API admission returned 201 and the gateway log carried verified-admission-context. Tests reject browser identity fields and missing/malformed context before dispatch; JWT tests verify issuer, audience, HS256 and ctx. No fallback identity.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1.5-3 h)
  - Depends on: API-08, API-09, SH-03 · Needs: X-14, X-23, X-27 · Provides: X-26
  - Paths: `apps/api/src/gateway-client/gateway-client.service.ts`,
    `apps/api/src/config/environment.ts`, `apps/api/src/config/app-config.service.ts`
  - Work: Attach the operator context from API-08 to every command, in the form decision 4 records:
    an extension of the starter's service token or another service-identity mechanism. The context
    comes only from the verified session and trusted records. "A signed user identifier alone is
    insufficient if the user lacks permission to review the particular action", so Go still
    authorizes each command itself (X-27). Key material follows AGENTS.md working rule 8 and is
    wired through SH-20.
  - Done when: Implementer 2's first integrated deliverable holds: "An authenticated command reaches
    Go with verifiable actor and organization context", and Go rejects a command whose context is
    missing or altered (X-27).
  - Tests: stub-server specs: every command carries the context and the request id; nothing in the
    request body reaches the context; no secret appears in a log line or a result; against the real
    gateway, a command without valid context is rejected: `pnpm --filter api run test`; `pnpm smoke`
    with the checks SH-23 adds at M1.
  - Report: "Technical architecture and service ownership" (Interfaces and repository strategy);
    "Threat model limits and unresolved design choices" ("The service token in the starter requires
    replacement or extension for authenticated operator context"); "Delivery scope and six person
    ownership" (Proposed team ownership)
  - Blocked by: `decision 4 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **API-11 · Start a run through `POST /api/runs`**
  - **Report 1.2 change:** The facade also forwards the catalog validation request of API-33.
  - **Report 1.1 change:** Module proposal RuntimeModule.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: API-10 · Needs: X-03, X-07, X-13, X-15, X-28 · Provides: X-31 (part)
  - Paths: `apps/api/src` (new run facade module, named at M0 by its owner),
    `apps/api/src/app.module.ts`, `apps/api/src/openapi.ts`
  - Work: Validate the body strictly against the start-run contract (X-07), take actor and
    organization only from the verified context, forward to `POST /internal/runs` (X-28) and return
    its result. An admission rejection reaches the browser in the shared envelope with its stable
    reason code, a safe message and the scope or limit that must change; NestJS never narrows the
    request itself. Document the route in Swagger with DTO classes that implement the contract types,
    and replace the starter's title and description in `apps/api/src/openapi.ts`.
  - Done when: an authenticated operator's request leads Go to store the passport, run and job, and
    an over-scope request is rejected with its explanation and no passport ("Trusted admission":
    "Reject an unauthorized task and prevent passport creation").
  - Tests: specs with a stub Go: a body with a field the contract does not define (such as an actor
    or organization) or a wrong enum value is rejected before any upstream call; a Go rejection keeps
    its reason code and explanation; a timeout is reported as unconfirmed, not as started; by hand
    against the real gateway, a passport exists after an accepted request and none after a rejected
    one: `pnpm --filter api run test`; `pnpm smoke`.
  - Report: "Illustrative passport and interface contracts" (Proposed browser and runtime
    operations); "Trusted authority and passport invariants" ("Silently reducing the scope would
    make the accepted task differ from the user's request"); "Functional requirements MVP boundary
    and deferred scope" (Trusted admission)
  - Blocked by: `decision 7 in docs/product/README.md`; `command timeout budget`

- [ ] **API-12 · Serve the task form's options**
  - **Report 1.2 change:** Model choices come only from the active catalog allowlist.
  - **Report 1.1 change:** The options add which internal evidence may be consulted and which report types may be created.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1.5-3.5 h)
  - Depends on: API-04, API-08 · Needs: X-06, X-21, X-24; X-25 unless `form options` chooses app
    records · Provides: X-25 if `form options` chooses app records
  - Paths: `apps/api/src` (the configuration module from API-04)
  - Work: Serve, read-only and for the verified organization only, what the task form offers: the
    template, the vendor, the invoice set, the allowed destination, which action needs review and
    the available limits. The source is what `form options` decides: SH-18's records through a read
    grant, a Go endpoint, or app records this side serves; the route is named at M0 by its owner,
    because the report's operations table has no such read. Values come from the authoritative
    records and the policy fixture (X-06), never from constants in code.
  - Done when: the form can offer the choices "in business language: which invoices to reconcile,
    which vendor is involved, where a report may go, which action needs review, and how long the run
    may continue", for the operator's organization only.
  - Tests: specs: another organization's template and records never appear; an unavailable source
    answers an error, never an empty or default list; database-backed tests against the seed through
    X-24: `pnpm --filter api run test`.
  - Report: "Project definition purpose and intended outcome" (What a passport would contain);
    "Architecture and chart reading guide" (Interpreting the full architecture)
  - Blocked by: `form options`; `decision 7 in docs/product/README.md`

- [x] **API-13 · Serve the run and usage view through `GET /api/runs/{id}`**
  - Done (2026-10-04): per the lead's clarification, two separate routes relay X-11 RunState (including resultReference) and RunUsage unchanged after verified membership and Go object authorization. Shared schemas and run-reference matching fail closed; no combined contract or default usage. Checks: API lint, typecheck and build exited 0; API unit tests: "180 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". HTTP tests cover authentication, malformed ids, exact forwarding, 401/403/404/503, transport failures, unknown fields and mismatched run references. No database writes.
  - **Report 1.1 change:** The architecture places runtime read views in ActivityModule, as a proposal; reads stay per `read path`.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: API-08; API-10 if the read path uses Go endpoints · Needs: X-08, X-11, X-24, X-29 ·
    Provides: X-31 (part)
  - Paths: `apps/api/src` (the run facade module from API-11),
    `apps/api/src/database/database.module.ts`,
    `apps/api/src/gateway-client/gateway-client.service.ts`
  - Work: Return the authorized run and usage view (status, usage and terminal reason) and the
    passport if `passport in the run view` says so. Read it the way the read path decides: from
    sanitized runtime views with schema-qualified SQL, or from a private Go endpoint with the
    operator context. Either way select by the verified organization, refuse an upstream answer
    whose organization differs from the operator's, and relay only the contract's fields.
  - Done when: "Organization and object access checked on every read" holds for the run view
    ("Authorized visibility": "Expose organization-scoped run state and sanitized ordered events"),
    and the interface can read the state Go persisted.
  - Tests: specs: another organization's run, an unknown run and a malformed identifier are refused;
    an upstream body with a field outside the contract is not relayed; an unavailable read source
    answers 503, never a default state; database-backed tests through X-24 if the read path uses
    views: `pnpm --filter api run test`.
  - Report: "Illustrative passport and interface contracts" (Proposed browser and runtime
    operations: "Read authorized run and usage view"); "Data ownership and the transition from
    starter to product" ("NestJS should receive only the runtime views necessary for the
    interface"); "Functional requirements MVP boundary and deferred scope" (Authorized visibility)
  - Blocked by: `read path`; `passport in the run view`; `decision 7 in docs/product/README.md`

- [x] **API-14 · Serve sanitized events by cursor through `GET /api/runs/{id}/events`**
  - Done (2026-10-04): authenticated Go read facade forwards only validated run and cursor references with verified session context. Shared X-12/X-30 schemas reject unknown fields; organization, run, strict event ordering and next-cursor consistency are checked before returning the unchanged page. Go 404/401/403/503 are preserved; malformed or unavailable reads fail closed with 503. Checks: API lint, typecheck and build exited 0; API unit tests: "156 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". Tests cover missing session, invalid references/pagination, supplied identity, cross-organization/run pages, duplicate/reversed events, unknown fields, empty pages and invalid cursor. No database writes or schema changes.
  - **Report 1.1 change:** The architecture places runtime read views in ActivityModule, as a proposal; reads stay per `read path`.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: API-08; API-10 if the read path uses Go endpoints · Needs: X-12, X-24, X-30 ·
    Provides: X-31 (part)
  - Paths: `apps/api/src` (new activity feed module, named at M0 by its owner),
    `apps/api/src/database/database.module.ts`,
    `apps/api/src/gateway-client/gateway-client.service.ts`
  - Work: Serve the run's sanitized events in order after a cursor, for authenticated polling;
    server-sent events wait for the optional API-25. Read them the way the read path decides, select
    by the verified organization and run, and relay only the safe event contract's fields (X-12):
    masked metadata, never raw arguments, review content or tool output.
  - Done when: the interface can poll the run's "sanitized ordered events" ("Authorized
    visibility"), with "no unrestricted raw payload stream", and each event arrives once and in order
    across polls.
  - Tests: specs: a cursor returns only newer events, in order, without gaps or repeats; another
    organization's run is refused; an upstream event with a field outside the contract is refused,
    not relayed; database-backed tests through X-24 if the read path uses views:
    `pnpm --filter api run test`.
  - Report: "Illustrative passport and interface contracts" (Proposed browser and runtime
    operations: "Read sanitized events with cursor or SSE"); "Durable state idempotency audit and
    uncertain outcomes" (Evidence without creating a second disclosure channel); "Relative
    implementation milestones and critical dependencies" (Critical path and sensible reductions)
  - Blocked by: `read path`; `decision 7 in docs/product/README.md`

- [ ] **API-15 · Carry the worker readiness change through the diagnostics route and page**
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: B · Size: S (estimate 1-2 h, this roadmap's estimate)
  - Depends on: nothing · Needs: X-32 · Provides: nothing
  - Paths: `apps/api/src/gateway-client/gateway-client.service.ts`,
    `apps/api/src/diagnostics/diagnostics.controller.ts`,
    `apps/api/src/diagnostics/diagnostics.dto.ts`, `apps/web/src/lib/service-checks.ts`,
    `apps/web/src/app/diagnostics/diagnostics-panel.tsx`
  - Work: When the gateway readiness check starts to cover the worker (X-32, a shared contract change
    reviewed in SH-14), update the API's readiness interpretation, the diagnostics DTO classes and
    the web's check interpretation in the same change as the contract, so that the diagnostics page
    reports the worker from the real response. Health and diagnostics stay public.
  - Done when: the diagnostics page shows the worker's readiness only from a real gateway response
    and never shows it healthy when the gateway reports it not ready, and `pnpm smoke` still passes.
  - Tests: `gateway-client.service.spec.ts` and `diagnostics.controller.spec.ts` with a worker that
    is not ready; `apps/web/src/lib/service-checks.test.ts`: such a report never yields "healthy":
    `pnpm --filter api run test`; `pnpm --filter web run test`; `pnpm smoke`.
  - Report: "Durable state idempotency audit and uncertain outcomes" (durable jobs claimed with a
    lease); the readiness rule is the repository rule of decision 5 in `docs/product/README.md`
  - Blocked by: `worker readiness`

### Next.js (report role: Implementer 1)

- [ ] **WEB-02 · Build the browser to API path chosen in decision 3**
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: A · Size: M (estimate 3-7 h)
  - Depends on: SH-02 · Needs: X-03 · Provides: X-31 (part)
  - Paths: `apps/web/src/server/upstream-proxy.ts`, `apps/web/src/server/upstream-proxy.test.ts`,
    `apps/web/src/app/api` (new route handlers, named at M0 by their owner), `apps/web/README.md`
  - Work: Today three exact-path GET handlers buffer JSON and forward no cookie or authorization
    header, and that stays so until decision 3 is recorded. If decision 3 chooses a same-origin
    forwarder (proposed, not decided), build it to the forwarder constraints above, with request
    bodies for the commands and streamed responses. If it chooses direct browser calls, the CORS
    change is API-07 and the web app reaches the API origin as decision 3 records. Either way the
    three existing handlers keep working, and the approval and cancel operations (M3) use the same
    path.
  - Done when: the browser reaches `POST /api/runs`, `GET /api/runs/{id}` and
    `GET /api/runs/{id}/events` behind authentication on the chosen path (X-31, with API-07, API-11,
    API-13 and API-14), and "The browser must never receive provider credentials, tool credentials,
    internal service secrets, or an unrestricted runtime command interface" holds.
  - Tests: route-handler specs with a stub upstream (the pattern of `upstream-proxy.test.ts`):
    spoofed `x-forwarded-*`, `host` and hop-by-hop headers never reach the upstream; encoded
    traversal and paths outside the allowlist are refused before any upstream call; two `Set-Cookie`
    headers both arrive; the configuration is read per request; a stream is not buffered and is not
    ended by the 10 s deadline of a buffered call; the events read's cursor reaches the upstream
    while the three existing handlers still drop the query; a 204 or empty 2xx body that a frozen
    contract allows passes through: `pnpm --filter web run test`; `pnpm smoke` (its leak check finds
    no secret in a page or asset).
  - Report: "Architecture and chart reading guide"; "Technical architecture and service ownership"
  - Blocked by: `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **WEB-03 · Add a typed client for the product operations**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 1.5-4 h)
  - Depends on: SH-11 · Needs: X-07, X-11, X-12, X-13 · Provides: nothing
  - Paths: `apps/web/src/lib/fetch-json.ts`, `apps/web/src/lib/fetch-json.test.ts`, `apps/web/src/lib`
    (new client module, named at M0 by its owner)
  - Work: `fetchJson` sends GET to relative URLs only. Add the commands the operations need, guards
    that accept a response only when it matches its frozen contract, and one mapping from the adopted
    reason codes (X-13) to safe operator messages, identical to the runtime's vocabulary. Every
    outcome stays a value, never a throw, and a timed-out command is reported as unconfirmed. A 204
    or empty body that a frozen contract allows is a success, not `invalid_json` (today `fetchJson`
    returns `invalid_json` for any empty 2xx body). The transport follows decision 3; the guards and
    the mapping do not wait for it.
  - Done when: every screen reads and sends through this client, and a response that does not match
    its contract is never shown as a successful state ("These names are proposed contract
    vocabulary. The final implementation should keep the vocabulary stable across the UI, runtime,
    tests, and evidence").
  - Tests: specs against the contract fixtures: each fixture passes its guard; a fixture with an
    unknown status, a missing field or an unknown reason code is refused or gets the generic message;
    a timeout yields "unconfirmed"; a command never repeats itself after a failure:
    `pnpm --filter web run test`.
  - Report: "Illustrative passport and interface contracts" (Decision and error semantics); "Risk
    register and scope controls" (NestJS/Go contract drift)
  - Blocked by: `decision 3 in docs/product/README.md` (the transport only)

- [ ] **WEB-04 · Add sign-in, the operator display and the development demonstration label**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-5 h)
  - Depends on: WEB-02, API-06 · Needs: X-22 · Provides: nothing
  - Paths: `apps/web/src/components/app-navigation.tsx`, `packages/ui/src/components/app-shell.tsx`,
    `apps/web/src/app/layout.tsx`, `apps/web/src/app` (new sign-in page, named at M0 by its owner)
  - Work: Add the sign-in and sign-out flow decision 7 defines, show the signed-in operator and
    organization (the `sidebarFooter` slot of `AppShell` is unused today), and label the seeded
    operator as a development demonstration on every product page. A product page sends a visitor
    without a session to sign in; health and diagnostics stay public unless the team decides
    otherwise, and whether the home and components pages also need a sign-in is settled in this
    task, after decision 7 and `smoke under login`, not before.
  - Done when: the seeded operator signs in with the real credential check and every product page
    carries the development demonstration label ("One seeded operator can replace enterprise
    onboarding only within a clearly labeled development demonstration").
  - Tests: specs for the pure parts: the label is derived from the session on every product page; an
    expired session leads to sign-in, never to an empty product page; a browser check against the
    real stack, quoted: `pnpm --filter web run test`; `pnpm smoke`.
  - Report: "Architecture and chart reading guide" (Interpreting the full architecture); "Users
    operating model and proposed user journeys"
  - Blocked by: `decision 7 in docs/product/README.md`; `smoke under login`

- [ ] **WEB-05 · Build the task form**
  - **Report 1.2 change:** Model selection is constrained by the catalog allowlist.
  - **Report 1.1 change:** The form adds internal evidence and report types; route proposal `/tasks/new`.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2.5-5 h)
  - Depends on: WEB-03, WEB-04, API-11, API-12 · Needs: X-06, X-25 · Provides: nothing
  - Paths: `apps/web/src/app` (new task setup page, named at M0 by its owner),
    `apps/web/src/components/app-navigation.tsx`, `packages/ui/src/components/select.tsx`,
    `packages/ui/src/components/checkbox.tsx`
  - Work: Let the operator choose, in business language, the fixed reconciliation template, the
    vendor, the invoices, the requested destination and the available limits from the options
    NestJS serves, expose which action needs review as X-06 and X-07 record it at SH-10, then
    submit the start-run request. The form sends no actor, organization or grant, offers only what
    the server returned, and opens the run page with the run reference Go issued.
  - Done when: demo beat 1 can be performed ("Operator selects the fixed reconciliation task and
    designated synthetic invoices"), and the run that starts is the one Go admitted (Journey 1).
  - Tests: specs: the request built from the form holds only start-run contract fields and never a
    value the server did not offer; a failed or empty options read shows an error, never a default
    selection; a browser check against the real stack, quoted: `pnpm --filter web run test`.
  - Report: "Users operating model and proposed user journeys" (Journey 1 create and delegate a
    task); "Project definition purpose and intended outcome" (What a passport would contain); "Live
    demonstration storyboard and proof checks" (Proposed demo sequence, beat 1)
  - Blocked by: `form options`; `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **WEB-06 · Show the run's persisted events on the run page**
  - **Report 1.1 change:** Route proposal `/runs/:id`.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: WEB-03, WEB-05, API-13, API-14 · Needs: X-03 · Provides: nothing
  - Paths: `apps/web/src/app` (new run page, named at M0 by its owner),
    `apps/web/src/app/diagnostics/diagnostics-panel.tsx` (its abort-on-unmount fetch pattern),
    `packages/ui/src/components/loading-state.tsx`, `packages/ui/src/components/error-state.tsx`
  - Work: Load the run view and poll the events read with its cursor, cancel requests when the page
    unmounts, and render only what the server returned: no state comes from a missing response, and
    the browser keeps no state of its own as the truth. This is the first slice of the run page;
    WEB-08 to WEB-10 complete it.
  - Done when: Implementer 1's first integrated deliverable holds: "Start a run and display a real
    persisted event through the NestJS API", and the M1 exit's "the interface displays the actual
    persisted result" is observed in SH-22.
  - Tests: specs: the cursor advances only past events received; a failed poll keeps the last real
    events and shows the failure, never an invented state; a browser check against the real stack,
    quoted: `pnpm --filter web run test`.
  - Report: "Delivery scope and six person ownership" (Proposed team ownership); "Relative
    implementation milestones and critical dependencies" (Proposed 24-hour implementation sequence)
  - Blocked by: `read path`; `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [x] **WEB-07 · Replace the starter texts that the product makes untrue**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: B · Size: S (estimate 0.5-1.5 h)
  - Depends on: API-04 · Needs: nothing · Provides: nothing
  - Paths: `apps/web/src/app/layout.tsx`, `apps/web/src/app/page.tsx`,
    `apps/web/src/components/app-navigation.tsx`, `apps/web/README.md`
  - Work: The layout says "A generic full-stack project starter." and titles the app "Starter", as
    the navigation brand does; the home page is headed "Project starter" and says "It contains
    infrastructure and reusable building blocks only, ready for a product to be built on top." and
    "No tables yet."; all of these describe the starter, not the product, once SH-15 creates the
    `app` schema. Replace them with plain product text, add each product page to the navigation in
    the change that adds the page, and keep the components and diagnostics pages with their
    sample-data wording. Hand the matching texts of `README.md` and `docs/architecture.md`
    ("The starter has only `GET` routes", the CORS sentence, the empty "Product modules" table) to
    integration.
  - Done when: no page states something the build contradicts (AGENTS.md guardrail 9; "Verify that
    spending uncertainty, simulated effects and unimplemented features remain visible in the report
    and demonstration").
  - Tests: `pnpm smoke` (the three pages still answer 200); a browser check of the shell, quoted.
  - Report: "Threat model limits and unresolved design choices" (Verification priorities);
    "Research documentation and submission workflow" (From requirements to verified presentation)
  - Blocked by: nothing
  - Progress (2026-10-04): the home page (`apps/web/src/app/page.tsx`) is rewritten: it no longer mounts `RunTimeline`, whose rows are hardcoded sample events, and says in plain text what admission, the passport and a run are, with the `SyntheticDataLabel` and `SimulatedOutboxLabel` for what is simulated; it shows no run data. `/components` has a "Truthful labels" section with every label. `apps/web/src/app/page.test.ts` checks the page carries no sample row, no starter wording and the label texts. Checks: `pnpm --filter web run lint`, `typecheck` and `test` exit 0 (62 passed, 5 of them new), `pnpm format:check` exit 0. `layout.tsx` now titles the app "Task Passport" with a truthful description and the pages table of `apps/web/README.md` describes `/`, `/login` and `/tasks/new` (the lead granted both files on 2026-10-04). Not done, so not ticked: `run-timeline.tsx` keeps its mock data for whoever mounts it next (WEB-06 replaces it). Evidence (2026-10-04, my own stack on ports 3150/3151/8150, signed in as the seeded demo operator, an isolated headless Chromium; screenshots not committed): `/`, `/components` and `/diagnostics` answer 200 with the titles "Task Passport", "Components | Task Passport" and "Diagnostics | Task Passport"; none contains "Sample Data" or "Starter"; `/` and `/components` show the "Simulated outbox: a database record, no email is sent" and "Synthetic records" labels; the navigation shows "Development Demonstration". `pnpm smoke` exited 1 with 22 passed, 8 failed, 6 skipped: `web: GET /` and `web: GET /components` expect HTTP 200 and got 307, because the middleware sends an unauthenticated request to `/login`, and the six web leak checks then report "not every page could be loaded" (`scripts/smoke.mjs` is the lead's to adjust). Seen in the browser and not mine: `GET /api/auth/me` answers 404 (Noyan's route is pending) and `GET /api/runs/options` answers 400 "Invalid record identifier" (the API has no `options` route, so the request matches `:id`), so the task form shows "An unknown error occurred." with a Retry button.

## M2 (hours 6-10)

- Team focus (report 1.2): "Complete the four adapters, scoped reads, trusted source manifests, both fixed report templates, inherited classifications, destination checks and bounded feedback. Implement current-catalog lookup and versioned signature-rule activation; add otherwise permitted action semantic checks."
- Exit condition (report 1.1): "An internal report is retained for authorized internal viewing; its
  vendor export is denied; a separate approved-field vendor report can be created." This side adds
  API-28 to API-30, WEB-27 and WEB-28 and needs X-68, X-69 and X-71; X-37 and X-38 are needed by M4
  now.
- Report 1.2 sync points (spine): X-82, X-87, X-88.
- Sync points needed by the end (spine): X-33 to X-38, X-63 and X-64 (X-65 only if the replay is
  triggered through NestJS). This side consumes X-34, X-35, X-36 and X-64, and X-65 in that case,
  and provides none of them; the X-65 tasks (the facade operation and the control that triggers it) are added once that outcome is chosen (spine, X-65).

The report's sequence names no web or API work in this window. Following the spine, it holds the
ownership-table work the sequence leaves unnamed: for the interface the passport summary, the run
timeline and the terminal states (WEB-08 to WEB-10), with the admission rejection, the report view
and the labels; for the application API the membership checks on every read and command (API-16), the
API's own database role and the stored report read. Task configuration landed in M1 (API-04 and
API-12), because admission (X-18, X-21) and the task form (X-25) need it there. SH-36 observes the
M2 exit across the services, and the interface shows its denied proposal (WEB-09) and its report
(WEB-12).

### NestJS (report role: Implementer 2)

- [x] **API-16 · Check role and object access on every read and command**
  - Done (2026-10-04): per the lead's facade split, NestJS verifies the session/current app membership and reviewer roles before Go; Go remains the authoritative organization/object check and its 404 is preserved. Database-backed tests seed two organizations in rollback-only app transactions and authenticate via real scrypt credentials and stored hashed sessions. They cover state, usage, events, report, review, approval, cancellation and judge forwarding, foreign-object 404, current-role revocation and deleted membership refusal before Go. The Go side in these database tests is explicitly a labelled contract fixture, not runtime execution. Checks: API lint, typecheck and build exited 0; API unit tests: "319 passed"; `pnpm test:db api`: "59 passed, 0 failed, 0 skipped"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". No test writes runtime or demo state.
  - **Report 1.2 change:** Roles add the configuration operator (policy reload) and the security reader (audit export); the policy administrator edits "the validated central policy file".
  - **Report 1.1 change:** Roles: "approval cannot widen the passport or override a source restriction"; the policy administrator role is to "Maintain versioned policies, trusted source classifications, and the two permitted template manifests".
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: API-11, API-13, API-14 · Needs: X-24, X-34 · Provides: nothing
  - Paths: `apps/api/src/auth/auth.types.ts`, `apps/api/src` (the modules from API-05, API-11 and
    API-14)
  - Work: Apply one check to every product route: the record belongs to the operator's organization,
    and the operator holds the role the report's table "Proposed user responsibilities" gives for the
    operation. Operations user: create tasks, inspect permitted run details, request cancellation;
    authorized reviewer: approve or reject a stored action; policy administrator: reduce or revoke
    authority; audit reader: read sanitized decisions and evidence, with "separate authority" for
    full sensitive payloads. Roles stay distinct even when one demonstrator holds several. A rule the
    table does not settle (for example whether an operations user may read or cancel another
    operator's run) is brought to the document owner and recorded in `docs/product/README.md` before
    it is enforced. Go repeats its own checks; this one never replaces them.
  - Done when: with the seeded second organization (X-34), "A user outside the organization cannot
    read another organization’s run or approval" ("Authorized visibility") holds on the public path,
    and an operation without its role is refused.
  - Tests: database-backed specs through X-24 with both seeded organizations: the second
    organization's operator is refused on every route of this file; an operator without the needed
    role is refused; after a refusal the stub Go has received nothing: `pnpm --filter api run test`.
  - Report: "Users operating model and proposed user journeys" (Proposed user responsibilities);
    "Functional requirements MVP boundary and deferred scope" (Authorized visibility); "Risk
    register and scope controls" (Data leakage through secondary views)
  - Blocked by: `decision 7 in docs/product/README.md`

- [ ] **API-17 · Connect the API with its own database role**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 1-2.5 h)
  - Depends on: API-04 · Needs: X-24, X-35 · Provides: nothing
  - Paths: `apps/api/src/config/environment.ts`, `apps/api/src/config/environment.spec.ts`,
    `apps/api/src/config/app-config.service.ts`, `apps/api/src/database/typeorm-options.ts`,
    `apps/api/src/database/data-source.ts`, `apps/api/src/database/database.module.ts`
  - Work: When SH-26 creates the service roles, connect the running API with the NestJS application
    role while the migration CLI keeps the migration owner's credentials. The new variables go
    through `environmentSchema.extend` and `AppConfigService`, not `databaseEnvironmentSchema`,
    which the CLI parses, and reach `.env.example`, the Compose map and, for a secret, the dev
    runner's web filter through infrastructure.
  - Done when: the API runs with its own role and "Credentials and database roles must enforce the
    intended boundary": a runtime write from the API's connection fails (SH-26's test).
  - Tests: `environment.spec.ts`: the new variables are required and reported by name, never by
    value, and the CLI still loads with `POSTGRES_*` alone; SH-26's database-backed tests through
    X-24; `pnpm db:migration:show`; `pnpm smoke`.
  - Report: "Architecture and chart reading guide"; "Technical architecture and service ownership"
    (Proposed ownership: "Explicit service privileges and transactions; no assumed tenant
    isolation")
  - Blocked by: `decision 2 in docs/product/README.md`; `read path`

- [x] **API-18 · Serve the stored report and its registered template**
  - Done (2026-10-04): GET /api/runs/{id}/reports/{reportId} relays the shared ReportView unchanged through verified membership and Go's organization/content access boundary. Strict response validation preserves stored classifications, source lineage, template/projection versions and withheld content; mismatched run/report references or leaked withheld content fail closed. Checks: API lint, typecheck and build exited 0; API unit tests: "224 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". Tests cover both report fixtures, unknown fields, incorrect lineage/projection, missing sessions, 401/403/404/503 and transport failures. The source-trail behavior required by API-30 is included; its separate evidence task remains open. No runtime/demo SQL writes or computed labels.
  - **Report 1.1 change:** Serves the classification, template and projection versions, content hash and destination class with the report.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1.5-3 h)
  - Depends on: API-16 · Needs: X-64 · Provides: nothing
  - Paths: `apps/api/src` (the run facade module from API-11)
  - Work: Serve the run's stored report (stable identifier, version, source references and
    structured content) and its registered template, read-only, after the organization, role and
    object check; the route is named at M0 by its owner, because the report's operations table has
    no such read. Read it the way `stored report read` decides: from the stored-report view
    (SH-24) with schema-qualified SQL, or from a private Go endpoint with the operator context.
    Relay only the contract's fields.
  - Done when: the interface can follow "The interface renders its substantive content from the
    stored report and registered template, rather than displaying unrestricted model prose as the
    authoritative deliverable" for the operator's organization only.
  - Tests: specs: another organization's report and another run's report are refused; an unavailable
    source is an error, never an empty report: `pnpm --filter api run test`.
  - Report: "Illustrative passport and interface contracts" (Narrow final result and context
    boundary); "Project definition purpose and intended outcome" ("A legitimate invoice task would
    finish and leave an inspectable report in application state")
  - Blocked by: `stored report read`; `final result format`; `decision 7 in docs/product/README.md`; `report storage`

- [ ] **API-28 · Add the report template and projection rule records**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: API-04, SH-10 · Needs: X-06, X-24 · Provides: X-68 (part: entities)
  - Paths: `apps/api/src` (the policies module from API-04)
  - Work: `app` entities for the fixed, versioned report templates (`internal_investigation_v1`,
    `vendor_reconciliation_v1`) and the projection rules, seeded through SH-18, with no editor; table
    names proposal `report_templates` and `projection_rules`. SH-41 migrates them.
  - Done when: once SH-41 has migrated them, Go can read both templates and the vendor projection rule
    with their versions.
  - Tests: `pnpm --filter api run test`; `pnpm db:migration:generate <Name>` yields only `app`
    statements.
  - Report: "Delivery scope and six person ownership" (Implementer 2: "Own versioned source-policy and
    projection configuration"); "Functional requirements MVP boundary and deferred scope" (Trusted
    template manifests)
  - Blocked by: `vendor projection fields` (rule content)

- [ ] **API-29 · Record trusted source classifications and recipient rules, if `source classification storage` chooses `app`**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: API-04 · Needs: X-24 · Provides: X-70 (part)
  - Paths: `apps/api/src` (the policies module from API-04)
  - Work: Conditional. `app` records for the trusted source-field classifications and recipient
    rules, written by the policy administrator path. If `source classification storage` chooses
    `demo` records, this task becomes "Dropped: `source classification storage` chose demo
    records".
  - Done when: the Internal only label of the internal note comes from a stored record Go can read.
  - Tests: `pnpm --filter api run test`.
  - Report: "Functional requirements MVP boundary and deferred scope" (Trusted source
    classifications); "Users operating model and proposed user journeys" (Policy administrator)
  - Blocked by: `source classification storage`

- [x] **API-30 · Serve the report classification and trusted source trail to authorized users**
  - Done (2026-10-04): API-18's single stored-report route already returns X-64 classification, lineage and exact template/projection metadata unchanged from Go, including Internal-only withholding. API-30 records its separate authorization/source-trail evidence: report.controller.spec.ts covers both shared report fixtures, missing authentication, upstream 404, mismatched run/report, unknown source fields, projection consistency and forbidden withheld content; product-access.db-spec.ts checks real authenticated memberships in two organizations and foreign-report denial propagation. Go fixtures are labelled; no new live-model report was rendered by this API task. Checks: API lint, typecheck and build exited 0; API unit tests: "366 passed"; `pnpm test:db api`: "59 passed, 0 failed, 0 skipped"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". No duplicate endpoint, browser-derived labels or runtime/demo writes.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: API-16, API-18 · Needs: X-64, X-71 · Provides: nothing
  - Paths: `apps/api/src` (the module that serves API-18)
  - Work: Return the lineage summary after the organization, role and object checks; restricted
    source content only to authorized users; the route is named at M0.
  - Done when: an authorized operator reads the stored classification and source trail of both
    reports, and another organization's operator is refused.
  - Tests: `pnpm --filter api run test` with an authorized and an unauthorized caller.
  - Report: "Users operating model and proposed user journeys" (Journey 1); "Live demonstration
    storyboard and proof checks" (beat 4)
  - Blocked by: `stored report read`; `report storage`; `read path`; `decision 7 in docs/product/README.md`

- [x] **API-34 · Validate and import the signature feed**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: API-32 · Needs: X-87 · Provides: X-88
  - Paths: `apps/api/src` (the module from API-31)
  - Work: Validate the schema, the pinned issuer and signature or the authenticated trusted import, the bounded size and the supported pattern grammar before incorporating the feed into a catalog revision. "A content hash alone does not authenticate its publisher." Untrusted or invalid revisions keep the last-known-good feed and expose the failure.
  - Done when: the sample feed imports, and a tampered or oversized copy is rejected.
  - Tests: `pnpm --filter api run test` with valid, tampered, oversized and unsupported-grammar feeds.
  - Report: "Hybrid security controls and managed attack signatures" (Trusted historical attack feed)
  - Blocked by: `feed grammar and trust`
  - Completed (2026-10-03): `apps/api/src/policies/signature-feed.ts` validates the feed the way Go's `ParseFeed` does and stricter where they differ (closed schema, 64 KiB bound, strict JSON without duplicate keys, `schema_version` written as 1, `normalized_substring` rules with `block` response and printable-ASCII patterns, trusted issuer `task-passport-security` only); `importPolicyFile` takes the feed named by `signatures.path`, refuses a feed whose revision differs from `signatures.revision`, whose disabled rule IDs are not in it, or that reuses a stored revision with different bytes, stores the exact bytes with their SHA-256 (the same bytes reuse the row), and only requests the revision (the gateway activates it). Done-when met: the sample feed imports, and a tampered (changed bytes under a stored revision) or oversized copy is rejected. Checks on `main` e50e186: `pnpm --filter api run test` 125 passed (16 in `signature-feed.spec.ts`); `pnpm test:db api` 27 passed, 0 failed, 0 skipped (`signature-feed-import.db-spec.ts` refuses changed bytes, another revision, an unknown disabled rule, an untrusted issuer, an invalid feed and a missing feed with the pointer unchanged). Not covered here: the authenticated reload route that also imports a feed (API-33).

### Next.js (report role: Implementer 1)

- [x] **WEB-08 · Show the passport summary beside the timeline**
  - **Report 1.2 change:** The passport summary shows the active policy version and the allowed models.
  - **Report 1.1 change:** Shows the allowed report templates. No policy editor is planned (open item `policy editor`).
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 1.5-4 h)
  - Depends on: WEB-06 · Needs: X-08 · Provides: nothing
  - Paths: `apps/web/src/app` (the run page from WEB-06), `packages/ui/src/components/card.tsx`,
    `packages/ui/src/components/badge.tsx`
  - Work: Keep the issued passport visible next to the timeline, in business language: the goal of
    the task the passport admitted, permitted invoices, associated vendors, allowed recipient,
    approval requirement, expiry, remaining allowance, the task and policy versions and the rules
    the passport carries. State that records and recipients outside the passport are excluded
    instead of listing them, and show the persisted passport and the admitted task's goal from the
    server's data, never the request the operator typed. There is no policy editor; the fixed
    reviewed policy is shown as the passport admitted it.
  - Done when: demo beat 1's proof is visible: "Persisted passport matches authorized task scope;
    unrelated records and other recipients are visibly excluded." The admitted task's goal is shown
    with it (Journey 1: "The run page would then show the task goal, issued scope, current state,
    usage, and chronological activity").
  - Tests: specs over the passport fixtures and the fixture that carries the task goal: every field
    group renders from the persisted passport and the goal from the server's data; a missing field
    shows "not available", never a default or a value from the form: `pnpm --filter web run test`;
    a browser check, quoted.
  - Report: "Live demonstration storyboard and proof checks" ("Keep the passport visible beside the
    activity timeline"; Proposed demo sequence, beat 1); "Trusted authority and passport invariants"
    (Passport fields and their purpose); "Users operating model and proposed user journeys" (Journey
    1 create and delegate a task)
  - Blocked by: `passport in the run view`

- [x] **WEB-09 · Build the run timeline with attempts apart from effects**
  - **Report 1.1 change:** The export denial is WEB-28's; Report beats 5, 6 and 9.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: M (estimate 3-6 h)
  - Depends on: WEB-06 · Needs: X-12, X-36 · Provides: nothing
  - Paths: `apps/web/src/app` (the run page from WEB-06), `packages/ui/src/components/table.tsx`,
    `packages/ui/src/components/badge.tsx`, `packages/ui/src/components/empty-state.tsx`
  - Work: Render the sanitized events in order. An attempted operation and its decision are shown
    apart from a completed effect; a denial shows its stable reason code, the concrete rule and,
    where Go offers one, the bounded correction route; correction attempts are counted; a replayed
    proposal carries its replay label. Only masked metadata is shown, so an audit reader can trace
    the passport, action, rule version, reviewer decision and resulting effect.
  - Done when: "Show attempted operations separately from completed effects. A red event alone does
    not prove that a tool was prevented from changing state." holds on the run page, and beat 5's
    "the denial identifies the applicable rule" is visible.
  - Tests: specs over the safe event fixtures: a denied attempt never renders as an effect; an event
    marked as replay always shows the label; an unknown event kind is shown as unknown, not dropped:
    `pnpm --filter web run test`; a browser check of the labelled replay (X-36), quoted.
  - Progress (2026-10-04): the display component is on web/run-panels (lane f3):
    `RunEventTimeline` in `apps/web/src/components/run/run-event-timeline.tsx` takes the run's
    `SafeEvent[]` (and optional `AssessmentRecord[]`) and does no fetching; the pure model is
    `event-model.ts` (`describeEvent`, `summarizeEvents`). An attempt, its decision and a completed
    effect are different rows (kind from the event type alone: a denied attempt is never an effect,
    an unknown type is shown as unknown); a denial shows its reason code and sentence, the rule and
    the permitted alternative; denied proposals and completed effects (reads, reports stored,
    messages queued in the simulated outbox) are counted apart; a replay always carries its label.
    Specs (`event-model.test.ts`, `run-event-timeline.test.ts`, contract fixtures) pass. Not done:
    mounting on the run page (WEB-06, Batın's) and the browser check of the labelled replay.
  - Report: "Live demonstration storyboard and proof checks" (Proposed demo sequence, beats 5 and 6);
    "Users operating model and proposed user journeys" (Journey 3 recover cancel or investigate);
    "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a second
    disclosure channel)
  - Blocked by: nothing

- [x] **WEB-10 · Show the run's waiting and terminal states with their reasons**
  - **Report 1.1 change:** Cite Journey 3 (recover, cancel or investigate) of report 1.1.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-4.5 h)
  - Depends on: WEB-06 · Needs: X-11 · Provides: nothing
  - Paths: `apps/web/src/app` (the run page from WEB-06), `packages/ui/src/components/alert.tsx`,
    `packages/ui/src/components/error-state.tsx`
  - Work: Give every state of the run state contract a business-language view with its recorded
    reason: active, awaiting approval, completed, failed, stopped with its reason, and operator
    attention required for an unknown outcome (an admission rejection creates no run; WEB-11 shows
    it). Distinguish "inability to complete the task from a justified interruption that preserves a
    permitted path forward", and show why execution stopped. States come only from the persisted run
    view; the browser never infers one.
  - Done when: "The interface would clearly distinguish approval waiting, rejection, expiry,
    execution success, and an uncertain result" (Journey 2), and a stopped run shows its explanation
    (Journey 3).
  - Tests: specs: every state value of the contract has a view and an unknown value shows as unknown,
    never as success; a stopped run always shows its terminal reason: `pnpm --filter web run test`.
  - Progress (2026-10-04): the display component is on web/run-panels (lane f3): `RunStatePanel`
    in `apps/web/src/components/run/run-state-panel.tsx` takes the persisted `RunState` (and the
    recorded explanation from `terminalSafeMessage(events)`) and does no fetching; the model is
    `describeRunState` in `labels.ts`. Every status of the contract has a business view, and the
    view tells a justified interruption (paused at its allowance, waiting for a reviewer), an
    uncertain result needing an operator (paused with `outcome_unknown`), a deliberate stop and a
    failure apart; a paused, failed or stopped run always shows its terminal reason (or says none
    was recorded and flags the contract problem); an unknown status shows as unknown, never as
    success; a completed run lists the reports its final answer named. Specs over the run-state
    fixtures pass (`pnpm --filter web run test`). Not done: mounting on the run page (WEB-06,
    Batın's).
  - Report: "Users operating model and proposed user journeys" (Journey 2 review an exact outbound
    effect; Journey 3 recover cancel or investigate); "Mapping the proposal to the Goldman Sachs
    challenge" (Why successful work matters as much as blocked work)
  - Blocked by: nothing

- [x] **WEB-11 · Explain an admission rejection and require explicit resubmission**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: B · Size: S (estimate 1-2.5 h)
  - Depends on: WEB-05 · Needs: X-13 · Provides: nothing
  - Paths: `apps/web/src/app` (the task setup page from WEB-05)
  - Work: When admission rejects the request, keep the operator's choices on the form, show which
    requested authority was unavailable and which scope or limit must change, and let the operator
    submit an explicitly narrowed request as a new submission. Nothing narrows the request on its
    own, and no passport or run is shown for a rejected request.
  - Done when: the verification priority "Verify that scope rejection is explicit and the revised
    request must be resubmitted" holds in the interface ("A denied admission would explain which
    requested authority was unavailable").
  - Tests: specs: a rejection fixture shows its reason and the scope to change; the form never
    resubmits by itself and never changes a field the operator did not change; a browser check with
    an over-scope request, quoted: `pnpm --filter web run test`.
  - Report: "Trusted authority and passport invariants"; "Threat model limits and unresolved design
    choices" (Verification priorities); "Users operating model and proposed user journeys" (Journey
    1 create and delegate a task)
  - Blocked by: nothing
  - Progress (2026-10-04): the explanation is built but not mounted. `apps/web/src/lib/admission-rejection.ts` reads an admission rejection from a failed start-run response (`admissionRejectionFromError`: a 4xx whose safe error body carries `resource_out_of_scope`, `destination_not_allowed`, `template_not_allowed`, `limit_not_allowed` or `invalid_arguments`; anything else is not explained as one) and names the form fields to change (`explainAdmissionRejection`). `apps/web/src/components/admission-rejection.tsx` exports `AdmissionRejectionNotice({rejection, onResubmit, isSubmitting})`: it shows the reason code, the server's safe message, what was unavailable and the fields to change, says no passport or run exists and that nothing was narrowed, and resubmits only when the operator presses its button (type "button", never a form submit). For the form's owner to mount in `task-form.tsx`: on a failed `startRun`, call `admissionRejectionFromError(result.error)`, keep every field state as it is, render the notice, and call the existing submit only from `onResubmit`. Checks: `pnpm --filter web run lint`, `typecheck` and `test` exit 0 (74 passed, 12 of them new: a rejection shows its code, message and the scope to change; a non-admission failure is not explained as one; rendering never calls `onResubmit`; the button is not a form submit; markup in a server message is escaped). Not done, so not ticked: the notice is not mounted in the form, and the browser check with an over-scope request has not run.

- [x] **WEB-12 · Render the report from the stored report and its registered template**
  - **Report 1.1 change:** Renders both reports; beats 4 and 7.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: API-18, WEB-06 · Needs: X-64 · Provides: nothing
  - Paths: `apps/web/src/app` (the run page from WEB-06)
  - Work: Render the report's content from the stored report and its registered template, with its
    identifier, version and source references; never present model prose as the deliverable. Once
    the report is queued, mark its outbox entry as a simulated outbox message, not an email.
  - Done when: beat 3's proof is visible, rendered from the stored report: "the report references
    authorized records and contains the expected discrepancy".
  - Tests: specs: the rendering uses only stored-report fields; a report without its template shows
    an error, not free text: `pnpm --filter web run test`; a browser check after a permitted
    reconciliation, quoted.
  - Report: "Illustrative passport and interface contracts" (Narrow final result and context
    boundary); "Live demonstration storyboard and proof checks" (Proposed demo sequence, beat 3)
  - Blocked by: `stored report read`; `final result format`
  - Completed (2026-10-04): lane web/reports. The page `apps/web/src/app/runs/[id]/reports/[reportId]/page.tsx` loads one stored report through the same-origin route `app/api/runs/[id]/reports/[reportId]/route.ts` (proxyUpstream to `GET /api/runs/{id}/reports/{reportId}`) and `lib/clients/reports-client.ts`, whose guard `isReportView` accepts only the two registered templates, a consistent withheld state, a 64-hex content hash and a non-empty source trail; anything else is an error ("Report cannot be shown"), never free text. `components/report/report-content.tsx` renders the title, report id, run id, version and template with its version from the stored fields only, the content as plain text (never markup), or, when `contentWithheld` is set, the statement "Content withheld" and no content, even if a body were present. Checks: `pnpm --filter web run lint` (0 errors; 3 warnings in files this lane does not own), `typecheck` and `test` (49 passed, 12 new: the guard against both report-view fixtures and tampered copies, the client with encoded ids and the API's 404, the rendering of both fixtures, a tampered withheld report, markup as text, the failure words, and the route's upstream path). Browser check, quoted: on a live run (qwen3.5:4b, development demonstration) the model read both invoices, created the internal report, was denied its export (`report_export_restricted`), created the vendor report and waited for approval; as the signed-in demo operator, `/api/runs/{run}/reports/{vendor report}` answered 200 (404 for an unknown id, 401 without a session) and the page at `/runs/{run}/reports/{vendor report}` showed "Vendor reconciliation", version 1, `vendor_reconciliation_v1 (version 1)` and its stored content ("invoice_A01: external reference INV104 … duplicate reference: yes"). Not here: the classification label and source trail (WEB-27), the denial explanation (WEB-28), and marking a queued report's outbox entry as simulated, which is event data that `ReportView` does not carry (WEB-13 and the run timeline); the link from the run page is Batın's mount.

- [ ] **WEB-13 · Label the simulated outbox, replays and the development demonstration**
  - **Report 1.1 change:** Label quote: "Do not describe a simulated outbox as live email delivery or an invoice report as a real payment operation."
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 1-2 h)
  - Depends on: WEB-04, WEB-09, WEB-12 · Needs: X-16, X-36 · Provides: nothing
  - Paths: `apps/web/src/lib` (new label module, named at M0 by its owner), `apps/web/src/app` (the
    product pages)
  - Work: Keep the truthful labels in one place, worded as the demo specification fixes them (X-16):
    the simulated outbox ("a queued database message is a real prototype effect, but it is not
    evidence of email delivery"), replayed proposals, the development demonstration, sample data and
    estimated cost. Apply them on every view that shows such a thing, derived from the server's data
    (for example an event's replay mark), never guessed by the page.
  - Done when: "All effects should use synthetic data, with truthful labels for simulated delivery
    and captured-action replay" holds on every page.
  - Tests: specs: an event with the replay mark, an outbox effect, an estimated amount and the seeded
    operator each always carry their label, and an event without the mark carries none:
    `pnpm --filter web run test`.
  - Report: "Threat model limits and unresolved design choices"; "Live demonstration storyboard and
    proof checks" (Reliable demonstrations without invented behavior)
  - Blocked by: nothing
  - Progress (2026-10-04): the label module and components are done. `apps/web/src/lib/labels.ts` holds the wording of the demonstration specification's label table (X-16) and derives each label from server data, returning null when the data carries no mark: `replayLabel(replaySource)`, `verdictSourceLabel(verdictSource)` ("live" or "fixture", nothing else, never live by default), `outboxEffectLabel(effect)`, `developmentDemonstrationLabel(email)` (the session's `/me` has no demo flag, so this derives from the seeded operator's email; a server field is an open request), `estimatedCostLabel({pricingRule, unresolved})` (null without a rule: not shown), `recordingLabel(buildId)` and the fixed `LABELS`. `apps/web/src/components/labels` exports `LabelBadge`, `SimulatedOutboxLabel`, `OutboxEffectLabel`, `ReplayLabel`, `VerdictSourceLabel`, `DevelopmentDemonstrationLabel`, `EstimatedCostLabel`, `TestDoubleLabel`, `TestEvidenceLabel`, `SyntheticDataLabel` and `RecordingLabel`; each renders nothing when the data carries no mark. Checks: `pnpm --filter web run lint`, `typecheck` and `test` exit 0 (57 passed, 25 of them new: an event with the replay mark, an outbox effect, an estimate and the seeded operator each always carry their label, an unmarked event carries none, a mark in an unexpected form still shows the label, and a server-supplied mark is escaped). Missing half: the pages that show such things must mount them (the run page's timeline and outbox effect, the control views' verdicts, any cost figure, the navigation's operator); the Done-when ("every page") holds only when they have.

- [x] **WEB-27 · Show the report classification and source trail on the run page**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: WEB-12, API-30 · Needs: X-64, X-71 · Provides: nothing
  - Paths: `apps/web`
  - Work: Show the stored classification, the source and template versions and the content hash, and a
    renamed title beside the unchanged label. Never compute a label: "The interface must read stored
    metadata rather than compute its own public label."
  - Done when: the run page shows the Internal only report with its source trail from stored
    metadata.
  - Tests: `pnpm --filter web run test` with the contract fixtures.
  - Report: "Live demonstration storyboard and proof checks" (beat 4)
  - Blocked by: `stored report read`
  - Completed (2026-10-04): lane web/reports. `components/report/classification-badge.tsx` shows the stored classification ("Internal only" / "Vendor shareable") as a lookup of the stored field, never derived from the title, content or sources; `components/report/source-trail.tsx` shows the template and its version, the projection rule and version (or that the internal template has none), the destination class, the content hash with a copy button, and per source its kind and id, version, stored classification and consumed fields. Both are mounted in the report page's `ReportContent` (title beside the badge). Tests (54 in the web package, 5 new, with the report-view fixtures): the badge and label of a vendor report and of a withheld Internal only one; every source with version, classification and fields; template, projection, destination and hash; and a title that claims "Internal only" with a disagreeing source trail still shows the stored "Vendor shareable". `lint`, `typecheck` and `test` exit 0. Browser check, quoted: the Internal only report of a live run showed the "Internal only" badge beside "Internal investigation", template `internal_investigation_v1 (version 1)`, projection "None (the internal template reads sources directly)", destination "Internal reviewers", its content hash, and the trail `invoice_A01` version 1 Internal only (fields including `internal_note`) and `invoice_A02` version 1 Vendor shareable. Not here: the mount of the link on the run page (Batın's page), and the "renamed title beside the unchanged label", which waits on the open item `rename operation`; the page already shows the stored title next to the stored label.

- [x] **WEB-28 · Explain the export denial and the safe continuation**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: WEB-09, WEB-27 · Needs: X-12, X-13 · Provides: nothing
  - Paths: `apps/web`
  - Work: Show `report_export_restricted` with its rule, that the recipient was already permitted, and
    the offered vendor template, never as an approval request.
  - Done when: an operator can see why the export was denied and that the vendor report continues the
    task.
  - Tests: `pnpm --filter web run test` with the contract fixtures.
  - Report: "Live demonstration storyboard and proof checks" (beats 5 and 7); "Users operating model
    and proposed user journeys" (Journey 3)
  - Blocked by: nothing
  - Completed (2026-10-04): lane web/reports. `apps/web/src/components/report/export-denial.tsx` (`ExportDenial`, taking a `SafeEvent`) explains a `report.export_denied` event whose reason is `report_export_restricted`: the rule (a report built from an Internal only source inherits the restriction and cannot go to a vendor, whatever its title says), that the task's recipient was permitted and the denial comes from the report's own stored restriction, and the continuation: a separate vendor report rendered from approved database fields only, naming the template from `maskedSummary.alternativeTemplate` (or "No alternative was offered" when there is none). It says plainly that this is a denial, not an approval request, and has no button or action; it shows the event's `safeMessage`, a labelled replay (`replaySource`) as "Replay: scripted proposal, not generated by the model", and a link to the restricted report only when the event names it. It renders nothing for any other event or reason. `isExportDenial(event)` lets the timeline decide where to mount it. Tests (61 in the web package, 7 new, with `safe-event.export-denied.json` and `safe-event.admission-rejected.json`): the rule, recipient and continuation text; the report link; no button, "Approve" or "Reject"; an event without a report id (the gate's real event) shows no link and no invented template; no alternative; a replay label; nothing for other events. `lint`, `typecheck` and `test` exit 0; a temporary preview page, since removed, rendered the component in the browser from the fixture. Finding for the gate lane (w3): the real `report.export_denied` event from a live run carries `alternativeTemplate` and `safeMessage` but null `reportId`, `template` and `classification`, unlike the shared fixture, so the "View the restricted report" link appears only once the event carries the report id; the id is the stored report's, not a protected value. Mounting `ExportDenial` where the timeline shows `report.export_denied` is Batın's (WEB-09 and WEB-32).

- [x] **WEB-32 · Show hybrid decisions in the run timeline**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-3 h, this roadmap's estimate)
  - Depends on: WEB-06 · Needs: X-12, X-13 · Provides: nothing
  - Paths: `apps/web`
  - Work: Show allowed, blocked and redacted decisions with their rule, reason code, revision and model purpose. Blocked text is withheld and never shown as consumed by the agent; a guard failure is a visible pause.
  - Done when: the hostile-note run shows the block before context and the clean run shows its allow.
  - Tests: `pnpm --filter web run test` with the contract fixtures.
  - Progress (2026-10-04): `DecisionBadges` (`decision-badges.tsx`) is part of the WEB-09 timeline:
    each event shows the controls that decided it as deterministic, signature or semantic badges
    with the result, rule and revisions; a semantic verdict says live or fixture ("fixture verdict,
    not detection quality"), a semantic check with nothing to classify shows not applicable, and a
    blocked or redacted text is shown as withheld, never as consumed by the agent; a guard failure
    shows "Nothing was released and the run is paused". Exact controls come from stored
    `AssessmentRecord`s joined on the evaluation id; without them only a reason code that names
    one control (signature_match, semantic_injection_detected, ...) produces a badge, never a
    guessed one (no run-scoped assessment read exists in the API yet). Specs pass. Not done: the
    hostile-note and clean runs on the page (WEB-06, Batın's).
  - Report: "Live demonstration storyboard and proof checks" (beat 10); "Users operating model and proposed user journeys" (Journey 3)
  - Blocked by: nothing

## M3 (hours 10-14)

- Team focus (report 1.2): "Add frozen action previews, authorized approval decisions, approval consumption, model/tool reservations, and controlled stopping. Finish shared/purpose token limits, request timeout and local concurrency enforcement, validated policy reload and last-known-good behavior."
- Exit condition (report): "The reviewed action executes once; changed content and depleted
  allowance cannot dispatch an operation."
- Report 1.2 sync points (spine): X-83.
- Sync points needed by the end (spine): X-39 to X-46, X-66 and X-67. This side provides X-43 (its
  approval part from API-20 and WEB-14, its cancel part from API-21 and WEB-15), X-44 (WEB-18, its
  Next.js and NestJS part), X-46 (WEB-17, its Next.js part) and X-66 (API-22), and already delivers
  X-47's write path (API-22) and X-55's cancel part (API-21), which M4 needs. It consumes X-40, X-41
  and X-42; X-67 (beat 6) is the Go side's evidence.

SH-28 captures the vertical-slice evidence (Legitimate task, Resource boundary, Destination boundary,
Approval integrity, the limit-triggered stop and beat 6's continuation in the same run) from runs
this side starts and reviews.

### NestJS (report role: Implementer 2)

- [x] **API-19 · Serve the exact review payload to authorized reviewers only**
  - Done (2026-10-04): GET /api/actions/{id}/review refuses a non-reviewer with 403 before Go, using roles only from verified membership. It relays the Go-owned ReviewView (commit 32b4a76) unchanged, including snake_case keys, frozen recipient, report and source manifest. Shared-schema validation restores queue_report conditionals; unknown fields, incomplete frozen material and mismatched action references are refused. Cache-Control: no-store. Checks: API lint, typecheck and build exited 0; API unit tests: "280 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; host smoke: "28 passed, 0 failed, 5 skipped". Tests use the shared fixture generated from real Go output; a new live model review was not exercised.
  - **Report 1.1 change:** The payload adds the report identifier, content hash, source manifest and digest, classification, template and projection versions; "Review payloads and source manifests need their own access rules". ActivityModule is the architecture's proposal.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h)
  - Depends on: API-16 · Needs: X-09, X-41 · Provides: nothing
  - Paths: `apps/api/src` (the run facade module from API-11)
  - Work: Serve the stored action's exact review payload (recipient, rendered content, referenced
    report and version, reason for review) only to an operator who holds the reviewer role in the
    action's organization; the route is named at M0 by its owner, because the operations table has
    no such read. Keep the payload out of the event feed, the run view and every error text.
  - Done when: "Review payloads need their own access rules": a reviewer reads the exact stored
    effect, and an operator without the reviewer role or from another organization is refused.
  - Tests: specs: a non-reviewer and another organization's reviewer are refused; the payload never
    appears in the events or run view responses: `pnpm --filter api run test`.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a
    second disclosure channel); "Exact action approval versioning and execution rechecks"; "The
    enforcement loop and data minimization" ("authorized reviewers may see the specific content
    necessary to make a decision")
  - Blocked by: `read path`; `review payload read`; `decision 7 in docs/product/README.md`

- [x] **API-20 · Forward approval decisions through `POST /api/actions/{id}/approval`**
  - Done (2026-10-04): reviewer-only POST /api/actions/{id}/approval accepts exactly the shared X-10 decision and forwards it to Go; NestJS creates no grant or runtime state. Go-owned ApprovalResponse from 32b4a76 is validated and relayed unchanged; action/decision mismatches fail closed, 401/403/404/409/503 remain upstream errors, and timeout is 504 outcome_unconfirmed. Checks: API lint, typecheck and build exited 0; API unit tests: "299 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; host smoke: "28 passed, 0 failed, 5 skipped". Tests cover approve/reject, non-reviewer refusal before Go, extra identity/payload fields, invalid decisions, transport failures and response mismatches. Full live approval/outbox execution was not exercised by this API task.
  - **Report 1.1 change:** A forbidden export never reaches approval, and an approval cannot override it.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1.5-3 h)
  - Depends on: API-10, API-16 · Needs: X-10, X-13, X-40 · Provides: X-43 (part: approval)
  - Paths: `apps/api/src` (the run facade module from API-11)
  - Work: Accept only the decision (approve or reject) for the stored action the path names, with no
    replacement payload. Check the reviewer role and the organization, then forward to the internal
    approval command (X-40), where Go checks reviewer authority, action integrity and expiry and
    stores the grant; NestJS records no grant. Map Go's outcomes (for example an expired or changed
    action, in the adopted vocabulary) to the envelope, report a timeout as unconfirmed, never as
    approved, and document the route in Swagger.
  - Done when: "The command sent back to Go identifies that stored action; it does not replace it
    with a new browser-supplied payload", and an authorized reviewer's decision reaches Go while a
    non-reviewer's is refused before it.
  - Tests: specs with a stub Go: a body with any field besides the decision is refused; a
    non-reviewer is refused before any upstream call; each Go outcome keeps its reason code; a
    timeout is unconfirmed; against the real gateway, an expired or changed action is refused:
    `pnpm --filter api run test`.
  - Report: "Exact action approval versioning and execution rechecks"; "Users operating model and
    proposed user journeys" (Journey 2 review an exact outbound effect); "Illustrative passport and
    interface contracts" (Proposed browser and runtime operations)
  - Blocked by: `decision 7 in docs/product/README.md`

- [x] **API-21 · Forward cancellation through `POST /api/runs/{id}/cancel`**
  - Done (2026-10-04): forwards cancellation with verified membership and no extra role, per product decision 30. Returns Go's X-11 state unchanged; browser command fields are refused; a timeout is 504 outcome_unconfirmed and never cancellation success. Swagger states that future dispatches stop and committed effects remain. Checks: API lint, typecheck and build exited 0; API unit tests: "192 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". Real API/Go calls returned 200 for state, usage and cancel and persisted cancelRequestedAt; the current gateway returned the run.cancel_requested event. Host smoke: "28 passed, 0 failed, 5 skipped" (host logs unavailable). No database writes by NestJS.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: API-10, API-16 · Needs: X-42 · Provides: X-43 (part: cancel); X-55 (part: cancel
    command)
  - Paths: `apps/api/src` (the run facade module from API-11)
  - Work: Check that the "Actor may manage this run within this organization", forward to the
    internal cancel command (X-42), which persists cancellation before future dispatches, and return
    its result. The response says that cancellation stops future dispatches and does not reverse
    committed effects. Document the route in Swagger.
  - Done when: "Future dispatches stop while committed effects remain correctly recorded"
    ("Cancellation and revocation") for a run cancelled through the public path, with the
    cancellation visible in the run view, and another organization's operator cannot cancel it.
  - Tests: specs with a stub Go: an operator who may not manage the run is refused before any
    upstream call; each Go outcome keeps its reason code; a timeout is unconfirmed; against the real
    gateway, the cancellation timestamp and the next refused dispatch appear in the run view and
    events: `pnpm --filter api run test`.
  - Report: "Atomic allowances hard limits and estimated cost" (Cancellation and time limits); "Users
    operating model and proposed user journeys" (Journey 3 recover cancel or investigate);
    "Validation plan and evidence matrix" (critical check Cancellation and expiry)
  - Blocked by: `decision 7 in docs/product/README.md`

- [ ] **API-22 · Record revocations in the app schema**
  - **Report 1.1 change:** Adds source and template revocations.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 1.5-3.5 h)
  - Depends on: API-04, API-16 · Needs: X-18, X-24 · Provides: X-47 (part: NestJS write path);
    X-66
  - Paths: `apps/api/src` (the configuration module from API-04),
    `apps/api/src/database/typeorm-options.ts`
  - Work: Add the revocation records with `schema: "app"` in the shape the web + API implementer and the Go implementer, with the
    lead for the grant, agree under `revocation reads` in SH-14's review, and a trusted write path only a policy administrator
    can use: an explicit command, which stays within "reduce or revoke authority through trusted
    configuration", or an authenticated operation, named at M0 by its owner. The report's table
    "Proposed browser and runtime operations" has no revocation operation, so an authenticated
    operation needs the document owner's record first (spine, "Scope changes"). A revocation only
    reduces authority and never edits Go's runtime state; Go enforces it before dispatch and before
    execution (X-47). SH-38 migrates the entity (the same person by default) and SH-26 gives Go's read grant (the lead), through X-66.
  - Done when: once SH-38 has migrated the records, a policy administrator can "reduce or revoke
    authority through trusted configuration", no other role can, and Go reads the record before its
    next dispatch, also after a review wait.
  - Tests: specs: a non-administrator is refused; a revocation cannot widen authority or change a
    passport; database-backed tests through X-24 for the record; against the real gateway, a run
    waiting for review dispatches nothing after the revocation: `pnpm --filter api run test`.
  - Report: "Exact action approval versioning and execution rechecks" (Versioned policy and current
    revocation); "Users operating model and proposed user journeys" (Proposed user
    responsibilities); "Threat model limits and unresolved design choices" ("Verify that
    cancellation and revocation prevent future dispatch after a review wait")
  - Blocked by: `revocation reads`; `decision 7 in docs/product/README.md`

- [ ] **API-33 · Serve the authenticated policy reload and activation**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 3-5 h, this roadmap's estimate)
  - Depends on: API-32, API-16 · Needs: X-79 · Provides: X-83 (part: import and pointer)
  - Paths: `apps/api/src` (the module from API-31 and the runtime facade)
  - Work: `POST /api/policies/reload`: verify the actor is an authorized configuration operator, import the file through API-32, ask Go to validate and acknowledge the candidate, then publish the active pointer. Expose the requested, validated and active revisions and the reload error. Invalid reloads keep the last-known-good revision.
  - Done when: a valid edit becomes active without restarting any service and an invalid edit leaves the previous revision active with a visible error.
  - Tests: `pnpm --filter api run test` for authority, validation failure and the Go acknowledgement failure.
  - Report: "Central policy configuration and safe reload"; "Illustrative passport and interface contracts" (Proposed browser and runtime operations)
  - Blocked by: `catalog activation protocol`; `decision 7 in docs/product/README.md`

### Next.js (report role: Implementer 1)

- [x] **WEB-14 · Build the approval preview of the stored action**
  - **Report 1.1 change:** Shows the recipient, rendered content, classification, report version, approved source fields, and source, template and projection versions; route proposal `/approvals` for one action (a list needs `list reads`); beat 8.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: M (estimate 3.5-6.5 h)
  - Depends on: API-19, API-20, WEB-10 · Needs: X-09, X-10, X-41 · Provides: X-43 (part: the
    approval operation on the browser path)
  - Paths: `apps/web/src/app` (new review page, named at M0 by its owner),
    `packages/ui/src/components/confirm-dialog.tsx`, `packages/ui/src/components/card.tsx`
  - Work: Render the stored action from the server each time the page opens: recipient, rendered
    content, referenced report and version, and the reason review is required. Approve and reject
    send only the displayed action identifier and the decision; the controls show only for an
    operator with the reviewer role, while NestJS and Go decide. After a decision show the outcome:
    approved and queued, rejected, expired, or changed (a new review is needed). The waiting state
    is read from the server, so it survives a reload or a worker restart. Add the operation to the
    browser path of WEB-02 if that path lists routes.
  - Done when: beat 7 can be performed ("Operator sees the stored report content and recipient, then
    approves the outgoing action"), with its proof: "Approval binds to the displayed action ID and
    content; one approved outbox row is created, with no broader grant."
  - Tests: specs: the decision request holds only the action identifier and the decision, never
    content or a recipient; an expired or changed outcome never shows as approved; a browser check
    of approve, reject and an expired action against the real stack, quoted:
    `pnpm --filter web run test`.
  - Completed (2026-10-04, lane 3c on web/approval): `app/runs/[id]/review/[actionId]/page.tsx` with
    `components/approval/review-panel.tsx` reads `GET /api/actions/{id}/review` (review-view,
    snake_case) and the run state on every visit and shows the exact recipient (address, vendor,
    reference), the stored report content with classification, report and template versions, projection
    rule and SHA-256 content hash, the source manifest (each source, version, classification, fields
    used, manifest digest), the policy revision and the expiry. Approve and Reject each confirm, then
    send exactly `{"decision": …}` to `POST /api/actions/{id}/approval` (new same-origin routes
    `app/api/actions/[id]/{review,approval}/route.ts`). Every error is named: 403 not a reviewer, 404
    nothing awaits review, 401 signed out, 409 expired, changed, run stopped or already decided (each
    "nothing was sent"), 504 or a lost connection "outcome unconfirmed", 503 unavailable; an expired or
    changed outcome never reads as approved; a closed review (run no longer awaiting approval) disables
    the decision; the simulated outbox is labelled with c1's `SimulatedOutboxLabel`. Tests:
    `actions-client.test.ts` (10: the decision body is only the decision; 409 kinds never approved; an
    answer about another action or decision is unconfirmed; non-id refused before any request;
    error-status mapping; review and run state of another action or run refused). Browser checks
    (headless Chromium, real stack, demo operator): approve — run 78fc91be, action 62bdba4e, all
    sections shown, one request body `{"decision":"approve"}`, "Approved" with the simulated-outbox
    label, buttons disabled; the run resumed and completed with 1 outbox row; reopening shows "This
    review is closed: the run is completed" with Approve disabled. Reject — run 87d0ed10, action
    59ab12ca, body `{"decision":"reject"}`, "Rejected … Nothing will be sent", outbox unchanged; Go
    recorded `approval_rejected` and the model proposed again. Not verified live: an expired review (the
    expiry follows the run lifetime, 15 minutes); covered by the client tests.
  - Report: "Users operating model and proposed user journeys" (Journey 2 review an exact outbound
    effect); "Exact action approval versioning and execution rechecks"; "Live demonstration
    storyboard and proof checks" (Proposed demo sequence, beat 7)
  - Blocked by: `review payload read`; `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [x] **WEB-15 · Add the cancel control that says cancellation is not a reversal**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: B · Size: S (estimate 0.5-1.5 h)
  - Depends on: API-21, WEB-10 · Needs: nothing · Provides: X-43 (part: the cancel operation on the
    browser path)
  - Paths: `apps/web/src/app` (the run page from WEB-06), `packages/ui/src/components/confirm-dialog.tsx`
  - Work: Offer cancellation to an operator who may manage the run, with a confirmation that says it
    stops future dispatches and does not reverse committed effects. After cancelling, keep the
    committed effects listed and show the cancelled state from the server. Add the operation to the
    browser path of WEB-02 if that path lists routes.
  - Done when: "The operator could cancel future dispatches, while the interface would retain
    already committed effects and explain that cancellation is not a reversal mechanism" (Journey 3).
  - Tests: specs: the confirmation states the limitation; after the cancelled state arrives, earlier
    effects still render; a browser check, quoted: `pnpm --filter web run test`.
  - Completed (2026-10-04, lane 3c on web/approval): `components/approval/cancel-run-button.tsx`
    (`CancelRunButton`, for the run page to mount; also on the review page) opens a confirmation that
    states the limitation ("stops future work … does not reverse anything already done: reports already
    created and messages already queued stay recorded"), sends `POST /api/runs/{id}/cancel` with `{}`
    through the new same-origin route `app/api/runs/[id]/cancel/route.ts`, and shows the state the
    server recorded (stopped, or stopping before the next step for a running run; never more); a
    finished run offers no cancel. Client `lib/clients/actions-client.ts` `cancelRun` (a lost connection
    is "unconfirmed", never "nothing happened"). Tests: `cancel-run-text.test.ts` (the confirmation
    states the limitation; the reported state never exceeds the server's), `actions-client.test.ts`
    (empty command, unknown run). Browser check (headless Chromium against the real stack: web 3110, API
    3111, gateway 8110, demo operator): run 87d0ed10 awaiting approval, Cancel run → confirmation text
    as above → one request `POST /api/runs/87d0ed10-…/cancel` body `{}` → "Cancellation recorded. The
    run is stopped. Work already done is not reversed and stays listed."; the run is `stopped` /
    `run_cancelled`, and its 2 reports and the outbox (1) are unchanged.
  - Report: "Users operating model and proposed user journeys" (Journey 3 recover cancel or
    investigate); "Validation plan and evidence matrix" (Interpreting results honestly:
    "Cancellation can prevent future dispatches but cannot retract information already sent or undo
    committed effects")
  - Blocked by: `decision 3 in docs/product/README.md`; `decision 7 in docs/product/README.md`

- [ ] **WEB-16 · Show usage with reported, reserved and estimated amounts apart**
  - **Report 1.2 change:** Shows agent and security usage separately.
  - **Report 1.1 change:** Beat 9; X-46 as amended in the spine.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 1.5-3.5 h)
  - Depends on: WEB-08 · Needs: X-11 · Provides: nothing
  - Paths: `apps/web/src/app` (the run page from WEB-06), `packages/ui/src/components/table.tsx`
  - Work: Show request and token limits apart from estimated monetary spending, and reported usage
    (measured units), reserved allowance and estimated cost as separate figures. Money is labelled
    estimated whenever pricing or usage is uncertain, and an unresolved reservation shows as
    uncertain, never as zero. The accounting rule is decision 6's. Tier A because WEB-17, a Tier A
    task, depends on it.
  - Done when: "the interface should distinguish reported usage, reserved allowance and estimated
    cost" holds, and the early signal of the risk "Budget display implies certainty" ("Unknown usage
    is shown as zero or estimated currency is described as an exact bill") cannot occur.
  - Tests: specs over the usage fixtures: a missing reported amount with a retained reservation
    renders as uncertain, not zero; an estimated amount always carries its label; limits render from
    the passport, never from constants: `pnpm --filter web run test`.
  - Progress (2026-10-04): the display component is on web/run-panels (lane f3): `RunUsagePanel`
    in `apps/web/src/components/run/run-usage-panel.tsx` takes the `RunUsage` of
    `GET /api/runs/:id/usage` and does no fetching; the model is `usage-model.ts`
    (`describeUsage`). Per purpose it shows requests sent with their outcomes, reported tokens (the
    provider's measured usage) and reserved tokens (allowance held, not usage) as separate columns,
    never added; the allowance and request limits come from the run's own ledger, never from
    constants; an unknown-usage request shows "+ uncertain" next to the reported figure and the
    shared label "Uncertain: an unresolved reservation, not a measured amount and not zero". No
    amount of money is ever shown: the contract carries no cost (a local model, no pricing rule),
    so the panel says that estimated cost is not shown, which is c1's rule "estimated, with the
    pricing rule, or not shown". Specs over the usage fixtures pass. Not done: mounting on the run
    page (WEB-06, Batın's).
  - Report: "Atomic allowances hard limits and estimated cost"; "Validation plan and evidence
    matrix" (Interpreting results honestly); "Risk register and scope controls" (Budget display
    implies certainty); "Live demonstration storyboard and proof checks" (Proposed demo sequence,
    beat 9)
  - Blocked by: `decision 6 in docs/product/README.md` (the estimated cost display)

- [ ] **WEB-17 · Show the limit-triggered stop with its terminal reason**
  - **Report 1.1 change:** Beat 9; X-46 as amended in the spine.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 0.5-1.5 h)
  - Depends on: WEB-10, WEB-16 · Needs: X-34 · Provides: X-46 (part: the visible terminal reason)
  - Paths: `apps/web/src/app` (the run page from WEB-06)
  - Work: With the small configured allowance from the fixtures, start beat 9's separate run through
    the interface and confirm that the run page shows the persisted terminal reason and the usage at
    the stop; a runtime test used instead is labelled as one.
  - Done when: beat 9's proof is visible: "The next request is rejected before dispatch; a terminal
    reason is visible and the ledger does not record an unaccounted call", with the terminal reason
    read from the run view.
  - Tests: the beat 9 run by hand against the real stack, with the run view and a screenshot quoted;
    WEB-10's specs for the stopped state.
  - Progress (2026-10-04): the display component is on web/run-panels (lane f3):
    `LimitStopNotice` in `apps/web/src/components/run/limit-stop-notice.tsx` takes the persisted
    `RunState`, the run's `RunUsage` (or null) and the recorded explanation; `limitStopKind` reads a
    limit stop from the persisted state alone (paused or stopped with `allowance_exhausted`,
    `security_allowance_exhausted` or `run_expired`) and the component renders nothing for any other
    run. It shows the terminal reason with its code, which request limits are at their cap, the
    token allowances at the stop (reported, reserved, available), whether every dispatched request
    is accounted for (or which purpose has an unaccounted one), and unknown usage as uncertain;
    "no further model request is sent" is stated as following from the run being paused or stopped,
    and the usage as of the latest read. Specs pass (`limit-stop-notice.test.ts`). Not done: the
    beat 9 run by hand on the real stack with a screenshot (needs WEB-06's page).
  - Report: "Live demonstration storyboard and proof checks" (Proposed demo sequence, beat 9);
    "Threat model limits and unresolved design choices" (the smallest credible vertical slice: "one
    limit-triggered stop")
  - Blocked by: nothing

- [ ] **WEB-18 · Run the legitimate task through the interface**
  - **Report 1.1 change:** The run covers the internal report, the denied export, the vendor report, review and one outbox row; beats 1, 3 to 5, 7 and 8.
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: A · Size: S (estimate 1-3 h)
  - Depends on: WEB-09, WEB-12, WEB-13, WEB-14 · Needs: X-34 · Provides: X-44 (part: run start,
    approval and event display)
  - Paths: `apps/web/src/app` (the task setup, run and review pages)
  - Work: On the seeded fixtures, start the legitimate reconciliation from the task form, follow its
    events, review and approve the queue action, and see the completed run, the report and the
    simulated outbox entry, all through the interface and the API. Record the run, action and request
    identifiers so that SH-28 can attach the Go-side records.
  - Done when: critical check "Legitimate task" holds through the interface ("Reconciliation
    finishes and the approved report is queued once"), with its evidence "Report references,
    expected discrepancy, one outbox row, and completed run events" retrievable for SH-28.
  - Tests: the run by hand against the real stack, with the identifiers and screenshots quoted.
  - Report: "Validation plan and evidence matrix" (Proposed critical checks: Legitimate task); "Live
    demonstration storyboard and proof checks" (Proposed demo sequence, beats 1, 3 and 7)
  - Blocked by: nothing

- [ ] **WEB-29 · Show the active controls, policy revision and reload state**
  - **Progress (2026-10-04, web/security), not ticked:** the panel is built and sits at the top of `/security` (`components/security/active-controls.tsx`, `catalog-view.ts`, `lib/clients/security-client.ts`, route handler `app/api/policies/status/route.ts`) against lane 08's `CatalogStatus` contract (`GET /internal/catalog/active`; contract files taken from `web/catalog-status` d5bea8c, TypeScript side only). It shows the active, requested and validated revisions, the policy and feed digests, the feed revision and rule count, the disabled feed rules, and each of the three controls with its state (a disabled one shown as disabled), response, threshold and boundaries. A requested revision that is not active yet is shown as pending and the panel asks again every 3 seconds until it is in force; a rejected change shows its code, message and stage with the active revision still in force. A 404 or the web proxy's refusal of a path it does not forward is shown as "Not available yet", a 503 as an outage and never as an empty catalog; 401, 403 and a body outside the contract are handled; no upstream text is shown. Checks: `pnpm --filter web run lint` exit 0 (0 errors; 3 warnings in other lanes' files), `typecheck` exit 0, `test` exit 0 (131 passed, with the view model and client tested against both real fixtures), contracts lint, typecheck, test (9) and build exit 0. In headless Chromium on the local stack: the live page shows "Not available yet"; with the page's own request answered by the real contract fixtures, the active, rejected, disabled-control, pending (becoming in force without a click), nothing-active and every failure state rendered correctly and the page has no horizontal scroll at 390 px. **Missing for the done-when ("after a valid and an invalid reload the page shows the right active revision and the error"):** the API route `GET /api/policies/status` (Noyan) and `"/api/policies"` in the web proxy's `UPSTREAM_PREFIXES` (the lead's `upstream-proxy.ts`); until both exist the live page cannot show a reload, so this task is ticked only after a live valid and invalid reload through the real API.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 2-3 h, this roadmap's estimate)
  - Depends on: API-33 · Needs: X-83 · Provides: nothing
  - Paths: `apps/web`
  - Work: Show the active revision, the enabled controls and thresholds, and the requested, validated and active reload states with any rejection, "so judges can see when a change is effective".
  - Done when: after a valid and an invalid reload the page shows the right active revision and the error.
  - Tests: `pnpm --filter web run test` with the contract fixtures.
  - Report: "Users operating model and proposed user journeys" (Journey 4); "Central policy configuration and safe reload"
  - Blocked by: nothing

## M4 (hours 14-18)

- Team focus (report 1.2): "Run the automated positive/negative/redaction/budget/exploit suite; exercise approval concurrency, source-version changes, policy changes, guard failures and waiting-state recovery. Complete management summary, sanitized audit export, latency summary and judge client."
- Exit condition (report 1.2): "Actual assertion results are retained; missing or failed checks remain visible; judge can change a rule and submit an unexpected input through real controls."
- Report 1.2 sync points (spine): X-89 (complete), X-90 to X-104.
- Sync points needed by the end (spine): X-02 and X-47 to X-57. This side provides X-50 (API-24),
  X-53 (WEB-19) and X-56 (API-23), each for its part; X-47 (API-22) and X-55 (API-21) were
  delivered in M3. It consumes X-48 (WEB-20); X-02 and X-49 reach it through the spine.

SH-31 records the remaining critical checks from this side's evidence and the Go side's. SH-29's
reset is repeated through the interface (WEB-20), and SH-30's deployment procedure brings up the web
and API images with the variables this side added. The optional server-sent events and baseline view
sit here, before the final build's evidence is captured, and are cut first.

### NestJS (report role: Implementer 2)

- [ ] **API-23 · Record organization access evidence on the public path**
  - **Report 1.1 change:** Adds the report and source trail reads; no restricted source content in general views.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 1.5-3 h)
  - Depends on: API-16, API-18, API-19, API-20, API-21 · Needs: X-24, X-34 · Provides: X-56 (part:
    public path)
  - Paths: `apps/api/src` (the specs of the product modules)
  - Work: As the seeded second organization's operator, call every public read and command against
    the first organization's run, action and report, keep the results, and confirm through the run
    view and the events that no runtime state changed. The internal boundary is the Go side's part
    of X-56.
  - Done when: critical check "Organization access" ("Another organization cannot inspect the run,
    approve its action, or claim its resources") has its public-path evidence, "Rejected
    read/command requests and absence of runtime mutation", recorded with the build identifier for
    SH-31.
  - Tests: the database-backed specs of API-16 through X-24, which the evidence quotes; the same
    requests by hand against the running stack, quoted.
  - Report: "Validation plan and evidence matrix" (Proposed critical checks: Organization access;
    Interpreting results honestly: "Test identity and authorization through the public path and the
    internal service boundary; a hidden URL is not a protection")
  - Blocked by: nothing

- [ ] **API-24 · Record field minimization evidence for the activity views**
  - **Report 1.1 change:** Adds the report and source trail reads; no restricted source content in general views.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 1-2.5 h)
  - Depends on: API-14, API-19 · Needs: X-06, X-12, X-34 · Provides: X-50 (part: safe activity
    views)
  - Paths: `apps/api/src` (the activity feed module from API-14)
  - Work: For the protected fields the policy fixture defines (X-06), serialize the events, run view
    and error responses the public API serves for a full run and confirm that none contains a
    protected value; only the reviewer read carries the exact review content. Keep the serialized
    payloads as evidence.
  - Done when: critical check "Field minimization" ("Protected fields are omitted from model-facing
    results and safe activity views") has the event payload part of its evidence recorded for
    SH-31.
  - Tests: specs that scan the serialized responses for the fixture's protected values; the same
    scan on a real run, quoted: `pnpm --filter api run test`.
  - Report: "Validation plan and evidence matrix" (Proposed critical checks: Field minimization);
    "Risk register and scope controls" (Data leakage through secondary views)
  - Blocked by: nothing

- [ ] **API-25 · Optional: serve the activity feed over server-sent events**
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: C · Size: S (estimate 2-4 h)
  - Depends on: API-14 · Needs: X-30 · Provides: X-60 (part)
  - Paths: `apps/api/src` (the activity feed module from API-14), `apps/api/src/main.ts`
  - Work: Only after authenticated polling works, stream the same sanitized events with the same
    organization and object checks, resuming from the cursor. If events stream from Go, the
    gateway's 30 s write timeout ends a long-lived response; how the API's own server timeouts affect
    a long-lived response has not been tested. Under time pressure this task becomes "Dropped: time
    pressure" ("use authenticated polling before adding SSE if necessary").
  - Done when: the activity API serves the "SSE updates" of Figure 1 with the events polling
    returns and "no unrestricted raw payload stream".
  - Tests: specs: the stream carries only contract fields, ends when the session ends and resumes
    from the cursor; another organization is refused: `pnpm --filter api run test`.
  - Report: "Relative implementation milestones and critical dependencies" (Critical path and
    sensible reductions); "Illustrative passport and interface contracts" (Proposed browser and
    runtime operations)
  - Blocked by: `read path`; `decision 3 in docs/product/README.md`

- [x] **API-35 · Serve the security summary**
  - Done (2026-10-04): verified organization members read GET /api/security/summary through the private Go route. The shared SecuritySummary is strictly validated and returned unchanged; a mismatched organization is refused with 503, and missing authentication is refused before Go. Live/fixture verdict labels, purpose usage and timings remain Go facts. Checks: API lint, typecheck and build exited 0; API unit tests: "235 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; host smoke: "28 passed, 0 failed, 5 skipped" (host logs unavailable). Real authenticated API/Go summary returned 200 for the verified organization. Tests use labelled shared fixtures; aggregate reconciliation evidence remains API-37.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: API-16 · Needs: X-85, X-95 · Provides: X-93
  - Paths: `apps/api/src` (the activity module; the architecture proposes ActivityModule)
  - Work: `GET /api/security/summary`: organization-scoped active controls, allowed, blocked and redacted counts, rule hits, security failures, unresolved usage and agent and security consumption, with no raw protected content.
  - Done when: the summary's counts reconcile with the persisted decisions for a seeded run.
  - Tests: `pnpm --filter api run test` with an authorized and a cross-organization caller.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a second disclosure channel)
  - Blocked by: `read path`

- [x] **API-36 · Serve the sanitized audit export**
  - Done (2026-10-04): lead-approved GET /api/security/export?kind=events|assessments&format=json|csv&after=...&limit=... checks reviewer from verified membership before any Go call. Browser after maps to Go cursor; maximum page size is 500. JSON returns exactly the shared Go page; CSV serializes only existing record fields and neutralizes formula triggers (including whitespace/control prefixes), with Go nextCursor in X-Next-Cursor. Invalid cursor ranges, ordering, foreign event organizations, unknown fields and transport errors fail closed. Checks: API lint, typecheck and build exited 0; API unit tests: "264 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; real authenticated API/Go event and assessment exports returned 200 in both JSON and CSV. Tests include missing session, non-reviewer refusal before Go and CSV formula cases. No added record fields or runtime/demo writes; full reconciliation evidence remains API-37.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-4 h, this roadmap's estimate)
  - Depends on: API-16 · Needs: X-79, X-85 · Provides: X-94
  - Paths: `apps/api/src` (the activity module)
  - Work: `GET /api/security/audit/export`: sanitized JSON or CSV decision records for authorized security readers, scoped by organization, time or cursor and run, with explicit range limits. Records carry stable rule and reason codes, model purpose, policy and feed revisions and timings; "CSV text cells are neutralized against spreadsheet formula interpretation". Not a tamper-proof audit system.
  - Done when: an authorized reader exports both formats and an unauthorized or cross-organization request is rejected.
  - Tests: `pnpm --filter api run test`, including a CSV cell starting with `=`.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a second disclosure channel)
  - Blocked by: `read path`

- [x] **API-37 · Prove the audit export and security summary**
  - Done (2026-10-04): sanitized capture docs/evidence/api-audit-export-2026-10-04.json records real authenticated API/Go summary reconciliation with 14 event records and 2 deterministic assessment records. Every decision/assessment group count matches; cursor pages contain no duplicate ids; JSON and CSV counts match and CSV fields exactly match the shared record schemas. Counts were stable before/after capture (not an atomic snapshot). A real judge signature input was denied as signature_match and produced the deterministic assessments. Authorization and CSV-formula regressions run in the existing API unit/database suites, with the Go fixture limitation explicit in the evidence. Checks: API lint, typecheck and build exited 0; API unit tests: "366 passed"; latest `pnpm test:db api` on unchanged code: "59 passed, 0 failed, 0 skipped"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". Evidence contains counts/metadata only, no raw content or credentials. Live-model approval/outbox execution, semantic detection quality, Docker and browser consumption were not verified.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 1-2 h, this roadmap's estimate)
  - Depends on: API-35, API-36 · Needs: X-89 · Provides: X-104
  - Paths: none (tests run through the X-89 suite)
  - Work: Reconcile the summary with the event records, export authorized JSON and CSV, reject a cross-organization request and show that protected text is omitted.
  - Done when: the evidence of X-104 is captured.
  - Tests: the suite cases with the results quoted.
  - Report: "Validation plan and evidence matrix" (Audit export and security summary)
  - Blocked by: nothing

- [x] **API-38 · Serve the live test entry for judge input**
  - Done (2026-10-04): POST /api/control/evaluate validates all three X-91 boundary inputs and forwards with service identity and verified session context only. It returns Go's validated ControlEvaluationResponse unchanged; deny is a real 200 decision, never an issued grant. Unknown fields, malformed boundary combinations and mismatched run responses are refused. Checks: API lint, typecheck and build exited 0; API unit tests: "319 passed"; `pnpm verify`: "6 passed, 0 failed, 0 skipped"; host smoke: "28 passed, 0 failed, 5 skipped". A real run admitted through the same session was evaluated by Go: out-of-scope invoice proposal denied, actionId null, control.evaluated event labelled judge. The draft scripts/judge-client.mjs still needs its owner's X-91 update; this API route is complete independently. Live semantic detection quality was not measured.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: A · Size: S (estimate 2-3 h, this roadmap's estimate)
  - Depends on: API-11, API-16 · Needs: X-91 · Provides: X-106
  - Paths: `apps/api/src` (the runtime facade module from API-11)
  - Work: Figure 2, "Public API and live test entry": an authenticated public operation, named at M0,
    that accepts a judge's ad-hoc interaction, action proposal or tool result for an existing run and
    forwards it to `POST /internal/control/evaluate` with service identity and the verified operator
    context. It returns the recorded decision and the active catalog revision, never issues a grant,
    and exposes no model or tool credentials.
  - Done when: an ad-hoc input submitted through the entry is decided by the same gates as the agent
    path and appears in the decision records and the security summary.
  - Tests: `pnpm --filter api run test` for authentication, organization scope and forwarding errors.
  - Report: "Architecture and chart reading guide" (Figure 2, Figure 12); "Technical architecture and
    service ownership" (Small integration boundary)
  - Blocked by: `decision 7 in docs/product/README.md`

### Next.js (report role: Implementer 1)

- [ ] **WEB-19 · Show unknown usage as uncertain on a real run**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: B · Size: S (estimate 0.5-1.5 h)
  - Depends on: WEB-16 · Needs: nothing · Provides: X-53 (part: the visibly uncertain state)
  - Paths: `apps/web/src/app` (the run page from WEB-06)
  - Work: When the Go side produces a run whose provider usage is missing, open it in the interface,
    confirm that the retained reservation and the estimated cost show as uncertain, and keep the
    capture.
  - Done when: critical check "Unknown usage" ("Missing provider usage does not free unresolved
    spending allowance as though cost were zero") has the interface part of its evidence, the
    "visibly uncertain estimated-cost state", recorded for SH-31.
  - Tests: the run by hand, with the run view and a screenshot quoted; WEB-16's specs.
  - Progress (2026-10-04): `UnknownUsageNotice` in `run-usage-panel.tsx` (lane f3, on
    web/run-panels, shown inside `RunUsagePanel`) appears whenever a request has unknown usage: "Usage
    is uncertain", the shared uncertain label, and the statement that the reserved tokens stay held
    and are counted neither as used nor as zero. WEB-16's specs cover it (a retained reservation with
    no reported amount renders as uncertain, not zero). Not done: opening a real run with missing
    provider usage in the interface and keeping the capture for SH-31 (needs WEB-06's page).
  - Report: "Validation plan and evidence matrix" (Proposed critical checks: Unknown usage); "Atomic
    allowances hard limits and estimated cost"
  - Blocked by: `decision 6 in docs/product/README.md`

- [ ] **WEB-20 · Repeat the workflow through the interface after a reset**
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: B · Size: S (estimate 0.5-1.5 h)
  - Depends on: WEB-18 · Needs: X-48 · Provides: nothing
  - Paths: `apps/web/src/app` (the task setup and run pages)
  - Work: After the documented reset command, sign in again if decision 7's mechanism requires it,
    run the legitimate task a second time through the interface and confirm that no page shows state
    from before the reset.
  - Done when: the M4 exit's "the team can reset fixtures and repeat the workflow" holds through the
    interface, with the second run's counts matching the first as SH-29 compares them.
  - Tests: the second run by hand, quoted.
  - Report: "Relative implementation milestones and critical dependencies" (Proposed 24-hour
    implementation sequence)
  - Blocked by: nothing

- [ ] **WEB-21 · Optional: receive the activity feed over server-sent events**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: C · Size: S (estimate 1-3 h)
  - Depends on: API-25, WEB-09 · Needs: X-60 · Provides: nothing
  - Paths: `apps/web/src/app` (the run page from WEB-06), `apps/web/src/server/upstream-proxy.ts`
  - Work: Only after polling works, read the event stream instead of polling, fall back to polling
    when the stream fails, and keep the same rendering. A forwarded stream is not buffered and is
    not ended by the proxy's 10 s deadline, which today also bounds reading the body (the forwarder
    constraints above). Under time pressure, or when API-25 is dropped, this task becomes
    "Dropped: reason".
  - Done when: the run page receives the "SSE updates" with the events polling shows, and polling
    still works when the stream is unavailable.
  - Tests: specs: the stream and polling produce the same timeline; a dropped stream falls back to
    polling without losing an event; a stream stays open longer than 10 s:
    `pnpm --filter web run test`; a browser check, quoted.
  - Report: "Relative implementation milestones and critical dependencies" (Critical path and
    sensible reductions)
  - Blocked by: `decision 3 in docs/product/README.md`

- [ ] **WEB-22 · Optional: show the demonstration baseline in the interface**
  - **Report 1.1 change:** Beat 2 content per `demonstration baseline`: the note with its label, invoice versions and the empty outbox.
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: C · Size: M (estimate 3-6 h)
  - Depends on: WEB-18 · Needs: nothing · Provides: nothing
  - Paths: `apps/web/src/app` (new page, named at M0 by its owner), `apps/api/src` (a new read,
    named at M0 by its owner)
  - Work: Only if the document owner names the interface as beat 2's surface under
    `demonstration baseline`, show the starting record versions, report count and outbox count
    before execution and again after it, through a read the read path and the roles allow. Until
    then the documented read-only queries of SH-28 are the surface.
  - Done when: beat 2's proof is visible in the interface: "Capture starting record versions, report
    count, and outbox count so later effects can be compared."
  - Tests: API and web specs for the read and its rendering; a before and after comparison around a
    denied action, quoted.
  - Report: "Live demonstration storyboard and proof checks" (Proposed demo sequence, beat 2); "Risk
    register and scope controls" (Demo proves logs, not prevention)
  - Blocked by: `demonstration baseline`; `read path`

- [x] **WEB-30 · Build the security posture dashboard**
  - **Done (2026-10-04, web/security):** `/security` (`app/security/page.tsx`, `components/security/*`) renders the real `GET /api/security/summary` through the same-origin route `app/api/security/summary/route.ts` and a typed client (`lib/clients/security-client.ts`) that checks the response against the contract's shape before rendering and maps 401, 403, 5xx, network, timeout and a body outside the contract to fixed messages (no upstream text is shown). It shows decision events by type, decision, reason and input source (agent run versus judge probe); the allowed, blocked, redacted and sent-to-review counts split by source; deterministic blocks; security evaluation failures; assessments by control with live, fixture, no-model-call and check-did-not-complete kept apart (an errored check is never counted as a live verdict; lane c1's `VerdictSourceLabel` and `LABELS` supply the fixture wording); runs by status; model usage by purpose with unknown usage and held tokens kept visible; and measured phase durations, with the statement that no cost is shown or estimated. Not shown because the summary does not carry it: the active controls and policy revision (WEB-29) and matched rule names (those are in the audit export). Checks: `pnpm --filter web run lint` exit 0 (0 errors; 3 warnings, all in `login-form.tsx` and `task-form.tsx`), `typecheck` exit 0, `test` exit 0 (111 passed: view model against the contract fixture and synthetic counts, client status mapping and shape guard, route handlers against a real stub upstream). Live, on the local stack (web 3130, API 3131, gateway 8130, signed in as the development demo operator through the real form), in headless Chromium: after a run and three judge probes through `POST /api/control/evaluate`, the page showed Blocked 3 (all judge probes), Allowed 1 (the run's `run.queued`), one deterministic block, one security evaluation failure (no model configured, so the semantic check failed closed), one assessment with no verdict, and the same rows as the served summary; at 390 px width the page has no horizontal scroll; 503 shows a retry that recovers, 401 offers sign-in, 403 and a malformed body are refused instead of rendered. Checks per commit: see the commit message.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 3-5 h, this roadmap's estimate)
  - Depends on: API-35 · Needs: X-93 · Provides: nothing
  - Paths: `apps/web`
  - Work: Management aggregates for allowed, blocked and redacted interactions, active controls, rule hits, security failures, unresolved usage and agent and security consumption, with measured durations kept apart from cost estimates. Show only real responses.
  - Done when: the dashboard matches the summary for a seeded run.
  - Tests: `pnpm --filter web run test` with the contract fixtures.
  - Report: "Durable state idempotency audit and uncertain outcomes" (Evidence without creating a second disclosure channel); "Delivery scope and six person ownership" (Implementer 1)
  - Blocked by: nothing

- [x] **WEB-31 · Offer the authorized audit export**
  - **Done (2026-10-04, web/security):** `/security/export` (`app/security/export/page.tsx`, `components/security/audit-export.tsx`) offers the sanitized export of `GET /api/security/export` through the same-origin route `app/api/security/export/route.ts`, which streams a CSV body (the buffering proxy refuses a non-JSON body as 502). The web has no role information of its own (the API has no `/api/auth/me`), so the page asks the real export route whether the viewer may export (one record, discarded): 403 shows the refusal and no control at all, 401 offers sign-in, anything else shows that authorization could not be confirmed and no form. An authorized reviewer chooses the records (decision events or control assessments), the cursor to start after and the page size (1 to 500, validated field by field), reads a JSON page (record count, next cursor, an indented preview) and downloads JSON or CSV; JSON is the page already read, CSV re-requests the same page as CSV, and both are saved byte for byte as the server sent them. Checks: `pnpm --filter web run lint` exit 0 (0 errors; the same 3 warnings in other lanes' files), `typecheck` exit 0, `test` exit 0 (114 passed: query validation, file naming, the authorization probe, JSON and CSV page handling, content-type checks, the preview, and the route handlers including CSV streaming and the 403 pass-through). Live, in headless Chromium against the local stack as the seeded development demo operator (roles operator and reviewer): 5 events read, next cursor shown, `security-events-from-start.json` saved (3542 bytes, parses, equal to the bytes the route serves) and `security-events-from-start.csv` saved (header plus 5 rows); a bad limit disables the button and both bad fields explain themselves at once; no horizontal scroll at 390 px. The unauthorized path was exercised by answering the page's own request with 403 (no non-reviewer account is seeded): the refusal showed, the read button and every input were absent and no upstream text appeared; 401, 503, a 403 after authorization and a body that is not a page were each handled; the API's own refusal of a non-reviewer is covered by `audit-export.controller.spec.ts` ("refuses a non-reviewer before contacting Go"). Not done: the CSV response's `X-Next-Cursor` header is not forwarded by the shared proxy (it forwards only content-type, request id and cookies), so the next cursor comes from the JSON page. Checks per commit: see the commit message.
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: A · Size: S (estimate 1-2 h, this roadmap's estimate)
  - Depends on: API-36 · Needs: X-94 · Provides: nothing
  - Paths: `apps/web`
  - Work: Let an authorized security reader choose the range and format and download the export through the server-side proxy; hide the control from others.
  - Done when: an authorized reader downloads JSON and CSV and an unauthorized one sees no control and gets a refusal.
  - Tests: `pnpm --filter web run test`.
  - Report: "Durable state idempotency audit and uncertain outcomes"
  - Blocked by: nothing

## M5 (hours 18-21)

- Team focus (report 1.2): "Polish comprehension and error states; rehearse the complete story; capture evidence from the final build. Run live semantic fixtures and performance measurements on the presentation machine; rehearse configuration changes and ad-hoc input."
- Exit condition (report): "A reviewer can understand the task boundary, attempted action, decision,
  actual effect, and limitation without narration filling gaps."
- Report 1.2 sync points (spine): X-105.
- Sync points needed by the end (spine): X-58 and X-59 (X-60 and X-61 are optional). This side
  provides X-58 (WEB-24); API-27 and WEB-26 consume X-59 in M6.

SH-32 recaptures the evidence from the final build (X-59) and SH-33 rehearses the storyboard (it
needs X-58). A change that lands after SH-32 needs its evidence recaptured.

### NestJS (report role: Implementer 2)

- [x] **API-26 · Clean up the API's error states**
  - Done (2026-10-04): error-code allowlist consumes all 31 shared X-13 codes; unknown upstream codes are never relayed. General 5xx logs contain only correlation/status metadata, not exception messages, untrusted exception names or stacks that can carry credentials/review content. Session lookup dependency errors now return 503 while invalid/expired credentials remain 401. All facades preserve Go status or return fail-closed transport errors; command timeouts remain unconfirmed. Checks: API lint, typecheck and build exited 0; API unit tests: "366 passed"; `pnpm test:db api`: "59 passed, 0 failed, 0 skipped"; `pnpm verify`: "6 passed, 0 failed, 0 skipped". Host log scan: service token, operator signing key, demo password and shared restricted-review fixture text absent. Tests cover every shared reason code and secret-bearing exceptions in both response/log output. Live model execution quality is outside this evidence.
  - **Report 1.2 change:** Uses the 20 proposed reason codes.
  - **Report 1.1 change:** Uses the 13 proposed reason codes.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 1-2 h)
  - Depends on: API-20, API-21 · Needs: X-13 · Provides: nothing
  - Paths: `apps/api/src/common/all-exceptions.filter.ts`,
    `apps/api/src/common/all-exceptions.filter.spec.ts`, `apps/api/src` (the product modules)
  - Work: Go through the failure paths of every product route: each answers with the shared
    envelope, the adopted reason code and a safe message; 5xx messages stay generic; a database or
    Go outage answers 503 or a proxy code, never a default success; a provider failure Go recorded
    reaches the run view as that failure. No stack trace, payload, token or connection string
    appears in a response or a log line.
  - Done when: "Responses should include a stable reason code, a safe operator message, and relevant
    action or run references. Detailed sensitive payloads belong in restricted review storage rather
    than general error text." holds for every product route.
  - Tests: `all-exceptions.filter.spec.ts` extended with the product reason codes; a spec per route
    for its upstream-down case; a log scan of a full run for the token and the review content:
    `pnpm --filter api run test`.
  - Report: "Illustrative passport and interface contracts" (Decision and error semantics);
    "Relative implementation milestones and critical dependencies" (Proposed 24-hour implementation
    sequence)
  - Blocked by: nothing

### Next.js (report role: Implementer 1)

- [x] **WEB-23 · Polish the error states**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: B · Size: S (estimate 2-4 h)
  - Depends on: WEB-14, WEB-15, WEB-16 · Needs: X-13 · Provides: nothing
  - Paths: `apps/web/src/lib/service-checks.ts`, `apps/web/src/lib` (the client from WEB-03),
    `apps/web/src/app` (the product pages), `packages/ui/src/components/error-state.tsx`
  - Work: Give every failure its own truthful state: the proxy codes (`upstream_unreachable`,
    `upstream_timeout`, `upstream_invalid_response`, `configuration_error`), the API's envelope and
    reason codes, an unconfirmed command, an expired session and a provider failure. A provider
    failure shows the actual failure state, apart from the working gateway demonstration.
  - Done when: "On provider failure, show the actual failure state and distinguish the working
    gateway demonstration from unavailable live model behavior" holds, and no failure shows as a
    success or as an empty page.
  - Tests: specs: each failure kind maps to its own state and none maps to a success state; a browser
    check with the API stopped and with the provider failing, quoted: `pnpm --filter web run test`.
  - Report: "Live demonstration storyboard and proof checks" (Reliable demonstrations without
    invented behavior); "Risk register and scope controls" (Provider instability or unsuitable
    output); "Relative implementation milestones and critical dependencies" (Proposed 24-hour
    implementation sequence)
  - Blocked by: nothing
  - Completed (2026-10-04): lane web/reports, in new files. `lib/errors/failure.ts` (`classifyFailure`, `failureFromReason`) maps every failed same-origin request to its own state: offline, client timeout, cancelled, an unreadable response, an expired session, forbidden, not found, conflict, bad request, too large, the proxy codes (`configuration_error`, `upstream_unreachable`, `upstream_timeout`, `upstream_invalid_response`), the API's gateway-side codes (`upstream_unavailable`, `outcome_unconfirmed`, `timeout`, ...), a server error, and every X-13 reason code (`lib/errors/reason-failures.ts`: words for all 31 codes, the gateway's fixed safe texts, checked against the contracts enum and free of run-status words). Each state has its own title and fixed words (never a server message), says where it failed (browser, web server, API, gateway, live model, session, request), and offers retry, sign-in or nothing. A command that got no answer is unconfirmed: no retry, "check the run before submitting it again"; a failure that certainly did not run (401, 404, 400, unreachable API) is not. A provider failure (`outcome_unknown`, `security_evaluator_unavailable`, `model_not_allowed`) sits on "The live model" with the note that the gateway reported it itself and deterministic checks and labelled replays do not need the model. `components/errors/failure-state.tsx` (`FailureState`) renders the state (index: `@/components/errors`); an expired session links to `/login?callbackUrl=<this page>`. Applied in the report page (`ReportPanel`), which keeps its own words only for "Report not found" and "Report cannot be shown". Tests (195 in the web package, 46 new): every reason code has words; each failure kind maps to its own state and title, none reads as a success; a server message never reaches the page; commands versus reads; overrides; the component's sign-in link, retry, unconfirmed and live-model notes. Browser checks, quoted, on the report page: with the API stopped (the web server running) the state was "The API could not be reached" (upstream_unreachable, Where: The API, request id, Try again); with a stale session cookie "Your session has ended" with a Sign in link to `/login?callbackUrl=%2Fruns%2F…%2Freports%2F…`; with the browser request aborted (a simulated disconnect) "You appear to be offline", and Try again recovered the report. Not checked in a browser: a provider failure, which needs the run page being rebuilt (its states are specified and tested here); the other pages adopt `FailureState` by importing it from `@/components/errors`.

- [ ] **WEB-24 · Make every demonstration beat observable for the rehearsal**
  - **Report 1.2 change:** Twelve beats in three evidence segments.
  - **Report 1.1 change:** Nine beats.
  - Owner: Web + API implementer (report roles: Implementer 1, interface, and Implementer 2, application API) · Tier: B · Size: S (estimate 1-3 h)
  - Depends on: WEB-13, WEB-17, WEB-18, WEB-23 · Needs: X-16 · Provides: X-58
  - Paths: `apps/web/src/app` (the product pages)
  - Work: Walk the twelve beats of the demo specification (X-16) on the final build and fix what the
    interface does not show: the passport, the attempted action and its decision, the actual effect
    and the limitation, with the labels for the simulated outbox, replays and estimated cost. Beat
    2's baseline uses SH-28's documented read-only queries unless `demonstration baseline` names
    another surface.
  - Done when: someone who did not build the screens can observe the M5 exit: "A reviewer can
    understand the task boundary, attempted action, decision, actual effect, and limitation without
    narration filling gaps."
  - Tests: a walk-through of the twelve beats on the final build with screenshots, quoted.
  - Report: "Live demonstration storyboard and proof checks" (Proposed demo sequence); "Relative
    implementation milestones and critical dependencies" (Proposed 24-hour implementation sequence)
  - Blocked by: nothing

- [ ] **WEB-25 · Optional: polish comprehension beyond the exit condition**
  - **Report 1.1 change:** Quote: "A polished dashboard would help explain the boundary, while stored report lineage, a denied export, and the resulting simulated outbox would supply concrete evidence."
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: C · Size: S (estimate 1-3 h)
  - Depends on: WEB-24 · Needs: nothing · Provides: nothing
  - Paths: `apps/web/src/app` (the product pages), `packages/ui/src/components`
  - Work: Only after X-58, improve wording, layout or visual detail that the exit condition does not
    need, and only if SH-32 can still recapture the evidence afterwards. Cut first under time
    pressure: the task then becomes "Dropped: time pressure".
  - Done when: the change is in the build whose evidence SH-32 records, and the M5 exit still holds.
  - Tests: `pnpm --filter web run test`; WEB-24's walk-through repeated, quoted.
  - Report: "Design decision record" ("The team should first remove optional breadth when delivery
    is at risk, while keeping the checks required to substantiate the control-layer proposition");
    "Positioning differentiation and credible product claims" ("A polished dashboard could help
    explain the result, but it would not substitute for proving that a disallowed action failed to
    reach the executor or create application state")
  - Blocked by: nothing

## M6 (hours 21-24)

- Team focus (report 1.2): "Freeze the submission build and configuration, fix critical faults before the confirmed deadline, capture test/telemetry evidence, finalize the maximum 10-slide PDF and HackTribe fields, and submit. Preserve the submitted commit and artifact checksums."
- Exit condition (report): "Submission checklist is complete; the demonstration matches the
  submitted build and its documented limitations."
- Sync points needed by the end (spine): X-62. This side provides it with the Go side (API-27 and
  WEB-26 for this side's part, both consuming X-59).

Both implementers take part in SH-34: no feature lands after the freeze, and each failing critical
check of this side is fixed or its supported behaviour and claims are narrowed. SH-35 assembles the
handoff from X-62, and RS-09 submits.

### NestJS (report role: Implementer 2)

- [ ] **API-27 · Supply the NestJS part of the technical handoff**
  - **Report 1.2 change:** Adds the policy file, catalog reload, security summary and audit export.
  - **Report 1.1 change:** Adds the report templates and projection rules.
  - Owner: Web + API implementer (report role: Implementer 2, application API) · Tier: B · Size: S (estimate 0.5-1.5 h)
  - Depends on: SH-34 · Needs: X-59 · Provides: X-62 (part)
  - Paths: `apps/api/README.md`, `packages/contracts/README.md`, `docs/architecture.md`,
    `README.md`, `docs/setup.md`
  - Work: Write the NestJS text: setup and variables, the product modules for "Product modules" in
    `docs/architecture.md`, the public operations and the contracts in `packages/contracts`, the
    authentication mechanism labelled as a development demonstration, and this side's known
    limitations (for example polling instead of server-sent events if API-25 was dropped).
    `apps/api/README.md` and `packages/contracts/README.md` are this role's own; integration places
    the rest in `README.md` and `docs`.
  - Done when: SH-35 can include the NestJS part of "setup instructions, architecture and
    boundaries, synthetic-data reset, tool contracts, policy fixture, known limitations, dependency
    disclosures where required, and the critical-check outcomes", matching the submitted build.
  - Tests: a teammate follows the NestJS setup on a clean checkout, quoted.
  - Report: "Research documentation and submission workflow" (From requirements to verified
    presentation)
  - Blocked by: nothing

### Next.js (report role: Implementer 1)

- [ ] **WEB-26 · Supply the Next.js part of the technical handoff**
  - Owner: Web + API implementer (report role: Implementer 1, interface) · Tier: B · Size: S (estimate 0.5-1.5 h)
  - Depends on: SH-34 · Needs: X-59 · Provides: X-62 (part)
  - Paths: `apps/web/README.md`, `packages/ui/README.md`, `docs/architecture.md`, `README.md`
  - Work: Write the Next.js text: the browser path and its forwarding rules, the product pages and
    their labels, the sign-in as a development demonstration, what the UI tests cover (pure-function
    tests and quoted browser checks, no component or browser end-to-end tests unless they were
    added), and this side's known limitations. `apps/web/README.md` and `packages/ui/README.md` are
    this role's own; integration places the rest.
  - Done when: SH-35 can include the Next.js part of "setup instructions, architecture and
    boundaries, synthetic-data reset, tool contracts, policy fixture, known limitations, dependency
    disclosures where required, and the critical-check outcomes", matching the submitted build.
  - Tests: a teammate follows the web setup on a clean checkout, quoted.
  - Report: "Research documentation and submission workflow" (From requirements to verified
    presentation)
  - Blocked by: nothing

## Coverage

### Critical checks, MVP requirements and the vertical slice

MVP requirements (table "Proposed MVP requirements and acceptance evidence", "Functional
requirements MVP boundary and deferred scope"):

| Requirement                 | This side's part                                                                                                                            | Tasks                                  |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------- |
| Trusted admission           | Shared: authenticates, checks membership and forwards without narrowing; shows the rejection and requires explicit resubmission. Go admits. | API-06, API-08, API-11, WEB-05, WEB-11 |
| Task-scoped actions         | Go side (the action gate). The interface shows each denial with its rule.                                                                   | WEB-09                                 |
| Exact-action approval       | Shared: the reviewer-only payload, the decision for the stored action only, the preview of the stored action. Go binds and consumes.        | API-19, API-20, WEB-14                 |
| Durable execution           | Go side. The interface reads the waiting state from the server, so it survives a reload or a worker restart.                                | WEB-14                                 |
| Bounded usage               | Go side. The interface shows limits, usage and reservations.                                                                                | WEB-16                                 |
| Safe outcomes and retries   | Go side. The interface shows an unknown outcome as operator attention required.                                                             | WEB-10                                 |
| Data minimization           | Shared: the activity views relay only the safe event contract and the review payload stays with reviewers; model context is Go's.           | API-14, API-19, API-24                 |
| Authorized visibility       | This side, on the public path; Go checks its own reads and commands.                                                                        | API-13, API-14, API-16, API-23         |
| Bounded recovery            | Go side. The interface shows the correction route and the count.                                                                            | WEB-09                                 |
| Cancellation and revocation | Shared: forwards cancellation and writes revocations; Go enforces both.                                                                     | API-21, API-22, WEB-15                 |

Critical checks (table "Proposed critical checks", "Validation plan and evidence matrix"):

| Check                          | Evidence provider (spine)                  | Tasks here                                   |
| ------------------------------ | ------------------------------------------ | -------------------------------------------- |
| Legitimate task                | Go side and this side (X-44)               | WEB-18                                       |
| Resource boundary              | Go side (X-37)                             | none; WEB-09 shows the denial                |
| Destination boundary           | Go side (X-38)                             | none; WEB-09 shows the denial                |
| Field minimization             | Go side and this side (X-50)               | API-24                                       |
| Approval integrity             | Go side (X-45)                             | none; WEB-14 shows a changed action          |
| Approval replay                | Go side (X-51)                             | none                                         |
| Budget concurrency             | Go side (X-52)                             | none                                         |
| Unknown usage                  | Go side and this side (X-53)               | WEB-16, WEB-19                               |
| Waiting-state restart          | Go side (X-54)                             | none; WEB-14 reads the waiting state from Go |
| Cancellation and expiry        | Go side and this side (X-55)               | API-21, WEB-15                               |
| Organization access            | This side (public path) and Go side (X-56) | API-16, API-23                               |
| Database execution transaction | Go side (X-57)                             | none                                         |

The smallest credible vertical slice and the four protections ("Threat model limits and unresolved
design choices"; "Relative implementation milestones and critical dependencies"):

| Element                                  | Tasks                                                     |
| ---------------------------------------- | --------------------------------------------------------- |
| "one authorized invoice/report workflow" | API-11, WEB-05, WEB-06, WEB-12, WEB-14, WEB-18            |
| "one denied out-of-scope operation"      | WEB-09 (Go denies it)                                     |
| "one exact-action approval"              | API-19, API-20, WEB-14                                    |
| "one limit-triggered stop"               | WEB-17 (Go stops the run)                                 |
| service-side authorization               | API-03, API-06, API-08, API-10, API-16                    |
| scope enforcement                        | Go side; API-11 forwards the request without narrowing it |
| pre-dispatch limits                      | Go side; WEB-16 and WEB-17 show them                      |
| exact-action approval                    | API-20, WEB-14                                            |

Verification priorities ("Threat model limits and unresolved design choices"):

| Priority                                                                                                                         | Tasks                                    |
| -------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------- |
| "Verify that scope rejection is explicit and the revised request must be resubmitted."                                           | API-11, WEB-11                           |
| "Verify that every adapter checks arguments and resource relationships, not only the tool name."                                 | none (Go side)                           |
| "Verify that approval cannot enlarge the passport and changed content requires new review."                                      | none (Go side); WEB-14 shows the outcome |
| "Verify that two concurrent attempts cannot consume one approval or allowance twice."                                            | none (Go side)                           |
| "Verify that cancellation and revocation prevent future dispatch after a review wait."                                           | API-21, API-22                           |
| "Verify that blocked operations create no synthetic outbox effect and successful retries do not duplicate it."                   | none (Go side)                           |
| "Verify that spending uncertainty, simulated effects and unimplemented features remain visible in the report and demonstration." | WEB-07, WEB-13, WEB-16, WEB-19           |

Demonstration beats (table "Proposed demo sequence", "Live demonstration storyboard and proof
checks"):

| Beat                        | Tasks                                                          |
| --------------------------- | -------------------------------------------------------------- |
| 1. Establish the job        | WEB-05, WEB-08                                                 |
| 2. Establish the baseline   | none: SH-28's documented read-only queries; optional WEB-22    |
| 3. Show useful work         | WEB-09, WEB-12                                                 |
| 4. Introduce hostile input  | WEB-09 shows what follows; the hostile note is SH-25's fixture |
| 5. Prove the boundary       | WEB-09 (the denial and its rule; the Go side proves no effect) |
| 6. Continue productively    | WEB-09 (the correction count), WEB-10                          |
| 7. Review exact effects     | API-19, API-20, WEB-14                                         |
| 8. Show controlled stopping | WEB-16, WEB-17                                                 |

### Components, journeys and browser operations

Diagram 1 (section 1 of `docs/product/project-architecture.md`), the components in the WEB and NEST
groups and the edges that touch them:

| Component or edge                                                  | Tasks                                                                                        |
| ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| UI: task setup, policies, approvals, live activity                 | WEB-05; WEB-08 (the policy as admitted, no policy editor); WEB-14; WEB-06, WEB-09 and WEB-21 |
| API: public API, authentication and organization access            | API-03, API-06, API-07, API-08, API-16                                                       |
| CONFIG: users, task templates, versioned policies and tool catalog | API-04, API-05, API-12, API-22                                                               |
| FACADE: runtime facade for start, approve, reject and cancel       | API-09, API-10, API-11, API-20, API-21                                                       |
| FEED: activity API and SSE, authorized and sanitized events        | API-14, API-24; API-25 (server-sent events, Tier C)                                          |
| USER -> UI                                                         | WEB-04, WEB-05                                                                               |
| UI -> API ("Authenticated HTTPS")                                  | WEB-02, API-07                                                                               |
| API -> CONFIG, API -> FACADE, API -> FEED                          | API-08, API-12; API-11, API-20, API-21; API-14                                               |
| CONFIG -> APPDB ("Owns application writes")                        | API-04, API-05, API-22 (the migrations are SH-15's and, for the revocation records, SH-38's) |
| FACADE -> INTERNAL ("Private authenticated API")                   | API-09, API-10 (X-26, with X-27 from the Go side)                                            |
| FEED -> RUNDB ("Read authorized event view")                       | API-13, API-14, if `read path` chooses runtime views                                         |
| FEED -> UI ("SSE updates")                                         | API-25, WEB-21 (Tier C); authenticated polling in WEB-06 and WEB-09                          |

Diagram 2 (section 2 of `docs/product/project-architecture.md`), the steps this side carries or shows:

| Step                                                        | Tasks                               |
| ----------------------------------------------------------- | ----------------------------------- |
| START: the operator creates a task                          | WEB-05                              |
| AUTH: NestJS authenticates and checks membership            | API-06, API-08                      |
| AUTH -> ADMIT: the request reaches Go with operator context | API-10, API-11                      |
| VALID -> REJECT: rejection with an explanation              | API-11, WEB-11                      |
| USAGE, MFAIL: usage and retained reservations               | WEB-16, WEB-19                      |
| WAIT -> HUMAN: the interface learns of the pending action   | WEB-06, WEB-10, API-19, WEB-14      |
| HUMAN -> APPROVED: the reviewer's decision reaches Go       | API-20, WEB-14                      |
| APPROVED -> STOP ("Cancel") and cancellation at any step    | API-21, WEB-15                      |
| STOP, FAILED, COMPLETE, ATTENTION: terminal states          | WEB-10, WEB-12 (the report), WEB-17 |

Browser operations (table "Proposed browser and runtime operations", "Illustrative passport and
interface contracts"):

| Operation                                                                        | Tasks                                                     |
| -------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `POST /api/runs`                                                                 | API-11; WEB-05, WEB-11                                    |
| `POST /api/runs/{id}/cancel`                                                     | API-21; WEB-15                                            |
| `POST /api/actions/{id}/approval`                                                | API-20; WEB-14                                            |
| `GET /api/runs/{id}`                                                             | API-13; WEB-06, WEB-08, WEB-10, WEB-16                    |
| `GET /api/runs/{id}/events`                                                      | API-14; WEB-06, WEB-09; server-sent events API-25, WEB-21 |
| No listed operation: `form options`, `review payload read`, `stored report read` | API-12, API-19, API-18; WEB-05, WEB-14, WEB-12            |

User journeys ("Users operating model and proposed user journeys"):

| Journey                                   | Tasks                                                                                                               |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| Journey 1 create and delegate a task      | API-06, API-08, API-11, API-12, WEB-05, WEB-06, WEB-08, WEB-09, WEB-10, WEB-11, WEB-16                              |
| Journey 2 review an exact outbound effect | API-19, API-20, WEB-10, WEB-14                                                                                      |
| Journey 3 recover cancel or investigate   | WEB-09 (rule, correction route, trace), WEB-10 (stop and attention), API-21, WEB-15 (cancel), API-16 (audit reader) |

User responsibilities (table "Proposed user responsibilities"): operations user API-11, API-13,
API-16, API-21; authorized reviewer API-16, API-19, API-20, WEB-14; policy administrator API-16,
API-22 (no policy editor); audit reader API-16, WEB-09; tool developer: none here, the registered
typed adapters are Go-side work.

### Sync points

Provided by this side. This side has delivered its share of a sync point when every listed task is
done; a sync point with two providers in the spine is reached only when both have delivered:

| Sync point | Tasks                                                                           | Needed by       |
| ---------- | ------------------------------------------------------------------------------- | --------------- |
| X-17       | API-04, API-05                                                                  | M1              |
| X-25       | API-12, only if `form options` chooses app records                              | M1              |
| X-26       | API-10                                                                          | M1              |
| X-31       | API-07, API-11, API-13, API-14, WEB-02                                          | M1              |
| X-43       | API-20, WEB-14 (approval part); API-21, WEB-15 (cancel part)                    | M3              |
| X-44       | WEB-18 (this side's part: run start, approval, event display)                   | M3              |
| X-46       | WEB-17 (this side's part: the terminal reason)                                  | M3              |
| X-47       | API-22 (the NestJS write path; the migration and the Go read grant are SH-38's) | M4              |
| X-50       | API-24 (the safe activity views)                                                | M4              |
| X-53       | WEB-19 (the visibly uncertain state)                                            | M4              |
| X-55       | API-21 (the cancel command)                                                     | M4              |
| X-56       | API-23 (the public path)                                                        | M4              |
| X-58       | WEB-24                                                                          | M5              |
| X-60       | API-25 (optional, Tier C)                                                       | None (optional) |
| X-62       | API-27, WEB-26 (this side's part)                                               | M6              |
| X-66       | API-22                                                                          | M3              |

Consumed by this side: X-03 (API-03, API-11, WEB-02, WEB-06), X-06 (API-12, API-24, WEB-05), X-07
(API-11, WEB-03), X-08 (API-13, WEB-08), X-09 (API-19, WEB-14), X-10 (API-20, WEB-14), X-11
(API-13, WEB-03, WEB-10, WEB-16), X-12 (API-14, API-24, WEB-03, WEB-09), X-13 (API-09, API-11,
API-20, API-26, WEB-03, WEB-11, WEB-23), X-14 (API-10), X-15 (API-11), X-16 (WEB-13, WEB-24), X-18
(API-06, API-08, API-22), X-21 (API-12), X-22 (API-06, API-08, WEB-04), X-23 (API-06, API-10), X-24
(API-04, API-05, API-06, API-08, API-12, API-13, API-14, API-16, API-17, API-22, API-23), X-25
(API-12 unless it provides it, WEB-05), X-27 (API-10), X-28 (API-11), X-29 (API-13), X-30 (API-14,
API-25), X-32 (API-15), X-34 (API-16, API-23, API-24, WEB-17, WEB-18), X-35 (API-17), X-36 (WEB-09,
WEB-13), X-40 (API-20), X-41 (API-19, WEB-14), X-42 (API-21), X-48 (WEB-20), X-59 (API-27, WEB-26),
X-60 (WEB-21) and X-64 (API-18, WEB-12). X-01, X-02, X-05 and X-49 reach this side through the
spine: the plan assumes the starter may be used (X-01, so a negative answer changes it), SH-09's
container run checks this side's images (X-02), the requirements sheet feeds the demo specification
(X-05 into X-16), and the deployment procedure is SH-30's.

**Totals (estimates, not a schedule).** 70 tasks; their ranges add up to 113.5-237.5 h, 175.5 h at the
midpoints (Tier A 138.5 h, Tier B 25.5 h, Tier C 11.5 h). Report 1.1 added API-28 to API-30, WEB-27 and
WEB-28 (14 h at the midpoints) and report 1.2 added API-31 to API-38 and WEB-29 to WEB-32 (34 h).
Per milestone: P 0.5-1 h, M0 6-11 h, M1 35.5-75 h, M2 27-57.5 h, M3 18-36.5 h, M4 20.5-41.5 h, M5
5-12 h, M6 1-3 h; the SH tasks the web + API implementer owns by default (SH-01 to SH-03, SH-11, SH-39,
SH-42 and the migrations and seeds) come on top. Tier A alone is 91.5-185.5 h, now including M4 work,
against at most 24 person-hours for one implementer working all 24 hours without a break. M1 holds
35.5-75 h (Tier A 34-71.5 h; Tier B 1.5-3.5 h in API-15 and WEB-07; no Tier C) against 4
person-hours in hours 2-6. Removing every Tier B and Tier C task does not close the gap; narrowing
inside Tier A is the team decision described in the spine's "Tiers".

### Report items that need no task in this file

The contents of the frozen contracts are not built here: the passport fields, the tool argument
boundaries and the other contracts to freeze first are frozen by SH-10 and landed by SH-11 in the
spine (X-08 and X-09), and the Go side checks the tool arguments. This file shows them: WEB-08
(passport summary), WEB-14 (the stored action under review), WEB-16 (limits and usage) and WEB-09
(the correction count). API-02 and WEB-01 check the contract examples at M0, and API-08 supplies
the verified operator context the passport records.

During planning on 2026-10-03 the report's NestJS and Next.js obligations were listed and mapped to
the tasks in this file. That working list is not kept in the repository; each task's "Report" field
names its source in the report, and the tables above map the report's checks, requirements,
components, journeys and operations to tasks.
