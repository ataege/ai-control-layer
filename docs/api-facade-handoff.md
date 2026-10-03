# NestJS runtime facade handoff

All product routes require the stored HttpOnly session and current organization membership.
NestJS supplies the service token and signed operator context only to the private gateway;
the browser supplies record references, never identity. Go authorizes objects and owns effects.
Run state and usage are separate responses; no combined wire shape exists.

| Public route                                                                       | Response contract                                                        | Additional access check                                                           |
| ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `POST /api/runs`                                                                   | `StartRunResponse`                                                       | Go admission                                                                      |
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
worker field; API-15's individual worker presentation still requires a coordinated contract and web change.
