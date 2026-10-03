# Team workflow and module ownership

A short guide for a team of six working in this repository at the same time. The binding rules are
in [AGENTS.md](../AGENTS.md); this document explains how to apply them. If the two ever disagree,
`AGENTS.md` wins and this file needs a fix.

## Roles and ownership

Six roles, each with a Claude Code project agent of the same name in `.claude/agents`. The roles
are organized by code area. The project report organizes the six people by responsibility, so one
person can drive more than one role and three people share the go role; the mapping of people to
roles is in the "Repository map and ownership" section of [AGENTS.md](../AGENTS.md).

| Role           | Owns                                                                                                                                                                                                                                                                                                                                                                                                         | Checks before reporting                                                                                                                             |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| frontend       | `apps/web`, `packages/ui`                                                                                                                                                                                                                                                                                                                                                                                    | `pnpm --filter web run lint\|typecheck\|test\|build`, `pnpm --filter @workspace/ui run lint\|typecheck`                                             |
| nestjs         | `apps/api`, including its entities and the migration tooling; `packages/contracts`                                                                                                                                                                                                                                                                                                                           | `pnpm --filter api run lint\|typecheck\|test\|build`; `pnpm --filter @workspace/contracts run lint\|typecheck\|test\|build` when a contract changed |
| go             | `services/gateway`, with one owner per Go package recorded in its README                                                                                                                                                                                                                                                                                                                                     | `pnpm --filter gateway run format:check\|lint\|typecheck\|test\|build`                                                                              |
| infrastructure | `infra`, `scripts` (`setup.mjs`, `compose.mjs`, `dev.mjs`, `with-env.mjs`, `lib`), `.env.example`, `.gitignore`, `.dockerignore`, `.nvmrc`, `.prettierignore`                                                                                                                                                                                                                                                | `pnpm run setup`; `pnpm infra:up` / `infra:down`, `pnpm stack:up` / `stack:down` and `pnpm smoke` where Docker exists                               |
| integration    | Migration files (`apps/api/src/database/migrations`, `db/migrations`), service database roles, synthetic fixtures and the reset procedure, `packages/config`, root `package.json`, `pnpm-workspace.yaml`, `turbo.json`, `pnpm-lock.yaml`, `AGENTS.md`, `CLAUDE.md`, `.claude/agents`, `scripts/check-instructions.mjs`, `scripts/verify.mjs`, `scripts/smoke.mjs`, `README.md`, `docs` except `docs/product` | `pnpm db:migration:show` when a migration changed, `pnpm check:instructions`, `pnpm format:check`, `pnpm verify`                                    |
| reviewer       | Nothing. Reads everything, edits nothing.                                                                                                                                                                                                                                                                                                                                                                    | Confirms that every claimed check has a quoted command and real output                                                                              |

The roles are a collaboration agreement, not filesystem isolation. Nothing technically prevents an
edit outside your paths; staying inside them is your responsibility.

`README.md` and `docs` have one owner, integration, except `docs/product`, which belongs to the
researcher (document owner) together with the project report. Every other role supplies the text
for its own area. The frontend, nestjs and infrastructure roles also run `pnpm exec prettier --check` on their
own paths; the full list of checks per role is in its agent file.

A feature belongs to the service that owns its responsibility. Do not place logic in a service
because it is convenient to reach from there.

## Single owner for shared assets

Three things affect everyone, so each has exactly one owner:

| Shared asset         | What it covers                                                                                                                                     | Who does the work                                                                                                                                                                                                   |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shared contracts     | `packages/contracts` (types, schemas, fixtures) and the matching Go DTOs                                                                           | nestjs (Implementer 2) coordinates and lands every change; each contract's recorded owner decides its shape after a quick shared review; Go stays the authority for action canonicalization; go applies the Go side |
| Dependency lockfiles | `pnpm-lock.yaml`, the `catalog`, `allowBuilds` and `minimumReleaseAgeExclude` in `pnpm-workspace.yaml`; `services/gateway/go.sum` together with go | integration runs the install and commits the lockfile                                                                                                                                                               |
| Migrations           | The single TypeORM toolchain in `apps/api`, used for every table, including tables the gateway reads                                               | integration (Implementer 5) writes and orders the migrations and owns the database roles; nestjs maintains the tooling and the `app` entities                                                                       |

