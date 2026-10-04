# Technical handoff

The index of the technical handoff (SH-35). It links to the text each side owns instead of copying
it, and it holds three things nobody else holds: the run order, the known limits in one place and
the pointers to the evidence.

**Status.** Assembled on 2026-10-04 from `main` 639f40b. It is not tied to the submitted build yet:
the build identifier, the final recheck of every pointer below and the evidence recapture (X-59) wait
for the freeze (SH-34, SH-32). Where a statement below needs that recheck it says so.

## What the handoff contains

| What the report asks for                                | Where it is                                                                                                                                                                                                                                                                                        |
| ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Setup instructions, one-command setup                   | [how-to-open.txt](how-to-open.txt) (plain text, for the submission and the judges); [README.md](../README.md) "Quick start"; [setup.md](setup.md) (per-OS tools, environment loading, presentation machine)                                                                                        |
| Local model acquisition and setup                       | [how-to-open.txt](how-to-open.txt) sections 1 and 4; [setup.md](setup.md) section 7                                                                                                                                                                                                                |
| Architecture and boundaries                             | [architecture.md](architecture.md) (wiring, request ids, health, error envelope, contracts, product modules); [product/README.md](product/README.md) (decisions); the report and its twelve figures in [product](product/task-passport-project-report.docx)                                        |
| Next.js part                                            | [apps/web/README.md](../apps/web/README.md) (browser path, proxy rules, routes, pages, labels, failure states, tests, limits); [packages/ui/README.md](../packages/ui/README.md)                                                                                                                   |
| NestJS part                                             | [apps/api/README.md](../apps/api/README.md) (setup, identity, checks, limits); [api-facade-handoff.md](api-facade-handoff.md) (every public route and its contract); [packages/contracts/README.md](../packages/contracts/README.md) (wire contracts, fixtures, ownership)                         |
| Go part                                                 | [services/gateway/README.md](../services/gateway/README.md) "Technical handoff (GO-61)": setup, how a run flows through the packages, the production chain, boundaries and their fail-closed behaviour, tools, accounting, live versus fixture versus replay, known limitations, evidence commands |
| Tool contracts                                          | [services/gateway/README.md](../services/gateway/README.md) "Tools (GO-07)" (the four tools, arguments, model-facing results, effects); schemas in [packages/contracts](../packages/contracts/README.md)                                                                                           |
| Adapter contract and judge client                       | `POST /api/control/evaluate` (X-91): schemas `control-evaluation-request` and `-response` in `packages/contracts/schemas`, route in [api-facade-handoff.md](api-facade-handoff.md); the judge console `/judge`; the CLI `pnpm judge` ([README.md](../README.md) "`pnpm judge`")                    |
| Policy fixture, `policy.yaml`, feed schema and revision | [config/README.md](../config/README.md) (every field, import, reload, rejected files); `config/policy.yaml`; `config/attack-signatures.json`; [gateway README](../services/gateway/README.md) "Signature feed matching and catalog settings (GO-78)"                                               |
| Synthetic data and reset                                | [fixtures/README.md](../fixtures/README.md); [README.md](../README.md) "`pnpm db:seed`" and "`pnpm reset:demo`"                                                                                                                                                                                    |
| Audit export, security summary, telemetry               | [api-facade-handoff.md](api-facade-handoff.md) (`/api/security/summary`, `/api/security/export`); [gateway README](../services/gateway/README.md) "Performance telemetry (GO-80)"; the pages `/security`, `/security/export` and `/diagnostics`                                                    |
| Control test suite                                      | [README.md](../README.md) "`pnpm verify:controls`"; [how-to-open.txt](how-to-open.txt) section 9                                                                                                                                                                                                   |
| Demonstration script                                    | [demo-runbook.md](demo-runbook.md); [product/storyboard.md](product/storyboard.md)                                                                                                                                                                                                                 |
| Dependency disclosures and licenses                     | [architecture.md](architecture.md) "Selected versions"; [product/source-register.md](product/source-register.md) "Licenses"; [preparation-record.md](preparation-record.md) (starter dependencies); the model: `qwen3.5:4b`, Apache 2.0 (`ollama show --license qwen3.5:4b`)                       |
| Critical-check outcomes and claim evidence              | [product/claim-to-proof.md](product/claim-to-proof.md); "Evidence" below                                                                                                                                                                                                                           |

