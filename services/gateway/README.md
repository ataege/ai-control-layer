# gateway

Internal Go service of the starter: a small HTTP server with health endpoints, one
authenticated ping route and a PostgreSQL connection pool. It contains infrastructure only.

## Routes

| Route                | Purpose                                                                                |
| -------------------- | -------------------------------------------------------------------------------------- |
| `GET /health/live`   | Process liveness. Never touches PostgreSQL.                                            |
| `GET /health/ready`  | `200` when a PostgreSQL ping succeeds within `DATABASE_TIMEOUT_MS`, else `503`.        |
| `GET /internal/ping` | Requires `Authorization: Bearer <GATEWAY_SERVICE_TOKEN>`. Does not touch the database. |

Every other routed request returns the shared JSON error envelope (`404 not_found`,
`405 method_not_allowed`, `401 unauthorized`, `500 internal_error`). Every response produced by the
handler chain carries `x-request-id`: an inbound value is reused when it is 1-64 characters of
`[A-Za-z0-9._-]`, otherwise a new one is generated.

Requests that Go's `net/http` rejects or answers before routing (a malformed request line or URI
such as `/%zz`, oversized headers, `OPTIONS *`) get the standard library's plain response, without
the envelope, a request id or an access-log entry.

Response shapes live in `internal/health/dto.go` and mirror `packages/contracts`. The tests in
`internal/health/dto_test.go` read the shared fixtures in `packages/contracts/fixtures`:

- Each fixture is decoded strictly and re-encoded, so a field that is added, renamed or removed in
  the contract without being mirrored here fails `go test`.
- The status and service values the handlers emit (`ok`, `unavailable`, `up`, `down`, `gateway`)
  are compared with the gateway fixtures, so a changed value in a fixture fails too.

The tests do not validate responses against the JSON schemas. A schema change that no fixture
exercises (for example a new optional property) is not detected; mirror those by hand.

## Configuration

Read from environment variables (the root scripts pass the root `.env` to the process).

| Variable                | Default     | Notes                                                             |
| ----------------------- | ----------- | ----------------------------------------------------------------- |
| `GATEWAY_HOST`          | `127.0.0.1` | Bind address. The image and Compose set `0.0.0.0`.                |
| `GATEWAY_PORT`          | `8080`      |                                                                   |
| `GATEWAY_SERVICE_TOKEN` | required    | At least 32 characters, no leading or trailing whitespace.        |
| `POSTGRES_HOST`         | `localhost` |                                                                   |
| `POSTGRES_PORT`         | `5432`      |                                                                   |
| `POSTGRES_USER`         | required    | Must not be blank.                                                |
| `POSTGRES_PASSWORD`     | required    | Must not be blank. Any characters are safe; the value is escaped. |
| `POSTGRES_DB`           | required    | Must not be blank.                                                |
| `DATABASE_TIMEOUT_MS`   | `3000`      | Bounds one connection attempt and one readiness ping (100-20000). |
| `LOG_LEVEL`             | `info`      | `debug`, `info`, `warn` or `error`.                               |

`DATABASE_TIMEOUT_MS` is capped at 20000 so a readiness response always fits inside the server's
30 s write timeout. The API accepts the same range for this variable.

All problems are reported together in one JSON log line that names the variables and never their
values; the process then exits with code 1.

## Behaviour worth knowing

- The service starts while PostgreSQL is down. The pool connects lazily and readiness reports the
  state; no restart is needed once the database is back.
- The service never creates tables or runs migrations.
- Logs are JSON lines on stdout. The access log records method, path, status, duration and
  request id only. Health probes are logged at `debug`.
- Secrets are wrapped in `logging.Secret`, which prints `[REDACTED]` through `slog`, `fmt` and
  `encoding/json`. Use `Reveal()` only where the value is consumed.
- The readiness response carries a fixed message; the driver error (which names host and user)
  goes to the server log only.
- `SIGINT`/`SIGTERM` drain in-flight requests (up to 8 s), then close the pool. Request contexts
  are cancelled by the signal, so a readiness ping still waiting on PostgreSQL answers `503` at
  once instead of holding up the drain.
- `gateway -healthcheck` requests its own `/health/live` and exits `0` or `1`. It is meant for
  container health checks and needs only `GATEWAY_HOST`/`GATEWAY_PORT`, which it trims the same
  way the server does.

## Layout

### Package ownership

As directed by the user on 3 October 2026, the user is the sole owner and implementer of Go work.
The report's Implementer 3/4/5 labels group responsibilities; they do not assign separate people
to this Go plan. See the SH-07 Go ownership update in `docs/product/README.md` for planned modules.

| Existing package      | Owner                      |
| --------------------- | -------------------------- |
| `cmd/gateway`         | User (sole Go implementer) |
| `internal/config`     | User (sole Go implementer) |
| `internal/logging`    | User (sole Go implementer) |
| `internal/database`   | User (sole Go implementer) |
| `internal/health`     | User (sole Go implementer) |
| `internal/httpserver` | User (sole Go implementer) |

New packages get their ownership row when their first real code lands.

