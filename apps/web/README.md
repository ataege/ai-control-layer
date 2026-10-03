# web

Next.js App Router front end (React, strict TypeScript, Tailwind CSS v4). UI primitives come from
[`@workspace/ui`](../../packages/ui/README.md); response types come from `@workspace/contracts`.

## Pages

| Route          | Purpose                                                            |
| -------------- | ------------------------------------------------------------------ |
| `/`            | Task setup and overview                                            |
| `/tasks/new`   | Alias for task setup                                               |
| `/runs/[id]`   | View run details and execution timeline                            |
| `/components`  | Showcase of every shared primitive and generic component           |
| `/diagnostics` | Live health of the API, the gateway and their database connections |

## Server proxy

The browser only talks to this app. Route handlers forward to the API using the proxy helpers:

| Route                          | Upstream                                      |
| ------------------------------ | --------------------------------------------- |
| `GET /api/health/live`         | `${API_UPSTREAM_URL}/api/health/live`         |
| `GET /api/health/ready`        | `${API_UPSTREAM_URL}/api/health/ready`        |
| `GET /api/diagnostics/gateway` | `${API_UPSTREAM_URL}/api/diagnostics/gateway` |
| `GET /api/runs/*`              | `${API_UPSTREAM_URL}/api/runs/*`              |
| `POST /api/runs/*`             | `${API_UPSTREAM_URL}/api/runs/*`              |
| `GET /api/auth/me`             | `${API_UPSTREAM_URL}/api/auth/me`             |
| `POST /api/auth/sign-in`       | `${API_UPSTREAM_URL}/api/auth/sign-in`        |
| `POST /api/auth/sign-out`      | `${API_UPSTREAM_URL}/api/auth/sign-out`       |

The shared helper is `src/server/upstream-proxy.ts`. It passes the upstream status and JSON body
through, forwards or generates `x-request-id` and session cookies, but never forwards the caller's path, query, or
other headers. Failures of the proxy itself use the shared `ErrorResponse` envelope:

| Status | Code                        | Meaning                                          |
| ------ | --------------------------- | ------------------------------------------------ |
| 500    | `configuration_error`       | `API_UPSTREAM_URL` is missing or invalid         |
| 502    | `upstream_unreachable`      | The API refused or dropped the connection        |
| 502    | `upstream_invalid_response` | The API answered with something that is not JSON |
| 504    | `upstream_timeout`          | No complete answer within 10 seconds             |

To proxy another API route, create a matching `route.ts` that calls `proxyUpstream`.

## Environment

| Variable           | Used for                                       | Default     |
| ------------------ | ---------------------------------------------- | ----------- |
| `API_UPSTREAM_URL` | Base URL of the API, read at request time only | none        |
| `WEB_PORT`         | Port for `dev` and `start`                     | `3000`      |
| `WEB_HOST`         | Bind address for `dev` and `start` (optional)  | `127.0.0.1` |

There are no `NEXT_PUBLIC_*` variables, and this app never reads the service token or database
settings. The build needs no environment at all.

## Scripts

Run from the repository root with `pnpm --filter web run <script>`:

| Script      | Does                                                        |
| ----------- | ----------------------------------------------------------- |
| `dev`       | `next dev` on `WEB_HOST`:`WEB_PORT` (via `scripts/dev.mjs`) |
| `build`     | `next build` (standalone output in `.next/standalone`)      |
| `start`     | Standalone server on `WEB_PORT` (via `scripts/start.mjs`)   |
| `lint`      | ESLint                                                      |
| `typecheck` | `next typegen`, then `tsc --noEmit`                         |
| `test`      | Vitest (fetch helper, proxy, check interpretation)          |

`pnpm run dev:web` from the root loads the root `.env` first. `@workspace/contracts` must be built
once (`pnpm --filter @workspace/contracts run build`); Turborepo does this automatically.

`dev` and `start` are small Node launchers in `scripts/`, so they behave the same in every shell.

`start` needs a finished `build`. `next start` does not support `output: "standalone"`, so the
launcher copies `.next/static` and `public` next to the standalone server, then runs
`.next/standalone/apps/web/server.js` with `PORT` set from `WEB_PORT` and `HOSTNAME` set from
`WEB_HOST`. An inherited `HOSTNAME` is ignored because many shells export the machine name there. The
container image runs the same server directly (see `infra/docker/web.Dockerfile`).

## Adding a shadcn component

Run the CLI from this directory. It writes the component into `packages/ui/src/components/`:

```bash
cd apps/web
pnpm dlx shadcn@4.21.1 add <component>
```

Then run `pnpm run format` at the repository root and import it as
`@workspace/ui/components/<component>`.

## Where code goes

- `scripts/` Node launchers for the `dev` and `start` package scripts
- `src/app/` routes, with page-specific components next to their page
- `src/components/` components shared by several pages of this app
- `src/lib/` browser-safe helpers (`fetch-json.ts`, `service-checks.ts`)
- `src/server/` server-only code (imports `server-only`)
- Reusable, product-neutral components belong in `packages/ui`