## Run order

For a machine that has never seen the project, the order is:

1. Install the tools and pull the model: [how-to-open.txt](how-to-open.txt) section 1.
2. `pnpm install`, `pnpm run setup`, then set `MODEL_NAME` in `.env` (section 2).
3. `pnpm infra:up`, `pnpm db:migration:run`, `pnpm db:roles`, `pnpm db:seed` (section 3). The seed
   also creates the demo operator and requests the policy catalog; the gateway activates it by
   itself when `pnpm dev` starts, so `pnpm catalog:activate` is only needed without a gateway.
4. Warm the model, `pnpm dev`, wait until `http://localhost:3001/api/health/ready` answers 200,
   `pnpm smoke` (section 4).
5. Sign in as the demo operator (section 5), then walk through the scenario (section 6) and the judge
   console (section 7).
6. Change the rules and watch the reload (section 8), run `pnpm verify:controls` (section 9), read the
   reports (section 10).
7. `pnpm reset:demo` between attempts; Ctrl+C and `pnpm infra:down` to stop (section 11).

Several checkouts on one machine each need their own Compose project and ports:
[infra/README.md](../infra/README.md) "Several checkouts on one machine" and
[README.md](../README.md) "Changing ports". The presentation machine has its own list in
[setup.md](setup.md) section 8.

## Known limits in one place

These are the limits each side recorded. The linked text has the detail and the numbers; this list
exists so a reader finds them without opening five files. Nothing here is a hidden defect: each is
also stated where it applies.

**The model and the semantic check**

- The semantic check is a small local model. It is not complete detection and its verdicts vary near
  the threshold; a fixture verdict tests handling, not detection quality. The deterministic controls
  do not depend on it, and a semantic verdict can only block or redact. It runs on free text only: the
  invoice note in tool results and model input, not on the arguments of the four MVP tools.
- A 4B model's choices vary between runs: it proposed the internal-report export in 1 of 3 live runs,
  so the export-denial step is always a labelled, scripted replay and needs a run younger than the
  15-minute passport.
- Model calls are slow when the machine is shared (request deadline 20 s); a timed-out call pauses
  the run as `outcome_unknown`, with usage unknown and held. Calls are never retried.
- Live results (detection counts, latencies, benchmark numbers) are observations of single runs on
  one machine under a stated load, not rates or distributions.

**Gateway and data**

- One gateway process per database and model host: two gateways on one database can each send a
  model request for the same run.
- The signature feed has no signing key; trust is the authenticated import plus the file's SHA-256.
  It is not called "signed".
- The audit stream is application evidence, not tamper-proof (the database owner can change it).
- Two identical start-run requests create two runs (`command idempotency keys` is open).
- Unknown outcomes have no reconciliation operation; the run stays in the attention state.
- Protected-field inspection is literal: a transformed or encoded value is not found.
- The organization-wide event and assessment pages show nothing new while any long transaction is
  open in the database; nothing is lost.
- Recipient references are bounded to the passport's one vendor; there is one organization and one
  operator, no organization switch and no production onboarding or identity federation.
- Revoking a grant during a run is not implemented; cancellation and expiry are.
- The outbox is simulated (a database row, no email), and all business data is synthetic.

**API**

- The judge CLI `pnpm judge` was sending a request the API refuses (`run_id` instead of `runId`, and
  missing null fields); a fix was in progress when this was written, so check
  [apps/api/README.md](../apps/api/README.md) and run `pnpm test:judge` plus one evaluation from the
  `/judge` page before relying on the CLI.
