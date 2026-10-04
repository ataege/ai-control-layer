# api

NestJS 12 API (ESM, strict TypeScript). It authenticates operators through stored sessions, resolves
current organization membership and forwards runtime reads and commands to Go. Go owns execution,
approvals, classification, usage and tool effects; the API never writes runtime or demo decisions.

## Development demonstration setup

Follow the repository [setup instructions](../../docs/setup.md) for Node, pnpm, Go, Docker and the
local model. From the repository root, with the required tools installed:

```sh
pnpm install
pnpm run setup
pnpm infra:up
pnpm db:migration:run
pnpm db:roles
pnpm db:seed
pnpm catalog:activate
pnpm dev
```

The explicit seed creates the synthetic organization and `demo-operator@example.com`, labelled
Development Demonstration Operator, with `operator` and `reviewer` membership roles. Its password
comes from ignored `.env` variable `DEMO_OPERATOR_PASSWORD`, never a hard-coded credential.
Seeding is idempotent and refuses conflicting credentials/memberships. Nothing migrates or seeds
at application startup. `pnpm reset:demo` is the explicit reset procedure; see root documentation
for its destructive scope.

Passwords use scrypt. Sign-in sets a 24-hour HttpOnly, SameSite=Lax session cookie, Secure in
production; the database stores only the SHA-256 hash of its opaque identifier. Logout deletes
that stored session. Every protected request resolves current membership and roles again.
This is development-demonstration identity, not federation or production onboarding.

Required API variables are `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`,
`GATEWAY_URL`, `GATEWAY_SERVICE_TOKEN` and `OPERATOR_CONTEXT_SIGNING_KEY`. Optional settings and
defaults are defined in `src/config/environment.ts` and the root README environment table.
`GATEWAY_TIMEOUT_MS` defaults to 3000 ms; `COMMAND_TIMEOUT_MS` to 10000 ms; both accept 100–20000 ms.
`DEMO_OPERATOR_PASSWORD` belongs to the explicit seed, not the API runtime. Keep all secrets in
ignored `.env`; real environment variables take precedence. The service token and HS256 operator
context are sent only to Go's private routes, never the browser or logs.

## Routes

| Route                             | Purpose                                                                 |
| --------------------------------- | ----------------------------------------------------------------------- |
| `GET /api/health/live`            | Process liveness. No dependencies.                                      |
| `GET /api/health/ready`           | Read-only `SELECT 1` against PostgreSQL. `503` with the report if down. |
| `GET /api/diagnostics/gateway`    | Authenticated ping and readiness of the Go gateway.                     |
| `GET /api/docs`, `/api/docs-json` | Swagger UI and the OpenAPI document.                                    |

These infrastructure routes are public in local development. The diagnostics ping authenticates
the API service to Go; it does not require an operator session. Gateway readiness includes the
worker and catalog: HTTP 503 remains degraded even if Go's database check is up.

Authentication routes:

| Route                     | Purpose                                                                                 |
| ------------------------- | --------------------------------------------------------------------------------------- |
| `POST /api/auth/sign-in`  | Check email/password and issue the stored-session cookie.                               |
| `POST /api/auth/sign-out` | Delete the session and clear its cookie.                                                |
| `GET /api/auth/me`        | Verified user's id, email, name, organizationId and current membership roles; no-store. |

The full product route table and contracts are in the [runtime facade handoff](../../docs/api-facade-handoff.md).
Run state and usage are separate exact Go responses. Reviewer membership is required before Go
for action review, approval and audit export; summary requires verified organization membership.
Go returns 404 for unknown/foreign objects, preserving the organization boundary without existence leaks.
Approval forwards only the exact `decision` body, never replacement content or an API-created grant.
Commands whose deadline expires return 504 `outcome_unconfirmed`; transport/malformed reads fail
with 503. Known Go error statuses remain errors, with safe shared envelopes.

Diagnostics status mapping: both checks up `200 ok`; ping up but gateway not ready `503 degraded`;
ping timed out `504 unavailable`; ping unreachable, unauthorized or unexpected `502 unavailable`.
Every other error uses the shared `ErrorResponse` envelope with the request id.

## Commands

Run from the repository root (the root scripts load `.env`):

```sh
pnpm dev:api                          # watch mode
pnpm --filter api run build           # compile to dist/
pnpm --filter api run lint
pnpm --filter api run typecheck
pnpm --filter api run test
pnpm test:db api
pnpm verify
pnpm db:migration:show                # also: run, revert
pnpm db:migration:create <Name>
pnpm db:migration:generate <Name>
```

`pnpm --filter api run dev` and `node dist/main.js` also work on their own: the app loads the
repository-root `.env` itself, and real environment variables win over the file.

## Layout

