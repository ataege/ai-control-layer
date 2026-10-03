# @workspace/contracts

Wire contracts shared by the services. The baseline defines the generic ones: health, service diagnostics and the error envelope. Product contracts are added here by the integration owner, contract first.

| Piece                   | Location                                            |
| ----------------------- | --------------------------------------------------- |
| TypeScript types        | `src/index.ts`                                      |
| Language-neutral schema | `schemas/*.schema.json` (JSON Schema draft 2020-12) |
| Shared sample payloads  | `fixtures/<schema>.<case>.json`                     |
| Go DTOs                 | `services/gateway/internal/health/dto.go`           |

`pnpm --filter @workspace/contracts test` validates every fixture against its schema and compares
the typed samples with the fixture files.
The gateway's Go tests decode the same fixtures with unknown fields disallowed.

The package is compiled to `dist/` because the API runs as plain Node ESM.
Turborepo builds it before any task that depends on it.

Changing a contract means updating the type, the schema, the fixtures and the Go DTO together.
One owner, the integration role, merges those changes.
