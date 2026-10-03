# Infrastructure

Local container setup for the starter. Everything here is driven by the root scripts; you rarely
call `docker compose` yourself.

## Files

| File                        | Purpose                                                                           |
| --------------------------- | --------------------------------------------------------------------------------- |
| `compose.yaml`              | PostgreSQL (always on) plus `web`, `api` and `gateway` behind the `full` profile. |
| `compose.debug.yaml`        | Optional override that publishes the gateway port on localhost.                   |
| `docker/web.Dockerfile`     | Next.js standalone image (`node:24-alpine`, non-root).                            |
| `docker/api.Dockerfile`     | NestJS image with production dependencies only (`node:24-alpine`, non-root).      |
| `docker/gateway.Dockerfile` | Static Go binary on `alpine:3.24` (non-root, no Node.js).                         |

All images are built with the repository root as the build context, so the root `.dockerignore`
applies. Image builds need no `.env`, no database and no running service.

## Two modes

### Host development (default)

Only PostgreSQL runs in a container; web, api and gateway run on your machine.

```sh
pnpm run setup     # once: creates .env with generated local secrets
pnpm infra:up      # starts PostgreSQL and waits until it is healthy
pnpm dev           # web, api and gateway on the host
pnpm smoke         # checks the running services
pnpm infra:down    # stops PostgreSQL, keeps the data volume
```

### Full-container mode (`full` profile)

All four services run in containers.

```sh
pnpm stack:up                  # builds the images, starts everything, waits for health checks
pnpm smoke --mode=container    # gateway checks are reported as skipped (port not published)
pnpm stack:down                # stops everything, keeps the data volume
```

Inside the Compose network the services use container wiring instead of the `.env` host values:
`POSTGRES_HOST=postgres`, `POSTGRES_PORT=5432`, `GATEWAY_URL=http://gateway:8080`,
`API_UPSTREAM_URL=http://api:3001`, and for the gateway only
`MODEL_BASE_URL=http://host.docker.internal:11434` (the host's Ollama; unverified). The api and gateway images themselves set `API_HOST` /
`GATEWAY_HOST` to `0.0.0.0`, so they are reachable inside any container network; on the host the
default stays `127.0.0.1`.

Stop one mode before starting the other: both publish the same host ports. After `pnpm stack:up`
use `pnpm stack:down` (not `infra:down`) so the profiled containers are removed as well.

## What is published

| Service  | Host address                        | Notes                                                        |
| -------- | ----------------------------------- | ------------------------------------------------------------ |
| postgres | `127.0.0.1:${POSTGRES_PORT}` (5432) | Both modes.                                                  |
| web      | `127.0.0.1:${WEB_PORT}` (3000)      | Full-container mode.                                         |
| api      | `127.0.0.1:${API_PORT}` (3001)      | Full-container mode.                                         |
| gateway  | not published                       | Reachable only as `gateway:8080` inside the Compose network. |

Every published port is bound to localhost only. To reach the gateway from the host while
debugging, add the override file:

```sh
pnpm stack:up --debug      # also publishes 127.0.0.1:${GATEWAY_PORT} (8080)
pnpm stack:down --debug
```

## Secrets

`POSTGRES_PASSWORD` and `GATEWAY_SERVICE_TOKEN` are read from the root `.env` (or the real
environment) at start time. They are never written into these files or baked into an image, and
Compose stops with a clear message when one is missing. The `web` container receives neither the
service token nor any `POSTGRES_*` or `MODEL_*` variable, and `pnpm dev` / `pnpm dev:web` strip the same
variables from the environment of the web process on the host. Nested `.env` files (for example
`apps/web/.env.local`) are excluded from the build context as well.

## Data

Database files live in the named volume `starter_postgres-data`, mounted at `/var/lib/postgresql`
(the layout used by the PostgreSQL 18 images). The `down` scripts never remove it. `POSTGRES_USER`,
`POSTGRES_PASSWORD` and `POSTGRES_DB` only take effect when the volume is first created; to start
over with new values, remove the volume yourself with `docker volume rm starter_postgres-data`.

## Health checks

Each check uses a program that exists in its image: `pg_isready` over TCP for PostgreSQL,
`node -e fetch(...)` for web and api, and the gateway binary's own `-healthcheck` flag. They probe
liveness; database readiness is reported by the `/health/ready` endpoints and by `pnpm smoke`.
