# Product design: Task Passport

Task Passport is the team's entry for the Goldman Sachs AI Control Layer challenge at HackYeah 2026. This folder holds its product definition. Everything here is a proposed design, not a record
of implemented behaviour; [docs/architecture.md](../architecture.md) describes what exists in the
repository. Owner: the researcher (document owner).

| File                                                                   | Content                                                                                                                                                                                                                             |
| ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [task-passport-project-report.docx](task-passport-project-report.docx) | Project report, version 1.0, design baseline of 3 October 2026: product definition, challenge mapping, users, MVP boundary, architecture, invariants, team plan, demonstration, validation plan, risks and illustrative contracts.  |
| [task-passport-architecture.svg](task-passport-architecture.svg)       | Components and who owns what: the Next.js interface, the NestJS application service, the Go gateway as agent execution authority, one PostgreSQL with the schemas `app`, `runtime` and `demo`, and the LLM provider.                |
| [task-passport-run-lifecycle.svg](task-passport-run-lifecycle.svg)     | The life of one run: task creation and admission, durable job claim with a lease, model allowance reservation and model call, policy decision, approval, recheck, tool execution with an idempotency key, retry or unknown outcome. |

The SVGs are rendered Mermaid exports. The report lists its internal sources as
`project-architecture.md`, `project-architecture.mmd`, `task-execution-flow.mmd` and
`CLAUDE_SETUP_PROMPT.md`; none of them is in the repository. Adding the `.mmd` files next to the
SVGs keeps the diagrams editable and diffable.

The report's example limits (call counts, attempts, expiry) are illustrative values, not
requirements. Its validation targets are not test results.

## Ownership

- **NestJS** owns users, organization access and membership checks, task templates, versioned
  policies, the tool catalog, the public API, the authorized activity feed and the runtime facade
  toward Go. It forwards commands; it does not perform agent effects or write runtime decisions.
- **Go** is the final authority for every agent operation: admission and the immutable passport,
  the bounded agent loop, the model gateway, the action gate, exact-action approvals, allowance
  reservations, the tool executor, the four typed tool adapters, data minimization and the runtime
  repository. It holds the model provider and tool credentials.
- **Next.js** presents task setup, run detail, exact-action review and activity. It calls NestJS
  and cannot grant scope or execute tools.

| Schema    | Main records                                                                      | Write authority                             |
| --------- | --------------------------------------------------------------------------------- | ------------------------------------------- |
| `app`     | Users, memberships, task versions, policy versions, tool definitions, revocations | NestJS                                      |
| `runtime` | Passports, runs, jobs, actions, approvals, usage and decision events              | Go                                          |
| `demo`    | Synthetic invoices, vendors, reports and the simulated outbox                     | Go tool adapters, through restricted access |

The people behind these components are listed in the "Repository map and ownership" section of
[AGENTS.md](../../AGENTS.md).

## Contracts to freeze first

The report asks for the first six of these shared contracts, each with one recorded owner, before
parallel work starts. The last row is a repository addition, not in the report's list; its shape
waits on decision 7. Implementer 2 coordinates them in `packages/contracts`; each recorded owner
decides its contract's shape after a quick shared review, and Go stays the authority for action
canonicalization.

| Contract                                                                    | Owner        |
| --------------------------------------------------------------------------- | ------------ |
| Start-run request                                                           | not recorded |
| Passport representation                                                     | not recorded |
| Action proposal                                                             | not recorded |
| Approval decision                                                           | not recorded |
| Run state                                                                   | not recorded |
| Safe event                                                                  | not recorded |
| Authenticated operator context (repository addition, depends on decision 7) | not recorded |

The report also proposes a reason vocabulary for decisions and errors, for example
`resource_out_of_scope`, `destination_not_allowed`, `approval_required` and `outcome_unknown`. Keep
whatever vocabulary is chosen identical across the UI, the runtime, the tests and the evidence.

## Decisions between the starter and the design

The numbers stay stable because the agent files refer to them. Write each outcome down here when it
is settled.

1. **Schemas. Settled by the report:** `app`, `runtime` and `demo` in one PostgreSQL instance, with
   the write authority in the table above. The starter still uses the default `public` schema. A
   throwaway experiment on 2026-10-03 with TypeORM 1.1.1 showed that `migration:generate` never
   emits `CREATE SCHEMA`, so the first migration of each schema creates it by hand. TypeORM's
   `migrations` bookkeeping table stays in `public`. Owner: integration (migrations) with nestjs.
2. **Database roles. Settled by the report:** each service gets explicit, narrow privileges.
   NestJS keeps separate application privileges. For the local prototype, the report recommends (not
   yet adopted) one Go executor connection that holds the runtime writes and the limited demo report and outbox operations, so the effect, its
   completion record and its event commit in one transaction. The starter still has one database
   user. Owner: integration (Implementer 5).
3. **Browser to API path. Open.** The Next.js proxy forwards no cookies or authorization headers and
   buffers a JSON response with a ten second timeout. Authenticated operator calls need a different
   route design, or a direct connection from the browser to the API with credentialed CORS. The
   report allows authenticated polling before server-sent events. Proposed, not decided: one
   same-origin route handler that forwards an allowlist of top-level API prefixes, the session cookie
   and a fixed set of headers, and streams the response. Owner: frontend with nestjs.
4. **Operator context to Go. Requirement settled, mechanism open.** The report requires Go to verify
   service identity and the authenticated operator context, then authorize each command against its
   organization and run. A signed user identifier alone is not enough when the user lacks permission
   for the specific action, so Go checks reviewer authority itself. The report makes this the first
   integrated deliverable of Implementer 2. The starter's service token proves reachability only and
   must be replaced or extended; the report leaves that choice open. Owner: nestjs with go.
5. **Background worker in Go. Settled by the report:** durable jobs in PostgreSQL, claimed with a
   lease and released during an approval wait, and no message broker. Lease expiry does not prove
   that a dispatched operation failed. The report names one worker process as the simple option for
   the prototype and adds more workers only on demonstrated need. Repository rule: the worker must be
   covered by graceful shutdown and by the readiness check. Owner: go (Implementer 3).
6. **Model provider. Credentials settled, provider open.** One provider, called only by Go, which
   holds the credentials. That needs a new variable in `.env.example` (name only), read by Go alone.
   Which provider and model, and their documented accounting rule for estimated cost, are still
   open. Owner: go (Implementer 3) with infrastructure.
7. **Authentication mechanism. Open.** The report requires NestJS to authenticate users and check
   organization membership, and allows one seeded operator in a clearly labelled development
   demonstration. Identity federation and production onboarding are deferred. The credential and
   session mechanism is not specified. Owner: nestjs (Implementer 2).
8. **Reuse of pre-event work. Open.** The report says not to presume that pre-event code or
   prepared assets are eligible. The researcher confirms with the organizers whether this starter
   may be used and how it must be disclosed; `docs/preparation-record.md` is the disclosure record.
