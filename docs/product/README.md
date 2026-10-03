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
| [organizer-questions.md](organizer-questions.md)                       | RS-01: the organizer message, why each question is asked, the verbatim answer table and the proposed pre-event disclosure.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| [requirements.md](requirements.md)                                     | RS-03: every requirement of the rules [S9], criteria [S10], general HackYeah rules [S11] and FAQ [S12], with roadmap coverage, the report-vs-official differences D-1 to D-10 with outcomes, the proposed sample signatures and gaps.                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| [storyboard.md](storyboard.md)                                         | RS-04: the twelve demo beats in three segments with proofs, captures, replay use, judge interactions, labels and fallbacks.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| [claim-to-proof.md](claim-to-proof.md)                                 | RS-05: every presentation claim with its scope, proof and status, and the claims to avoid.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| [submission-checklist.md](submission-checklist.md)                     | RS-09: HackTribe fields, freeze record, description skeleton, submit and after-deadline steps.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| [presentation.md](presentation.md)                                     | RS-08: the ten-slide content with final-build placeholders.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| [source-register.md](source-register.md)                               | RS-06: sources S1 to S15 with what was checked, and licenses of everything added after the starter.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| [comparison.md](comparison.md)                                         | RS-07: comparison with existing controls from primary pages and the OWASP LLM Top 10 2025 mapping.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

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
the trusted source-field classifications and recipient rules (`source classification storage`). Both
are now decided by the lead's delegate: `runtime.report_lineage` with `demo.reports`, and two new
`demo` columns ("Decisions recorded by the lead's delegate", items 3 and 6).
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
`policy_reload_rejected` and `model_not_allowed`. Decided by the lead's delegate (X-13): 28 codes, these
20 plus `tool_not_registered`, `invalid_arguments`, `tool_not_allowed`, `decision_unavailable`,
`content_blocked`, `content_too_large`, `multiple_actions_not_supported` and `run_expired`, on `main` in `packages/contracts/schemas/reason-code.schema.json` (31 on `main` at 1dad371: with `limit_not_allowed`, `run_not_active` and `approval_rejected`). "Scores and model explanations are evidence, not
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
4. **Operator context to Go. Settled with decision 7 by the lead on 2026-10-03.** NestJS turns the
   verified operator context (actor, organization, roles) into a short-lived signed JWT and sends it to
   Go in an `X-Operator-Context` header on every runtime command, next to the service token. Go verifies
   the signature, the expiry and the service identity before it acts, and still authorizes each command
   against its organization and run itself. This matches the architecture's "signed, short-lived
   operator context containing the user and organization". Not implemented yet; the signing key is an
   API and gateway secret, never sent to the browser (SH-20). Owner: the web + API implementer with the
   Go implementer.
5. **Background worker in Go. Settled by the report:** durable jobs in PostgreSQL, claimed with a lease
   and released during an approval wait, and no message broker; one worker process is the report's
   simple option. The worker must be covered by graceful shutdown and the readiness check. Owner: the Go
   implementer.