The reason is practical: lockfiles and migration order do not merge well. One person changing them
in sequence avoids conflicts that cost more than the wait.

## Changing a shared contract

1. The person who needs the change agrees the new shape with the contract's recorded owner in a
   quick shared review (field names, optionality, which endpoint). The owners are listed in
   `docs/product/README.md`.
2. The nestjs owner edits, together: the type in `packages/contracts/src/index.ts`, the schema in
   `packages/contracts/schemas`, and the fixtures in `packages/contracts/fixtures`.
3. The nestjs owner runs `pnpm --filter @workspace/contracts run test` and
   `pnpm --filter @workspace/contracts run build`.
4. The go owner mirrors the shape in `services/gateway/internal/health/dto.go` and runs
   `pnpm --filter gateway run test`. The test decodes the shared fixtures with unknown fields
   disallowed and compares the values the handlers emit, so a mismatch fails. It does not validate
   against the JSON Schemas: a schema change that no fixture exercises must be mirrored by hand.
5. The nestjs owner updates the Swagger DTO classes (they implement the contract interfaces, so
   `pnpm --filter api run typecheck` fails until they match). The frontend owner updates the
   consumers in `apps/web`.
6. The nestjs owner runs `pnpm verify`. The contract change and its consumers are merged together or in
   direct succession, so the default branch never holds a half-applied contract.

Never change a response shape on one side only.

## Adding a dependency

Use exact versions, never ranges.

| Case                                                              | Where the version goes                                                           |
| ----------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Used by one workspace                                             | Exact version in that workspace's `package.json`.                                |
| Used by more than one workspace, or already listed in the catalog | `catalog` in `pnpm-workspace.yaml`; workspaces reference it as `"catalog:"`.     |
| Go module                                                         | `go get <module>@<version>` in `services/gateway`; commit `go.mod` and `go.sum`. |

Steps for an npm dependency:

1. Tell the integration owner the package, the exact version and the workspace.
2. Edit the `package.json` (and the catalog if shared).
3. The integration owner runs `pnpm install` and commits `pnpm-lock.yaml` together with the
   manifest change. Nobody else commits lockfile changes.
4. If pnpm reports `ERR_PNPM_IGNORED_BUILDS`, the integration owner decides the `allowBuilds` entry.
   If the version is younger than one day, prefer the previous version; pnpm otherwise records it
   in `minimumReleaseAgeExclude`, and that block must be committed.
5. Run `pnpm verify`.

For a Go module, the go owner makes the change and tells integration, which owns lockfiles across
the repository. The gateway uses the standard library wherever it is enough.

shadcn/ui components are added with the CLI from `apps/web` (see `packages/ui/README.md`); the
generated files land in `packages/ui` and belong to the frontend owner.

## Branching and integration habits

- Work in small branches and merge often. Pull or rebase before you start and before you merge.
- Keep the default branch green: run the checks for your area before merging and `pnpm verify`
  before anything that touches shared files.
- Announce changes to shared files (contracts, root configuration, `.env.example`, Compose files)
  before you make them, and hand them to the owner.
- A new environment variable is agreed between the infrastructure owner and the owner of the code
  that reads it, then added to `.env.example`, the Compose file and the README table in one change.
  Optional variables with a safe default (`API_HOST`, `GATEWAY_HOST`, `NODE_ENV`, `WEB_HOST`) are
  listed in the README's second table only.
- Run `pnpm format` before committing. `pnpm verify` fails on unformatted files.
- Whoever completes a task, person or agent, commits and pushes it right away, as "Committing and
  pushing" in `AGENTS.md` describes. Nothing is published or deployed automatically.
