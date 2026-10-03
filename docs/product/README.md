# Product design: Task Passport

Task Passport is the team's entry for the Goldman Sachs AI Control Layer challenge at HackYeah 2026.
This folder holds its product definition. Everything here is a proposed design, not a record of
implemented behaviour; [docs/architecture.md](../architecture.md) describes what exists in the
repository. Owner: the document owner. No researcher is assigned yet; the lead acts as document owner
until the team assigns the role.

| File                                                                   | Content                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [task-passport-project-report.docx](task-passport-project-report.docx) | Project report, version 1.2, "Official requirements and hybrid security controls", 3 October 2026: everything in version 1.1 (report provenance with two classifications and two fixed templates, the denied export and the vendor continuation) plus the deliverables of the challenge criteria: hybrid deterministic and semantic security checks, editable `policy.yaml` with validated live reload into one control catalog, a local-model path, a versioned attack-signature feed, an executable automated test suite, a security summary and audit export, performance telemetry, a small adapter contract and judge client, twelve demonstration beats and 28 critical checks. |
| [project-architecture.md](project-architecture.md)                     | Architecture specification: system architecture, task execution flow and report information flow as Mermaid source, service ownership, routes, modules, data ownership, interfaces, tools and key checks. Proposed design; kept verbatim (listed in `.prettierignore`).                                                                                                                                                                                                                                                                                                                                                                                                               |
| [competition-rules.pdf](competition-rules.pdf)                         | Terms and conditions of the "AI Control Layer" competition. The PDF prints the start and end times as 11:00 PM; the team lead confirmed that both are 11:00 AM: start no earlier than 11:00 on 3 October 2026, submission no later than 11:00 on 4 October 2026. `AGENTS.md`, "Current phase", summarizes the rules.                                                                                                                                                                                                                                                                                                                                                                  |
| [competition-criteria.pdf](competition-criteria.pdf)                   | The detailed challenge criteria ("AI Control Layer", supplied as `CRIETRIA AI Control Layer.pdf`; the report cites it as [S10]): the control layer, centralized policy engine, deterministic and semantic controls, budget governance, historical attack mitigation, security reporting and a self-testing suite; judges run the tests, submit ad-hoc prompts, edit configuration and inspect telemetry; no paid subscriptions are provided. Its weights (30/20/20/15/15) differ from the rules (30/20/20/20/10) for the self-testing suite and practicality (`scoring weights`).                                                                                                     |

The report lists its internal sources as `project-architecture.md`, `project-architecture.mmd`,
`task-execution-flow.mmd`, `report-information-flow.mmd`, `hybrid-security-flow.mmd` and
`CLAUDE_SETUP_PROMPT.md`. The current diagrams are the report's twelve figures, embedded in the docx as images (Figures 1 to
3 system architecture, 4 to 10 task execution flow with the tool-result security check, 11 report
information flow, 12 central catalog and hybrid controls). Of the source files only
`project-architecture.md` is in the repository; it embeds the Mermaid sources of version 1.1 and none
of the report 1.2 additions, so where it differs from the figures the open item `architecture
specification version` records it. The SVG exports of
version 1.0 were removed. The report says "The original Mermaid sources and this report describe the
same updated design"; the open items below record where they still differ.

The report's example limits (call counts, attempts, expiry) and example records are illustrative
values, not requirements. Its validation targets are not test results. The approved vendor field list
is a team policy decision ("The final approved field list would be a team policy decision, not a
sponsor requirement"), fixed at the M0 freeze. The same holds for the report 1.2 example `policy.yaml` (24 model calls with
12 agent and 12 security calls, 20,000 tokens, 20 seconds per request, two concurrent local
requests, 15-minute expiry, a semantic threshold of 0.75): "illustrative team settings, not sponsor
requirements or measured performance".

## Team

One Go implementer, one web + API implementer, and the lead, who helps both sides when needed. The
report's six roles are responsibility areas mapped onto these people; the mapping is in
[AGENTS.md](../../AGENTS.md), "Repository map and ownership". Two staffing items are open: who holds
the researcher, document owner and presenter role, and confirmation of the default shared-track
assignment (both listed under "Open items" below).

## Ownership

