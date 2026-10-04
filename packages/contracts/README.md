# @workspace/contracts

Wire contracts shared by the services. The baseline defines the generic ones: health, service diagnostics and the error envelope. Product contracts are added here by the nestjs role (the web + API implementer), contract first.

| Piece                   | Location                                                                                                                                                                               |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| TypeScript types        | `src/index.ts`                                                                                                                                                                         |
| Language-neutral schema | `schemas/*.schema.json` (JSON Schema draft 2020-12)                                                                                                                                    |
| Shared sample payloads  | `fixtures/<schema>.<case>.json`                                                                                                                                                        |
| Go DTOs                 | `services/gateway/internal/health/dto.go` (generic), `services/gateway/internal/contracts` (runtime: X-07 mirror, X-08, X-09, X-11, X-12, X-13), and the Go-owned read contracts below |

`pnpm --filter @workspace/contracts test` validates every fixture against its schema and compares
the typed samples with the fixture files.
The gateway's Go tests decode the same fixtures with unknown fields disallowed.

The package is compiled to `dist/` because the API runs as plain Node ESM.
Turborepo builds it before any task that depends on it.

## Go-owned review and approval contracts (lane w3); NestJS consumes

The gateway's review and approval routes answer with these shapes; the request of the approval
route is the X-10 `ApprovalDecision` above. The fixtures were generated from Go's own encoding of a
real frozen review and real decisions, and `TestReviewContractFixturesMatchTheGoTypes`
(`services/gateway/internal/policy`) decodes each strictly into its Go type, re-encodes it unchanged
and recomputes the review payload's digest. A change starts in Go (lane w3) and lands here with its
fixture; NestJS consumes the shapes and does not reshape them.

| Contract           | Route                                                             | Schema                          | Go type                   | Owner                               |
| ------------------ | ----------------------------------------------------------------- | ------------------------------- | ------------------------- | ----------------------------------- |
| `ReviewView`       | `GET /internal/actions/{actionId}/review`                         | `review-view.schema.json`       | `policy.ReviewPayload`    | Go-owned (lane w3); NestJS consumes |
| `ApprovalResponse` | `POST /internal/actions/{actionId}/approval` (`ApprovalDecision`) | `approval-response.schema.json` | `policy.approvalResponse` | Go-owned (lane w3); NestJS consumes |

`ReviewView` keys are snake_case: the frozen payload's stored SHA-256 digest is computed over exactly
this encoding, so they are not renamed. A `queue_report` review always carries the resolved
`recipient` (reference, vendor id, registered address) and the stored `report` (content, hash,
template and projection versions, classification, sorted source manifest and its digest); the
review screen shows these, never values from the model. The review expires with the run's passport.

Error statuses (shared `ErrorResponse` envelope, `error.code` in brackets):

| Status | Review route                                                       | Approval route                                                                                                                                            |
| ------ | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 400    | malformed action id (`bad_request`)                                | body not a strict X-10 decision, any extra field, oversized (over 1 KiB), or a malformed id (`bad_request`)                                               |
| 401    | missing service token or operator context (`unauthorized`)         | same (`unauthorized`)                                                                                                                                     |
| 403    | the operator is not a reviewer of the organization (`forbidden`)   | same (`forbidden`); reviewer authority is read from `app.memberships`, not only from the token                                                            |
| 404    | no frozen review for this action in the organization (`not_found`) | no action awaiting approval here, including another organization's (`not_found`)                                                                          |
| 409    | none                                                               | `approval_expired`; `action_changed` (action or frozen material altered); `run_cancelled` (run stopped or cancel requested); `conflict` (already decided) |
| 503    | review unavailable (`decision_unavailable`)                        | nothing stored (`decision_unavailable`)                                                                                                                   |

## Go-owned read contracts (lane w2); NestJS consumes

The gateway's private read routes answer with these shapes. Their source is the Go type; the
fixtures were generated from Go's own encoding, and `TestReadContractFixturesMatchTheGoTypes`
(`services/gateway/internal/reads`) decodes every fixture strictly into that type and re-encodes it
unchanged, so the TypeScript type, the schema and the Go type cannot drift apart. A change starts
in Go (lane w2) and lands here with its fixture in the same change; NestJS consumes the shapes and
does not reshape them.

| Contract            | Route                                               | Schema                            | Go type                   | Owner                               |
| ------------------- | --------------------------------------------------- | --------------------------------- | ------------------------- | ----------------------------------- |
| `RunUsage`          | `GET /internal/runs/{runId}/usage`                  | `run-usage.schema.json`           | `reads.RunUsage`          | Go-owned (lane w2); NestJS consumes |
| `RunEventsPage`     | `GET /internal/runs/{runId}/events?after=&limit=`   | `run-events-page.schema.json`     | `reads.RunEventPage`      | Go-owned (lane w2); NestJS consumes |
| `SecuritySummary`   | `GET /internal/security/summary`                    | `security-summary.schema.json`    | `reads.SecuritySummary`   | Go-owned (lane w2); NestJS consumes |
| `AssessmentRecord`  | items of `AssessmentPage`                           | `assessment-record.schema.json`   | `reads.AssessmentRecord`  | Go-owned (lane w2); NestJS consumes |
| `AssessmentPage`    | `GET /internal/security/assessments?cursor=&limit=` | `assessment-page.schema.json`     | `reads.AssessmentPage`    | Go-owned (lane w2); NestJS consumes |
| `SecurityEventPage` | `GET /internal/security/events?cursor=&limit=`      | `security-event-page.schema.json` | `reads.SecurityEventPage` | Go-owned (lane w2); NestJS consumes |
| `CatalogStatus`     | `GET /internal/catalog/active`                      | `catalog-status.schema.json`      | `reads.CatalogStatus`     | Go-owned (lane w2); NestJS consumes |
| `ReportView` (X-64) | `GET /internal/runs/{runId}/reports/{reportId}`     | `report-view.schema.json`         | `provenance.ReportView`   | Go-owned (lane w2); NestJS consumes |

