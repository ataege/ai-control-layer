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

## Known limits of the database and identity records

Checked by `src/database/app-entities.db-spec.ts` (`pnpm test:db api`); none of these is fixed by a
migration yet.

- Membership roles are a free `text[]` column with no CHECK: a typo in a role name is stored, and
  nothing in the database limits the values to the report's roles. The API only compares against
  the roles it knows (`reviewer`).
- The registry foreign keys (`task_templates`, `policy_versions`, `tool_definitions` to
  `organizations`) cascade on organization delete, while the membership foreign keys are
  `NO ACTION`: deleting an organization removes its registry rows but is refused while memberships
  exist.
- Membership uniqueness is per (user, organization) pair, not one organization per user. A user with
  several memberships gets the oldest one (`createdAt` ascending, in the default-deny guard); there
  is no organization switch.
- The registry tables (`app.task_templates`, `app.policy_versions`, `app.tool_definitions`) are
  unused by admission (report 1.2: the control catalog is the policy source). No Go code reads them,
  the gateway role has no grant on them, and nothing limits `tool_definitions.name` to the four
  registered tools. Cancelling a run (`POST /api/runs/{id}/cancel`) is checked at organization and
  run scope only: any member of the organization may cancel any of its runs.
- The API connects to PostgreSQL as the owner role (`POSTGRES_USER`); a least-privilege API role
  (`task_passport_api`, which exists but has no login and grants only on the catalog tables) is
  planned (API-17, deferred).

## Verification and handoff limits

API lint, typecheck and build passed; unit tests: 407 passed; database tests: 71 passed,
0 failed, 0 skipped; policy/shared contract tests: 9 passed. `pnpm verify` currently reports
4 passed, 2 failed, 0 skipped: the merged web has formatting failures and two homepage tests fail.
Those failures are not reported as passing and require the web owner. The real web/API
sign-in → profile → sign-out → revoked-profile check returned 200/200/200/401. These results
cover this branch's current code, not a frozen submission build. The sanitized audit evidence
and its build identifier are linked in the handoff.

The merged web middleware redirects `/` and `/components` to login. Current host smoke still
expects 200 there and reported 22 passed, 8 failed, 6 skipped after restarting the stack.
The web/script owners must settle that expectation; it is not recorded as passing.
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
The draft judge CLI needs the lead's X-91 update. Activity uses polling, not SSE.
API-15 browser presentation, teammate clean-checkout setup, Docker, and a new live-model
approval/outbox rehearsal were not verified in this API work. Fixtures prove contract and
authorization behavior, not semantic detection quality. Cookie auth has no production identity
federation, onboarding or production hardening claim.