6. **Model provider. Provider type settled by report 1.2; model decided by the lead's delegate on
   3 October 2026: `qwen3.5:4b` through Ollama, with `think: false`.** "The primary self-contained
   model path would be a locally hosted model, for example through Ollama; one provider may serve agent
   and security requests under separate metered purposes." Go calls it and holds any credential. The
   criteria provide no paid subscriptions. The user selected Ollama on a separate MacBook with an M1 Pro
   and 16 GB RAM. `qwen3.5:4b` is licensed Apache 2.0 (model card and the license text bundled with the
   model; [source-register.md](source-register.md)). `qwen2.5:3b` is excluded: its Qwen Research
   License grants use "FOR NON-COMMERCIAL PURPOSES ONLY". On `main`, the Go model client sends `think: false` (`services/gateway/internal/model/accounting.go`).
   **Frozen for the demonstration on 2026-10-03 (GO-03; commit `452a358`, on `main`; adopted by the lead's delegate):** `qwen3.5:4b`, Ollama digest
   `2a654d98e6fba55d452b7043684e9b57a947e393bbffa62485a7aac05ee4eefd` (4.7B, Q4_K_M), on Ollama 0.35.1
   with `think: false` and a context of 8192 tokens for agent and security requests. Measured on an
   Apple M1 Pro with 16 GB (MacBookPro18,1, macOS 27.0) with Ollama on the gateway's machine: the model
   is resident in 3.33 GB on the GPU; live agent calls take p50 3.9 s and p95 5.4 s (30 calls, GO-27
   live runs, load about 12 on 10 CPUs); the live semantic check p50 3.2 s (4 calls, same runs) and
   1.93 s in the quiet-machine benchmark (GO-81) (`services/gateway/README.md`, "Model and hardware
   freeze (GO-03)"). **Measured on the team's M1 Pro; whether it is the presentation machine is to be
   confirmed by the user** (SH-45, SH-50). The live latencies were taken under load and are not slide
   numbers (decision 22). Owner: the Go implementer with the lead (infrastructure).
7. **Authentication mechanism. Settled by the lead on 2026-10-03: an HttpOnly cookie carrying a
   signed JWT.** NestJS (`AuthModule`) checks the operator's credential, then issues a JWT signed with a
   symmetric key and sets it in an HttpOnly cookie that browser JavaScript cannot read; NestJS also
   checks organization membership and roles. The seeded operator is a development demonstration user,
   created by an explicit seed command and labelled "Development Demonstration" in the interface. Not in
   the repository yet: no implementation of this has been pushed, so guardrail 2 still applies
   (`UnimplementedAuthProvider` answers 501 until the real provider lands). Details the web + API
   implementer fixes in the implementation: password hashing, token lifetime, cookie attributes,
   logout (a stateless JWT cannot be revoked before it expires, so keep its lifetime short) and the
   signing-key variable, generated by `pnpm run setup` and never sent to the browser (SH-20). Owner:
   the web + API implementer.

   **Open for the user (not decided), 3 October 2026:** the record above says a signed JWT, but the
   implementation on `main` (44e925f) is a stored session: `apps/api/src/auth/auth.controller.ts`
   stores a SHA-256 hash of the session identifier (`Session` entity) and reads it from the session
   cookie. The lead recommends keeping the stored session and changing this record to match. Until the
   user decides, the record and the implementation disagree.

## Decisions recorded by the lead's delegate (3 October 2026)

Each item below was **decided by the lead's delegate** on 3 October 2026 and recorded by the
researcher. "On `main`" names what was merged when the researcher last checked each item: c968034 (3 October 2026, every item re-checked after items 24 to 27 arrived), again at 1dad371 for items 10, 21 and 25 to 28, and at e50e186 for items 6 (decision), 9, 15, 16 and 23;
anything else is a decision, not an implemented behaviour.

1. **Read path (`read path`).** Operator reads go through private Go `/internal` endpoints, mounted
   behind the service token and the verified `X-Operator-Context`. On `main`: yes. `api.Commands` mounts the start-run, cancel, control-evaluate, stored-report, approval, review, run-state, run-events, run-usage and security read routes behind the service token and the operator-context check (`services/gateway/internal/api/api.go`, `httpserver/internal_commands.go`).
2. **Stored report read (`stored report read`).** Go serves `GET /internal/runs/{runId}/reports/{reportId}`
   (GO-37, Worker 2). Internal only content is withheld unless the viewer may read internal content,
   which is any verified operator of that organization (the MVP has no vendor users). Labels and lineage are always shown. On `main`: the route is mounted (`provenance.StoredReportHandler`); the withholding rule was not traced.
3. **Report storage (`report storage`).** Lineage in `runtime.report_lineage`, reports in
   `demo.reports`. On `main`: yes, migrations `1791041440000-AddReportsAndOutbox` (`demo.reports`) and
   `1791060000000-AddProvenanceColumnsAndLineage` (`runtime.report_lineage`).
4. **Internal report rendering (`internal report rendering`).** Deterministic Go templates only; GO-71
   (a model-written internal summary with a conservative context manifest) is dropped. On `main`: the
   templates are Go constants in `services/gateway/internal/provenance`.
5. **Vendor projection (`vendor projection fields`).** `vendor_invoice_fields_v1`: invoice reference,
   external reference, duplicate-reference flag, currency, total, due date; never the internal note.
   Templates and the projection rule are Go constants in `internal/provenance`. On `main`: yes
   (`VendorInvoiceFieldsV1` in `provenance.go`).
6. **Source classification storage (`source classification storage`).** New columns
   `demo.invoices.internal_note_classification` and `demo.vendors.registered_reporting_address`. On
   `main`: yes, migration `1791060000000`.
7. **Semantic verdict (`classifier prompt and verdict schema`, verdict part).** Exactly
   `{risk_category, score from 0 to 1, reason_code}`, validated in Go; a score at or above the threshold
   fires the guard; redact masks the whole field. On `main`: the verdict schema is in
   `services/gateway/internal/security/semantic.go`; the classifier prompt's tested limits are still to
   be recorded.
8. **Redaction scope (`redaction rules`).** Semantic calls run only on `read_invoice`'s
   `internal_note`. Deterministic content rules and signature matching run on every free-text field.
   Per-field withholding: a blocked field is withheld and the other fields return; a guard failure
   withholds the whole result and pauses the run. On `main`: the `internal/security` package exists;
   this exact scope was not traced in its code by the researcher.
9. **Feed grammar and trust (`feed grammar and trust`).** Normalized-substring rules. Trust is the
   authenticated import plus the SHA-256 of the file bytes, pinned by `signatures.revision`. There is
   no signing key; this is a documented limitation, so the feed is **not** called "signed". The feed
   is `config/attack-signatures.json`, revision `feed_v1`, with four rules (SH-46). On `main`: yes, `config/attack-signatures.json`, revision `feed_v1`, four rules (`prompt_ignore_previous_v1`, `code_exec_python_import_v1`, `unsafe_deserialization_pickle_v1`, `model_repo_trust_remote_code_v1`); its own scope text says it protects no model-loading infrastructure. Importing it into a catalog revision is item 16.
10. **Reason codes (X-13).** 28 codes: the report's 20 plus `tool_not_registered`, `invalid_arguments`,
    `tool_not_allowed`, `decision_unavailable`, `content_blocked`, `content_too_large`,
    `multiple_actions_not_supported` and `run_expired`. On `main`: yes, 31 values in `packages/contracts/schemas/reason-code.schema.json` at 1dad371: these 28 plus `limit_not_allowed` (admission rejects requested limits above the catalog, GO-13), `run_not_active` (item 26) and `approval_rejected`.
11. **Worker readiness (decision 5, X-32).** While the worker loop is not running, `/health/ready`
    answers 503 with the real database check; no schema change. On `main`: yes
    (`services/gateway/internal/health/health.go`).
12. **Task template.** Admission accepts exactly `reconcile_atlas_v1`, a Go-registered task template
    constant. On `main`: yes, `admission.TaskTemplateReconcileAtlas` in `services/gateway/internal/admission/admission.go`.
13. **Budget authority.** The ledger is the single authority for model calls, tokens, time and
    concurrency; `runtime.model_calls` is written only by `internal/budget`; `budget_reservations`
    holds tool attempts only (`docs/contracts/runtime-schema-alignment.md`). On `main`: yes
    (`services/gateway/internal/budget`, the alignment document).
14. **Model.** Recorded as decision 6 above.
15. **Catalog activation protocol (`catalog activation protocol`).** `pnpm policy:import` validates and
    stores the immutable catalog revision and the feed row, then sets only `requested_revision_id`. Go
    validates the requested revision with its own parsers. On success it sets the validated and active
    revisions to the requested one, plus `active_feed_revision_id`, in one transaction: that is its
    acknowledgement. On failure it records `last_error` and keeps the last good revision; with no good
    revision the gateway is not ready and admission refuses. Runs keep the revision on their passport.
    On `main`: the pointer table `app.control_catalog_pointer` (migration `1791038985994-AddControlCatalog`), the `pnpm policy:import` command and Go's validation and acknowledgement (`services/gateway/internal/catalog/activation.go`, GO-73). `pnpm catalog:activate` runs the gateway's own activation once for setups without a running gateway (`cmd/catalogactivate`, commit `ec7c815`). The feed half of the import is item 16.
16. **Feed import (API-34).** Lane c1 builds the feed half of API-34 inside `pnpm policy:import`.
    The user approved merging lane c1. On `main`: yes, `apps/api/src/policies/policy-catalog-importer.ts` and `import-policy.command.ts` validate and import the feed named by `signatures.path` (`validateSignatureFeed`, a size cap, the bare file name next to the policy file).
17. **Reviewer authority.** A reviewer is a verified user whose `app.memberships` role list holds
    `reviewer` for that organization; a signed claim alone never suffices. The demonstration seed gives
    the demo operator the roles `operator` and `reviewer`. On `main`: the memberships `roles` column, `ReviewerRole = "reviewer"` in `services/gateway/internal/policy/approvals.go` and migration `1791110000000-GrantGatewayMembershipRead`, which lets the gateway read `app.memberships`. `scripts/seed-demo.mjs` still does not seed memberships (its own TODO, SH-19), so the demo operator has no `reviewer` role yet.
18. **Approval contract (X-10).** `POST /internal/actions/{actionId}/approval` with exactly
    `{"decision": "approve" | "reject"}`, and `GET /internal/actions/{actionId}/review` for the frozen
    review payload. On `main`: yes, both routes are mounted by `api.Commands` (`services/gateway/internal/policy/approval_handlers.go`).
19. **Frozen review payload.** It may contain the exact registered reporting address, for the
    authorized reviewer only; never in model context, events, logs or the audit export. GO-45 rechecks
    at execution that the address is unchanged. This amends the GO-07 rule that the address leaves the
    database only as the outbox recipient (`docs/roadmap/go.md`, GO-07 completion note: "only as a
    run-scoped reference, resolved inside queue_report"). On `main`: the review route exists; whether its payload carries the address as decided was not traced.
20. **Gateway database role (GO-38).** The gateway connects as `task_passport_gateway` with a
    generated `POSTGRES_GATEWAY_PASSWORD` and refuses to start without it; `pnpm db:roles` sets the
    login password, and `pnpm reset:demo` runs it too. On `main`: yes. The role is created by migration `1791050000000-CreateServiceRoles`; the gateway configuration requires `POSTGRES_GATEWAY_PASSWORD` (`internal/config/config.go`); `pnpm db:roles` (`scripts/db-roles.mjs`) sets the password. That `pnpm reset:demo` runs it too was not traced.

21. **Final result format.** The model's final answer is exactly
    `{"status": "completed", "report_ids": [...]}` with one or two unique lowercase UUIDs. Every
    identifier must be a report of this run and organization; otherwise the answer is rejected, never
    trimmed, and the rejection counts as a correction under GO-29. Go stores
    `runs.result_reference = {"report_ids": [...]}` with no prose, in the same transaction as the
    completion. On `main`: the format check `internal/runresult` (`Parse` rejects anything but the exact shape) and the `result_reference` write in `internal/repository/runs.go`. Its hook into the agent loop is item 27, also on `main`.
22. **Measurement method (`measurement method`).** Decided by Worker 2 as the Go implementer for
    GO-81 and accepted by the lead's delegate: concurrency 1; warmup excluded; the configurations
    without a model interleaved sample by sample; GO-80's phase names (`policy_lookup`,
    `deterministic`, `semantic`, `provider`, `commit`, `total`); nearest-rank p50 and p95; null before
    any observation; errors counted, not timed; semantic inspection on and off derived in the process,
    without editing the catalog. Numbers must be measured again on an idle machine before any slide:
    the current numbers were taken at a load average of 70 to 150. On `main`: the benchmark
    (`services/gateway/cmd/benchmark`) and its method in `services/gateway/README.md`.
23. **Semantic classifier prompt (`classifier prompt and verdict schema`, prompt part).**
    `classifier_v2` (lane c1, commit `b9f94c2`) was measured at 0 wrong of 84 cases
    on `qwen3.5:4b`, against 4 of 84 for v1. Limits: v2 was written with the case it now catches in
    view, the benign probes are few, and variance between runs remains. A claim says "on this fixture
    set", never universal detection. On `main`: yes, `classifier_v2` (commit `b9f94c2`) and the results table below (commit `6bbaac1`). The measurement is the lane's, not final-build evidence (X-59).

    Later evidence from lane c1 (on `main`: root `README.md` lines 404 to 411, in the `pnpm verify:controls` section; commit `ec7c815`): three runs on `classifier_v2`, fixture version 3,
    with the same model and prompt, all reported. Run 1 (`78d386c`) and run 2 (`45308f8`) **failed**:
    run 1 on two Go story tests ("no active control catalog": the request-only feed import activates
    nothing and a test database has no gateway), run 2 on a live label mismatch the test treated more
    strictly than the rule (fixed: a mismatch is now recorded, `--strict-live` fails on it). Run 3
    (`6bbaac1`) passed, exit 0, 1100 cases (Go 897, API unit 125, API database 27, fixtures 18, none
    failed or skipped); live model 27 of 29 matched labels, 0 false positives, 2 false negatives
    (`direct_relabel_report_v1` at 0.5, and `signature_code_exec_import_v1` at 0.69, which its signature
    rule blocks deterministically), 0 guard failures; load average 9.0 before and 12.5 after. The lane's
    own sentence: "The live label counts moved between runs of identical code (a different false
    positive in runs 1 and 2, two false negatives in run 3): that is the model's variance near the 0.75
    threshold, which is why a label mismatch is recorded and does not fail the suite (`--strict-live`
    does)." The earlier "0 wrong of 84" above does not repeat in run 3, so claims quote run 3 and the
    variance, not the best result. The results JSON is gitignored and was on the lead's machine only (`.verify-controls/results-2026-10-03T21-16-48Z.json`); the lead has asked lane c1 for a sanitized copy under `docs/evidence/`, which is not on `main` yet, so the numbers are not verifiable from the repository until it lands.

24. **Model call retries (`model call retries`).** There are no automatic model-call retries in the
    MVP. A failed, timed-out or unknown agent or security call is never re-sent; its reservation settles
    or is held as `usage_unknown`, and the run pauses (or the security check denies). This is the retry
    policy the GO-02 decision refers to. On `main`: the failure path writes the call as usage-unknown and
    holds the reservation (`internal/agent/loop_failure_postgres_test.go`, `internal/budget/README.md`);
    the proof is Worker 3's GO-42 follow-up, pending.
25. **Semantic check scope on action proposals (design point and limitation).** The semantic evaluator
    classifies only free-text argument values. Arguments the strict decoder constrains (identifiers that
    match their pattern, registered template names, UUIDs, run-scoped recipient references) get no
    model call, and a `not_applicable` control record is written. For the four MVP tools that means no
    semantic action call: the action control is the deterministic gate (scope, destination,
    provenance) plus signature matching. Reason: the live smoke of run `da88594f` showed
    `qwen3.5:4b` scoring benign `read_invoice` and `create_report` proposals at 0.85 to 1.0, so no
    report could be created. This narrows the report's Figure 6, which shows a semantic action check
    after the deterministic checks; it is recorded as a limitation, not hidden. On `main`: yes, the `not_applicable` outcome with reason `no_free_text_arguments` (`services/gateway/internal/security/action.go`, documented in `services/gateway/README.md`).
26. **Judge evaluations (X-91, GO-82).** Organization-wide for any verified operator of the
    organization; the evaluating `actorId` is recorded; `maskedSummary.inputSource` is `"judge"`; an
    evaluation is refused before any model call on a cancelled, expired or finished run, with the new
    X-13 code `run_not_active` (30 codes). The judge client should admit its own dedicated run. Score
    and category are returned, bounded per run by `calls_security`. On `main`: the route is mounted and `run_not_active` is in the reason-code schema; the refusal rules and the recorded actor were not traced.
27. **Final result in the loop.** The final-result format of item 21 is hooked into the agent loop by
    lane f3 (commit `e03ab44`). On `main`: yes.

28. **Beat 5 is shown with the labelled replay.** The internal report's export to the correct vendor
    (demo beat 5) is shown with the labelled replay (`services/gateway/cmd/replay`), not as a
    spontaneous live-model action. Live sample, recorded in the GO-27 block of `docs/roadmap/go.md` by
    lane f3 (commit `a9f8004`, on `main`), with the task
    instruction of the storyboard: run set 1 (before the fixes) created the internal report in 3 of 3
    runs, attempted the export in 0 of 3, mangled the recipient reference in 2 of 3 and had clean
    proposals denied by the semantic check in 2 of 3; run set 2 (after the fixes, build `87f22f0` plus
    the recipient sentence) created the internal report in 3 of 3, **attempted the export and was
    denied `report_export_restricted` in 1 of 3 runs** (run `8b812e16`), mangled the recipient in 0 of 3
    and had no semantic denials. So the live model attempts the export in fewer than 2 of 3 runs, which
    is why the demonstration uses the replay. The replay goes through the real production gate, prints
    `LABELLED REPLAY`, and its stored action and events carry the label `labelled_replay:<fixture id>`;
    the scripted fixture story (`internal/scenario`, w2) proves the same denial through the agent loop.
    The slide and the presenter text say "labelled replay" for that beat and never imply that the live
    model tried the export. On `main`: the replay command and its documentation
    (`services/gateway/README.md`; a check on 2026-10-03 where all three fixtures printed the expected
    denial, exit 0, with no execution attempt). Both run sets are samples of three on one machine, one
    model and one build.

Live end-to-end completions (3 October 2026), with where each is recorded. All used `qwen3.5:4b`, and
none is final-build evidence (X-59):

- Worker 2's GO-47 live run is **on `main`**: `docs/roadmap/go.md`, GO-47 block, the "Completed
  (2026-10-03)" line (commit `3aeade7`). `TestLiveStoryThroughTheProductionChain`, labelled live: step 1
  proposed several actions at once and was denied (`multiple_actions_not_supported`); then
  `read_invoice` for A01 and A02, `create_report` `vendor_reconciliation_v1`, `queue_report` awaiting
  approval; the reviewer approved; one simulated outbox message was queued to the registered address
  and the run completed (6 agent calls, 1 security call, outbox rows 1). That record says: "The model
  did not create the internal report in this run; the scripted run covers that beat."
- Lane 3c's run `66cbb01a` (GO-26 tick, commit `711ece3`, on `main`): completed after the vendor
  report was approved and queued, with `result_reference` `{"report_ids": [...]}` naming its own
  vendor report. The clean-checkout rehearsal run `e350fea7` (GO-27 progress note, commit `7ccacbb`, on `main`) was set up from a fresh `git clone` of `main` at `87f22f0` by the README and
  `docs/setup.md` only, and completed the same way (5 agent calls, 1 security call each).
- Lane f3's GO-27 runs (commit `a9f8004`, on `main`): runs `8b812e16`, `cfbd598b` and `b4a7a4c8` (run
  set 2 above; the last two completed with outbox 1) and the GO-27 live run `7c1bc441`, which reached
  `awaiting_approval` with the internal report labelled `internal_only`, source trail A01@1
  `internal_only` and A02@1 `vendor_shareable`, and the finding "INV104: A01, A02".

