# api

NestJS 12 API (ESM, strict TypeScript). The baseline ships generic infrastructure: health checks, a
gateway diagnostics endpoint, configuration, database wiring and error handling. Product modules are
added as `src/<module>` directories when their first code lands.

## Routes

| Route                             | Purpose                                                                 |
| --------------------------------- | ----------------------------------------------------------------------- |
| `GET /api/health/live`            | Process liveness. No dependencies.                                      |
| `GET /api/health/ready`           | Read-only `SELECT 1` against PostgreSQL. `503` with the report if down. |
| `GET /api/diagnostics/gateway`    | Authenticated ping and readiness of the Go gateway.                     |
| `GET /api/docs`, `/api/docs-json` | Swagger UI and the OpenAPI document for the three routes above.         |

Diagnostics status mapping: both checks up `200 ok`; ping up but gateway not ready `503 degraded`;
ping timed out `504 unavailable`; ping unreachable, unauthorized or unexpected `502 unavailable`.
Every other error uses the shared `ErrorResponse` envelope with the request id.

## Commands

Run from the repository root (the root scripts load `.env`):

```sh
pnpm dev:api                          # watch mode
pnpm --filter api run build           # compile to dist/
pnpm --filter api run lint | typecheck | test
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
- `src/database/migrations` - TypeORM migrations (empty in the starter). See `db/migrations/README.md`.
- `src/health`, `src/diagnostics`, `src/gateway-client` - the three routes and the outbound gateway client.
  The readiness check runs on its own pooled connection and discards it when the check fails or
  times out, so a silently dropped connection cannot use up the pool.
- `src/common` - request id, request logging, exception filter, JSON logger.
- `src/auth` - extension point only: `AuthProvider` interface, `AUTH_PROVIDER` token and a provider that
  answers 501. No guard, no user model, no endpoints.
- `src/testing` - test helper that mirrors the HTTP wiring of `main.ts` (excluded from the build).

## Notes

- Response types come from `@workspace/contracts`; build it first (`turbo` does this through `^build`).
- Swagger DTO classes implement the contract interfaces, so a contract change that is not mirrored
  fails the type check.
- The gateway token, database password and raw upstream bodies are never logged or returned.
- `"files": ["dist"]` in `package.json` limits `pnpm deploy` (the container image) to the compiled
  output; it does not affect the scripts above.
- Files created by the migration commands are not formatted; run `pnpm format` afterwards.
