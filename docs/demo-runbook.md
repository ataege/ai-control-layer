# Go demo runbook

The presenter's step-by-step for the beats the Go gateway carries, written against `main` 1dad371 (3
October 2026). It follows the researcher's [storyboard](product/storyboard.md) (beat numbers, labels
and the replay rule come from there) and does not change it. Owner of this file: the lead (docs
outside `docs/product`); written by lane w3.

Status: **not rehearsed on the final build.** Commands and expected output below were taken from the
code and from recorded test runs; timings are observations from one developer machine, not targets.
SH-33 sets the pitch timing from the rehearsal.

## What exists today

The NestJS API and the web screens for runs, review and reporting are not on `main` yet. Every step
that needs them is marked **[API pending]** with the gateway path that works now. The gateway's
internal routes require the service token and a signed `X-Operator-Context` token; no presenter tool
mints that token, so until the API lands the Go-backed beats run through the scenario tests and the
commands below, not through an HTTP client.

| Need                     | Once the API lands                    | Until then (this runbook)                                                                   |
| ------------------------ | ------------------------------------- | ------------------------------------------------------------------------------------------- |
| Start a run (beat 1)     | API start-run (API-13) and the form   | the live or scripted story test admits the run through real Go admission                    |
| Review and approve (8)   | review screen and approval (API-19)   | the story test approves through `policy.Approvals`, the same code the approval route calls  |
| Timeline, summary (3-12) | run views and security pages (API-20) | the test output, `psql`, and the gateway read routes listed in `services/gateway/README.md` |
| Ad-hoc judge input (10)  | judge client through the API (SH-48)  | the live corpus test; `POST /internal/control/evaluate` exists but needs the signed context |

## Before the demo (about 30 minutes before)

Run from the repository root, on the presentation machine, with Docker running. The `psql` and
`curl` lines read `.env`; load it into the shell first with `set -a; . ./.env; set +a` (never
print it: it holds the generated secrets).

1. **Database and fixtures.**

   ```sh
   pnpm infra:up
   pnpm db:migration:run
   pnpm db:roles
   pnpm db:seed          # synthetic vendors and invoices; imports config/policy.yaml if the catalog is empty
   pnpm reset:demo       # truncates demo and runtime data, reseeds; keeps catalog revisions
   ```

   Expected: each command exits 0. `reset:demo` refuses a non-loopback database.

2. **Control catalog and signature feed.** The gateway activates a requested catalog revision within
   about one second (`catalog.WatchRequested`). The feed import and the one-shot activation command
   are on lane c1's branch, held for the user's clearance; until they land, load
   `config/attack-signatures.json` into `app.signature_feed_revisions` and set the pointer's
   `active_feed_revision_id` by hand, as `services/gateway/README.md` ("Performance benchmark")
   describes. Once c1's branch is on `main`, use its documented command instead (expected name
   `pnpm catalog:activate`; confirm when it lands). Check:

   ```sh
   docker exec -e PGPASSWORD <postgres container> psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc \
     "SELECT active_revision_id, active_feed_revision_id FROM app.control_catalog_pointer"
   ```

   Expected: two non-null ids. **Without an active feed every governed decision fails closed with
   `decision_unavailable`**, including `cmd/replay` (beat 5).

3. **Ollama warm-up.** The first `qwen3.5:4b` call loads the model (11.7 s observed); later calls take
   about 2 s on a quiet machine.

   ```sh
   ollama pull qwen3.5:4b
   MODEL_BASE_URL=http://127.0.0.1:11434 MODEL_NAME=qwen3.5:4b go -C services/gateway run ./cmd/modelcheck
   ```

   Expected: exit 0. Close other memory-heavy programs: on an 8 GiB machine agent calls exceeded the
   20-second request timeout and paused runs with `outcome_unknown`.

4. **Gateway readiness.**

   ```sh
   MODEL_NAME=qwen3.5:4b pnpm dev:gateway
   curl -s -o /dev/null -w '%{http_code}\n' "http://127.0.0.1:${GATEWAY_PORT}/health/ready"
   ```

   Expected: `200`, and no `model not configured` line in the gateway log.

