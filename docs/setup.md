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
- generates `POSTGRES_PASSWORD` (32 characters), `GATEWAY_SERVICE_TOKEN` (48 characters),
  `AUTH_JWT_SECRET` and `OPERATOR_CONTEXT_SIGNING_KEY` (64 characters each) when they are empty, and prints only the names of the keys it generated;
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
`GATEWAY_SERVICE_TOKEN`, `AUTH_JWT_SECRET`, `OPERATOR_CONTEXT_SIGNING_KEY`, any `POSTGRES_*` variable
and any `MODEL_*` variable; the gateway process without `AUTH_JWT_SECRET`; the API process without
any `MODEL_*` variable (see section 3 for the API's own `.env`
load). The local model is set up separately, in section 7.

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

| Entry point                                         | How it gets the variables                                                                                                                                                                                                                                                                                         |
| --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pnpm dev`, `pnpm dev:web\|api\|gateway`            | `scripts/dev.mjs` reads `.env` and passes the merged environment to the gateway without `AUTH_JWT_SECRET`; the API gets it without `MODEL_*`; the web process gets it without `GATEWAY_SERVICE_TOKEN`, `AUTH_JWT_SECRET`, `OPERATOR_CONTEXT_SIGNING_KEY`, `POSTGRES_*` and `MODEL_*`. Fails if `.env` is missing. |
| `pnpm db:migration:*`                               | `scripts/with-env.mjs` reads `.env`, then runs the command. Fails if `.env` is missing.                                                                                                                                                                                                                           |
| `pnpm infra:*`, `pnpm stack:*`                      | `scripts/compose.mjs` passes `--env-file .env` to `docker compose`. Fails if `.env` is missing.                                                                                                                                                                                                                   |
| `pnpm smoke`                                        | Reads `.env` for the ports and for the two secrets it searches for and sends.                                                                                                                                                                                                                                     |
| API started directly (`node dist/main.js`)          | The API itself loads the repository-root `.env` if present. A missing file is fine when the variables are set.                                                                                                                                                                                                    |
| `pnpm lint`, `typecheck`, `test`, `build`, `verify` | Need no `.env`. Turborepo runs tasks in strict environment mode, so only `NODE_ENV` and the Go variables pass through from your shell.                                                                                                                                                                            |

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
  `web` container receives neither the service token nor any `POSTGRES_*` variable. Only the
  `gateway` service lists `MODEL_BASE_URL` and `MODEL_NAME` (see section 6).
- **The web process on the host follows the same rule.** `pnpm dev` and `pnpm dev:web` remove
  `GATEWAY_SERVICE_TOKEN`, `AUTH_JWT_SECRET`, `OPERATOR_CONTEXT_SIGNING_KEY`, every `POSTGRES_*`
  variable and every `MODEL_*` variable from the environment of the web child, including ones set in your shell. `scripts/with-env.mjs` does not
  filter: it passes the full environment to whatever command you give it, `MODEL_*` included.
- **`MODEL_*` reach the gateway child only.** `pnpm dev` and `pnpm dev:api` also remove every
  `MODEL_*` variable from the environment of the API child. Known gap: the API then loads the root
  `.env` itself (`apps/api/src/config/app-config.module.ts`) for every key not already set, so on
  the host the running API process still holds `MODEL_BASE_URL` and `MODEL_NAME` when they are in
  `.env`. Both are non-secret, so no credential is exposed; how the API stops loading Go-only keys
  is open in SH-13. The API container has no `.env`, so this gap does not apply there.
- **`AUTH_JWT_SECRET` reaches the API only.** `pnpm dev` and `pnpm dev:gateway` remove it from the
  gateway child; the gateway does not load `.env` itself. In Compose only the `api` service lists it,
  and `OPERATOR_CONTEXT_SIGNING_KEY` is listed for `api` and `gateway` only.
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

This mode ran on 2026-10-03 on macOS with Docker Desktop; the results are in "Verification status"
in the README. The gateway container reaches the host's Ollama at
`http://host.docker.internal:11434` on Docker Desktop for macOS (see section 7 for the Linux caveat).

## 7. Local model (Ollama)