- **NestJS** owns users, organizations, memberships, task templates and versions, versioned policies,
  versioned source and template policy, fixed report-template and projection definitions, the tool
  catalog, explicit revocations, the public API, the authorized activity feed and the runtime facade
  toward Go. Report 1.2 adds the import and validation of `policy.yaml` and the signature feed into
  immutable control-catalog revisions, the active-revision pointer, the authenticated reload, the
  organization-scoped security summary and the sanitized JSON or CSV audit export. It forwards commands; it does not perform agent effects or write runtime decisions.
- **Go** is the final authority for every agent operation: admission and the immutable passport, the
  bounded agent loop, the model gateway, the action gate, report provenance and export decisions
  (trusted source lineage, inherited classification, template and projection checks), exact-action
  approvals, allowance reservations, the tool executor, the four typed tool adapters, data
  minimization, audit events and the runtime repository. Report 1.2 adds the hybrid security controls
  (deterministic content rules, the signature matcher, the semantic evaluator as a separately metered
  security purpose of the same local model), active-catalog checks before evaluation and dispatch,
  shared and per-purpose accounting, the model allowlist, request timeouts, the local concurrency cap,
  performance telemetry and the documented adapter contract (`POST /internal/control/evaluate`). It holds the model provider and tool
  credentials and is the authority for action canonicalization and artifact classification.