Setup caveats recorded by the lanes: the signature feed and the app records were loaded by hand or by
the test harness (`openStory`, `completeSeededCatalog`), not by the feed import (item 16) and the app
seed (item 17); the rehearsal imported the feed twice because the first import activates without
gateway validation. These runs show that the path can work on one machine. They stay "not final-build
evidence (X-59)" until the feed import and the app seed replace the hand-loaded setup.

The control evaluation adapter (X-91) is `POST /internal/control/evaluate` (GO-82); a `model_input`
evaluation never dispatches the agent model. On `main`: the route is mounted by `api.Commands` and its
request and response schemas are in `packages/contracts`; the `model_input` behaviour was not traced.

Known limitations recorded with these decisions: `command idempotency keys` is open, so two identical start-run requests create two runs; the signature feed has no signing key (item 9); there are no automatic model-call retries (item 24); the semantic check does not run on the four MVP tools' action proposals (item 25).

## Open items between the report and the architecture specification

Each item is open until the document owner records the outcome here; the roadmap cites them in
"Blocked by" (`docs/roadmap/README.md`, "Open decisions and blockers").

- `read path` (**decided by the lead's delegate**, item 1 above): the architecture reads authorized runtime views ("NestJS has read access only to
  authorized runtime views needed for the interface"); the report also says NestJS calls "corresponding
  private Go endpoints".
- `report storage` (**decided by the lead's delegate**, item 3 above): lineage in `runtime` (`report_lineage`) per the architecture, or with the reports in
  `demo` per the report; context manifests appear only in the report.
- `internal report rendering` (**decided by the lead's delegate**, item 4 above: deterministic templates, GO-71 dropped): whether the internal report may hold model-written text and so needs a
  conservative context manifest (report), or is rendered deterministically (architecture).
- `source classification storage` (**decided by the lead's delegate**, item 6 above): where the trusted source-field classifications and recipient rules
  live and who writes them.
- `vendor projection fields` (**decided by the lead's delegate**, item 5 above): which fields the vendor projection may hold; the report's two example
  lists differ, and the final list is a team policy decision.
- `passport report fields`: singular or plural destinations, template and projection fields.
- `policy editor`: the architecture lists a policy editor and a `/policies` route; the report keeps "A
  policy editor" outside the initial delivery scope.
- `rename operation`: no operation renames or copies a report in either source, yet the demonstration
  renames one.
- `list reads`: `/runs` and `/approvals` need list reads that neither source defines.
- `command idempotency keys`: the architecture puts idempotency keys on commands; the report on actions.
  Still open; until it is settled, two identical start-run requests create two runs (a known limitation).
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
  organizer confirmation. **Deadline confirmed in writing by the organizers on 3 October 2026**: editing
  stays open "until 11:00 AM tomorrow", so 11:00 AM on 4 October ([organizer-questions.md](organizer-questions.md),
  answer 2). The start time is not in that message; it stays 11:00 AM as the lead confirmed.
- `catalog activation protocol`, `classifier prompt and verdict schema`, `redaction rules`,
  `feed grammar and trust` and `measurement method`: implementation choices report 1.2 leaves open
  ("Configure these explicitly and record their tested limits"). **Decided by the lead's delegate**:
  the verdict schema (item 7 above), `redaction rules` (item 8) and `feed grammar and trust` (item 9).
  `catalog activation protocol` is decided too (item 15), `measurement method` (item 22) and the
  classifier prompt (item 23, measured on its fixture set only). Nothing in this group is still open.
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

On 3 October 2026, the user adopted report 1.2’s primary local-model path, replacing the earlier OpenAI `gpt-5.6-sol` choice. The user selected Ollama on a separate MacBook with an M1 Pro and 16 GB RAM. The user selected `qwen3.5:4b` as the current provisional model candidate; it may change after testing. It is not a frozen model contract. The hardware information is user-reported, not a measured performance result. GO-03 and SH-04 must record hardware fit, client choice, agent/security purpose accounting, reported token usage, call counts, request duration and reservation rules. The Go connectivity diagnostic passed on the developer M2/8 GiB machine; its exact outcome and limits are recorded in `services/gateway/README.md`. Production runtime governance and the shared model/hardware freeze remain unverified; the user adopted the Go-side accounting rules recorded below.

#### GO-03 implementation input (proposal, pending SH-04)

Report basis: "Technical architecture and service ownership", "Atomic allowances hard limits and estimated cost" and "Central policy configuration and safe reload".

- Use a small Go `net/http` client for Ollama's native `POST /api/chat`, with `stream: false`, bounded request/response sizes and a request deadline. No new provider dependency is proposed. Both agent and security requests use this client and the same reservation authority, with separate metered purposes.
- Record a dispatch count before every request, including retries. Set an explicit output ceiling and context limit for the chosen model. Input estimation alone is not a proven token upper bound: SH-04 must settle a conservative reservation strategy before a hard token-budget claim or live dispatch implementation.
- Read input/output counts from `prompt_eval_count` and `eval_count`; cached-token counts are supplementary metadata, not an additional token charge. Validate nonnegative integer fields and retain an unresolved reservation when required usage is absent or invalid. Missing usage is not zero.
- Measure request wall time in Go separately from Ollama's provider timing fields (nanoseconds). Cancellation or timeout does not prove the remote inference stopped; recovery and concurrency accounting must preserve that uncertainty.
- Local usage has no configured commercial tariff. Monetary cost is unavailable, rather than an invented zero bill. An optional future tariff uses explicit integer minor units or decimal arithmetic and is labelled estimated.
- Validate one supported action or a constrained final answer after each agent response. Ollama accepts tools and structured-output schemas, but that API support does not prove that the selected model follows the contract. GO-01 still rejects multiple actions; security verdicts are separately schema-validated and cannot grant authority.

Official references checked on 3 October 2026: [chat API](https://docs.ollama.com/api/chat), [usage metrics](https://docs.ollama.com/api/usage), [network and context configuration](https://docs.ollama.com/faq). These document API behavior; no request to the presentation machine has been made.

Before presentation-machine rollout: record the installed Ollama version, exact model identifier and digest, context/output settings, reachable endpoint and authenticated transport arrangement through infrastructure. Run an agent-response and semantic-verdict fixture on the M1 Pro machine, recording latency and memory fit. Ollama on the other machine is a network dependency of Go; do not assume this developer machine's localhost reaches it. GO-03, SH-04 and X-04 remain incomplete.

Model candidate reference: [Ollama qwen3.5:4b](https://ollama.com/library/qwen3.5:4b), checked on 3 October 2026. The installed model digest and agent/security suitability must be verified on the presentation machine. A model change before the freeze updates this decision and its checks; after admission it must obey the immutable passport and active model allowlist rather than silently substitute another model.

#### GO-06 developer connection evidence

On 3 October 2026 at 14:56:15 UTC, the developer M2/8 GiB machine completed two explicit synthetic Go provider calls through Ollama 0.35.1 to `qwen3.5:4b` (ID `2a654d98e6fb`), with `think: false`. Both passed the diagnostic response schema and reported 38 input and 6 output tokens; the command exited 0. The gateway README records the build, command and measured durations. This establishes developer-machine connectivity only; the later Go-side accounting adoption is recorded below, while the full GO-03/SH-04 model/hardware freeze remains pending. It does not establish a semantic security verdict, task reservations or presentation-machine readiness.

### GO-06 MVP accounting adopted with the user (3 October 2026)

The user settled the Go-side calculation dependency: reserve compact input JSON UTF-8 bytes
(including system prompt, history, tool results, tools and schemas) plus 1,024 template tokens and
maximum output before dispatch. Agent output is 512 and security output 256; both share an initial
20,000-token total. Native Ollama uses `think: false`, `stream: false` and `options.num_predict`.
The existing central policy/catalog carries these configurable settings; there is no second
configuration authority. The three optional v1 budget additions are documented in `config/README.md`.

Both valid counters on a completed response settle actual usage and refund unused allowance.
Missing usage or timeout retains the full reservation as `usage_unknown`; missing counters are
never zero. No automatic timeout retry occurs. Trusted late usage reconciles once. An overrun
records the complete measured count and pauses further model dispatch. TypeORM provides the
ledger migration; Go owns balance mutations. These settings are adopted MVP engineering choices,
not sponsor requirements or a proof that the byte estimate bounds every possible tokenizer input.

The gateway README records four successful live estimator fixtures and a live catalog-backed
agent/security accounting diagnostic on the developer M2/8 GiB machine. GO-06 is complete for the
user's expanded accounting scope. This does not settle presentation-machine readiness, the full
SH-04 hardware freeze, semantic verdict quality, catalog activation, service-role grants or the
future passport/worker wiring.
