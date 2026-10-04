# WEB-01: frozen fixtures against what the web renders

Review of 2026-10-04 on branch `web/audit-01-02`. It was done by reading the files, not by running
the web app against the API: the API does not compile on the merged `main` of that day, so nothing
here was observed live. Four findings (1, 3, 5 and 6) were re-checked by hand against
`apps/api/src/runs/runs.controller.ts`, `apps/api/src/auth/auth.controller.ts` and
`apps/web/src/lib/product-client.ts`; the rest are the reviewer's reading and carry file references.

## Headline

The run page, `product-client.ts` and `run-timeline.tsx` are built on the legacy contract shapes
`RunView` and `SanitizedEvent`. The API serves the frozen `RunState` and `RunEventsPage` of
`SafeEvent`. Frozen-shape view modules exist (`RunEventTimeline`, `describeRunState`,
`PassportPanel`), but only `ReportPanel` is mounted on a page.

## Roadmap field to fixture to page

| Field                                         | Carried by                                                                 | Rendered by                                                              |
| --------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Task goal                                     | No field (`passport.atlas.json` has `taskVersion` only): open item         | Run page prints `passport.template` as "Goal" (legacy)                   |
| Passport scope, expiry                        | `passport.atlas.json`                                                      | `PassportPanel`, mounted on no page                                      |
| Remaining allowance, usage                    | `run-usage.*.json` (no cost or estimate field: open item)                  | Nothing; no usage route in the web                                       |
| Run states and stop reasons                   | `run-state.*.json` (4 of 7 statuses have a fixture)                        | `describeRunState` is imported by no page; the page switches on legacy   |
| Events: attempt vs effect, rule, replay label | `safe-event.*`, `run-events-page.*`                                        | `RunEventTimeline`, mounted on no page; the page uses legacy timeline    |
| Correction count                              | No `SafeEvent` field: open item                                            | Legacy timeline reads `details.correctionCount`, which no fixture has    |
| Review: recipient, content, version, reason   | `review-view.queue-report.json`                                            | Nothing; no `/api/actions/*` web route                                   |
| Classification, source trail, export denial   | `report-view.*`, `safe-event.export-denied.json`                           | `ReportPanel` omits classification and lineage; denial only in unmounted |
| Report types and internal evidence, task form | No field in `task-form-options.atlas.json` or `StartRunRequest`: open item | Not in `task-form.tsx`                                                   |

`catalog-status.*`, `security-summary`, `security-event-page` and `assessment-*` are rendered by no
page and have no web proxy route; `/security` is in the navigation with no page.

## Mismatches

Blocks the demonstration:

1. `isRunView` (`lib/product-client.ts:74`) requires `id`; the API returns `runId`. Every run read
   becomes `invalid_json` and the page shows "Could not load run". (Re-checked.)
2. Legacy `RunView` and `SanitizedEvent` (`packages/contracts/src/index.ts`) are still consumed by
   `product-client.ts`, `app/runs/[id]/page.tsx` and `components/run-timeline.tsx`;
   `apps/api/src/runs/dto/run-view.dto.ts` has no controller use. Migrate the page to `RunState`,
   `SafeEvent`, `Passport` and `RunUsage`.
3. Events are fetched with `?cursor=`; the API accepts only `after` and `limit`. (Re-checked.)
4. `run-timeline.tsx` reads `event.id`, `event.type`, `event.details.*`; the API sends `eventId`,
   `eventType`, `maskedSummary`. `event.details.isReplay` throws on the first event. Mount
   `RunEventTimeline` and delete the legacy component.
5. The API has no `GET /api/runs/options`; `:id` would take "options". The task form cannot load.
   (Re-checked.)
6. The web proxies `/api/runs/{id}/passport` and `/api/auth/me`; the API has neither route. Both
   404 until API work lands. (Re-checked.)
7. The task form's template id is the task version `reconcile_atlas_v1`; report types and internal
   evidence (report 1.1) have no contract field. Record as an open item with the document owner.

Gaps:

8. No usage fetch or panel (remaining allowance, reserved, `usageUnknown`); estimated cost has no
   fixture (local model, no tariff).
9. No review or approval screen and no web route for `/api/actions/:id/review` or `/approval`.
10. Status alerts cover only the legacy statuses; `stopped`, `queued` and `awaiting_approval` are not
    shown; result report ids are not linked to the report page.
11. `ReportPanel` omits classification, destination class, projection rule, content hash and lineage,
    although its guard requires them.

Cosmetic: `/security` nav link without a page (12); truthful label components render only on
`/components` (13); the web's reason-code messages cover about 22 of 31 codes, `run/labels.ts` is
complete (14); the home page renders a permanently empty `RunTimeline` (15).

## Status after the run-page rebuild (main 3e54f50)

Re-checked on 2026-10-04 on a merge of `origin/main` (3e54f50) into this branch. The run-page rebuild
is `fix/run-page`, merged as e4cbb88, whose last commit is 37f34d5. "Read" means the source was read;
"live" means observed through the web proxy against a running API and gateway.

| Item                  | Status                                                                                                                                                                                    |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1, 2                  | Fixed (read): `isRunView` and `RunView` are gone from `lib/product-client.ts` and the run page. Live: `GET /api/runs/{id}` returned the frozen `RunState` with `runId` through the proxy. |
| 3                     | Fixed (read, live): the client sends `after`; `GET /api/runs/{id}/events?after=0&limit=3` returned a `SafeEvent` page through the proxy.                                                  |
| 4                     | Fixed (read): `RunEventTimeline`, `RunStatePanel` and `PassportPanel` are mounted in `app/runs/[id]/page.tsx`.                                                                            |
| 5                     | Not re-checked.                                                                                                                                                                           |
| 6                     | Still open (live): `GET /api/runs/{id}/passport` and `GET /api/auth/me` answer 404 from the API for a real run and a signed-in operator. Waiting for the API routes (Noyan).              |
| 8                     | Partly fixed: a web `usage` route exists and `GET /api/runs/{id}/usage` returned a `RunUsage` live; whether the page shows it was not checked. Estimated cost stays an open item.         |
| 9                     | Partly fixed (read): `approval` and `reports` branches were merged; the review screen was not re-checked.                                                                                 |
| 12                    | Fixed (read): `app/security` exists.                                                                                                                                                      |
| 7, 10, 11, 13, 14, 15 | Not re-checked.                                                                                                                                                                           |
