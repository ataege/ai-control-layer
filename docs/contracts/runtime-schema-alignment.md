# DRAFT: runtime schema alignment with the GO-06 token ledger

> **For the Go implementer to decide.** Nothing here changes the GO-06 migration
> (`origin/feat/go-roadmap`, commit `0e683b6`,
> `apps/api/src/database/migrations/1791043000000-AddModelTokenBudgets.ts`). It lists every
> difference between that ledger and the DRAFT migrations on `feat/35-fixtures-and-tests`
> (SH-16, SH-17, SH-44, SH-27, SH-24), and what the drafts already changed to avoid two accounting
> authorities. All five drafts stay pending the Go implementer's approval.

## Decided: the GO-06 ledger is the model-token authority

"There must not be two accounting authorities for the same budget." The drafts were changed so
that model tokens are recorded only in `runtime.model_token_budgets` and
`runtime.model_token_reservations`:

| Draft                       | Before                                          | Now                                                                                                                                                                          |
| --------------------------- | ----------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| SH-16 `model_calls`         | `input_tokens`, `output_tokens`                 | Removed. The table keeps GO-02's pre-dispatch record only (purpose, model, outcome, timestamps).                                                                             |
| SH-44 `model_usage`         | One usage row per model call                    | Removed; the migration is renamed `AddControlAssessmentsAndTiming`.                                                                                                          |
| SH-27 `budget_reservations` | Reserved input and output tokens per model call | Removed. The table is now a **proposal** for what the ledger does not cover: tool-attempt reservations, call counts per purpose, the concurrency slot, the request deadline. |

The remaining `budget_reservations` uses the ledger's state vocabulary (`reserved`, `usage_unknown`,
`settled`) so the two read the same.

## Differences for the Go implementer to decide

| #   | Topic                             | GO-06 ledger                                                                              | Drafts                                                                                                                                     | Options                                                                                                                                                                                      |
| --- | --------------------------------- | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Run identifier                    | `run_id text`, no foreign key                                                             | `runtime.runs.id uuid`; children reference `(id, organization_id)`                                                                         | (a) the ledger's `run_id` becomes `uuid` referencing `runtime.runs`; (b) `runs.id` becomes `text`; (c) keep them unlinked and document why.                                                  |
| 2   | Organization                      | No `organization_id`                                                                      | `organization_id` on every row, organization-safe composite references                                                                     | (a) add `organization_id` to both ledger tables with the composite reference to `runs`; (b) keep the ledger keyed by run only, organization derived through `runs`.                          |
| 3   | Model call identity               | `call_id text`, primary key `(run_id, call_id)`                                           | `model_calls.id uuid` (GO-02 pre-dispatch record); SH-44 assessments and SH-27 reservations reference it                                   | (a) `call_id` = `model_calls.id` with a foreign key; (b) the ledger reservation becomes the pre-dispatch record and `model_calls` is dropped (its model and timestamps move to the ledger).  |
| 4   | Per-purpose sub-budgets           | One shared token balance for agent and security                                           | Calls counted per purpose in `budget_reservations` (proposal); no per-purpose token limit                                                  | The report asks for "separate agent/guard suballowances inside aggregate task limits". Neither side has per-purpose **token** sub-limits yet; decide where they live.                        |
| 5   | Call, time and concurrency limits | Not covered                                                                               | `budget_reservations` (proposal): one row per dispatch, `concurrency_slot` held while `reserved` or `usage_unknown`, `request_deadline_at` | Adopt, reshape or replace with Go-side fields on the ledger.                                                                                                                                 |
| 6   | Token limit source                | `token_limit` set by `CreateRun` at admission                                             | `passports.limits` jsonb holds the passport's ceilings (X-08 not frozen)                                                                   | Record that `token_limit` is copied from the passport at admission and never raised afterwards ("raising configured limits cannot exceed an existing passport").                             |
| 7   | Paused budget and run state       | `paused boolean`; "orchestration must propagate that flag to the corresponding run state" | `runs.status` text, values from X-11                                                                                                       | Name the run status and terminal reason that a paused budget maps to (X-11).                                                                                                                 |
| 8   | Schema ownership                  | `CREATE SCHEMA IF NOT EXISTS "runtime"`; `down()` keeps the schema                        | SH-16 creates `runtime` the same way; its `down()` drops it without `CASCADE`                                                              | One migration should own the schema. Proposal: SH-16 (the earliest timestamp) owns creation and the final drop; the ledger's `IF NOT EXISTS` and its keep-the-schema `down()` stay harmless. |
| 9   | Timestamp order                   | `1791043000000`                                                                           | `1791041200000` to `1791041440000`                                                                                                         | The ledger sorts **after** all five drafts, not between them. See "Migration order" below.                                                                                                   |
| 10  | Token integer type                | `bigint`                                                                                  | No token columns remain in the drafts                                                                                                      | Nothing to decide; noted because the earlier drafts used `integer`.                                                                                                                          |

### Migration order

On a fresh database the drafts run first (earlier timestamps) and create `runtime`; the ledger then
adds its tables. If the ledger lands on `main` first and a database has already run it, TypeORM
still runs the older-timestamped drafts later as pending migrations. A `pnpm db:migration:revert`
then undoes the most recently executed migration first, so reverting SH-16 would reach
`DROP SCHEMA "runtime"` while the ledger's tables still exist; that statement fails (no `CASCADE`),
which is safe but blocks the revert until the ledger is reverted. Item 8 removes the ambiguity.

## Database test variables

- The GO-06 tests read `GATEWAY_TEST_DATABASE_URL` first, then fall back to `POSTGRES_HOST`,
  `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB`, and treat
  `TEST_DATABASE_REQUIRED=1` as "missing configuration is a failure". `pnpm test:db gateway` sets
  exactly the `POSTGRES_*` settings and `TEST_DATABASE_REQUIRED=1`, so the two already work
  together.
- One edge was fixed on this branch: `pnpm test:db` finds the Go database tests by running
  `go test` once without database settings and collecting the tests that skip. It removed
  `POSTGRES_*` and `TEST_DATABASE_REQUIRED` but not `GATEWAY_TEST_DATABASE_URL`; a developer with
  that variable set would have seen the GO-06 tests run instead of skip, and the command would have
  reported "no Go test needs the database". The identification run now removes it too.
- Still for the Go implementer: the GO-06 tests need the ledger tables to exist ("No test creates
  schemas or tables"), so `pnpm db:migration:run` must run first, and they write uniquely named rows
  into the configured database and remove them afterwards. The API's draft-schema tests
  (`apps/api/src/database/draft-schemas.db-spec.ts`) instead create and drop their own temporary
  database. Either is fine; say which one the team's suite (SH-47) should expect.

## Answers already applied to the drafts

1. Money: integer minor units plus a currency code (`bigint`, `character(3)`), as in SH-17.
2. Passport scope and limits stay `jsonb` for the draft. Identity, organization, task version,
   admission catalog revision, issue time and expiry are separate columns.
3. The passport trigger now rejects `UPDATE` and row `DELETE`. `TRUNCATE` is deliberately not
   blocked: `pnpm reset:demo` truncates the runtime schema as the owner, and the service roles
   (SH-26) grant neither `DELETE` nor `TRUNCATE`.
4. `audit_events.run_id` is nullable for events without a run (a rejected admission, a failed
   policy reload); no placeholder run is created. An event that names an action must name its run,
   and `(organization_id, id)` is indexed for organization-wide reads.
5. Database tests: `apps/api/src/database/draft-schemas.db-spec.ts`, run by `pnpm test:db api`.