```
cmd/gateway/          wiring, signals, -healthcheck
internal/config/      environment validation
internal/logging/     JSON slog logger, Secret
internal/database/    pgxpool construction
internal/health/      handlers and wire DTOs
internal/httpserver/  routes, middleware, error envelope, server lifecycle
scripts/go.mjs        pnpm/turbo wrapper around the Go toolchain (not part of the build)
```

## Proposed tool results and idempotency (GO-07)

**Draft, 3 October 2026; not a frozen contract or implemented behavior.** Owner: the user, sole Go
implementer. Sources: report, "Illustrative passport and interface contracts" (Proposed tool
argument boundaries; Concrete synthetic business example; Narrow final result and context
boundary) and "Durable state idempotency audit and uncertain outcomes".

SH-10 must freeze tool arguments and X-06 field rules before this draft is adopted. The lists below
describe candidate result fields, not final JSON property names or database columns. No provider,
limit value or shared wire contract is selected here.

| Tool            | Proposed result field allowlist                                                                                                                                               | Protected values                                                                                                                   |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `read_invoice`  | Invoice reference and version, associated vendor reference, external invoice reference, total in the agreed numeric format, status, synthetic untrusted note used in the demo | Raw fields outside the frozen allowlist are omitted; protected values required by the workflow remain run-scoped opaque references |
| `read_vendor`   | Vendor reference and version, synthetic display name, trusted recipient reference                                                                                             | Recipient address remains an opaque reference; no raw contact or bank details reach the model                                      |
| `create_report` | Stored report reference and version, authorized source invoice references, registered template reference, structured duplicate-reference finding                              | No unrestricted model prose or raw protected values; exact finding shape awaits the freeze                                         |
| `queue_report`  | Report reference and version, simulated outbox entry reference and queued status                                                                                              | Raw recipient and rendered review content stay out of model-facing results and general events                                      |

All four adapters verify organization and passport/resource relationships themselves. Opaque
references resolve only after authorization inside the adapter, and do not resolve in another run
or organization. A source version in a result is evidence for later precondition checks, not
authority. SH-10 must decide versions, the numeric format, the note's inclusion and the registered
template fields; this draft does not settle those open items.

Idempotency and retries follow the stable action identity:

- `read_invoice` and `read_vendor` have no business write. A completed action returns its persisted
  minimized result on recovery rather than silently reading a newer version under the same
  completed action. A known failed read may retry if fresh checks pass and allowance remains.
- `create_report` produces at most one report per action. A retry uses the same action and
  idempotency key, and verifies the original material and preconditions; it cannot replace the
  report with new content. Changed material requires a new proposal.
- `queue_report` produces at most one simulated outbox entry per action, using the frozen reviewed
  content and trusted recipient. A completed action returns the stored result. A retry cannot
  create a new action identifier to bypass uniqueness or obtain a broader approval.
- For both write tools, effect, completion and event share the transaction selected by SH-06;
  database uniqueness is tied to action identity. This guarantee is conditional on SH-06 and the
  migration constraints, not supplied by this draft.
- Known-safe failed attempts may retry only with the same frozen action, fresh authorization,
  current run/precondition checks and available allowance; each retry is a counted attempt.
  Where an approval was consumed, retries stay bound to that same action and grant.
- A timeout or lost connection is not proof of no effect. Establish the transaction outcome and
  former worker ownership as GO-02 requires; otherwise persist unknown outcome, pause for
  attention and do not blindly retry.

The simulated outbox creates a database record and sends no email. GO-17, GO-23, GO-31 to GO-35
and GO-53 will test the adopted field rules, direct adapter authorization, stable identities,
duplicate prevention and uncertain outcomes. GO-07 stays open until SH-10 and X-06 are settled.

## Commands

Go 1.27 or newer must be on `PATH`. The wrapper prints install guidance and exits non-zero when
it is missing; it never installs anything.

From the repository root:

```sh
pnpm dev:gateway                      # build bin/gateway-dev and run it with the root .env
pnpm --filter gateway run build       # go build -trimpath -o bin/gateway ./cmd/gateway
pnpm --filter gateway run test        # go test ./...
pnpm --filter gateway run lint        # go vet ./... and a gofmt check
pnpm --filter gateway run typecheck   # go build ./...
pnpm --filter gateway run format      # gofmt -w .
```

The dev command compiles `bin/gateway-dev` and runs that binary as a direct child instead of using
`go run`, so the service stays in the caller's process group: stop signals are forwarded to it,
and a forced kill of the group (as `pnpm dev` does after its grace period) cannot leave it behind.
Restart the command to pick up code changes.

Plain Go commands work the same from `services/gateway` (for example `go test -race ./...`).
`package.json` and `scripts/go.mjs` only connect the module to the workspace scripts.

## Renaming the module path

The module is called `starter/services/gateway`. To publish it under your own path:

```sh
cd services/gateway
go mod edit -module github.com/your-org/your-repo/services/gateway
# macOS sed shown; on Linux use `sed -i` without the empty string
grep -rl '"starter/services/gateway/' --include='*.go' . \
  | xargs sed -i '' 's#"starter/services/gateway/#"github.com/your-org/your-repo/services/gateway/#g'
gofmt -l . && go build ./... && go test ./...
```