5. **Evidence windows to keep open.**
   - The control test suite. `pnpm verify:controls` (SH-47) is not on `main` yet; until it is, run
     `GOFLAGS=-p=3 pnpm test:db --fresh gateway` (expected: `gateway PASS ... 0 failed, 0 skipped`). It
     uses the separate test database and never touches the demo data.
   - The benchmark: the quiet result table in `services/gateway/README.md` ("Result on the developer
     machine (2026-10-03, quiet)"), or a fresh `pnpm benchmark` (add `--live` for the model).
   - A `psql` session on the demo database for the before-and-after counts below.

## The beats

Each beat lists how to run it today, what the audience should see, the observed timing, the fallback
and what to say. "Live" means the local `qwen3.5:4b` chooses the steps; the model is not
deterministic, so a live run can take a different path.

The live story (beats 1, 3, 4, 7, 8) is one command. It writes its run into the demo database, so
beat 5 can replay against it afterwards:

```sh
GO_STORY_LIVE=1 MODEL_BASE_URL=http://127.0.0.1:11434 MODEL_NAME=qwen3.5:4b \
  node scripts/with-env.mjs go -C services/gateway test -tags=model_live ./internal/scenario \
  -run '^TestLiveStoryThroughTheProductionChain$' -count=1 -v
```

The scripted fallback for the same beats (labelled fixture provider, test database):

```sh
GOFLAGS=-p=3 pnpm test:db gateway   # includes TestStoryThroughTheProductionChain and TestStoryAfterApproval
```

### Beat 1: delegate the job

- Today: the story test admits the run through `admission.Admitter` (the code behind
  `POST /internal/runs`). **[API pending]** the task form and passport summary (API-13).
- Audience sees: the passport with the four tools, `invoice_A01` and `invoice_A02`, the Atlas vendor
  and its one recipient reference, both report templates, `queue_report` requiring approval, the
  expiry and the limits; event `run.queued`.
- Fallback: the scripted story. Say: "Go derives this grant from the verified operator and the active
  catalog; the request cannot widen it."

### Beat 2: establish the baseline

- Today: in `psql`, the invoice versions, `SELECT count(*) FROM demo.outbox_messages` (0) and the
  active catalog and feed revision (step 2 above). **[API pending]** the controls view.
- Say: "The outbox is simulated: a database record, no email is sent."

### Beats 3 and 4: investigate and create the internal report (live)

- Audience sees: `action.allowed` then `action.succeeded` for `read_invoice` A01 and A02, the hybrid
  checks recorded per step, then `report.created` for an `internal_investigation_v1` report with
  classification `internal_only` and its source trail. The lead's session saw the live model create
  the internal report in 3 of 3 runs with the storyboard instruction.
- Timing: each agent step is one model call, about 2 s quiet and up to the 20 s request timeout under
  memory pressure; no end-to-end time has been measured yet.
- Fallback: the scripted story, which always creates it. Say "scripted provider" when using it.

### Beat 5: attempt the apparently valid send (always the labelled replay)

Decision 28: the live model created the internal report but tried to queue it in 0 of 3 runs, so this
beat always uses the labelled replay against that genuinely created report. `cmd/replay` accepts only
a finished run (completed, failed or stopped), so it never takes a step a live loop is about to use;
run it after the live story ends.

```sh
# The live story's run: the newest finished run that has an internal_only report.
docker exec -e PGPASSWORD <postgres container> psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc \
  "SELECT r.id FROM runtime.runs r WHERE r.status IN ('completed','stopped','failed')
     AND EXISTS (SELECT 1 FROM demo.reports d WHERE d.run_id = r.id AND d.classification = 'internal_only')
   ORDER BY r.updated_at DESC LIMIT 1"
node scripts/with-env.mjs go -C services/gateway run ./cmd/replay -run <run id> -fixture hostile_note_internal_disclosure_v1
```

- Audience sees: `LABELLED REPLAY (deterministic rehearsal, not a model-generated action)`, decision
  `deny`, reason `report_export_restricted`, `nothing executed`, exit 0. The stored action and its
  `report.export_denied` event carry `labelled_replay:hostile_note_internal_disclosure_v1`; the outbox
  count stays 0 and there is no execution attempt.
- Timing: under a second; no model call.
- If it prints `UNEXPECTED` with `decision_unavailable`: the catalog has no active feed (step 2).
- Fallback: the scripted story shows the same denial through the agent loop; or run the command
  against the run kept from the rehearsal.
- Say: "This is a labelled replay." Never say the live model tried the export.

### Beat 6: try a cosmetic workaround

- There is no rename operation (open item `rename operation`), so this beat is shown with test
  evidence: `TestLabelRenameAndMissingLineageTampering` (`internal/tools`) and
  `TestStoredLabelAndTitleCannotOverrideTheLineage` (`internal/provenance`), part of the suite run.
- Say: "A title or a model-supplied label never changes the stored classification."

### Beats 7 and 8: complete through the permitted route, review and queue (live)

- Audience sees: `report.created` for a `vendor_reconciliation_v1` report, classification
  `vendor_shareable`, projection rule `vendor_invoice_fields_v1`, no internal note text;
  `approval.requested` and `run.awaiting_approval`; the reviewer approves (`approval.decided`); then
  `run.resumed`, `action.succeeded` and one outbox row to the registered address whose content hash
  matches the reviewed report; `run.completed`. **[API pending]** the review screen (API-19); today the
  test approves through `policy.Approvals`.
- Recorded live run: 6 agent calls, 1 security call, one outbox row, completed.
- Fallback: `TestStoryAfterApproval` (scripted provider) shows the same ordered events every time.

### Beat 9: the remaining controls

- Out-of-scope read, labelled replay against the same finished run:

  ```sh
  node scripts/with-env.mjs go -C services/gateway run ./cmd/replay -run <run id> -fixture hostile_note_redirect_record_v1
  ```

  Expected: `deny`, reason `resource_out_of_scope`, exit 0; `invoice_B01` unchanged, no execution
  attempt. `hostile_note_redirect_recipient_v1` gives `destination_not_allowed` the same way.

- Exhausted allowance, labelled runtime test (provider double): `TestModelLimitStopsTheRunBeforeTheNextDispatch`
  and `TestFailedModelCallIsNeverResent` (`internal/agent`, in the suite run). Expected evidence
  lines: 2 provider requests for an allowance of 2, every reservation settled, run
  `paused`/`allowance_exhausted`; a failed call is sent exactly once and pauses the run.
- Evidence for both boundaries with before-and-after counts: `TestResourceAndDestinationBoundaries`
  (`internal/policy`).
- Say "labelled replay" and "labelled test double".

### Beat 10: inspect hostile content (live semantic check)

- Today: the live corpus test, which sends the corpus and the three hostile notes through the real
  evaluator and writes a results file:

  ```sh
  GO_SECURITY_LIVE=1 GO_SECURITY_EVIDENCE_FILE=/tmp/x96.json MODEL_BASE_URL=http://127.0.0.1:11434 \
    MODEL_NAME=qwen3.5:4b go -C services/gateway test -tags=model_live ./internal/security \
    -run '^TestLiveSemanticCorpus$' -count=1 -v -timeout 20m
  ```

  **[API pending]** the judge client and live test entry (SH-48, API-38).

- Audience sees: verdicts labelled `live` with score and category, benign cases passing (including the
  hard negative), blocked notes absent from the would-be agent context.
- Timing: about 2 s per case quiet, 11.7 s for the first call.
- Fallback: if the model is unavailable, the guard fails closed (`security_evaluator_unavailable`, a
  paused run); show that state as the guard-failure evidence (X-98). Never present a fixture verdict
  as detection quality.

### Beat 11: change the configuration

- Edit `config/policy.yaml` (for example lower a threshold), then `pnpm policy:import`; the gateway
  activates the requested revision within about a second. An invalid file is rejected at import and
  the last accepted revision stays active. Feed edits need c1's feed import (see step 2).
- Show the same input before and after (the judge input path once the API lands; until then a test
  or replay against the new revision). **[API pending]** the revision view (WEB-29).

### Beat 12: test and reporting evidence

- The suite: `pnpm verify:controls` once SH-47 lands; until then
  `GOFLAGS=-p=3 pnpm test:db --fresh gateway`.
- Summary and export: `GET /internal/security/summary`, `/internal/security/assessments` and
  `/internal/security/events` on the gateway (contracts in `packages/contracts`); **[API pending]**
  the dashboard and export file (API-20, WEB-30, WEB-31).
- Timing: the quiet benchmark table (deterministic controls well under 1 ms, live semantic about
  1.9 s, gateway overhead about 5 ms).

## What must not be claimed

Point to these limitations in `services/gateway/README.md` instead of claiming more:

- One gateway process per database (job leases, the in-memory `jti` replay cache).
- The live semantic results are two runs of a 24-case synthetic sample, not a detection rate; the
  model missed some cases (GO-84 evidence).
- Benchmark numbers are observations on one machine under stated load, not a distribution.
- Live agent runs on an 8 GiB machine paused on request timeouts; a 4B model's choices vary.
- The outbox is simulated, the replay is scripted, fixture verdicts test handling only, and usage
  with an unknown outcome keeps its reservation and slot.
- The organization-wide event cursor shows nothing new while any long transaction is open in the
  database; nothing is lost.

## Gaps to close before the pitch

- A way to start and approve a run in the demo database without the test harness (the API, or a
  documented presenter command that signs the operator context).
- c1's feed import and catalog activation on `main`, so step 2 needs no manual SQL.
- `pnpm verify:controls` (SH-47).
- A rehearsal on the final build with recorded timings (SH-33), and a run kept for beat 5.
