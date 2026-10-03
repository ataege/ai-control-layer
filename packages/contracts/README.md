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
| `ReportView` (X-64) | `GET /internal/runs/{runId}/reports/{reportId}`     | `report-view.schema.json`         | `provenance.ReportView`   | Go-owned (lane w2); NestJS consumes |

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
