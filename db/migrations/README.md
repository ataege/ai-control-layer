# db/migrations

This directory intentionally contains no migration files.

## Where migrations live

TypeORM migrations live in the API package, in `apps/api/src/database/migrations`. The integration role writes and orders them; the nestjs role maintains the tooling (see `AGENTS.md`).
They cannot live here: the TypeORM CLI and `tsc` cannot compile migration classes outside the API
package (verified: `TS6059` "file is not under rootDir", and the `typeorm` import cannot be resolved
from outside the package under pnpm's strict `node_modules`). The architecture specification shows
`db/migrations/` and `db/seeds/`; migrations still cannot live here for that reason, and where seeds
live is not decided (open item `repository layout`).

There is one migration owner and one toolchain for the shared PostgreSQL instance. The Go gateway
adds no migration framework; schema changes for tables it reads also go through the API migrations.

Nothing runs migrations automatically. The API starts with `synchronize: false` and
`migrationsRun: false`, so tables only appear after an explicit command.

## Commands (from the repository root)

```sh
pnpm db:migration:show              # list migrations and whether they ran
pnpm db:migration:create <Name>     # empty migration file
pnpm db:migration:generate <Name>   # migration from the difference between entities and the database
pnpm db:migration:run               # apply pending migrations
pnpm db:migration:revert            # undo the most recent migration
```

They need the root `.env` (`pnpm run setup`) and, except for `create`, a running PostgreSQL
(`pnpm infra:up`).

## Adding the first entity and migration

1. Create the entity class in the API module that owns it (`apps/api/src/<module>/<name>.entity.ts`),
   with `schema: "app"`. Only `app` tables become entities; `runtime` and `demo` tables get
   hand-written migrations (`pnpm db:migration:create <Name>`) from the migration owner.
2. Add the class to the `entities` array in `apps/api/src/database/typeorm-options.ts`. The Nest runtime
   and the CLI share this file.
3. Run `pnpm db:migration:generate <Name>`. The file is written to `apps/api/src/database/migrations`.
   `generate` never emits `CREATE SCHEMA`, so the first migration of each schema adds it by hand.
4. Review the generated SQL, run `pnpm format`, then apply it with `pnpm db:migration:run`.
5. Commit the entity and the migration together.

## Expected behaviour with no entity change

When the entities and the migrated database already agree (also the case with zero entities, as in the
starter baseline), `pnpm db:migration:generate <Name>` prints

```
No changes in database schema were found - cannot generate a migration. To create a new empty migration use "typeorm migration:create" command
```

and exits with a non-zero code. That is correct: it shows the migrations cover every entity.

TypeORM keeps its bookkeeping in a table named `migrations` in the `public` schema.
`db:migration:show` creates that table, empty, when it is missing (observed on an empty database);
`run` and `revert` also create it if it is missing. `generate` and `create` do not create it.

## Model token ledger (GO-06)

`1791043000000-AddModelTokenBudgets.ts` adds `runtime.model_token_budgets` and
`runtime.model_token_reservations`. Go owns every balance mutation; NestJS only supplies this
migration through the shared toolchain. This task adds no runtime TypeORM entities or startup
schema changes. The migration's down step drops only its own two tables and retains the schema.
Go package behavior, test commands and the future service-role/run relationship are documented
in `services/gateway/internal/budget/README.md` and `services/gateway/README.md`.