Report 1.2 makes a locally hosted model, for example through Ollama, the primary model path. Only
the Go gateway calls it. Ollama is not a fifth application component: it runs on the host, outside
Compose, and the starter neither installs nor starts it. Reading `MODEL_BASE_URL` and `MODEL_NAME`
in Go is GO-06's work on another branch; until it lands, the gateway ignores both variables.

### Install

- macOS: the app from <https://ollama.com/download>, or `brew install ollama`.
- Linux: the official install script from <https://ollama.com/download> (Linux tab).
- WSL 2: install Ollama inside the WSL distribution with the Linux script, so the gateway, which
  also runs inside WSL, reaches it on `localhost`. A Windows-native Ollama is not necessarily
  reachable on `localhost` from WSL 2. Not exercised on any team machine.

Check with `ollama --version`.

### Start the server

The macOS app keeps a server running while it is open. Otherwise start one in its own terminal:

```sh
ollama serve
```

Either way the server listens on `http://localhost:11434`, Ollama's default port.

### Pull a model

The model is not fixed yet (decision 6 in `docs/product/README.md`). The provisional model for both
the agent and the security purpose is `qwen3.5:4b` (Apache 2.0), probed on the presentation machine
(see "Hardware" below):

```sh
ollama pull qwen3.5:4b
```

Before adopting any model, read the license bundled with it, `ollama show --license <model>`, and
record it with the team's model choice. Some small models are licensed for research or evaluation
only (for example `qwen2.5:3b`, under the Qwen Research License Agreement).

### Verify

```sh
curl http://localhost:11434/api/tags
ollama run <model> "hello"
```

`/api/tags` lists the pulled models as JSON; `ollama run` should print a short answer and exit.

### Point the gateway at it

Set `MODEL_NAME` in `.env` to the tag you pulled (empty in `.env.example`), for example
`MODEL_NAME=qwen3.5:4b`. `MODEL_BASE_URL` defaults to `http://localhost:11434`; change it only if
your Ollama listens elsewhere. Neither variable is a secret, and local Ollama needs no credential,
so there is no API key variable.

Which processes receive them:

| Mode                        | Gateway                                                                                                                                                                                   | API                                                                         | Web  |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ---- |
| Host (`pnpm dev`)           | Both, from `.env` through the dev runner                                                                                                                                                  | Removed from the child environment, but re-read from `.env` (see section 3) | None |
| Full container (`stack:up`) | `MODEL_BASE_URL` is set by Compose to `http://host.docker.internal:11434` (a value in `.env` does not apply); `MODEL_NAME` from `.env`. Reachability verified on Docker Desktop for macOS | None                                                                        | None |

Container mode on Linux (unverified): `host.docker.internal` resolves through `extra_hosts`, but
Ollama listens on `127.0.0.1` by default, so the container cannot reach it. Do not fix that with
`OLLAMA_HOST=0.0.0.0`: Ollama has no authentication, and that setting exposes it on every interface,
the local network included. Docker Desktop on macOS reaches a loopback-bound Ollama without any change.

### Hardware

The presentation machine is the lead's MacBook Pro (M1 Pro, 16 GB); the Go implementer develops on
an M2 with 8 GB. Pick a model that runs on both, and record on each machine which model and
version it ran (SH-45):

| Machine                     | Hardware      | Model tag                                      | Ollama version   | Result                         |
| --------------------------- | ------------- | ---------------------------------------------- | ---------------- | ------------------------------ |
| Lead (presentation machine) | M1 Pro, 16 GB | `qwen3.5:4b` (ID `2a654d98e6fb`, 4.7B, Q4_K_M) | 0.35.1           | Probe on 2026-10-03, see below |
| Go implementer              | M2, 8 GB      | not yet recorded                               | not yet recorded | not yet recorded               |

`ollama list` shows the pulled tags and their IDs; `ollama --version` shows the version.

**Probe on the lead's machine, 2026-10-03** (Ollama 0.35.1, `POST /api/chat`, `stream: false`,
`think: false`, temperature 0; a quick check, not the benchmark GO-81 builds or a detection-quality
claim). The agent request offered one `read_invoice` tool; the security request sent one delimited
note with a JSON schema for `risk_category`, `score` (0 to 1) and `reason_code`.

