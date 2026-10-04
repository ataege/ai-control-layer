# web

Next.js App Router front end (React, strict TypeScript, Tailwind CSS v4). UI primitives come from
[`@workspace/ui`](../../packages/ui/README.md); response types come from `@workspace/contracts`.

## Browser path

The browser talks to this app only, with relative URLs. Every route handler under `src/app/api`
forwards to the NestJS API through `proxyUpstream` (`src/server/upstream-proxy.ts`); nothing here
calls the gateway, the database or the model.

```text
browser -> src/lib/fetch-json.ts -> route handler -> proxyUpstream -> API (${API_UPSTREAM_URL}) -> gateway
```

`proxyUpstream` rules:

- Only paths under the allowlist prefixes `/api/health/*`, `/api/diagnostics/gateway`, `/api/runs`,
  `/api/actions`, `/api/auth`, `/api/control`, `/api/security` and `/api/policies` are forwarded. Anything else
  answers 500 `configuration_error`.
- It forwards the method, the query of the path the handler built, the `session` cookie only (never
  other cookies or headers), the request body and `content-type`, and an `x-request-id` (the
  caller's if it matches `[A-Za-z0-9._-]{1,64}`, else a new one). The API's request id comes back.
- It passes the API's status and JSON body through with `Cache-Control: no-store`. A paging cursor
  header `x-next-cursor` is passed on when it is a plain reference (the audit export's CSV pages).
- It never sees the service token: the web process has no `GATEWAY_SERVICE_TOKEN`, no `POSTGRES_*`
  variable and no `NEXT_PUBLIC_*` variable.
- Its own failures use the shared `ErrorResponse` envelope:

| Status | Code                         | Meaning                                               |
| ------ | ---------------------------- | ----------------------------------------------------- |
| 500    | `configuration_error`        | `API_UPSTREAM_URL` is missing or invalid, or bad path |
| 502    | `upstream_unreachable`       | The API refused or dropped the connection             |
| 502    | `upstream_invalid_response`  | The API answered with something that is not JSON      |
| 504    | `upstream_timeout`           | No complete answer within 12 seconds (45 s for judge) |
| 403    | `cross_site_request_refused` | A command came from another site or origin            |
| 415    | `unsupported_media_type`     | A command body was not `application/json`             |

Route handlers (each a thin file; GET unless noted):

| Route                                           | Upstream path (same)       | Notes                                        |
| ----------------------------------------------- | -------------------------- | -------------------------------------------- |
| `/api/health/live`, `/api/health/ready`         | same                       | Public                                       |
| `/api/diagnostics/gateway`                      | same                       | Public                                       |
| `POST /api/auth/sign-in`, `POST .../sign-out`   | same                       | Sets or clears the HttpOnly `session` cookie |
| `/api/auth/me`                                  | same                       | The signed-in operator (API-06)              |
| `POST /api/runs`                                | `/api/runs` plus the query | Start a run                                  |
| `/api/runs/options`                             | same                       | Task form options (API-12)                   |
| `/api/runs/[id]`, `/passport`, `/usage`         | same                       | Run state, passport, ledger-based usage      |
| `/api/runs/[id]/events`                         | same                       | Cursor page; streamed, not buffered          |
| `/api/runs/[id]/reports/[reportId]`             | same                       | Stored report with lineage                   |
| `POST /api/runs/[id]/cancel`                    | same                       | Cancel request                               |
| `/api/actions/[id]/review`, `POST .../approval` | same                       | Exact-action review and decision             |
| `POST /api/control/evaluate`                    | same                       | Judge input; 45 s timeout (local model)      |
| `/api/security/summary`, `/api/security/export` | same                       | Posture; the audit export is reviewer-only   |
| `/api/policies/catalog`                         | same                       | Active control catalog and reload state      |

To proxy another API route, add a prefix to `UPSTREAM_PREFIXES` (or keep it under an existing one)
and create a matching `route.ts`.

`src/lib/fetch-json.ts` is the only browser fetch helper. It accepts relative URLs, applies a 15 s
timeout (20 s for the export), and returns a result value, never a throw: `{ ok: true, data }` or
`{ ok: false, error }` with a kind (`network`, `timeout`, `aborted`, `http`, `invalid_json`).
Typed clients in `src/lib/clients` and `src/lib/product-client.ts` check each response with a shape
guard before a component sees it, so a malformed answer becomes `invalid_json`, not a render crash.

## Pages

`src/middleware.ts` redirects a request without a `session` cookie to `/login?callbackUrl=<path>`.
It checks only that the cookie exists; the API decides whether the session is valid, and an expired
one shows the "Your session has ended" failure state with a sign-in link.
`/login`, `/diagnostics`, `/api/*` and static assets are public.

| Route                           | Purpose                                                                                                                   |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `/`                             | Task form and a plain explanation of admission, passport and run; marks the data as synthetic and the outbox as simulated |
| `/login`                        | Sign in as the seeded development demonstration operator                                                                  |
| `/tasks/new`                    | The task form on its own page                                                                                             |
| `/runs/[id]`                    | One run: state, passport, usage, event timeline, limit stops, reports, cancel; polls every 3 s until a terminal status    |
| `/runs/[id]/review/[actionId]`  | Exact-action review: the stored action and its reasons, approve or reject (reviewer role)                                 |
| `/runs/[id]/reports/[reportId]` | A stored report: classification badge, source trail (template, projection, content hash), content or "Content withheld"   |
| `/judge`                        | Judge console: submit model input, a tool result or an action proposal and see the control layer's decision               |
| `/security`                     | Posture: decisions, controls that fired, the active control catalog, model usage per purpose, phase timings               |
| `/security/export`              | Sanitized audit export as JSON or CSV pages (reviewer role; others see a refusal and no control)                          |
| `/diagnostics`                  | Live health of the API, the gateway and their database connections, from real responses only                              |
| `/components`                   | Showcase of every shared primitive and generic component                                                                  |

What the product pages never do: compute a classification, a decision or a label from a title or a
guess. They show what the server stored (`classification`, `lineage`, `reasonCode`, `effect`,
`verdictSource`, `replaySource`); a browser value never sets one. A report whose `contentWithheld`
is true shows no text. An export denial (`report.export_denied`) is rendered as a denial with the
rule, "The recipient was permitted." and the continuation through a separate vendor report; it has
no approve button.

## Truthful labels

`src/lib/labels.ts` holds the wording the storyboard fixes and `src/components/labels` the badges.
A label function returns null when the data carries no mark, so an unmarked thing has no label and a
marked one always does:

| Label                     | Shown when                                                                      |
| ------------------------- | ------------------------------------------------------------------------------- |
| Simulated outbox          | An event's `effect` is `outbox_message_queued` (a database record, no email)    |
| Labelled replay           | An action or event carries a `replaySource` (`labelled_replay:<fixture>`)       |
| Fixture verdict / Live    | A control's `verdictSource` is `fixture` or `live`; anything else gets no label |
| Development demonstration | The signed-in operator is the seeded `demo-operator@example.com`                |
| Synthetic records         | Pages that show the demo data                                                   |

## Failure states

`@/components/errors` (WEB-23) turns any failed request into one explanation:
`classifyFailure(result.error, { command: true })` for a command, `failureFromReason(code)` for a
run's `terminalReason` or an event's `reasonCode` (all 31 shared reason codes have a fixed safe
message). `<FailureState failure requestId onRetry />` shows what happened, where (browser, web
server, API, gateway, the live model, session), and what to do. A command that got no answer is
marked outcome unconfirmed and offers no retry; a provider failure is attributed to the live model,
not to the working gateway. Raw error messages are never printed.

## Sign-in is a development demonstration

`/login` signs in against the API with a real credential check. The only account is the operator
created by `pnpm db:seed` (email `demo-operator@example.com`, password generated into the untracked
`.env` as `DEMO_OPERATOR_PASSWORD`). The interface labels it "Development demonstration". There is
no registration, no identity federation and no production onboarding.

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
| `test`      | Vitest, node environment, `src/**/*.test.ts`                |

`pnpm run dev:web` from the root loads the root `.env` first. `@workspace/contracts` must be built
once (`pnpm --filter @workspace/contracts run build`); Turborepo does this automatically.

`dev` and `start` are small Node launchers in `scripts/`, so they behave the same in every shell.

`start` needs a finished `build`. `next start` does not support `output: "standalone"`, so the
launcher copies `.next/static` and `public` next to the standalone server, then runs
`.next/standalone/apps/web/server.js` with `PORT` set from `WEB_PORT` and `HOSTNAME` set from
`WEB_HOST`. An inherited `HOSTNAME` is ignored because many shells export the machine name there. The
container image runs the same server directly (see `infra/docker/web.Dockerfile`).

## What the tests cover

`pnpm --filter web run test` runs 42 files, 409 tests (2026-10-04, main 39d5899): Vitest in the
node environment, so every test is a pure function or a server-rendered string. Covered:

- the proxy (allowlist, cookie and header forwarding, request ids, the four failure codes), the
  route handlers (path and method mapping) and the fetch helper (timeout, abort, malformed JSON);
- the typed clients' shape guards (accepted and refused answers, including a report whose content
  contradicts `contentWithheld`);
- view models: run state, event timeline, usage, passport, posture, export preview, judge result,
  admission rejection, limit stops;
- the label functions and the failure classification (every failure kind, all 31 reason codes);
- components rendered once with `react-dom/server` (report content, export denial, failure state,
  evaluation result) to check the words, links and absence of controls.

Not covered by automated tests: interaction in a real browser (clicks, form entry, polling timers,
downloads), layout and styling, and keyboard or screen-reader behavior. No component test with a DOM
and no browser end-to-end test exists. These were checked by hand in a browser against a running
stack, and the roadmap blocks (WEB-12, WEB-23, WEB-27, WEB-28) quote what was seen.

## Known limitations

- The `session` cookie check in the middleware is existence only; a forged cookie reaches the page
  shell and is refused by the API on the first data read.
- Polling, not push: the run page refreshes every 3 s while the run is not terminal.
- Failure states are applied on the report page; other pages that still print a short message
  (`getSafeMessage`) do not yet use `@/components/errors`.
- No dark/light toggle test, no mobile layout check, no accessibility audit.
- Not verified on Linux or WSL; Windows is untested.

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
- `src/components/` components by feature (`approval`, `errors`, `judge`, `labels`, `passport`,
  `report`, `run`, `security`)
- `src/lib/` browser-safe helpers (`fetch-json.ts`, `labels.ts`, `product-client.ts`, `clients/`,
  `errors/`, `service-checks.ts`)
- `src/server/` server-only code (imports `server-only`)
- Reusable, product-neutral components belong in `packages/ui`
