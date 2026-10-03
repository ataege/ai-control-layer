# Model allowance ledger

`PostgresStore` is the run's model allowance ledger and the single authority for model calls,
tokens (shared and per purpose), request time and the per-run concurrency slot (alignment decisions
1 to 7 in `docs/contracts/runtime-schema-alignment.md`). Admission opens the ledger with
`OpenRunLedger` inside its own transaction, copying the passport's limits; construction never
creates tables, ledgers or fixtures.

Tables (migrations `1791043000000-AddModelTokenBudgets` and `1791130000000-AlignTokenLedger`):

- `runtime.model_token_budgets`, one row per run: `run_id uuid` with an organization-safe foreign
  key to `runtime.runs`, `organization_id`, the shared `token_limit` with `reserved_tokens` and
  `used_tokens`, optional `agent_token_limit` and `security_token_limit` with per-purpose counters,
  `call_limit`, `agent_call_limit`, `security_call_limit` with `agent_calls` and `security_calls`,
  `request_timeout_ms`, `max_concurrent_calls` with `calls_in_flight`, and `paused`.
- `runtime.model_token_reservations`, one row per call: `call_id` is the call's
  `runtime.model_calls.id`; the foreign key `(call_id, organization_id, purpose)` means a call cannot
  spend the other purpose's capacity. Status `reserved`, `usage_unknown` or `settled`; settled rows
  carry input, output and actual tokens (actual = input + output). `slot_held` and
  `request_deadline_at` record the concurrency slot and the request deadline.

Rules:

- Every mutation locks the run's ledger row first, in a short transaction that never spans a model
  request: reserve, commit, dispatch, then settle in a later transaction.
- `Reserve` refuses before dispatch, writing nothing, when the call is already reserved, the ledger
  is paused, a shared or purpose call limit or token allowance would be exceeded (all
  `errors.Is(err, ErrExhausted)`), or all slots are held (`ErrConcurrencyLimit`). It needs the call's
  dispatch record. It returns the request timeout, and `model.AccountedCaller` bounds the provider
  request with it.
- `ReserveWithin` is `Reserve` under a `Ceiling` (the active catalog revision's call counts, token
  total and request time): each limit is the lower of the stored one and the ceiling, under the same
  lock, so a lowered revision refuses the next reservation and a raised one widens nothing (GO-86).
- A call counts when its reservation is granted and is never refunded: a dispatched call is a call.
  `CountAgentCalls` is the agent loop's step count.
- `MarkUnknown` keeps the whole reservation and the slot: a timeout or missing usage does not prove
  that nothing was used or that the provider stopped. `Snapshot` reports unresolved calls per
  purpose; unknown usage is never shown as zero.
- `Settle` records measured usage once, refunds the unused reservation and releases the slot. The
  same counters again return `AlreadySettled`, different ones `ErrConflict`. A measured count above
  the reservation is recorded in full and pauses the ledger; a count that cannot fit the aggregate
  pauses it and keeps the reservation.
- The per-run `calls_in_flight` is evidence and a per-run cap, not the report's process-wide local
  concurrency limit, which is GO-79.
- All storage failures fail closed and hide driver details. Local inference has no tariff, so no
  monetary cost is recorded; estimated cost is not invented as zero.

`budgettest` seeds synthetic runs with an open ledger for database tests.