Change log (additive only; existing fields keep their meaning):

- 2026-10-03: `SecuritySummary.decisions[].rejectionCause` (nullable: `not_json`, `extra_text`,
  `code_fence`, `wrong_status`, `wrong_fields`, `unknown_report`) counts rejected final answers
  (GO-26) by kind, null on every other row. The event pages carry the same value in
  `maskedSummary.rejectionCause` (X-12, 3c).

- 2026-10-04: `CatalogStatus` (WEB-29) added: the active catalog revision, policy and feed digests, the
  last rejected activation and the three controls' settings. The fixtures were generated from the
  handler's real output over the seeded policy; `catalog-status.rejected-request.json` has one
  hand-edited field (`requestedRevisionId: 2`, so the request matches `lastError.revisionId`).

Names: the TypeScript `RunEventsPage` (schema `run-events-page`) is the Go type `reads.RunEventPage`.
`modelCalls` and `modelUsage` always hold two entries in a fixed order, `agent` then `security`.
The fixture `report-view.internal-withheld.json` documents the withheld case the contract allows;
the API does not reach it today, because every verified operator of the organization may read
internal content (lead decision).

The run state (`GET /internal/runs/{runId}`) is X-11's `RunState` above; the event pages carry
X-12's `SafeEvent` by reference (`safe-event.schema.json`). Points that matter to a consumer:

- `RunUsage` counts recorded dispatches and the run's model ledger only: unknown usage is its own
  count and is never zero; there is no cost field (local inference has no tariff). `ledger` is null
  for a run without a ledger.
- The two organization-wide pages use an opaque window cursor (`v1.<low>.<high>.<afterId>`): every
  committed row is returned exactly once, ordered by id within a window, so sort by id for display.
  An empty page with an unchanged cursor means "nothing new is final yet" (a long transaction
  elsewhere can hold rows back), not "no records".
- An `AssessmentRecord`'s `verdictSource` is `live` or `fixture` on a semantic record that
  classified something, and null on a deterministic one and on a semantic check that made no model
  call (`not_applicable`, reason `no_free_text_arguments`). Its `reasonCode` is the control's own
  code, so it may be outside X-13. A `fixture` verdict is never detection quality.
- `inputSource: "judge"` marks a judge probe's evidence (GO-82), in assessments and summary counts;
  `judgeSecurityCalls` counts the probes' security model calls apart, while `modelUsage` counts all.
- `ReportView.content` is null exactly when `contentWithheld` is true; read `classification` from
  the view instead of computing a label.

Changing a contract means updating the type, the schema, the fixtures and the Go DTO together.
The nestjs role (the web + API implementer) coordinates and merges those changes after a quick shared review, and the go role mirrors the Go DTO. Each contract's recorded owner is listed in `docs/product/README.md`.

## NestJS policy reload contracts (API-33)

These public HTTP contracts are NestJS-owned and do not add a Go command endpoint:

- `PolicyReloadRequest`: POST `/api/policies/reload`, exactly `{}`. The reviewer and imported actor
  come from the verified current membership, never the request. No file upload or browser-selected path.
- `PolicyReloadResponse`: HTTP 202 requested with requestedRevisionId/fileDigest/feedRevision,
  or HTTP 200 unchanged with revisionId. IDs are positive decimal strings, preserving PostgreSQL
  bigint precision. feedRevision is the publisher's version label (for example feed_v1), null when absent.
- `PolicyReloadErrorResponse`: the shared envelope shape plus an optional error.issues list of
  safe path/message pairs for HTTP 400 policy/feed rejection. This is a separate NestJS schema;
  the Go-owned error schema is unchanged. A pending edit is HTTP 409 error.code revision_pending.
- `PolicyStatusResponse`: reviewer-only GET `/api/policies/status`, with requested/validated/active
  revision IDs, activeFeedRevisionId and lastError. Error mapping is approved by the lead:
  Go code/message/revision_id/stage become code/message/revisionId/stage; importer issues become
  policy_reload_rejected, joined issue messages, null revisionId and import_validation stage.
  No raw source text, digest extras or arbitrary stored error fields are returned.

Schemas and fixtures use the policy-reload-request, policy-reload-response,
policy-reload-error-response and policy-status-response prefixes. Typed fixture tests pin each shape.
Only Go's catalog watcher validates and activates a requested revision; 202 is not activation success.
