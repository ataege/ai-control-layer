# Team instructions

These instructions apply to every contributor and coding agent working in this repository.
`AGENTS.md` and `CLAUDE.md` are byte-identical copies; see "Keeping the instruction files in sync".

## Current phase

**Implementation.** The team is building the product, Task Passport, on top of the starter baseline for the HackYeah hackathon. This section was switched from "reusable scaffold preparation" on 2026-10-02.

This file does not describe the product. The product design is in `docs/product/`: the architecture and run lifecycle diagrams, and the open decisions between them and the starter. It is a design, not implemented code. If the feature you are about to build is not covered there, ask the integration owner before inventing architecture.

## Scope

Product features, entities, migrations, authentication and screens are now in scope. The baseline of 2026-10-02 still contains generic infrastructure only: health, diagnostics, configuration, error handling, HTTP clients, database connections (no tables), UI primitives and development tooling.

What was excluded during preparation, and where it stands now:

| Excluded during preparation                                                                | Status now                                                                                                                                      |
| ------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Task passports, agent loops, permissions or policy evaluation                              | In scope. Built in the service that owns the responsibility, after the owner and integration agree where it lives and what its contract is.     |
| Approval workflows, budget accounting, execution queues, tool execution or audit pipelines | In scope, same rule.                                                                                                                            |
| Product-specific controllers, DTOs, interfaces, screens or database records                | In scope.                                                                                                                                       |
| Invoices, vendors, reports, simulated payments or other business examples                  | In scope only if the product needs them. Label sample data as sample data.                                                                      |
| User entities, login, registration, sessions or authentication                             | In scope if the product needs them. Owned by nestjs, behind the existing `AuthProvider` interface. See the guardrails below.                    |
| Business migrations, seed data or database tables                                          | In scope through the single migration toolchain. Seed data only as an explicit, documented command.                                             |
| Live LLM calls, embeddings, vector databases or AI-provider integrations                   | Needs a team decision first: which provider, which service makes the calls, where the key lives (server side, in `.env`, never in the browser). |
| C++, Redis, Kafka, Kubernetes or additional services                                       | Needs a team decision first, recorded in `docs/architecture.md` together with the Compose, Dockerfile and smoke-check changes it requires.      |

### Guardrails that still apply

1. **No pre-created structure.** Add a module, package or directory when its first real code lands. No empty directories for future work, no stubs for features nobody is building, no unnecessary abstractions or generated layers.
2. **Authentication.** `UnimplementedAuthProvider` fails explicitly with 501 until a real provider replaces it. Never add an allow-all guard, a fabricated identity or a fake login. Health and diagnostics stay public in local development unless integration decides otherwise.
3. **No startup side effects.** Nothing at application startup runs migrations, creates tables or loads seed data.
4. **Truthful UI.** Diagnostics and health views show only real responses; never render a healthy state before one arrives. Data that is not real is labelled as sample data.

## Repository map and ownership

| Path                                                                         | Contents                                                                        | Owner role                               |
| ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | ---------------------------------------- |
| `apps/web`                                                                   | Next.js web app                                                                 | frontend                                 |
| `packages/ui`                                                                | Shared UI primitives (`@workspace/ui`)                                          | frontend                                 |
| `apps/api`                                                                   | NestJS API, generic database configuration                                      | nestjs                                   |
| `db/migrations`                                                              | Pointer README only; real migrations live in `apps/api/src/database/migrations` | nestjs (ordering decided by integration) |
| `services/gateway`                                                           | Go gateway                                                                      | go                                       |
| `infra`                                                                      | Compose files and container definitions                                         | infrastructure                           |
| `scripts`, including `scripts/lib`                                           | Setup and development helpers and their shared library                          | infrastructure                           |
| `.env.example`, `.gitignore`, `.dockerignore`, `.nvmrc`, `.prettierignore`   | Environment template and root tool/ignore files                                 | infrastructure                           |
| `packages/contracts`                                                         | Shared wire contracts (`@workspace/contracts`)                                  | integration                              |
| `packages/config`                                                            | Shared TypeScript, ESLint and Prettier configuration                            | integration                              |
| Root `package.json`, `pnpm-workspace.yaml`, `turbo.json`, `pnpm-lock.yaml`   | Workspace wiring                                                                | integration                              |
| `AGENTS.md`, `CLAUDE.md`, `.claude/agents`, `scripts/check-instructions.mjs` | Team instructions and project agents                                            | integration                              |
| `scripts/verify.mjs`, `scripts/smoke.mjs`                                    | Integration checks (changed together with infrastructure)                       | integration                              |
| `README.md`, `docs`                                                          | Project documentation (each role supplies the text for its own area)            | integration                              |
| Everything (read-only)                                                       | Review of scope, correctness and verification evidence                          | reviewer                                 |