- Secrets stay in the untracked `.env`. The service token never appears in web code, a
  `NEXT_PUBLIC_*` variable, a response or a log line.

## No destructive resets

Other people's uncommitted or unmerged work may be in the tree you are looking at. Do not run:

- `git reset --hard`
- `git checkout -- <path>`
- `git clean -fd`
- a force-push

If the working tree contains changes you did not make, leave them and ask. The same applies to
data: the `down` scripts keep the database volume, and removing it with
`docker volume rm starter_postgres-data` is a deliberate, announced decision, never a routine fix.

## Report only what ran

- Do not say a check passed unless you ran it. Quote the command and its result.
- Say plainly what you could not verify and why (for example no Docker on your machine).
- `pnpm verify` treats a skipped step as a failure for the same reason.
- The reviewer treats a claim without evidence as unverified.

## Implementation workflow

The project is in the implementation phase. Every feature follows the same loop.

1. **Place it.** Decide which service owns the responsibility, using the ownership in the report.
   If several services are involved, agree the split with the owners involved before anyone writes
   code.
2. **Contract first.** For a cross-service feature, the contract's recorded owner agrees the shape
   in a quick shared review, nestjs lands it (types, schemas, fixtures), the go owner mirrors it and
   consumers follow. See "Changing a shared contract".
3. **Build inside your paths.** Add a module or package when its first real code lands; do not
   create empty product directories. Hand off anything outside your paths.
4. **Test what can break silently:** failure mapping, rejection of bad input or credentials,
   persistence and migrations. A test that restates a constant is not worth writing.
5. **Verify and quote.** Run your area's checks, then `pnpm verify`. Run `pnpm smoke` against the
   running stack before a merge that touches service wiring. Report the commands and their real
   results.
6. **Record it.** Add the module to "Product modules" in `docs/architecture.md`. Add new
   environment variables to `.env.example` (names only), the Compose files and the README table
   through the infrastructure owner. When a design decision changes, tell the document owner, so
   the report's decision record, contracts, demonstration and claim-to-proof list change together.
7. **Commit and push.** One commit per completed task, with its ID in the subject, then
   `git pull --rebase --autostash` and `git push` (`AGENTS.md`, "Committing and pushing"). Merge
   small and often.

The report settles several team-level decisions: four components and no message broker, one model
provider called only by Go, organization-scoped authentication in NestJS with a seeded demo
operator, and synthetic data in the `demo` schema. Still open for the whole team: which model
provider, the authentication mechanism, and the other open decisions in
[docs/product/README.md](product/README.md). Demo data
is an explicit, documented command, never loaded at application startup, and labelled as sample
data in the UI.

## Before implementation starts

1. **Commit the baseline.** Done: the starter baseline is on `main` in the team repository.
2. **Each person, once:** install Go 1.27 or newer and Docker with the Compose plugin, then run the
   commands below. Report the real result of `pnpm verify`.

   ```sh
   pnpm install
   pnpm run setup
   pnpm verify
   ```

3. **One person with Docker** validates the container path, which has never been executed, and
   records the result in the README section "Verification status":

   ```sh
   pnpm stack:up
   pnpm smoke --mode=container
   pnpm stack:down
   ```

4. **Settle the open design decisions** listed in `docs/product/README.md` and give each outcome
   to the researcher (document owner), who records it there. Start with the browser to API path and the operator context sent to Go, because the
   first contracts depend on them.
5. **Record one owner per contract and per Go package.** The report asks for this before parallel
   work starts. The nestjs owner gives contract owners to the researcher for `docs/product/README.md`; Go package owners go in
   `services/gateway/README.md` when each package is created.
6. **Confirm reuse with the organizers.** The report says not to presume that pre-event code or
   prepared assets are eligible. The researcher confirms with the organizers whether this starter
   may be used and how it must be disclosed. `docs/preparation-record.md` lists what was built
   before the event and its third-party licenses.

## Using the Claude Code project agents

The six files in `.claude/agents` define project agents that Claude Code picks up automatically
when it runs in this repository: `frontend`, `nestjs`, `go`, `infrastructure`, `integration` and
`reviewer`.

