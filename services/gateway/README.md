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
| `cmd/modelcheck`      | User (sole Go implementer) |
| `cmd/budgetcheck`     | User (sole Go implementer) |
| `internal/config`     | User (sole Go implementer) |
| `internal/logging`    | User (sole Go implementer) |
| `internal/database`   | User (sole Go implementer) |
| `internal/health`     | User (sole Go implementer) |
| `internal/httpserver` | User (sole Go implementer) |
| `internal/model`      | User (sole Go implementer) |
| `internal/budget`     | User (sole Go implementer) |
| `internal/testdb`     | User (sole Go implementer) |
| `internal/security`   | Go implementer, session c1 |

New packages get their ownership row when their first real code lands.

```
cmd/gateway/          wiring, signals, -healthcheck
cmd/modelcheck/       explicit synthetic Ollama connectivity check
cmd/budgetcheck/      explicit central-catalog and PostgreSQL accounting diagnostic
internal/config/      environment and trusted accounting-catalog validation
internal/logging/     JSON slog logger, Secret
internal/database/    pgxpool construction
internal/health/      handlers and wire DTOs
internal/httpserver/  routes, middleware, error envelope, server lifecycle
internal/model/       bounded Ollama transport and accounted calls
internal/budget/      durable atomic shared token reservations
internal/testdb/      shared explicit PostgreSQL test harness (GO-20)
internal/security/    hybrid security controls: content rules (GO-74)
scripts/go.mjs        pnpm/turbo wrapper around the Go toolchain (not part of the build)
```

## Ollama transport (GO-06 progress)

`internal/model` implements the native Ollama `POST /api/chat` transport using Go's standard
HTTP client. A caller supplies a trusted base URL, a fixed model identifier, a positive timeout
and explicit request/response byte limits. Each request has an agent or security purpose and
explicit context/output ceilings; purposes remain Go metadata rather than being copied into the
provider body. The model is configured per client, so the provisional `qwen3.5:4b` candidate is
not hard-coded.

The client sends non-streaming JSON, follows no redirects and performs no application retries.
It returns safe errors without URLs, raw provider errors, prompts or tool arguments. Tool calls
are proposed data only; the client executes none. Usage preserves missing counts as unknown and
explicit zero as zero, including parseable usage on a rejected response. Go measures request wall
time separately from provider time. A timeout does not establish that remote inference stopped.

This is transport code, not a completed governed model gateway. The worker must still check
identity/passport and active model authority, reserve call-count/purpose allowances, persist
attempts and apply the concurrency cap before using it. Token reservations are now implemented
through `AccountedCaller` and the PostgreSQL ledger described below. The client is not called by startup or
an HTTP route. The explicit diagnostic command below loads the infrastructure-agreed model variables. Runtime
configuration wiring, remote-access setup and live presentation-machine evidence remain pending
GO-03, SH-04 and the infrastructure handoff; existing startup behavior is unchanged.
No provider credential mechanism is invented for the selected local Ollama setup.