- `src/config` - zod schema for the environment (`environment.ts`) and the typed `AppConfigService`.
  Validation runs at bootstrap and reports variable names only.
  `DATABASE_TIMEOUT_MS` and `GATEWAY_TIMEOUT_MS` accept whole milliseconds from 100 to 20000
  (default 3000), the same range as the gateway.
- `src/database` - one TypeORM options factory (`typeorm-options.ts`) shared by the Nest module and the
  CLI data source (`data-source.ts`). The app connects in the background with retries, so it starts
  without PostgreSQL. `synchronize` and `migrationsRun` are off: nothing creates tables at startup.
- `src/database/migrations` - the single TypeORM migration toolchain. See `db/migrations/README.md`.
- `src/health`, `src/diagnostics`, `src/gateway-client` - public diagnostics and the authenticated Go client.
  The readiness check runs on its own pooled connection and discards it when the check fails or
  times out, so a silently dropped connection cannot use up the pool.
- `src/common` - request id, request logging, exception filter, JSON logger.
- `src/auth`, `src/identity` - stored-session login, explicit demo seed, app identity entities and
  the global default-deny membership guard.
- `src/runs`, `src/actions`, `src/security` - strict Go facades for admission, run reads/events,
  stored reports, cancellation, frozen review/approval, judge evaluation and sanitized security reads.
- `src/registry`, `src/policies` - app configuration entities and the explicit policy/feed import;
  Go validates and activates requested revisions. Import is not activation.
- `src/testing` - test helper that mirrors the HTTP wiring of `main.ts` (excluded from the build).

## Notes

- Response types come from `@workspace/contracts`; build it first (`turbo` does this through `^build`).
- Swagger DTO classes implement the contract interfaces, so a contract change that is not mirrored
  fails the type check.
- The gateway token, database password and raw upstream bodies are never logged or returned.
- `"files": ["dist"]` in `package.json` limits `pnpm deploy` (the container image) to the compiled
  output; it does not affect the scripts above.
- Files created by the migration commands are not formatted; run `pnpm format` afterwards.

## Verification and handoff limits

Checked on 2026-10-04 on `api/w2`, which is main 31cff75 plus this branch's commits (not a frozen
submission build): API lint and typecheck exit 0; `pnpm --filter api run test` 428 passed;
`pnpm test:db --fresh` api 71 passed, gateway 965 passed, 0 failed, 0 skipped; `pnpm verify` 6 passed,
0 failed, 0 skipped; `pnpm smoke` (host mode) 36 passed, 0 failed, 6 skipped (the service-log leak
checks are skipped because host mode does not capture logs). The real sign-in, profile, sign-out and
revoked-profile flow returned 200/200/200/401, and a signed-in operator stays on `/tasks/new` with the
task form loaded. Nothing here called the live model. The sanitized audit evidence and its build
identifier are linked in the handoff.

API-12 form options forward GET /internal/task-options unchanged through the shared schema;
real Go/API/web-proxy reads returned 200 with identical bodies.
API-33 provides reviewer-only POST /api/policies/reload with exactly {} and GET /api/policies/status.
Reload reads the fixed repository config/policy.yaml and its named feed using the same bounded
reader as the CLI. The importer records reload/verified-user provenance and requests a revision;
Go's watcher activates it asynchronously. Responses are 202 requested, 200 unchanged,
400 with safe issue pairs or 409 revision_pending. Status shows stored revision pointers and the
sanitized last error. It never returns file content or waits for activation. Docker must make the
repository config files available at their expected path; missing files fail with 503.
Real authenticated API/Go checks observed unchanged 200, requested 202 followed by watcher
activation, invalid policy 400 with issue pairs and preservation of the active revision, and
the approved importer lastError mapping. The new catalog row recorded the verified actor.
The original configuration bytes were restored and reactivated after the check.

Known limits:

- The draft judge CLI (`pnpm judge`, `scripts/judge-client.mjs`) still sends `run_id` and omits the
  null fields; the API's X-91 request needs `runId` plus all of `kind`, `text`, `tool` and
  `arguments` and refuses unknown keys, so the CLI cannot reach a decision until it is updated.
  The API adds no compatibility defaults.
- Activity uses polling, not server-sent events (API-25 was not built).
- Organization access is proven through the public path with labelled Go response fixtures
  (`src/auth/product-access.db-spec.ts`: a second organization gets 404 on every object route).
  A live two-operator check is missing because no second-organization operator is seeded (API-23).
- Free-text fields the contract allows (for example an event's `safeMessage`) are relayed as Go sends
  them; the API cannot recognise a protected value inside them (API-24).
- Teammate clean-checkout setup, Docker, and a new live-model approval/outbox rehearsal were not
  verified in this API work. Fixtures prove contract and authorization behavior, not semantic
  detection quality. Cookie auth has no production identity federation, onboarding or hardening claim.