| Model                                            | Runs | Agent tool call                       | Hostile note ("ignore previous instructions ...") | Clean note                                 | Latency after the first call              |
| ------------------------------------------------ | ---- | ------------------------------------- | ------------------------------------------------- | ------------------------------------------ | ----------------------------------------- |
| `qwen2.5:3b` (historical; research-only license) | 1    | Correct (`read_invoice`, invoice_A01) | `prompt_injection`, score 90 (outside the range)  | `data_exfiltration`, score 3 (wrong label) | 3.3 s agent (first call), 0.6-0.8 s guard |
| `qwen3.5:4b`                                     | 3    | Correct in all three runs             | `prompt_injection`, score 0.95, every run         | `none`, score 0.0, every run               | about 0.9 s agent, 0.8 s guard            |

Ollama did not enforce the schema's numeric range for `qwen2.5:3b`, so Go must validate every
verdict itself (report 1.2: "Go rejects unsupported fields and malformed scores"). `qwen3.5:4b` is the
provisional model for both purposes; the M2 8 GB machine still has to run it before it is fixed.

## 8. Deployment on the presentation machine

The demonstration runs on the lead's MacBook Pro (Apple M1 Pro, 16 GB), the machine the probe in
section 7 ran on. Everything runs locally: the four components (web, API, gateway, PostgreSQL)
through Docker Compose, and Ollama on the host. Nothing is deployed to a remote server, and no paid
service is used. Rehearsal record: "SH-30" under "Verification status" in the README.

Commands marked "not on `main` yet" belong to open roadmap tasks; until they land, that step does
not exist and the demonstration cannot include what it provides.

### Which mode

The live demonstration runs in **host mode** (decided by the lead on 2026-10-03): PostgreSQL in
Docker through `pnpm infra:up`, and web, API and gateway on the host through `pnpm dev`. It keeps
editing `config/policy.yaml`, the reload and the local model simple for the judges: the file is
edited in the checkout, and the gateway reaches Ollama on `localhost`.

**Full-container mode** (`pnpm stack:up`) stays the documented and verified alternative ("Alternative:
full-container mode" below). How the API would read `config/policy.yaml` there (a bind mount of
`config/`, or another path) is unverified; the import itself (API-32) is not on `main` yet.

Both modes passed their smoke checks on this machine (README, "Verification status").

### Once per machine

| Tool           | Version used                                | Check                                      |
| -------------- | ------------------------------------------- | ------------------------------------------ |
| Node.js        | 24.18.0 (`.nvmrc`)                          | `node --version`                           |
| pnpm           | 11.10.0                                     | `pnpm --version`                           |
| Docker Desktop | Docker 29.8.1, Compose v5.5.1 (running)     | `docker compose version`                   |
| Ollama         | 0.35.1                                      | `ollama --version`                         |
| Go             | 1.27.1 (builds and runs the gateway)        | `go version`                               |
| Model          | `qwen3.5:4b`, ID `2a654d98e6fb`, Apache 2.0 | `ollama list`, `ollama show --license ...` |

Install the tools as in sections 1 and 7, then pull the model and confirm its license and ID:

```sh
ollama pull qwen3.5:4b
ollama show --license qwen3.5:4b   # expect the Apache License 2.0
ollama list                        # the ID is the exact model build; record it
```

Keep Ollama bound to `127.0.0.1` (its default). Never set `OLLAMA_HOST=0.0.0.0`: Ollama has no
authentication. Docker Desktop for macOS lets the gateway container reach the loopback-bound
Ollama at `http://host.docker.internal:11434` without any change.

### From a clean checkout

```sh
git clone https://github.com/ataege/ai-control-layer.git task-passport
cd task-passport
git checkout <submission-commit>   # the frozen build; write the hash down
nvm use                            # or any other way to get Node.js 24.18.0
pnpm install --frozen-lockfile
pnpm run setup                     # creates .env with generated secrets, values not printed
```

Then set `MODEL_NAME=qwen3.5:4b` in `.env`.

If another checkout on this machine has used the default Compose project `starter` (for example the
everyday working copy), its volume `starter_postgres-data` still exists and holds that checkout's
database password, so the fresh `.env` would not authenticate; stopping the other checkout is not
enough. Give the fresh checkout its own project name and PostgreSQL port (two lines in its `.env`,
[infra/README.md](../infra/README.md), "Several checkouts on one machine"), and stop the other
checkout's web, API and gateway so ports 3000, 3001 and 8080 are free.

