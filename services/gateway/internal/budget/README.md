# Durable model token accounting

`PostgresStore` owns shared per-run token balances. Call `CreateRun` explicitly at
admission; construction never creates tables, budgets or fixtures. The application
migration toolchain must provide:

- `runtime.model_token_budgets`: `run_id text PRIMARY KEY`, `token_limit bigint`
  greater than zero, `reserved_tokens bigint` and `used_tokens bigint` nonnegative
  with default zero, and `paused boolean NOT NULL DEFAULT false`.
- `runtime.model_token_reservations`: composite primary key `(run_id, call_id)`,
  `run_id` foreign key to the balance row, `purpose text` restricted to `agent`
  or `security`, positive `token_reservation bigint`, `status text` restricted to
  `reserved`, `usage_unknown` or `settled`, and nullable nonnegative `input_tokens`,
  `output_tokens` and `actual_tokens bigint`. Settled rows require all usage fields;
  their actual count equals input plus output. Other states have no usage fields.

Every balance mutation locks the run row first, then its reservation. Both agent
and security calls consume this same balance. `Reserve` rejects an already-used
call ID, a paused run or insufficient allowance before the caller dispatches.
A transport timeout or missing usage calls `MarkUnknown`: it releases nothing.
Reconstructing the store retains this state. A reliable late result can settle
once; the same counters return `AlreadySettled`, while different counters return
`ErrConflict`. The caller must authenticate and correlate late information before
calling `Settle`; a model response is not permission to reopen a call or retry it.

Settlement releases the whole outstanding reservation and records the entire
measured count as used. If measurement exceeds its reservation, the stored budget
is paused even when its overall balance would still suffice. The stored paused
flag fences subsequent model dispatch; admission/run orchestration must propagate
that flag to the corresponding run state. All storage failures fail closed and
return sanitized errors. Counter sums reject overflow. If an otherwise valid
measured count would overflow the aggregate balance, the transaction pauses the
budget, retains the full reservation as `usage_unknown`, and returns an accounting
error without clipping either balance. Public operations reject a nil context.

Run the integration tests only against a dedicated, migrated test database:

```sh
GATEWAY_TEST_DATABASE_URL=postgresql://... go -C services/gateway test -race ./internal/budget
```

The suite also accepts `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`,
`POSTGRES_PASSWORD` and `POSTGRES_DB`, matching the repository database test
runner. `TEST_DATABASE_REQUIRED=1` makes missing configuration a failure.

Tests create uniquely identified budget rows and remove only their own rows. They
cover concurrent agent/security reservations, restart reads, retained unknown
usage, one-time late settlement, conflicting late counters and untruncated
overruns. If database configuration is absent, these database tests explicitly skip; that is not
evidence of PostgreSQL verification. No test creates schemas or tables.
