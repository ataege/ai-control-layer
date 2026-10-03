# Go demo runbook

The presenter's step-by-step for the beats the Go gateway carries, written against `main` 93e1c96 (3
October 2026). It follows the researcher's [storyboard](product/storyboard.md) (beat numbers, labels
and the replay rule come from there) and does not change it. Owner of this file: the lead (docs
outside `docs/product`); written by lane w3.

Status: **not rehearsed on the final build.** Commands and expected output below were taken from the
code and from recorded test runs; timings are observations from one developer machine, not targets.
SH-33 sets the pitch timing from the rehearsal. The steps marked "checked" were run once on lane w3's
machine at `main` 93e1c96 (load average 15 to 18, not idle).

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
   pnpm db:seed          # synthetic vendors and invoices; imports policy.yaml and the feed if the catalog is empty
   pnpm reset:demo       # truncates demo and runtime data, reseeds; keeps catalog revisions, then activates
   ```

   Expected (checked): each command exits 0. `db:seed` prints `requested revision 1` and the stored
   feed revision; `reset:demo` ends with `[OK] control catalog active (pnpm catalog:activate)`. It
   refuses a non-loopback database.

2. **Control catalog and signature feed.** `pnpm policy:import` stores `config/policy.yaml` and the
   signature feed it names and only requests the revision; `pnpm catalog:activate` (or a running
   gateway's watcher, within a second or two) validates and activates it. No feed is loaded by hand.
   After a judge or the presenter edits the policy or the feed:

   ```sh
   pnpm policy:import      # "requested revision N"; the active one stays until activation
   pnpm catalog:activate   # "catalog activation: activated revision N with signature feed revision M"
   docker exec -e PGPASSWORD <postgres container> psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc \
     "SELECT active_revision_id, active_feed_revision_id FROM app.control_catalog_pointer"
   ```

   Expected (checked): both commands exit 0 and the query shows two non-null ids. A database set up by
   the older import makes `catalog:activate` stop and ask for one more `pnpm policy:import`; that is
   intended. If `policy:import` refuses with "this feed revision is already stored with different
   bytes; bump the revision", the feed was once loaded by hand into that database: do not bump the
   revision; recreate the database (`pnpm db:migration:run`, `pnpm db:roles`, `pnpm db:seed`,
   `pnpm catalog:activate` on a fresh volume). **Without an active catalog and feed every governed
   decision fails closed with
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
   - The control test suite: `MODEL_NAME=qwen3.5:4b pnpm verify:controls` (or `make verify-controls`).
     It recreates the separate test database and never writes the demo data, runs the Go, API and
     fixture tests and the live semantic cases labelled "live model", and writes
     `.verify-controls/results-<timestamp>.json`. Checked at 93e1c96: exit 0 in 115 s, 1130 cases, Go
     927, API unit 125, API database 27 and fixtures 18 passed, none failed or skipped; live model
     28/29 matched, 1 false positive, 0 false negatives, 0 guard failures. A label mismatch is recorded,
     not failed; an unavailable model makes the run INCOMPLETE and exit nonzero (`--no-live` says so).
   - The benchmark: the quiet result table in `services/gateway/README.md` ("Result on the developer
     machine (2026-10-03, quiet)"), or a fresh `pnpm benchmark` (add `--live` for the model).
   - A `psql` session on the demo database for the before-and-after counts below.

## The beats

Each beat lists how to run it today, what the audience should see, the observed timing, the fallback
and what to say. "Live" means the local `qwen3.5:4b` chooses the steps; the model is not
deterministic, so a live run can take a different path.

The live story (beats 1, 3, 4, 7, 8) is one command. It writes its run into the demo database, so
beat 5 can replay against it afterwards. Checked at 93e1c96: exit 0 in 25 s; the model read A01 and
A02, created the internal and the vendor report, read the vendor, queued the vendor report (approved
by the test), and the run completed with 7 agent calls, 1 security call and one outbox row. Another
run may take other steps.

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
  classification `internal_only` and its source trail. Lane f3's GO-27 live runs (a9f8004,
  `docs/roadmap/go.md`) created the internal report in 3 of 3 runs in each of two run sets, and the
  run checked for this runbook created it too.
- Timing: each agent step is one model call, about 2 s quiet and up to the 20 s request timeout under
  memory pressure; no end-to-end time has been measured yet.
- Fallback: the scripted story, which always creates it. Say "scripted provider" when using it.

### Beat 5: attempt the apparently valid send (always the labelled replay)

Decision 28: in lane f3's GO-27 live runs (a9f8004) the model created the internal report every time
but proposed queueing it in 0 of 3 runs in the first set and 1 of 3 in the second (run `8b812e16`,
denied `report_export_restricted`), so this beat always uses the labelled replay against the
genuinely created report. The live attempt is never presented as part of the demonstration. `cmd/replay` accepts only
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

- Checked at 93e1c96 against the live story's run in the demo database: all three replay fixtures
  printed the expected denial (`report_export_restricted`, `resource_out_of_scope`,
  `destination_not_allowed`), nothing executed, and the run's outbox kept its one approved row.
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

- Edit `config/policy.yaml` (for example lower a threshold) or the signature feed
  `config/attack-signatures.json` (with a new feed revision), then `pnpm policy:import` and
  `pnpm catalog:activate`, or let the running gateway activate the requested revision within a second
  or two. An invalid file is rejected and the last accepted revision stays active.
- One change at a time: change one value, run `pnpm policy:import`, wait for the gateway's log line
  `catalog revision validated and activated` (or `pnpm catalog:activate` printing the activated
  revision, or `/health/ready` answering 200), then make the next change. The gateway validates only
  the latest requested revision, so a second import before the first is activated replaces it: a good
  edit followed at once by a bad one leaves the revision from before both active.
- A real change moves the active revision, and an action approved or allowed under the previous
  revision is then refused with `source_policy_changed`, so do not change the configuration during
  a review wait. Re-importing an unchanged file creates a new revision today as well; after lane
  c1's importer fix it is a no-op.
- Show the same input before and after (the judge input path once the API lands; until then a test
  or replay against the new revision). **[API pending]** the revision view (WEB-29).

### Beat 12: test and reporting evidence

- The suite: `MODEL_NAME=qwen3.5:4b pnpm verify:controls` (about two minutes, checked; see "Evidence
  windows" above for the result). Open the results file it names.
- Summary and export: `GET /internal/security/summary`, `/internal/security/assessments` and
  `/internal/security/events` on the gateway (contracts in `packages/contracts`); **[API pending]**
  the dashboard and export file (API-20, WEB-30, WEB-31).
- Timing: the quiet benchmark table (deterministic controls well under 1 ms, live semantic about
  1.9 s, gateway overhead about 5 ms).

## What must not be claimed

Point to these limitations in `services/gateway/README.md` instead of claiming more:

- One gateway process per database (job leases, the in-memory `jti` replay cache).
- The live semantic results are not a detection rate. The current evidence is the root `README.md`
  record for `classifier_v2` (fixture version 3): three runs, the first two failed on setup and test
  strictness, the third passed with 1100 cases and live 27 of 29 matched, 0 false positives and 2 false
  negatives, at load 9.0 to 12.5, not an idle machine; the lane's variance sentence there applies. The
  `verify:controls` run checked for this runbook gave 28 of 29 with 1 false positive. The 24-case runs
  in the gateway README are the older `classifier_v1` history.
- Benchmark numbers are observations on one machine under stated load, not a distribution.
- Live agent runs on an 8 GiB machine paused on request timeouts; a 4B model's choices vary.
- The outbox is simulated, the replay is scripted, fixture verdicts test handling only, and usage
  with an unknown outcome keeps its reservation and slot.
- The organization-wide event cursor shows nothing new while any long transaction is open in the
  database; nothing is lost.

## Gaps to close before the pitch

- A way to start and approve a run in the demo database without the test harness (the API, or a
  documented presenter command that signs the operator context).
- A rehearsal on the final build with recorded timings (SH-33), and a run kept for beat 5.

Done on `main` since the first version: the feed import in `pnpm policy:import` with
`pnpm catalog:activate` (no hand load anywhere), and `pnpm verify:controls` (SH-47).
