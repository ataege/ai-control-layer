# NestJS runtime facade handoff

All product routes require the stored HttpOnly session and current organization membership.
NestJS supplies the service token and signed operator context only to the private gateway;
the browser supplies record references, never identity. Go authorizes objects and owns effects.
Run state and usage are separate responses; no combined wire shape exists.

`GET /api/auth/me` serves only the verified user's id/email/name and the current membership's
organizationId/roles, with no-store. The merged web's sign-in/profile/sign-out/revoked-profile
HTTP check returned 200/200/200/401. See the API README for setup and current check limitations.

| Public route                                                                       | Response contract                                                        | Additional access check                                                           |
| ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `POST /api/runs`                                                                   | `StartRunResponse`                                                       | Go admission                                                                      |
| `GET /api/runs/options`                                                            | `TaskFormOptions`                                                        | Verified organization; private Go task-options read, no extra role                |
| `GET /api/runs/{id}`                                                               | `RunState`, including `resultReference`                                  | Go object scope                                                                   |
| `GET /api/runs/{id}/usage`                                                         | `RunUsage`                                                               | Go object scope                                                                   |
| `GET /api/runs/{id}/events?after=&limit=`                                          | `RunEventsPage`                                                          | Go object scope; event organization/run and ordering checked                      |
| `POST /api/runs/{id}/cancel`                                                       | `RunState`                                                               | Go object scope; body empty                                                       |
| `GET /api/runs/{id}/reports/{reportId}`                                            | `ReportView`                                                             | Go object/content scope; withheld content remains null                            |
| `GET /api/actions/{id}/review`                                                     | `ReviewView`                                                             | `reviewer` before Go; snake_case payload unchanged                                |
| `POST /api/actions/{id}/approval`                                                  | `ApprovalResponse`                                                       | `reviewer` before Go; exactly `{"decision":"approve"}` or `{"decision":"reject"}` |
| `GET /api/security/summary`                                                        | `SecuritySummary`                                                        | Verified organization; summary organization checked                               |
| `GET /api/security/export?kind=events\|assessments&format=json\|csv&after=&limit=` | `SecurityEventPage` or `AssessmentPage`; CSV uses the same record fields | `reviewer` before Go                                                              |
| `POST /api/control/evaluate`                                                       | `ControlEvaluationResponse`                                              | X-91 input on an admitted run; Go active-run/scope/budget checks                  |

Audit export requires `kind`; omitted `format` selects JSON. A page is bounded to 500 records;
Go defaults to 100 when `limit` is omitted. Browser `after` maps to Go's `cursor`. JSON returns
the entire Go page unchanged. CSV serializes nested metadata as JSON cells and neutralizes
formula prefixes; its next page reference is the Go value in `X-Next-Cursor`.
The web proxy must preserve that header and the CSV content type. Web routes are owned by Batın
and were not changed in this API work.

Go's 401/403/404/409/503 remain errors. An unavailable or malformed read returns 503;
command timeout returns 504 `outcome_unconfirmed`, because it does not prove an effect failed.
A 200 control-evaluation denial is a recorded deny decision and never an approval grant.

For X-91, send all five fields: `runId`, `kind`, `text`, `tool`, `arguments`. `model_input`
uses non-null text with null tool/arguments; `tool_result` uses text/tool with null arguments;
`action_proposal` uses null text and non-null tool/arguments. The draft judge client still needs
the lead's contract update; do not add compatibility defaults to the API.

Verification: `pnpm --filter api run lint`, `typecheck`, `test`, `build`, `pnpm test:db api`,
and `pnpm verify`. The sanitized real API/Go audit capture is
[api-audit-export-2026-10-04.json](evidence/api-audit-export-2026-10-04.json).
It records stable before/after counts, not a transactionally atomic export or a semantic-quality
measurement. The API authorization database tests use real credentials/sessions and explicitly
labelled Go response fixtures. New live-model approval/outbox execution and Docker were not tested.

Gateway readiness already returns 503 when its worker/catalog is not ready. The existing diagnostics
route reports that aggregate result as degraded. The shared readiness contract has no separate
worker field. Decision 11 explicitly retains that shape: worker/catalog unavailability is represented
by upstream HTTP 503, even when its database check is up. The API preserves this as degraded;
API-15's browser presentation remains the web owner's verification task, with no invented worker field.

API-12 is implemented against the lead-approved private Go GET /internal/task-options route;
the shared schema is validated and the response is returned unchanged. Real Go/API/web proxy
reads returned identical 200 responses; source failures never return default choices.
API-33 authenticated policy reload is awaiting implementation against the lead's supplied flow.
The merged web still expects draft combined RunView and cursor query naming;
its owner must adopt the separate shared RunState/RunUsage and events after parameter.
Host smoke after the web merge reports 22 passed, 8 failed, 6 skipped: / and /components redirect
to login, whereas smoke expects 200 and therefore cannot scan those pages' assets. The API routes
and real web/API identity flow passed, but this is not a passing overall smoke run. The script/web
owners must reconcile public-page policy and smoke expectations before the local profile follow-up is pushed.