### Start

Use two terminals in the checkout.

```sh
# terminal 1
ollama run qwen3.5:4b --think=false "Say hello in one word."   # loads the model
pnpm infra:up                      # PostgreSQL in Docker, waits for its health check
pnpm db:migration:run              # applies pending migrations
pnpm dev                           # web, API and gateway; leave it running

# terminal 2, once the three services are up
pnpm smoke
```

- Seed (not on `main` yet: SH-18 for the policy fixture and synthetic records, SH-19 for the
  demonstration operator): runs after the migrations, through its documented command.
- The warm-up loads the model into memory (about 30 seconds on the first load in the rehearsal,
  then well under a second). Ollama unloads a model after five idle minutes by default; `ollama ps`
  shows whether it is loaded, so repeat the warm-up shortly before the presentation.
- `pnpm dev` prints every service's log in terminal 1. `pnpm smoke` in host mode reports its log
  leak checks as skipped, because it cannot read that terminal.

Open <http://localhost:3000>.

### Between rehearsals and before judging

- Reset the demonstration data with `pnpm reset:demo` or `make reset-demo` (SH-29, not on `main`
  yet). Do not remove the database volume as a reset: that deletes every table, and the migrations
  and seeds would have to run again.
- Run the control suite with `pnpm verify:controls` or `make verify-controls` (SH-47, not on `main`
  yet) and keep its machine-readable result with the build's commit hash.

### Stop

Ctrl+C in terminal 1 stops web, API and gateway (a process that ignores the signal is killed after
8 seconds), then:

```sh
pnpm infra:down                    # stops PostgreSQL, keeps the volume
```

### Alternative: full-container mode

```sh
pnpm infra:down                    # if host mode ran; Ctrl+C pnpm dev first
pnpm stack:up                      # builds the images on the first run, waits for the health checks
pnpm db:migration:run              # from the host: the API image has no migration tooling
pnpm smoke --mode=container
pnpm stack:down                    # removes the containers and the network, keeps the volume
```

- Migrations and seeds run from the host against the published PostgreSQL port, because the API
  image's runtime stage contains only the compiled API and its production dependencies.
- The gateway container reaches the host's Ollama at `http://host.docker.internal:11434`.
- `docker compose stop` waits 10 seconds per container by default, which covers the gateway's own
  8 second shutdown budget.
- Reading `config/policy.yaml` from a container is unverified (see "Which mode").

### Network and exposure

- Host mode: web (3000), API (3001) and gateway (8080) listen on `127.0.0.1`, and PostgreSQL is
  published on `127.0.0.1:5432`. The web launchers bind to `WEB_HOST` (default `127.0.0.1`); before
  that was added on 2026-10-03, `next dev` listened on every interface and answered on the Wi-Fi
  address, which also exposed the API through the web app's `/api` proxy. Leave `WEB_HOST` unset
  for the demonstration.
- Full-container mode: every published port is bound to `127.0.0.1` (web 3000, API 3001,
  PostgreSQL 5432); the gateway is not published (only `pnpm stack:up --debug` publishes 8080). All
  four containers share Compose's default network; the architecture's "NestJS and Go use a private
  service network" is open item `deployment network`.
- Judges work on the presentation machine itself; access from another machine is not set up (open
  item `judge access`).
- Secrets stay in the untracked `.env` of the checkout; the images contain no `.env` file.
- Ollama stays bound to `127.0.0.1`.

### Checklist before the presentation

- [ ] The checkout is at the submission commit; `git status` is clean.
- [ ] `ollama list` shows `qwen3.5:4b` with the recorded ID; the warm-up answered.
- [ ] `pnpm infra:up` and `pnpm dev` are running; `pnpm smoke` passed.
- [ ] `WEB_HOST` is unset, so the web app listens on `127.0.0.1` only (see "Network and exposure").
- [ ] Migrations and seeds ran; the demonstration data was reset.
- [ ] The control suite ran on this build and its result is saved.
- [ ] The laptop is on power and does not sleep (for example `caffeinate -dims` in a spare
      terminal).