Each role has a project agent in `.claude/agents`. The roles are a collaboration instruction, not filesystem isolation: nothing technically stops an edit outside your paths, so the discipline is yours.

### Single owner for shared assets

The **integration** role is the one owner of the three things that every service depends on:

- **Shared contracts**: `packages/contracts` (TypeScript types, JSON Schemas, fixtures) and keeping the Go DTOs in `services/gateway` in sync with them. The go owner applies the Go side; integration decides the shape.
- **Dependency lockfiles**: `pnpm-lock.yaml` and the `catalog` in `pnpm-workspace.yaml`. Changes to `services/gateway/go.sum` are coordinated with the go owner. Ask integration for a new shared version instead of editing the catalog.
- **Migrations**: there is a single migration toolchain, TypeORM in `apps/api`, run through the `db:migration:*` commands. The nestjs owner maintains that tooling; integration decides when a migration is added and in which order. The Go service must not add a migration framework.

## Commands

**Starter baseline verification status (2026-10-02, macOS arm64).** Work added during implementation is not covered by it. `pnpm install`, `pnpm run setup`, `pnpm dev`, `pnpm lint`, `pnpm format:check`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm verify`, `pnpm smoke` (host mode), `pnpm check:instructions` and the `db:migration:*` commands were run and behaved as documented. `pnpm infra:*`, `pnpm stack:*` and `pnpm smoke --mode=container` were NOT run, because Docker was unavailable on the preparation machine; treat them as unverified until they run on a machine with Docker. Details are in `README.md`, section "Verification status".

Run everything from the repository root. The commands below are the scripts in the root `package.json`.

| Command                      | Runs                                                                 | Purpose                                           |
| ---------------------------- | -------------------------------------------------------------------- | ------------------------------------------------- |
| `pnpm install`               | (pnpm)                                                               | Install all workspace dependencies                |
| `pnpm run setup`             | `node scripts/setup.mjs`                                             | Create the local `.env` with generated secrets    |
| `pnpm infra:up`              | `node scripts/compose.mjs infra-up`                                  | Start local infrastructure containers             |
| `pnpm infra:down`            | `node scripts/compose.mjs infra-down`                                | Stop local infrastructure containers              |
| `pnpm stack:up`              | `node scripts/compose.mjs stack-up`                                  | Start the whole stack in containers               |
| `pnpm stack:down`            | `node scripts/compose.mjs stack-down`                                | Stop the whole stack                              |
| `pnpm dev`                   | `node scripts/dev.mjs`                                               | Run web, API and gateway on the host              |
| `pnpm dev:web`               | `node scripts/dev.mjs web`                                           | Run only the web app                              |
| `pnpm dev:api`               | `node scripts/dev.mjs api`                                           | Run only the API                                  |
| `pnpm dev:gateway`           | `node scripts/dev.mjs gateway`                                       | Run only the gateway                              |
| `pnpm lint`                  | `turbo run lint`                                                     | Lint every workspace                              |
| `pnpm format`                | `prettier --write . && pnpm --filter gateway run format`             | Format all files                                  |
| `pnpm format:check`          | `prettier --check . && pnpm --filter gateway run format:check`       | Check formatting without writing                  |
| `pnpm typecheck`             | `turbo run typecheck`                                                | Typecheck every workspace                         |
| `pnpm test`                  | `turbo run test`                                                     | Run every test suite                              |
| `pnpm build`                 | `turbo run build`                                                    | Build every workspace                             |
| `pnpm verify`                | `node scripts/verify.mjs`                                            | Run the full static verification sequence         |
| `pnpm smoke`                 | `node scripts/smoke.mjs`                                             | Check the running services end to end             |
| `pnpm check:instructions`    | `node scripts/check-instructions.mjs`                                | Validate the instruction files and project agents |
| `pnpm db:migration:create`   | `node scripts/with-env.mjs pnpm --filter api run migration:create`   | Create an empty migration file                    |
| `pnpm db:migration:generate` | `node scripts/with-env.mjs pnpm --filter api run migration:generate` | Generate a migration from entity changes          |
| `pnpm db:migration:show`     | `node scripts/with-env.mjs pnpm --filter api run migration:show`     | List migrations and their state                   |
| `pnpm db:migration:run`      | `node scripts/with-env.mjs pnpm --filter api run migration:run`      | Apply pending migrations                          |
| `pnpm db:migration:revert`   | `node scripts/with-env.mjs pnpm --filter api run migration:revert`   | Revert the latest migration                       |

Always write `pnpm run setup`. Bare `pnpm setup` is a pnpm built-in that edits the shell profile; it does not run this repository's script.

One workspace at a time: `pnpm --filter <name> run lint|typecheck|test|build`, where `<name>` is `web`, `api`, `gateway` or `@workspace/contracts`. `@workspace/ui` has only `lint` and `typecheck`, and `@workspace/config` has no scripts.

No migration exists yet. The first one is added together with the first entity (see `docs/team-workflow.md`). Nothing runs migrations or creates tables at application startup.

## Implementation workflow

Every feature follows the same loop. `docs/team-workflow.md` has the details.

1. **Place it.** Decide which service owns the responsibility. If several services are involved, agree the split with the integration owner before anyone writes code.
2. **Contract first.** For a cross-service feature, integration lands the shared shape (types, schemas, fixtures), then the go owner mirrors it in the Go DTOs, then consumers follow.
3. **Build inside your paths.** Hand off anything outside them to the owner.
4. **Test what can break silently:** failure mapping, rejection of bad input or credentials, persistence. Do not write tests that restate a constant.
5. **Verify and quote.** Run your area's checks, then `pnpm verify`. Run `pnpm smoke` against the running stack before a merge that touches service wiring.
6. **Record it.** Add the module to "Product modules" in `docs/architecture.md`. Add new environment variables to `.env.example` (names only), the Compose files and the README table through the infrastructure owner.

## Working rules

1. **Preserve teammates' changes.** No destructive resets: no `git reset --hard`, no `git checkout -- <path>`, no `git clean -fd`, no force-push. If the working tree contains changes you did not make, leave them and ask.
2. **Coordinate shared files.** Before changing a file that another role owns, hand the change to that owner or agree on it first. Never edit another owner's paths silently.
3. **Integrate continuously.** Small merges, pull or rebase often, keep the default branch green. Do not let a long-lived branch drift.
4. **Features go where the responsibility lives.** Every future feature belongs to the service that owns its responsibility. Do not put logic in a service because it is convenient to reach from there.
5. **Report only what ran.** Do not claim a check passed unless it ran. Quote the exact command and its result; say plainly what was not verified and why.
6. **Keep the phase current.** When the team's phase or scope changes, the integration owner updates "Current phase" and "Scope" first. Do not start work the current phase does not cover.
7. **Nothing leaves the machine automatically.** No automatic pushing, publishing or deployment. Agents never push; a person pushes after checking the work.
8. **No secrets in tracked files.** Secrets live in the untracked `.env` created by `pnpm run setup`. Never log or return them.
9. **The service token never reaches the browser.** `GATEWAY_SERVICE_TOKEN` is held by the API and the gateway only: never in a `NEXT_PUBLIC_*` variable, a response, a log line or the web app.
10. **Readable code.** Descriptive names, no cryptic abbreviations. Comments explain intent and stay concise.

## Keeping the instruction files in sync

`AGENTS.md` is the source of truth. `CLAUDE.md` is an exact copy so that every tool reads the same text.

1. Edit `AGENTS.md`.
2. Copy it: `cp AGENTS.md CLAUDE.md`.
3. Run `pnpm check:instructions`.

The check fails when the two files differ by a single byte, and it also validates the agent files in `.claude/agents`.

- Tools that write to `CLAUDE.md` directly cause drift. Move such an edit into `AGENTS.md` and copy again.
- Do not use at-sign file imports in these files. Keep package names inside backticks so they are never read as an import.
- `turbo.json` sets `"agentGuidance": false` so that Turborepo does not append its own block to `AGENTS.md`. Keep that setting.
