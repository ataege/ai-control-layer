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

```
cmd/gateway/          wiring, signals, -healthcheck
internal/config/      environment validation
internal/logging/     JSON slog logger, Secret
internal/database/    pgxpool construction
internal/health/      handlers and wire DTOs
internal/httpserver/  routes, middleware, error envelope, server lifecycle
scripts/go.mjs        pnpm/turbo wrapper around the Go toolchain (not part of the build)
```

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