- Ask for an agent by name, for example "use the go agent to ..." or "have the reviewer agent
  check this change". Claude Code can also delegate to one on its own, based on the agent's
  description.
- Each agent carries its owned paths, its hand-off rules and the checks it must run and quote.
- All agents use `model: inherit` and the session's permission settings. None of them overrides a
  permission mode.
- `reviewer` has read-only tools (`Read`, `Grep`, `Glob`) and no shell. Give it the changed paths
  or the diff together with the commands that were run and their output.
- The agents follow the same rules as people: the phase, scope and guardrails in `AGENTS.md`, and
  the implementation workflow above.
- Agent scopes are instructions, not sandboxes. Review what an agent changed as you would review a
  teammate's work.
- `pnpm check:instructions` validates the agent files (required keys, allowed tools, `inherit`
  model, read-only reviewer, no permission overrides). It also fails when `AGENTS.md` and
  `CLAUDE.md` differ, contain an at-sign import or still carry the verification-status placeholder.

## Keeping `AGENTS.md` and `CLAUDE.md` in sync

`AGENTS.md` is the source of truth; `CLAUDE.md` is a byte-identical copy so that every tool reads
the same text.

```sh
# 1. edit AGENTS.md, then:
cp AGENTS.md CLAUDE.md
pnpm check:instructions
```

- Tools that write to `CLAUDE.md` directly cause drift. Move such an edit into `AGENTS.md` and copy
  again.
- Do not use at-sign file imports in these files; keep package names in backticks.
- When the team's phase or scope changes, update "Current phase" and "Scope" in `AGENTS.md`
  first.
- Only the integration owner edits these files.

## Adding the first entity and migration

The 2026-10-02 baseline has no entities, no migrations and no tables. When the team needs its first
table:

1. **Agree on it.** The owner of the feature and the migration owner (integration) agree that a
   migration is added now and where it sits in the order.
2. **Create the entity class** in the API module that owns the data:
   `apps/api/src/<module>/<name>.entity.ts`. Only NestJS-owned `app` tables become entities; set
   `schema: "app"` on them. Go-owned `runtime` and `demo` tables get no NestJS entity: the migration
   owner writes them by hand with `pnpm db:migration:create <Name>`.
3. **Register it** in the `entities` array of the shared options factory,
   `apps/api/src/database/typeorm-options.ts`. The NestJS runtime and the TypeORM CLI both use this
   file, so the entity is registered once.
4. **Generate the migration** with a running database:

   ```sh
   pnpm db:migration:generate <Name>
   ```

   `<Name>` is letters and digits, starting with a letter. The file is written to
   `apps/api/src/database/migrations/<timestamp>-<Name>.ts`. `generate` never emits
   `CREATE SCHEMA`: the first migration that touches a schema creates it by hand (see decision 1 in
   `docs/product/README.md`).

5. **Review the generated SQL.** Check both `up` and `down`. Generated code is a proposal.
6. **Format:**

   ```sh
   pnpm format
   ```

7. **Apply and check:**

   ```sh
   pnpm db:migration:run
   pnpm db:migration:show
   ```

8. **Commit the entity and the migration together**, in one change, through the migration owner
   (integration).

Rules that stay in force:

- Schema changes happen only through explicit migration commands. `synchronize`, `migrationsRun`
  and `dropSchema` stay `false`; nothing at application startup creates or changes tables.
- The Go gateway reads tables but never migrates. A table that only the gateway uses
  still gets its migration in `apps/api`. The gateway adds no migration framework.
- Teammates pick up a new migration by pulling and running `pnpm db:migration:run`.
- To undo the most recent migration locally: `pnpm db:migration:revert`.
- For a change that cannot be derived from entities, `pnpm db:migration:create <Name>` writes an
  empty migration to fill in by hand.
- With no entity change, `pnpm db:migration:generate <Name>` prints "No changes in database schema
  were found" and exits 1. That is the expected result, not an error in the tooling.
