# Product design: Task Passport

Task Passport is the team's entry for the Goldman Sachs AI Control Layer challenge at HackYeah 2026.
This folder holds its product definition. Everything here is a proposed design, not a record of
implemented behaviour; [docs/architecture.md](../architecture.md) describes what exists in the
repository. Owner: the document owner. No researcher is assigned yet; the lead acts as document owner
until the team assigns the role.

| File                                                                   | Content                                                                                                                                                                                                                                                                                                                                                                       |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [task-passport-project-report.docx](task-passport-project-report.docx) | Project report, version 1.1, "Report provenance and safe continuation", 3 October 2026: product definition, challenge mapping, users, MVP boundary, architecture, invariants, report provenance with two classifications and two fixed templates, the denied export and the vendor continuation, team plan, demonstration, validation plan, risks and illustrative contracts. |
| [project-architecture.md](project-architecture.md)                     | Architecture specification: system architecture, task execution flow and report information flow as Mermaid source, service ownership, routes, modules, data ownership, interfaces, tools and key checks. Proposed design; kept verbatim (listed in `.prettierignore`).                                                                                                       |
| [competition-rules.pdf](competition-rules.pdf)                         | Terms and conditions of the "AI Control Layer" competition. The PDF prints the start and end times as 11:00 PM; the team lead confirmed that both are 11:00 AM: start no earlier than 11:00 on 3 October 2026, submission no later than 11:00 on 4 October 2026. `AGENTS.md`, "Current phase", summarizes the rules.                                                          |

The report lists its internal sources as `project-architecture.md`, `project-architecture.mmd`,
`task-execution-flow.mmd`, `report-information-flow.mmd` and `CLAUDE_SETUP_PROMPT.md`. Only
`project-architecture.md` is in the repository; it embeds the Mermaid sources. The SVG exports of
version 1.0 were removed. The report says "The original Mermaid sources and this report describe the
same updated design"; the open items below record where they still differ.

The report's example limits (call counts, attempts, expiry) and example records are illustrative
values, not requirements. Its validation targets are not test results. The approved vendor field list
is a team policy decision ("The final approved field list would be a team policy decision, not a
sponsor requirement"), fixed at the M0 freeze.

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
  toward Go. It forwards commands; it does not perform agent effects or write runtime decisions.
- **Go** is the final authority for every agent operation: admission and the immutable passport, the
  bounded agent loop, the model gateway, the action gate, report provenance and export decisions
  (trusted source lineage, inherited classification, template and projection checks), exact-action
  approvals, allowance reservations, the tool executor, the four typed tool adapters, data
  minimization, audit events and the runtime repository. It holds the model provider and tool
  credentials and is the authority for action canonicalization and artifact classification.
- **Next.js** presents task setup, run detail, report classifications, source trails and export
  explanations, the safe-continuation explanation, exact-action review and activity. It calls NestJS,
  reads stored classifications instead of computing labels, and cannot grant scope or execute tools.
  Whether a policy editor and the `/policies` route are in scope is open (`policy editor`).

| Schema    | Main records (report 1.1)                                                                         | Table names (architecture proposal, adopted at the M0 freeze unless recorded otherwise)                                                              | Write authority                             |
| --------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `app`     | Memberships, task versions, source and template policy versions, tool definitions and revocations | users, organizations, memberships, task_templates, task_versions, policy_versions, tool_definitions, report_templates, projection_rules, revocations | NestJS                                      |
| `runtime` | Passports, runs, context manifests, jobs, actions, approvals, usage and decision events           | passports, runs, jobs, model_calls, actions, approvals, execution_attempts, report_lineage, budget_reservations, usage_entries, audit_events         | Go                                          |
| `demo`    | Synthetic invoices, vendors, immutable reports with lineage, and simulated outbox                 | versioned invoices and vendor records, classified reports, outbox_messages                                                                           | Go tool adapters, through restricted access |

The two sources place report lineage differently (`report storage`), and neither names a table for
the trusted source-field classifications and recipient rules (`source classification storage`).
Report content and its lineage commit atomically in both.

## Contracts to freeze first

The report asks for these shared contracts, each with one recorded owner, before parallel work
starts, and to "Freeze report classification, source manifest, template version, projection-rule
version and export denial fields alongside those contracts." The web + API implementer coordinates
them in `packages/contracts`; each recorded owner decides its contract's shape after a quick shared
review, and Go stays the authority for action canonicalization and artifact classification.

| Contract                                                                                                                        | Owner        |
| ------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| Start-run request                                                                                                               | not recorded |
| Passport representation                                                                                                         | not recorded |
| Action proposal                                                                                                                 | not recorded |
| Approval decision                                                                                                               | not recorded |
| Run state                                                                                                                       | not recorded |
| Safe event                                                                                                                      | not recorded |
| Report and lineage summary (report 1.1: "Shared schemas should define supported requests, report lineage summaries and errors") | not recorded |
| Authenticated operator context (repository addition, not in the report's list; depends on decision 7)                           | not recorded |

Proposed reason vocabulary (report 1.1): `resource_out_of_scope`, `destination_not_allowed`,
`report_export_restricted`, `report_lineage_missing`, `source_policy_changed`, `template_not_allowed`,
`approval_required`, `approval_expired`, `action_changed`, `resource_version_changed`,
`allowance_exhausted`, `run_cancelled`, `outcome_unknown`. "Denial may identify an authorized
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
6. **Model provider. Credentials settled, provider open.** One provider, called only by Go, which holds
   the credentials. Which provider and model, and their documented accounting rule, are open. Owner: the
   Go implementer with the lead (infrastructure).
7. **Authentication mechanism. Open, and on hold by the user's decision of 2026-10-03.** NestJS
   authenticates users and checks organization membership ("AuthModule: login/session verification,
   organization membership and role checks"); one seeded operator may stand in for onboarding in a
   clearly labelled development demonstration. The credential and session mechanism is not decided; the
   recorded proposal in the roadmap (SH-01) stays proposed, not decided. Owner: the web + API
   implementer.
8. **Reuse of pre-event work. Open.** The report says not to presume that pre-event code or prepared
   assets are eligible. The competition rules say work starts no earlier than 11:00 on 3 October 2026
   and say nothing about reusing prepared code. The document owner confirms with the organizers whether
   this starter may be used and how it must be disclosed; `docs/preparation-record.md` is the
   disclosure record, and the repository history shows when each part was made.

## Open items between report 1.1 and the architecture specification

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
- `researcher role` and `shared-track assignment`: the two staffing items above.
