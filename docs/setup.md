# Setup notes

Longer version of the quick start in the [README](../README.md). Run every command from the
repository root unless stated otherwise.

## 1. Tools per operating system

Required versions:

| Tool    | Version                               | Check                    |
| ------- | ------------------------------------- | ------------------------ |
| Node.js | `>=24.15.0 <25` (`.nvmrc`: `24.18.0`) | `node --version`         |
| pnpm    | `11.10.0`                             | `pnpm --version`         |
| Go      | `1.27` or newer                       | `go version`             |
| Docker  | Engine with the Compose plugin        | `docker compose version` |

The starter installs none of these and changes no machine-wide setting. `pnpm run setup` reports
what it finds: a wrong Node.js version or a missing pnpm is a failure, a missing Go or Docker is a
warning, and a pnpm version other than the pinned one is a warning.

### macOS

- Node.js 24: a version manager that reads `.nvmrc`, or the official installer.
- pnpm: `corepack enable pnpm` where Corepack ships with your Node.js, or
  `npm install --global pnpm@11.10.0`. Either is your decision; both affect only your user or
  Node.js installation.
- Go: the official package from <https://go.dev/dl/> or your package manager.
- Docker: Docker Desktop or another engine that provides `docker compose`.

### Linux

Same tools. Use Docker Engine with the `docker-compose-plugin` package (the standalone
`docker-compose` v1 binary is not used). Your user must be allowed to talk to the Docker daemon.

Linux was not exercised on the preparation machine. The helper scripts use only Node.js and POSIX
process groups, so no difference is expected, but treat the first run as a verification.

### Windows through WSL 2

Native PowerShell or `cmd` is not supported: the development runner signals POSIX process groups,
which do not exist there. The web `dev` / `start` scripts themselves are small Node launchers
without shell syntax, but they were run on macOS only.

1. Install WSL 2 with a Linux distribution.
2. Inside WSL, install Node.js 24, pnpm 11.10.0 and Go 1.27+ as on Linux.
3. Clone the repository into the Linux filesystem (for example `~/code/<repo>`). Do not work under
   `/mnt/c`: file watching and installs are slow and unreliable there.
4. Docker: enable WSL integration for your distribution in Docker Desktop, or install Docker
   Engine inside WSL. Check with `docker compose version` inside WSL.
5. Run all `pnpm` commands in the WSL shell. Published ports are reachable from Windows browsers
   at `http://localhost:<port>`.

WSL was not exercised on the preparation machine.

## 2. First-run walkthrough

### Install

```sh
pnpm install
```

Expected: a warning about peer dependencies of ESLint plugins (harmless, see the README
troubleshooting table). The install fails on purpose when Node.js is outside the supported range.

### Create the local environment file

```sh
pnpm run setup
```

Never `pnpm setup` (pnpm built-in that edits the shell profile).

The script:

- prints an `[OK]`, `[WARN]` or `[FAIL]` line for Node.js, pnpm, Go and Docker with Compose;
- creates `.env` from `.env.example` if it does not exist, with file mode 0600;
- generates `POSTGRES_PASSWORD` (32 characters) and `GATEWAY_SERVICE_TOKEN` (48 characters) when
  they are empty, and prints only the names of the keys it generated;
- on later runs keeps every existing non-empty value untouched and appends keys that are new in
  `.env.example`;
- exits 1 only when Node.js is outside the supported range, pnpm is not on `PATH`, `.env.example`
  is missing or a generated key is still empty.

It is safe to run again at any time.

### Start PostgreSQL

```sh
pnpm infra:up
```

Starts the `postgres` service of `infra/compose.yaml` (`postgres:18-alpine`, named volume
`starter_postgres-data`, published on `127.0.0.1:5432`) and returns when the health check passes.

### Start the applications

```sh
pnpm dev
```

The runner loads `.env`, checks that Go is on `PATH`, builds `@workspace/contracts` once and
starts three processes with prefixed output (`web |`, `api |`, `gateway |`). If one process exits,
the runner stops the others and exits non-zero. Ctrl+C stops all three; a process that ignores the
stop signal is killed after 8 seconds.

The gateway is compiled to `services/gateway/bin/gateway-dev` and that binary is run, so it stays
in the runner's process group and is covered by the forced stop. It has no hot reload: restart
`pnpm dev` (or `pnpm dev:gateway`) after changing Go code. The web process is started without
`GATEWAY_SERVICE_TOKEN` and without any `POSTGRES_*` variable.

Both backends start even when PostgreSQL is down. They report it through their readiness endpoints
and recover without a restart once the database is reachable.

### Check

```sh
pnpm smoke
```

Then open <http://localhost:3000/diagnostics>. Four cards (API liveness, API readiness, gateway
reachability, gateway database readiness) show real responses; stop the database with
`pnpm infra:down` to see the unavailable states.

### Stop

Ctrl+C in the `pnpm dev` terminal, then:

```sh
pnpm infra:down
```

The volume, and therefore the data, is kept.

## 3. How environment loading works

There is one environment file, `.env` in the repository root. Workspaces have no `.env` files of
their own.