- **Next.js** presents task setup, run detail, report classifications, source trails and export
  explanations, the safe-continuation explanation, exact-action review and activity. It calls NestJS,
  reads stored classifications instead of computing labels, and cannot grant scope or execute tools.
  Report 1.2 adds active controls and the policy revision with its reload state, blocked and redacted
  counts, rule hits, security failures, agent and security usage, and audit export access. Report 1.2
  replaces a policy-editor screen with the editable `policy.yaml` ("A small task form and editable
  policy.yaml replace a general policy-editor screen in the hackathon build"); the architecture still
  lists a policy editor and a `/policies` route (`policy editor`).

| Schema    | Main records (report 1.1)                                                                                                           | Table names (architecture proposal, adopted at the M0 freeze unless recorded otherwise)                                                              | Write authority                             |
| --------- | ----------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `app`     | Memberships, task/source/template versions, control catalogs, active revisions, trusted feed manifests and revocations (report 1.2) | users, organizations, memberships, task_templates, task_versions, policy_versions, tool_definitions, report_templates, projection_rules, revocations | NestJS                                      |
| `runtime` | Passports, jobs/actions, context manifests, model purposes, reservations, control assessments, timing and safe events (report 1.2)  | passports, runs, jobs, model_calls, actions, approvals, execution_attempts, report_lineage, budget_reservations, usage_entries, audit_events         | Go                                          |
| `demo`    | Synthetic invoices, vendors, immutable reports with lineage, and simulated outbox                                                   | versioned invoices and vendor records, classified reports, outbox_messages                                                                           | Go tool adapters, through restricted access |

The two sources place report lineage differently (`report storage`), and neither names a table for
the trusted source-field classifications and recipient rules (`source classification storage`).
Report content and its lineage commit atomically in both. The architecture's table names do not
cover the report 1.2 records (control catalog revisions, the active pointer, feed revisions, control
assessments, per-purpose reservations, timing); their names are decided at the M0 freeze.

## Contracts to freeze first

The report asks for these shared contracts, each with one recorded owner, before parallel work
starts, and to "Freeze report classification, source manifest, template version, projection-rule
version and export denial fields alongside those contracts." The web + API implementer coordinates
them in `packages/contracts`; each recorded owner decides its contract's shape after a quick shared
review, and Go stays the authority for action canonicalization and artifact classification.

| Contract                                                                                                                                                                  | Owner                                                                       |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| Start-run request                                                                                                                                                         | web + API implementer                                                       |
| Passport representation                                                                                                                                                   | Go implementer                                                              |
| Action proposal                                                                                                                                                           | Go implementer                                                              |
| Approval decision                                                                                                                                                         | web + API implementer                                                       |
| Run state                                                                                                                                                                 | Go implementer                                                              |
| Safe event                                                                                                                                                                | Go implementer                                                              |
| Report and lineage summary (report 1.1: "Shared schemas should define supported requests, report lineage summaries and errors")                                           | Go implementer                                                              |
| Authenticated operator context (repository addition, not in the report's list; depends on decision 7)                                                                     | web + API implementer, with the Go implementer (waits on decisions 4 and 7) |
| Policy activation and catalog revision, including the `policy.yaml` schema (report 1.2)                                                                                   | web + API implementer                                                       |
| Semantic verdict, the guard result (report 1.2: "a schema-validated risk category, score in the configured numeric range and a bounded reason code")                      | Go implementer                                                              |
| Per-purpose reservations and usage (report 1.2)                                                                                                                           | Go implementer                                                              |
| Security summary and audit export record (report 1.2)                                                                                                                     | web + API implementer                                                       |
| Telemetry fields (report 1.2)                                                                                                                                             | Go implementer                                                              |
| Control evaluation adapter, `POST /internal/control/evaluate` (report 1.2: "a documented Go client/HTTP contract for governed model calls and registered tool proposals") | Go implementer                                                              |

Owners recorded by the lead on 2026-10-03 (SH-07, contract part). The owner decides the shape after
the quick shared review; anyone may draft. Because the Go implementer owns most contracts and is
also building GO-06, the web + API side drafts the Go-owned contracts in `packages/contracts` on its
branch for the Go implementer's approval. A contract is frozen when its owner approves the draft and
it is merged into `main` through SH-11; until then it is a draft, and nothing built on it counts as
done. The M0 freeze (SH-10) is this per-contract approval, not one meeting.

Proposed reason vocabulary (reports 1.1 and 1.2): `resource_out_of_scope`, `destination_not_allowed`,
`report_export_restricted`, `report_lineage_missing`, `source_policy_changed`, `template_not_allowed`,
`approval_required`, `approval_expired`, `action_changed`, `resource_version_changed`,
`allowance_exhausted`, `run_cancelled`, `outcome_unknown`; report 1.2 adds `semantic_injection_detected`,
`security_evaluator_unavailable`, `security_allowance_exhausted`, `content_redacted`, `signature_match`,
`policy_reload_rejected` and `model_not_allowed`. "Scores and model explanations are evidence, not
authorization." "Denial may identify an authorized
alternative template without granting extra scope." "The UI, runtime, tests, and evidence should use
the same vocabulary." The architecture's event names (`report.export_denied`,
`report.safe_template_offered` and others) are illustrative input for the safe event contract.

## Decisions between the starter and the design

The numbers stay stable because the agent files and the roadmap refer to them. The document owner
writes each outcome down here when it is settled.

1. **Schemas. Settled by the report:** `app`, `runtime` and `demo` in one PostgreSQL instance, with the
   write authority above; the architecture's table names are the M0 proposal. TypeORM's
   `migration:generate` never emits `CREATE SCHEMA`, so the first migration of each schema creates it
   by hand; the bookkeeping table stays in `public`. Owner: the integration role (migrations by the
   web + API implementer by default).
2. **Database roles. Settled by the architecture specification:** explicit, narrow privileges per
   service, and "For local demo effects, the Go executor uses one PostgreSQL connection and transaction
   to commit the report or outbox effect, trusted lineage, runtime completion and associated events",
   which matches the report's recommendation. The starter still has one database user. Owner: the lead
   (database roles, by default).
3. **Browser to API path. Open.** The Next.js proxy forwards no cookies or authorization headers and
   buffers a JSON response with a ten second timeout. The report allows authenticated polling before
   server-sent events. Proposed, not decided: one same-origin route handler that forwards an allowlist
   of top-level API prefixes, the session cookie and a fixed set of headers, and streams the response.
   Owner: the web + API implementer.
4. **Operator context to Go. Requirement settled; the two sources differ on the mechanism.** The
   report requires Go to verify service identity and the authenticated operator context and lists the
   mechanism as unresolved ("The service token in the starter requires replacement or extension for
   authenticated operator context"). The architecture specifies "signed, short-lived operator context
   containing the user and organization"; that is an input to the decision, not the decision. Waits on
   decision 7. Owner: the web + API implementer with the Go implementer.
5. **Background worker in Go. Settled by the report:** durable jobs in PostgreSQL, claimed with a lease
   and released during an approval wait, and no message broker; one worker process is the report's
   simple option. The worker must be covered by graceful shutdown and the readiness check. Owner: the Go
   implementer.
6. **Model provider. Provider type settled by report 1.2, model open.** "The primary self-contained
   model path would be a locally hosted model, for example through Ollama; one provider may serve agent
   and security requests under separate metered purposes." Go calls it and holds any credential. The
   criteria provide no paid subscriptions. The user selected Ollama on a separate MacBook with an M1 Pro and 16 GB RAM. The current model candidate is `qwen3.5:4b`, selected by the user as provisional and
   subject to change. Measured hardware fit, network endpoint and accounting rule remain open
   ("Choose a local model that runs on the actual machine"). Owner:
   the Go implementer with the lead (infrastructure).
7. **Authentication mechanism. Open, and on hold by the user's decision of 2026-10-03.** NestJS
   authenticates users and checks organization membership ("AuthModule: login/session verification,
   organization membership and role checks"); one seeded operator may stand in for onboarding in a
   clearly labelled development demonstration. The credential and session mechanism is not decided; the
   recorded proposal in the roadmap (SH-01) stays proposed, not decided. Owner: the web + API
   implementer.
8. **Reuse of pre-event work. Open.** The report says not to presume that pre-event code or prepared
   assets are eligible. The competition rules say work starts no earlier than 11:00 on 3 October 2026
   and say nothing about reusing prepared code. Report 1.2: "The criteria allow pre-existing agents,
   applications and unrelated components [S10, section 5], but do not expressly resolve advance work on
   the assessed control layer or task-specific planning." The document owner confirms with the organizers whether
   this starter may be used and how it must be disclosed; `docs/preparation-record.md` is the
   disclosure record, and the repository history shows when each part was made.

## Open items between the report and the architecture specification

Each item is open until the document owner records the outcome here; the roadmap cites them in
"Blocked by" (`docs/roadmap/README.md`, "Open decisions and blockers").

- `read path`: the architecture reads authorized runtime views ("NestJS has read access only to
  authorized runtime views needed for the interface"); the report also says NestJS calls "corresponding
  private Go endpoints".
- `report storage`: lineage in `runtime` (`report_lineage`) per the architecture, or with the reports in
  `demo` per the report; context manifests appear only in the report.
- `internal report rendering`: whether the internal report may hold model-written text and so needs a
  conservative context manifest (report), or is rendered deterministically (architecture).
- `source classification storage`: where the trusted source-field classifications and recipient rules
  live and who writes them.
- `vendor projection fields`: which fields the vendor projection may hold; the report's two example
  lists differ, and the final list is a team policy decision.
- `passport report fields`: singular or plural destinations, template and projection fields.
- `policy editor`: the architecture lists a policy editor and a `/policies` route; the report keeps "A
  policy editor" outside the initial delivery scope.
- `rename operation`: no operation renames or copies a report in either source, yet the demonstration
  renames one.
- `list reads`: `/runs` and `/approvals` need list reads that neither source defines.
- `command idempotency keys`: the architecture puts idempotency keys on commands; the report on actions.
- `repository layout`: the architecture shows `db/migrations`, `db/seeds`, `infra/docker-compose.yml`
  and `docs/diagrams`; this repository keeps migrations in `apps/api/src/database/migrations` (verified
  technical reason in `db/migrations/README.md`), Compose in `infra/compose.yaml` and design files here.
- `deployment network`: the architecture's private service network against the current Compose file.
- `Go package layout`: inconsistencies inside the architecture's module list and package tree.
- `design source references`: the report's internal sources that are not in the repository.
- `architecture specification version`: the architecture specification holds the version 1.1
  Mermaid source; the report 1.2 figures in the docx are the current design, and the specification's
  routes, modules and tables lack the report 1.2 additions.
- `policy editor` (changed by report 1.2): the report now replaces the editor screen with `policy.yaml`.
- `test command`: the report names `make verify-controls` and `make reset-demo`; the repository has no
  Makefile and runs its commands through `package.json`.
- `scoring weights`: the rules (20% self-testing, 10% practicality) and the criteria (15% each) differ.
- `start time confirmation`: the rules print 11:00 PM; the lead confirmed 11:00 AM; report 1.2 asks for
  organizer confirmation.
- `catalog activation protocol`, `classifier prompt and verdict schema`, `redaction rules`,
  `feed grammar and trust` and `measurement method`: implementation choices report 1.2 leaves open
  ("Configure these explicitly and record their tested limits").
- `judge access`: how judges reach the running layer, the test suite and the configuration files.
- `researcher role` and `shared-track assignment`: the two staffing items above.

## Go runtime decisions

### Go ownership update (SH-07, Go part)

**Recorded at the user's direction on 3 October 2026.** The user is the sole implementer and owner
of all Go work. The report's Implementer 3, Implementer 4 and Implementer 5 labels remain as
responsibility groups, not separate people assigned to the current Go plan. Earlier Go decision
owner labels in this document resolve to the user through this update. The report's multi-person
effort and capacity estimates do not describe the current staffing.

The user owns admission and passports, the worker and model gateway, enforcement and approvals,
allowance accounting, execution claims and the executor, all four tool adapters, data minimization,
the runtime repository and events, the internal API, the Go DTO mirrors and labelled replay.
Transactional runtime/demo effects are placed in the Go executor with the runtime repository and
adapters sharing the transaction boundary selected in SH-06. The user also owns conditional Go
endpoints for form options, stored reports, run/usage/event reads, review payloads and event streams
if their respective open decisions select Go.

All existing Go packages (`cmd/gateway`, `internal/config`, `internal/logging`, `internal/database`,
`internal/health`, `internal/httpserver`) have that same owner, recorded in the gateway README.
Future packages inherit this responsibility assignment and receive their package row when real
code lands. No empty packages are created by this update.

This records the Go ownership portion of SH-07. The lead has since recorded shared contract
owners in the table above; contracts remain drafts until owner approval and the SH-11 merge.
SH-07 remains open for the shared-track assignment and researcher role. NestJS contract coordination and shared migration, database-role,
infrastructure and document responsibilities remain with their recorded roles; owning Go adapters
does not automatically transfer that shared work to the user.

### GO-01: multiple-action model responses

**Decided with the user on 3 October 2026; implementation pending.** Owner: go
(Implementer 3). Source: report, "The enforcement loop and data minimization" and
"Design decision record" (one action per model step).

If a model response proposes more than one tool action, Go rejects the entire response as a
denied proposal before any adapter executes. It never selects or executes a subset. The rejection
is recorded as a safe event without raw arguments or protected values. The dispatched model call
still counts toward its limits, and its usage follows the accounting rule adopted in decision 6;
rejecting the response does not release an unresolved reservation.

If correction allowance remains, Go returns safe feedback asking for exactly one action and counts
that correction against the passport's correction limit. The next model dispatch must pass the
usual run and allowance checks. If no correction remains, the run stops with the recorded reason.
Until bounded correction is implemented in GO-29, the blocked-action path stops the run.

Proposed reason code: `multiple_actions_not_supported`. SH-10 must freeze the code and safe message
in X-13, and SH-11 must land their shared contract fixtures before consumers implement them.
Provider-side single-action controls, if available after GO-03, supplement this runtime check.

Implementation and evidence follow in GO-10, GO-11 and GO-29: a multiple-action response must call
no adapter; correction and exhaustion scenarios must show their counted usage and recorded reason.

### GO-02: durable attempts and worker recovery

**Decided with the user on 3 October 2026; implementation pending.** Owner: go
(Implementer 3). Source: report, "Durable state idempotency audit and uncertain outcomes" and
"Threat model limits and unresolved design choices".

Before each model or tool dispatch, Go commits a durable attempt record in `runtime`. Each attempt
has its own stable identifier and belongs to a run; a tool attempt also references the immutable
action's stable identifier. Safe retries create another attempt for the same action, preserving
its idempotency key. Attempts link to their allowance reservation and record their outcome and
known usage when available. SH-14 carries these requirements to the migration owner before SH-27
defines the tables; table names and wire enums are not decided here.

The pre-dispatch record proves intent to dispatch, not that the request reached the provider or
adapter. A crash between committing the record and sending the request is therefore treated
conservatively. Lease expiry alone never establishes failure or authorizes a retry. Recovery must
establish that a former worker cannot still commit the attempt before treating an absent completion
as final; the worker and execution-claim implementation must enforce this ownership check.

Recovery follows the recorded outcome:

- A successful action is not executed again. Resume from its persisted result and continuation.
- An unsettled model attempt retains its unresolved reservation. It is not blindly resent or
  treated as zero usage; any permitted new model attempt follows the separately agreed retry
  policy, fresh run checks and a new reservation.
- For local `create_report` and `queue_report` effects, absence of a completion record establishes
  that the effect did not commit only after the old transaction has ended and only if SH-06 adopts
  the shared transaction for effect, completion and event. A safe retry uses the same action and
  idempotency key, with fresh authorization and allowance checks. Until that guarantee is in place,
  absence of completion is insufficient proof of no effect.
- A tool attempt whose outcome cannot otherwise be established enters the unknown-outcome path:
  persist operator attention, pause the run and do not automatically requeue the operation.
- An approval wait survives recovery and resumes the original stored action only after a valid
  decision and execution rechecks; recovery creates no approval grant.

Implementation and evidence follow in GO-08, GO-39, GO-45, GO-49 and GO-53. Crash checks cover the
boundaries before dispatch, after dispatch, after a local effect commits and during approval waits,
including an old worker still active after lease expiry. The prototype does not yet provide an
operator reconciliation operation; GO-61 must record that limitation.

### GO-04: canonical arguments and action digest

**Decided with the user on 3 October 2026; implementation pending.** Owner: go
(Implementer 4). Source: report, "Exact action approval versioning and execution rechecks",
"Technical architecture and service ownership" and "Terminology for developers and presenters".

Go defines a canonical encoding for each registered tool's typed arguments and computes a SHA-256
digest over a versioned canonical action representation. The digest covers the action and run
identifiers, tool, canonical arguments, passport reference, policy version, recipient, affected
resources and their relevant versions, exact outbound content and expiry. Canonicalization version
is included in the hashed representation. The digest itself and mutable execution status are not
part of that representation.

The encoding rules are:

- Decode against the frozen typed contract. Reject unknown or duplicate object fields, missing
  required fields, wrong types and unsupported or ambiguous values before canonicalization.
- Encode typed fields in a fixed order with one compact JSON representation. Input object field
  order and insignificant JSON whitespace do not affect the canonical bytes or digest. The
  implementation must not hash the raw incoming JSON or depend on arbitrary map iteration.
- Use no floating-point values. Numeric fields use the integer or decimal representation adopted
  in the relevant contract, with explicit validation; money representation remains a contract
  decision rather than being invented here.
- Preserve decoded identifier and content values exactly: do not trim, case-fold, normalize
  Unicode or rewrite line endings. Equivalent JSON escapes decode to the same value. Reject
  invalid text encodings instead of silently replacing them.
- Preserve list order unless the frozen contract explicitly defines a field as a set. For such
  fields, the contract must define sorting and duplicate handling before implementation.
- Give optional or inapplicable fields and timestamps one unambiguous representation, agreed in
  X-09. Do not silently equate absent, null and empty values or round an expiry.

Go hashes its authoritative stored action, including the material frozen for review, rather than a
replacement payload supplied by a browser. Before execution, it recomputes the digest from that
stored material and compares it with the recorded digest. Changed arguments, recipient, content,
expiry or bound resource versions invalidate the previous approval. Current source record versions
are also checked separately; a stored digest does not prove that a source record is unchanged.
The digest detects changes only and never replaces identity, scope or reviewer authorization.

SH-10 and the action contract owner carry these requirements into X-09; SH-11 lands types,
schemas and fixtures before GO-12 implements the encoding. This decision does not settle the open
`record versions` and `exact reviewed material` items. GO-12 tests equivalent representations,
rejected inputs and a change to each material field; GO-43, GO-45 and GO-46 verify the approval
and execution boundary. No canonicalization code or digest is implemented yet.

### GO-05: replay entry and labels

**Decided with the user on 3 October 2026; implementation pending.** Owner: the user, as the sole
Go implementer recorded in the SH-07 Go ownership update above.
Source: report, "Live demonstration storyboard and proof checks" (Reliable demonstrations without
invented behavior) and "Illustrative invoice scenario and future domain adaptations" (Scene 2).
Replay and contract ownership are recorded. The shared-track assignment and researcher role remain open;
X-09 and X-12 still need their freeze before replay implementation.

Chosen approach: a labelled Go runtime scenario test substitutes one stored prohibited proposal
for the next model step of a named synthetic run. The proposal then follows the production action
storage, gate and execution path, with no replay bypass of authorization. The replayed proposal and
all events it produces carry an explicit replay source, whose field name and value must be frozen
in X-09 and X-12. General events expose safe metadata only; fixture payloads are not copied into
the activity feed.

The run retains its original passport and allowance. A substituted proposal makes no provider call
and must not invent provider usage. Gate checks and any correction consume the limits applicable
to them, as the frozen policy defines. If a permitted correction follows, it completes useful work
within that same run and allowance. A labelled provider test double used for repeatability remains
distinct from a live model call, and its results are presented as test evidence.

This chooses GO-05 option (1), without a NestJS replay trigger or X-65. GO-36 will
implement the replay after its prerequisites land; no replay code exists yet.

### GO-03: local provider direction

On 3 October 2026, the user adopted report 1.2’s primary local-model path, replacing the earlier OpenAI `gpt-5.6-sol` choice. The user selected Ollama on a separate MacBook with an M1 Pro and 16 GB RAM. The user selected `qwen3.5:4b` as the current provisional model candidate; it may change after testing. It is not a frozen model contract. The hardware information is user-reported, not a measured performance result. GO-03 and SH-04 must record hardware fit, client choice, agent/security purpose accounting, reported token usage, call counts, request duration and reservation rules. The Go connectivity diagnostic passed on the developer M2/8 GiB machine; its exact outcome and limits are recorded in `services/gateway/README.md`. Production runtime governance and the shared model/accounting decision remain unverified.

#### GO-03 implementation input (proposal, pending SH-04)

Report basis: "Technical architecture and service ownership", "Atomic allowances hard limits and estimated cost" and "Central policy configuration and safe reload".

- Use a small Go `net/http` client for Ollama's native `POST /api/chat`, with `stream: false`, bounded request/response sizes and a request deadline. No new provider dependency is proposed. Both agent and security requests use this client and the same reservation authority, with separate metered purposes.
- Record a dispatch count before every request, including retries. Set an explicit output ceiling and context limit for the chosen model. Input estimation alone is not a proven token upper bound: SH-04 must settle a conservative reservation strategy before a hard token-budget claim or live dispatch implementation.
- Read input/output counts from `prompt_eval_count` and `eval_count`; cached-token counts are supplementary metadata, not an additional token charge. Validate nonnegative integer fields and retain an unresolved reservation when required usage is absent or invalid. Missing usage is not zero.
- Measure request wall time in Go separately from Ollama's provider timing fields (nanoseconds). Cancellation or timeout does not prove the remote inference stopped; recovery and concurrency accounting must preserve that uncertainty.
- Local usage has no configured commercial tariff. Monetary cost is unavailable, rather than an invented zero bill. An optional future tariff uses explicit integer minor units or decimal arithmetic and is labelled estimated.
- Validate one supported action or a constrained final answer after each agent response. Ollama accepts tools and structured-output schemas, but that API support does not prove that the selected model follows the contract. GO-01 still rejects multiple actions; security verdicts are separately schema-validated and cannot grant authority.

Official references checked on 3 October 2026: [chat API](https://docs.ollama.com/api/chat), [usage metrics](https://docs.ollama.com/api/usage), [network and context configuration](https://docs.ollama.com/faq). These document API behavior; no request to the presentation machine has been made.

Before GO-06: record the installed Ollama version, exact model identifier and digest, context/output settings, reachable endpoint and authenticated transport arrangement through infrastructure. Run an agent-response and semantic-verdict fixture on the M1 Pro machine, recording latency and memory fit. Ollama on the other machine is a network dependency of Go; do not assume this developer machine's localhost reaches it. GO-03, SH-04 and X-04 remain incomplete.

Model candidate reference: [Ollama qwen3.5:4b](https://ollama.com/library/qwen3.5:4b), checked on 3 October 2026. The installed model digest and agent/security suitability must be verified on the presentation machine. A model change before the freeze updates this decision and its checks; after admission it must obey the immutable passport and active model allowlist rather than silently substitute another model.

#### GO-06 developer connection evidence

On 3 October 2026 at 14:56:15 UTC, the developer M2/8 GiB machine completed two explicit synthetic Go provider calls through Ollama 0.35.1 to `qwen3.5:4b` (ID `2a654d98e6fb`), with `think: false`. Both passed the diagnostic response schema and reported 38 input and 6 output tokens; the command exited 0. The gateway README records the build, command and measured durations. This establishes developer-machine connectivity only; GO-03/SH-04 model/accounting adoption is still pending. It does not establish a semantic security verdict, task reservations or presentation-machine readiness.