- A live two-operator organization-access check is missing because no second-organization operator
  is seeded; the public-path evidence is a database-backed test with a labelled Go fixture (API-23).
- Free-text fields the contracts allow (for example an event's `safeMessage`) are relayed as the
  gateway sends them; the API cannot recognise a protected value inside them (API-24).
- Activity uses polling, not server-sent events.
- The API connects to PostgreSQL as the owner role; a least-privilege API role is planned.

**Web**

- The sign-in gate checks that a `session` cookie exists; the API decides whether it is valid.
- The run page polls every 3 seconds. Failure states (`@/components/errors`) are applied on the report
  page; other pages still print a short message.
- The content security policy allows `'unsafe-inline'` for scripts and styles because Next.js inlines
  its bootstrap ([apps/web/README.md](../apps/web/README.md)).
- No accessibility audit, mobile layout check or browser end-to-end test in the automated suite;
  browser checks were done by hand and are quoted in the roadmap.

**Platforms**

- Verified on macOS (Apple silicon). Linux and WSL 2 are expected to work and are untested. Host mode
  (`pnpm dev`) is the demonstrated mode; full-container mode (`pnpm stack:up`) ran on macOS (SH-09) and
  stays an alternative, and reading `config/policy.yaml` from a container is unverified
  ([setup.md](setup.md) section 8).

## Evidence

| Evidence                                                                               | Where                                                                                                                                                                                                                                                           |
| -------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Control test suite results (positive, negative, redaction, budget, exploit, live part) | `docs/evidence/verify-controls-*.json`; run it again with `pnpm verify:controls`                                                                                                                                                                                |
| Sanitized audit export from a real run                                                 | `docs/evidence/api-audit-export-2026-10-04.json` (limits stated in the file)                                                                                                                                                                                    |
| Claim to proof, with the claims not to make                                            | [product/claim-to-proof.md](product/claim-to-proof.md), "Evidence recorded on `main`"                                                                                                                                                                           |
| Go check results, live observations, evidence commands                                 | [gateway README](../services/gateway/README.md) "Evidence commands" and "Check results"                                                                                                                                                                         |
| Each task's quoted check results                                                       | the "Completed" lines in [roadmap/go.md](roadmap/go.md) and [roadmap/web-and-api.md](roadmap/web-and-api.md); the shared track in [roadmap/README.md](roadmap/README.md)                                                                                        |
| Clean-checkout runs by a second person                                                 | API-27 in [roadmap/web-and-api.md](roadmap/web-and-api.md) (Quick start on a fresh clone, smoke 36 passed, 0 failed, 6 skipped); the check of [how-to-open.txt](how-to-open.txt) (reload valid and invalid, `verify:controls --no-live`, reset, stop and start) |
| Field minimization on the activity views                                               | API-24 in [roadmap/web-and-api.md](roadmap/web-and-api.md) and `apps/api/src/runs/field-minimization.spec.ts`                                                                                                                                                   |
| Every stop, pause and failure is readable                                              | GO-58 in [roadmap/go.md](roadmap/go.md) (93 run ends: 31 reason codes by 3 statuses)                                                                                                                                                                            |
| Verification status of the starter baseline                                            | [README.md](../README.md) "Verification status"                                                                                                                                                                                                                 |

## Before this is final

- Name the submitted build here and recapture every check's evidence from it (SH-32, X-59); the
  numbers in the linked text were taken on `main` between 1c07e78 and 639f40b.
- Run the final live checks that need a quiet machine ([demo-runbook.md](demo-runbook.md) "Final live
  checks"); the live model steps of [how-to-open.txt](how-to-open.txt) (sections 4, 6 and 7) were not
  part of the clean-checkout runs above.
- Refresh the "Product modules" table in [architecture.md](architecture.md) if a module lands after this
  date, and keep this index in step with [how-to-open.txt](how-to-open.txt).
