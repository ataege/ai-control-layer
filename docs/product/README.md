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
6. **Model provider. Provider type settled by report 1.2, model open.** "The primary self-contained
   model path would be a locally hosted model, for example through Ollama; one provider may serve agent
   and security requests under separate metered purposes." Go calls it and holds any credential. The
   criteria provide no paid subscriptions. Which local model, whether it fits the actual machines
   ("Choose a local model that runs on the actual machine"), the runtime and the accounting rule
   (tokens where reported, calls, request duration; optional estimated commercial cost) are open. Owner:
   the Go implementer with the lead (infrastructure).
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
