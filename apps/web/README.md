# web

Next.js App Router front end (React, strict TypeScript, Tailwind CSS v4). UI primitives come from
[`@workspace/ui`](../../packages/ui/README.md); response types come from `@workspace/contracts`.

## Pages

| Route          | Purpose                                                            |
| -------------- | ------------------------------------------------------------------ |
| `/`            | Task form and a plain explanation of admission, passport and run   |
| `/login`       | Sign in as the seeded development demonstration operator           |
| `/tasks/new`   | The task form on its own page                                      |
| `/components`  | Showcase of every shared primitive and generic component           |
| `/diagnostics` | Live health of the API, the gateway and their database connections |

## Server proxy

The browser only talks to this app. Three route handlers forward to the API and nothing else:

| Route                          | Upstream                                      |
| ------------------------------ | --------------------------------------------- |
| `GET /api/health/live`         | `${API_UPSTREAM_URL}/api/health/live`         |
| `GET /api/health/ready`        | `${API_UPSTREAM_URL}/api/health/ready`        |
| `GET /api/diagnostics/gateway` | `${API_UPSTREAM_URL}/api/diagnostics/gateway` |

The shared helper is `src/server/upstream-proxy.ts`. It passes the upstream status and JSON body
through, forwards or generates `x-request-id`, and never forwards the caller's path, query, cookies or
other headers. Failures of the proxy itself use the shared `ErrorResponse` envelope:

| Status | Code                         | Meaning                                          |
| ------ | ---------------------------- | ------------------------------------------------ |
| 500    | `configuration_error`        | `API_UPSTREAM_URL` is missing or invalid         |
| 502    | `upstream_unreachable`       | The API refused or dropped the connection        |
| 502    | `upstream_invalid_response`  | The API answered with something that is not JSON |
| 504    | `upstream_timeout`           | No complete answer within 10 seconds             |
| 403    | `cross_site_request_refused` | A command came from another site or origin       |
| 415    | `unsupported_media_type`     | A command body was not `application/json`        |

To proxy another API route, add its path to `UPSTREAM_PATHS` and create a matching `route.ts`.

## Browser security: headers and the command guard

`next.config.ts` sets four headers on every route: `Content-Security-Policy`, `X-Content-Type-Options:
nosniff`, `Referrer-Policy: no-referrer` and `X-Frame-Options: DENY`. The policy allows this origin
only, forbids framing (`frame-ancestors 'none'`) and sets `object-src 'none'`, `base-uri 'self'` and
`form-action 'self'`. `src/server/security-headers.test.ts` pins it.

Known limitation: `script-src` and `style-src` allow `'unsafe-inline'`, because Next.js inlines its
bootstrap scripts and the app styles. An injected inline script would therefore not be stopped by the
policy. The app avoids the sinks that would let one in (no `dangerouslySetInnerHTML`; report, event and
judge text render as plain text), so the policy is a second layer, not the protection. A nonce-based
policy would need middleware and dynamic rendering of every page. In development the policy also
allows `'unsafe-eval'` and websockets for hot reload.

Commands (every method except GET and HEAD) pass `commandRefusal` in `src/server/upstream-proxy.ts`
before anything is forwarded: `Sec-Fetch-Site` must be `same-origin` or `none` when present, `Origin`
must be this site's own origin when present, and a body must be `application/json`. A browser always
sends these headers, so another site's page is refused even where the `SameSite=Lax` cookie would
travel (any other port on `localhost`, a sibling subdomain). A client that sends neither header (the
end-to-end script, `pnpm judge`) is not a browser and passes.

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