Unit tests use a labelled local HTTP provider test double; they are not live model evidence.
API reference: [Ollama chat](https://docs.ollama.com/api/chat).

### Test on the Ollama machine

From the repository root on the M1 Pro machine, with Go and Ollama installed and Ollama running:

```sh
ollama pull qwen3.5:4b
MODEL_BASE_URL=http://127.0.0.1:11434 MODEL_NAME=qwen3.5:4b go -C services/gateway run ./cmd/modelcheck
```

This command does not need PostgreSQL, a running API or a service token. It reads only
`MODEL_BASE_URL` (default `http://127.0.0.1:11434`) and required `MODEL_NAME` from its environment.
The command does not load `.env` automatically; explicit shell values above suffice. The model
tag can change without editing code. Invalid configuration exits unsuccessfully before dispatch.

It makes at most two sequential synthetic provider calls, labelled agent and security, through
the same Go transport. Each has a 30-second deadline, 1 MiB request/response ceiling, 4096 context
and 512 agent / 256 security output tokens, a tiny fixed JSON schema, and explicit `think: false`. These are diagnostic
settings, not adopted production limits. They must be supported by the installed Ollama/model.
Each response must be a completed assistant response with exactly `{"status":"ok"}` and known
input/output token counts. Only then does the command print a JSON `PASS` record with the purpose,
usage and measured wall time. Provider timing is included only when reported. An invalid response
or missing usage stops the command with exit code 1, without printing content or retrying.
SIGINT/SIGTERM cancel the local request; remote inference termination is not guaranteed.

Two `PASS` records and exit code 0 prove these diagnostic calls completed. They do not prove an
agent workflow, semantic detection quality, durable task budgets or concurrency enforcement. Save
the installed Ollama version, model digest, commit and command outcome with the live evidence.
Provider-double tests do not replace live evidence; GO-06 status and its shared decision
prerequisites are recorded in the roadmap.

If Go runs on a different machine, infrastructure must provide the trusted reachable endpoint
and access arrangement. This task does not expose Ollama on the network. The central catalog dependency from
`origin/feat/fd-catalog-and-tests` was merged into the Go feature branch for accounting integration.

### Developer-machine live result (2026-10-03)

Observed at `2026-10-03T14:56:15Z`, build `c8b6f55675110226c4f47d679f1309bce8fd49b4`,
on Apple M2 with 8 GiB memory, Ollama 0.35.1, model `qwen3.5:4b`, ID `2a654d98e6fb`.
The command below exited 0; both completed responses satisfied the diagnostic schema and usage
checks. This is local loopback connectivity evidence, not presentation-machine, semantic-detection
or task-budget evidence. No provider credential is applicable to this local diagnostic.

```sh
MODEL_BASE_URL=http://127.0.0.1:11434 MODEL_NAME=qwen3.5:4b go -C services/gateway run ./cmd/modelcheck
```

| Purpose  | Input tokens | Output tokens | Go wall time (ns) | Provider time (ns) | Outcome |
| -------- | ------------ | ------------- | ----------------- | ------------------ | ------- |
| agent    | 38           | 6             | 8752003584        | 8726185917         | PASS    |
| security | 38           | 6             | 333830375         | 332697500          | PASS    |

Each call used `think: false`, context 4096, output ceiling 256, deadline 30 seconds and 1 MiB
request/response limits. These are diagnostic settings. The durations are two observations, not
a benchmark, a throughput estimate or proof of a maximum latency. The configured model remains
a provisional choice. These observations predate the accounting adoption below; the shared
model/hardware freeze remains GO-03/SH-04.

## GO-06 token accounting

The user adopted the MVP accounting decision on 3 October 2026. `AccountedCaller` validates
requests before reserving, uses a single PostgreSQL run balance for agent and security calls,
forces `think: false`, and caps native `options.num_predict` from trusted catalog settings.
Input reservation is the UTF-8 length of compact JSON containing messages, tool definitions and
response schema, plus 1,024 template tokens; output allowance is then added. This includes system
prompts, history, tool results and tool-call arguments. These bytes are an estimate, not measured
input tokens or a universal tokenizer upper bound.

A completed response with both valid counters settles their full sum and refunds unused
reservation. Missing counters, transport errors and timeouts retain the complete reservation as
`usage_unknown`; no measured zero is invented. There is no automatic retry. A trusted late result
can reconcile once, including after constructing a new store. Identical repeated settlement
changes nothing; conflicting counters are rejected. A measured overrun records full usage and
persists a pause that blocks later reservations. The future run controller must propagate this
ledger pause to the run state; no worker or admission API is implemented by this task.

The TypeORM migration `1791043000000-AddModelTokenBudgets.ts` creates only the ledger tables.
Application startup performs no migration or budget creation. Go alone mutates the ledger;
service-role grants and the future runtime-run relationship remain integration work.

Central values live in `config/policy.yaml` and its accepted immutable catalog revision:
`tokens_total: 20000`, `agent_output_tokens: 512`, `security_output_tokens: 256`, and
`input_template_tokens: 1024`. Older v1 revisions use the last three defaults. Go reads the
active revision in one SQL snapshot and never reads the YAML file as a second runtime authority.
The accounting projection does not replace GO-72/73 full catalog/feed validation or activation.

For an explicit diagnostic on the other machine, with the database already configured and Ollama
running, use the existing commands from the repository root:

```sh
pnpm db:migration:run
pnpm policy:import
MODEL_NAME=qwen3.5:4b node scripts/with-env.mjs go -C services/gateway run ./cmd/budgetcheck
pnpm test:db
```

`budgetcheck` creates one labelled synthetic ledger run and leaves it durable for inspection.
It uses the active catalog model allowlist, shared total, timeout and accounting settings for two
sequential calls. It exits unsuccessfully if the active catalog or ledger is missing. It is not an
agent workflow, semantic-security evaluation or proof of passport/identity enforcement. Later
imports remain requested until the catalog activation protocol accepts them; editing YAML alone
never changes active settings.

### Accounting verification, 3 October 2026

The GO-06 working tree based on `e502857` was tested on M2/8 GiB with Ollama 0.35.1 and
`qwen3.5:4b` ID `2a654d98e6fb`, against an isolated PostgreSQL 17.11 on loopback port 55432.
The existing `pnpm db:migration:run` and `pnpm policy:import` succeeded. Explicit live
`cmd/budgetcheck` exited 0 against active catalog revision 1:

| Purpose  | Reservation | Measured total | Refunded |
| -------- | ----------- | -------------- | -------- |
| agent    | 1664        | 31             | 1633     |
| security | 1408        | 31             | 1377     |

Final shared balance: limit 20,000, used 62, reserved 0, paused false.

The explicit estimator command also exited 0:

```sh
GO_MODEL_ESTIMATOR_LIVE=1 MODEL_BASE_URL=http://127.0.0.1:11434 MODEL_NAME=qwen3.5:4b go -C services/gateway test -tags=model_live ./internal/model -run '^TestLiveInputEstimator$' -count=1 -v
```

| Synthetic fixture          | Estimated input | Reported input |
| -------------------------- | --------------- | -------------- |
| System message and Unicode | 1246            | 56             |
| History and tool result    | 1524            | 118            |
| Tool schema                | 1410            | 287            |
| Security response schema   | 1305            | 31             |

All four sampled inputs fit the estimate. This is model/template sample evidence, not a general
proof. Live model tests require the explicit tag and opt-in; ordinary verification makes no model
requests. Database-backed native HTTP tests use a labelled provider double and cover measured
settlement, missing counters, timeout without redispatch, insufficient allowance, concurrent
agent/security reservations and untruncated overruns. Ledger tests additionally cover restart
reads, concurrent late reconciliation and arithmetic overflow fencing.

The same diagnostic also passed with a newly imported test policy setting agent output to 128,
security output to 64 and template allowance to 800. Reservations became 1056 and 992, while
measured usage remained 31 per call. This verifies values are read from the active catalog rather
than fixed in the accounted caller.

Verification commands on the isolated test database:

- `pnpm test:db`: PASS, 11 Go database tests and 6 API database tests; no skipped database tests.
- `go -C services/gateway test -race ./... -count=1 -timeout=60s`: PASS with PostgreSQL enabled.
- `pnpm verify`: PASS, 6 passed, 0 failed, 0 skipped. Ordinary Go database tests intentionally skip
  without settings; the database command above verifies them separately.
- `pnpm smoke`: host PASS, 21 passed, 0 failed, 0 skipped.
- `pnpm db:migration:run`, `pnpm db:migration:revert`, `pnpm db:migration:run`: PASS on a separate
  empty test database. Reverting the ledger preserved the existing catalog tables.

## Integration handoff review (2026-10-03)

Read-only review of `origin/feat/fd-catalog-and-tests` at `538fed4`: `config/README.md`,
`config/policy.yaml`, the catalog entities and their migration. These are draft inputs to GO-72/73,
not an implemented activation protocol or frozen contract. The branch is not merged here.

| Go reader        | Draft storage                                                                                            |
| ---------------- | -------------------------------------------------------------------------------------------------------- |
| Catalog content  | `app.control_catalog_revisions`: immutable source text, digest and JSON content                          |
| Accepted feed    | `app.signature_feed_revisions`: immutable content, unique issuer/revision                                |
| Current snapshot | `app.control_catalog_pointer`: singleton ID 1, requested/validated/active catalog IDs and active feed ID |

Before implementation, freeze the feed issuer/digest binding (policy currently names a revision
while storage distinguishes issuers), integer upper bounds across TypeScript and Go, and the
activation transaction that binds the exact validated candidate and feed. Reject an enabled
signature control with no accepted feed rather than treating it as an empty rule set. Go must read
a coherent snapshot and fail closed on a missing pointer or active revision. Threshold comparison,
action-proposal redaction and feed grammar/trust remain explicitly open in the draft.

Decision 4 is settled on `origin/main` at `8170ba5`: Go checks the service identity and a signed,
short-lived `X-Operator-Context` JWT, then independently authorizes the command's organization and
run. This settles the transport mechanism, not the frozen claim schema, signing algorithm, issuer,
audience or key handoff. GO-62 mirrors the agreed operator-context contract when it lands; GO-21
uses it without treating a valid signature alone as object authorization. No operator-context
schema is currently present in `packages/contracts` on that main commit.

## Deterministic content controls (GO-74)

`internal/security` applies the `secret_pattern` guard of the active catalog to designated text
fields. The caller passes the catalog settings and trusted source metadata; the package reads
neither PostgreSQL nor `policy.yaml`, and it never sets or changes a field's source
classification, so masking an internal note does not make its report Vendor shareable.

| Boundary      | Designated fields (lead's delegate, 3 October 2026) |
| ------------- | --------------------------------------------------- |
| `tool_result` | `tool_result_text`, `internal_note`                 |
| `model_input` | `model_input_text`                                  |

- Rules: `secret_password_keyword_v1` and `secret_url_credential_v1` (password),
  `secret_api_token_keyword_v1`, `secret_iban_v1` (mod-97 checked) and `secret_payment_card_v1`
  (Luhn checked). Keyword rules need a credential-shaped value (letters and digits, minimum length),
  so "password policy" stays readable. The patterns are fixed Go code (RE2, linear time), not
  catalog data.
- Spans are byte offsets; they must lie in the text, be non-empty and fall on UTF-8 rune boundaries,
  and overlaps merge. An invalid span is an error and withholds the field.
- `redact` replaces each span with `[REDACTED:<kind>]` (`content_redacted`); `block` withholds the
  whole field (`content_blocked`). The record names the first matched rule and the evaluated catalog
  revision, and never holds the inspected text.
- A field over `MaxFieldBytes` (4096) or with invalid UTF-8 is withheld whole, never truncated
  (`field_limit`, `content_too_large`), whatever the guard settings. The bound keeps one security
  call's byte-based token reservation small against the shared run total.
- Missing catalog revision, an unknown mode or an undesignated field return an error and no text.

`content_blocked` and `content_too_large` were approved by the lead's delegate on 3 October 2026
and wait to be frozen in the reason vocabulary (X-13). Tests read `fixtures/semantic-corpus.json`:
the six secret cases must give exactly their fixture spans, and the benign, hard-negative and attack
cases and `fixtures/hostile-notes.json` must give none. This is a finite fixture set, not universal
secret detection.

## Proposed tool results and idempotency (GO-07)

**Draft, 3 October 2026; not a frozen contract or implemented behavior.** Owner: the user, sole Go
implementer. Sources: report, "Illustrative passport and interface contracts" (Proposed tool
argument boundaries; Concrete synthetic business example; Narrow final result and context
boundary) and "Durable state idempotency audit and uncertain outcomes".

SH-10 must freeze tool arguments and X-06 field rules before this draft is adopted. The lists below
describe candidate result fields, not final JSON property names or database columns. No provider,
limit value or shared wire contract is selected here.

| Tool            | Proposed result field allowlist                                                                                                                                                            | Protected values                                                                                                                   |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- |
| `read_invoice`  | Invoice reference and version, associated vendor reference, external invoice reference, integer total in minor units plus currency code, status, synthetic untrusted note used in the demo | Raw fields outside the frozen allowlist are omitted; protected values required by the workflow remain run-scoped opaque references |
| `read_vendor`   | Vendor reference and version, synthetic display name, trusted recipient reference                                                                                                          | Recipient address remains an opaque reference; no raw contact or bank details reach the model                                      |
| `create_report` | Stored report reference and version, authorized source invoice references, registered template reference, structured duplicate-reference finding                                           | No unrestricted model prose or raw protected values; exact finding shape awaits the freeze                                         |
| `queue_report`  | Report reference and version, simulated outbox entry reference and queued status                                                                                                           | Raw recipient and rendered review content stay out of model-facing results and general events                                      |

All four adapters verify organization and passport/resource relationships themselves. Opaque
references resolve only after authorization inside the adapter, and do not resolve in another run
or organization. A source version in a result is evidence for later precondition checks, not
authority. The user accepted integer minor units plus currency on 3 October 2026. SH-10 must still decide
versions, the note's inclusion and the registered template fields; this draft does not settle those open items.

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

## PostgreSQL test harness (GO-20)

All Go database tests use `internal/testdb.Open(t)`. It reads the same `POSTGRES_HOST`,
`POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB` settings supplied by
`pnpm test:db`. If all five variables are absent, an optional test visibly skips. Partial,
blank or malformed configuration fails; `TEST_DATABASE_REQUIRED=1` also makes missing settings
fail. The helper requires a successful ping within three seconds and reports fixed safe errors
without connection strings or credentials. It closes the pool after callers clean their fixtures.

`testdb.ID(t)` generates UUID v4 identifiers for isolated synthetic rows. Tests remove only their
own rows or roll back their own transactions; cleanup uses bounded contexts. The helper creates
no schemas, tables or seed data and never runs migrations. Its round-trip test uses the existing
GO-06 token ledger, so apply the normal TypeORM migrations before running database tests.

The gateway wrapper now runs `go test -count=1 -v ./...`: each skipped database test is named,
and changing database availability cannot reuse a cached pass. The former test-only
`GATEWAY_TEST_DATABASE_URL` is no longer accepted. X-24 deliberately removes `POSTGRES_*` for
its discovery run; a separate URL variable would bypass that isolation and hide required tests.
Ordinary tests do not read `.env`; the existing database command supplies its values explicitly.

```sh
pnpm --filter gateway run test       # unit tests, with visible database skips when unset
pnpm test:db                        # Go + API against the configured, migrated PostgreSQL
pnpm test:db gateway                # Go only; missing/unreachable database exits nonzero
```

Verification on 3 October 2026 used an isolated PostgreSQL 17.11 on loopback port 55432:
`pnpm --filter gateway run test` exited 0 with 12 explicitly named database skips when unconfigured.
With PostgreSQL enabled, `go -C services/gateway test -race ./... -count=1 -timeout=60s` passed.
`pnpm test:db` passed both sides with no skipped tests. After stopping that test PostgreSQL,
`pnpm test:db gateway` against port 55432 exited 1 as required (database FAIL; gateway not run). The harness also
checks rollback cleanup, preserved credentials, partial/invalid settings, cancellation and a
stalled PostgreSQL handshake. `pnpm verify` passed all six steps. The final database command reported 156 Go tests (12 needing
PostgreSQL) and 6 API tests passed, none skipped. No smoke was run for GO-20 because only test
support, the test wrapper and documentation changed.

## Runtime schema review input accepted with the user

On 3 October 2026 the user accepted these responses to the lead's five questions:

- Invoice money is stored in integer minor units plus a currency code (`bigint` in PostgreSQL,
  `int64` in Go), with no floating-point amount. Minor units are not always cents. Wire encoding
  still follows the shared contract; Ollama has no adopted tariff and must not show a measured
  zero monetary cost.
- Draft passport scope and limits may use `jsonb`; identity, organization, version and expiry stay
  explicit. Go must decode against the frozen typed contract before accepting runtime input.
- Keep passport update immutability. Changes require a new grant; cancellation/revocation is
  separate. Service-role protection for deletion and truncation also needs review.
- Execution events belong to a run. Admission failures before run creation and catalog reload
  failures cannot be forced into that assumption or given fabricated run IDs; their event contract
  must distinguish the scope.
- Prepare isolated schema tests now, but final runtime-schema acceptance waits for the Go owner's
  shape approval. These decisions do not approve the entire SH-16/SH-27 schema or freeze contracts.

The lead's `budget_reservations` / `model_usage` draft must be aligned with the GO-06 token ledger
before integration so there is one budget authority. This remains a shared schema review, not a
second ledger implementation. GO-07 likewise remains open for SH-10 tool arguments and X-06 field
rules; its numeric storage input above is settled without inventing the remaining contract.

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