| Entry point                                         | How it gets the variables                                                                                                                                                                          |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pnpm dev`, `pnpm dev:web\|api\|gateway`            | `scripts/dev.mjs` reads `.env` and passes the merged environment to the API and the gateway; the web process gets it without `GATEWAY_SERVICE_TOKEN` and `POSTGRES_*`. Fails if `.env` is missing. |
| `pnpm db:migration:*`                               | `scripts/with-env.mjs` reads `.env`, then runs the command. Fails if `.env` is missing.                                                                                                            |
| `pnpm infra:*`, `pnpm stack:*`                      | `scripts/compose.mjs` passes `--env-file .env` to `docker compose`. Fails if `.env` is missing.                                                                                                    |
| `pnpm smoke`                                        | Reads `.env` for the ports and for the two secrets it searches for and sends.                                                                                                                      |
| API started directly (`node dist/main.js`)          | The API itself loads the repository-root `.env` if present. A missing file is fine when the variables are set.                                                                                     |
| `pnpm lint`, `typecheck`, `test`, `build`, `verify` | Need no `.env`. Turborepo runs tasks in strict environment mode, so only `NODE_ENV` and the Go variables pass through from your shell.                                                             |

Rules:

- **Real environment variables win.** A variable already set in your shell or by Compose is never
  replaced by the file. Example: `LOG_LEVEL=debug pnpm dev:api`.
- **`scripts/with-env.mjs`** is the generic wrapper: `node scripts/with-env.mjs <command> [args...]`
  runs any command from the repository root with `.env` loaded and propagates its exit code.
- **Validation happens at runtime, per service.** The API (zod schema) and the gateway validate at
  process start and name the offending variables, never their values. The web app reads
  `API_UPSTREAM_URL` only when a proxy route is called, so `next build` needs no environment. The
  web launchers reject a `WEB_PORT` that is not a whole number from 1 to 65535.
- **Containers do not read `.env` directly.** Compose interpolates the values it needs and
  overrides the wiring variables with service names (see the README, "Full-container mode"). The
  `web` container receives neither the service token nor any `POSTGRES_*` variable.
- **The web process on the host follows the same rule.** `pnpm dev` and `pnpm dev:web` remove
  `GATEWAY_SERVICE_TOKEN` and every `POSTGRES_*` variable from the environment of the web child,
  including ones set in your shell. `scripts/with-env.mjs` does not filter: it passes the full
  environment to whatever command you give it.
- **Secrets stay in `.env`.** Never copy them into `.env.example` or any tracked file.

## 4. Running a single service

```sh
pnpm dev:web        # Next.js only; needs no Go, no database
pnpm dev:api        # NestJS only; needs no Go
pnpm dev:gateway    # Go only; compiles bin/gateway-dev and runs it (restart to pick up changes)
```

What to expect when the neighbours are not running:

| Running alone | Behaviour                                                                                              |
| ------------- | ------------------------------------------------------------------------------------------------------ |
| web           | Pages load. The proxy routes answer 502 `upstream_unreachable`; the diagnostics page shows that state. |
| API           | Liveness 200. Readiness depends on PostgreSQL. `/api/diagnostics/gateway` answers 502 `unavailable`.   |
| gateway       | Liveness 200. Readiness depends on PostgreSQL. `/internal/ping` works with the token from `.env`.      |

Calling the gateway by hand without printing the token:

```sh
node scripts/with-env.mjs sh -c 'curl -s -H "Authorization: Bearer $GATEWAY_SERVICE_TOKEN" http://localhost:${GATEWAY_PORT:-8080}/internal/ping'
```

Per-workspace tasks:

```sh
pnpm --filter web run test
pnpm --filter api run test
pnpm --filter gateway run test
pnpm --filter @workspace/contracts run test
```

`web` and `api` import the compiled contracts. `pnpm dev*` and Turborepo build them automatically;
if you run a workspace script by hand on a fresh clone, run
`pnpm --filter @workspace/contracts run build` first.

Inside `services/gateway` the plain Go commands work as well (`go test ./...`, `go vet ./...`).

To serve a production build of the web app on the host, without a container:

```sh
pnpm --filter web run build
API_UPSTREAM_URL=http://localhost:3001 pnpm --filter web run start
```

`start` runs the standalone server on `WEB_PORT` (default 3000), bound to `WEB_HOST` (default
`127.0.0.1`), and exits with "No production build found" when the build is missing. It does not
read `.env`, so pass the variables it needs on the command line as shown; do not wrap it in
`scripts/with-env.mjs`, which would hand the secrets to the web process as well.

## 5. Running without the Compose-provided PostgreSQL

Docker is only one way to provide the database in host development. Any PostgreSQL 18 that is
reachable with the `POSTGRES_*` values works:

1. Create a role and a database on your PostgreSQL instance.
2. Set `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB` in
   `.env` to match.
3. Skip `pnpm infra:up`; run `pnpm dev` and `pnpm smoke`.

Notes:

- Both backends connect with the discrete variables; there is no connection URL variable.
- The connections use no TLS settings. This is a local development setup.
- The starter needs no extensions and creates no tables. The default `public` schema is enough.
- This is how the starter was verified on the preparation machine (PostgreSQL 18.4, not from the
  Docker image). Other PostgreSQL major versions were not tried.

## 6. Full-container mode

```sh
pnpm stack:up
pnpm smoke --mode=container
pnpm stack:down
```

`stack:up` builds three images (`infra/docker/*.Dockerfile`, build context is the repository root)
and waits for all health checks. The image builds need no `.env` values, no database and no running
service; starting the stack needs `.env` for the two secrets. Details and the `--debug` override:
[infra/README.md](../infra/README.md).

This mode was not executed on the preparation machine. See "Verification status" in the README.
