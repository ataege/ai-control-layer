import type { MigrationInterface, QueryRunner } from "typeorm";

// GO-39 (Go lane f3): applies alignment decisions 1 to 5 of docs/contracts/runtime-schema-alignment.md
// to the GO-06 token ledger, without editing 1791043000000.
//
// 1. run_id becomes uuid with an organization-safe foreign key to runtime.runs.
// 2. Both ledger tables carry organization_id.
// 3. call_id is runtime.model_calls.id; the foreign key also binds the purpose, so a call cannot spend
//    the other purpose's capacity.
// 4. Per-purpose token sub-limits (nullable, as in X-08) and counters inside the shared token_limit.
// 5. The ledger is the single authority for model calls, request time and the concurrency slot.
//
// Pre-alignment diagnostic leftovers (old cmd/budgetcheck ledgers and old test rows) cannot satisfy
// the new keys: ledger rows without a runtime.runs row, and reservations whose call id is not a uuid,
// are deleted (approved by the lead). A reservation of an existing run whose uuid call id has no
// dispatch record gets a backfilled runtime.model_calls row labelled as such. Limits of existing
// ledgers are copied from their passport; a key missing there falls back to the report's
// illustrative values.
const UUID_PATTERN = "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$";

export class AlignTokenLedger1791130000000 implements MigrationInterface {
  name = "AlignTokenLedger1791130000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // Diagnostic leftovers without a run, or with a call id that cannot be a dispatch record.
    await queryRunner.query(`
      DELETE FROM runtime.model_token_reservations AS reservation
      WHERE reservation.call_id !~ '${UUID_PATTERN}'
         OR reservation.run_id !~ '${UUID_PATTERN}'
         OR NOT EXISTS (SELECT 1 FROM runtime.runs AS run WHERE run.id::text = reservation.run_id)`);
    await queryRunner.query(`
      DELETE FROM runtime.model_token_budgets AS budget
      WHERE budget.run_id !~ '${UUID_PATTERN}'
         OR NOT EXISTS (SELECT 1 FROM runtime.runs AS run WHERE run.id::text = budget.run_id)`);
    // Dispatches reserved before the call log existed get their pre-dispatch record backfilled.
    await queryRunner.query(`
      INSERT INTO runtime.model_calls (id, organization_id, run_id, purpose, model, outcome, dispatch_recorded_at)
      SELECT reservation.call_id::uuid, run.organization_id, run.id, reservation.purpose,
             'unrecorded (pre-alignment ledger backfill)',
             CASE reservation.status WHEN 'settled' THEN 'completed' ELSE 'usage_unknown' END, now()
      FROM runtime.model_token_reservations AS reservation
      JOIN runtime.runs AS run ON run.id::text = reservation.run_id
      WHERE NOT EXISTS (SELECT 1 FROM runtime.model_calls AS call WHERE call.id::text = reservation.call_id)`);

    await queryRunner.query(`
      ALTER TABLE runtime.model_token_reservations
        DROP CONSTRAINT model_token_reservations_run_id_fkey,
        DROP CONSTRAINT model_token_reservations_pkey,
        DROP CONSTRAINT model_token_reservations_call_id_check`);
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_budgets
        DROP CONSTRAINT model_token_budgets_pkey,
        DROP CONSTRAINT model_token_budgets_run_id_check`);

    // Ledger balances: uuid run id, organization, purpose sub-limits, calls, time and the slot.
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_budgets
        ALTER COLUMN run_id TYPE uuid USING run_id::uuid,
        ADD COLUMN organization_id uuid,
        ADD COLUMN agent_token_limit bigint,
        ADD COLUMN security_token_limit bigint,
        ADD COLUMN agent_reserved_tokens bigint NOT NULL DEFAULT 0,
        ADD COLUMN agent_used_tokens bigint NOT NULL DEFAULT 0,
        ADD COLUMN security_reserved_tokens bigint NOT NULL DEFAULT 0,
        ADD COLUMN security_used_tokens bigint NOT NULL DEFAULT 0,
        ADD COLUMN call_limit bigint,
        ADD COLUMN agent_call_limit bigint,
        ADD COLUMN security_call_limit bigint,
        ADD COLUMN agent_calls bigint NOT NULL DEFAULT 0,
        ADD COLUMN security_calls bigint NOT NULL DEFAULT 0,
        ADD COLUMN request_timeout_ms bigint,
        ADD COLUMN max_concurrent_calls integer,
        ADD COLUMN calls_in_flight integer NOT NULL DEFAULT 0`);
    await queryRunner.query(`
      UPDATE runtime.model_token_budgets AS budget
      SET organization_id = run.organization_id,
          agent_token_limit = (passport.limits ->> 'tokensAgent')::bigint,
          security_token_limit = (passport.limits ->> 'tokensSecurity')::bigint,
          call_limit = COALESCE((passport.limits ->> 'callsTotal')::bigint, 24),
          agent_call_limit = COALESCE((passport.limits ->> 'callsAgent')::bigint, 12),
          security_call_limit = COALESCE((passport.limits ->> 'callsSecurity')::bigint, 12),
          request_timeout_ms = COALESCE((passport.limits ->> 'requestTimeoutSeconds')::bigint, 20) * 1000,
          max_concurrent_calls = COALESCE((passport.limits ->> 'localMaxConcurrency')::integer, 2)
      FROM runtime.runs AS run
      JOIN runtime.passports AS passport
        ON passport.id = run.passport_id AND passport.organization_id = run.organization_id
      WHERE run.id = budget.run_id`);

    // Reservations: uuid ids, organization, request deadline and whether the call holds its slot.
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_reservations
        ALTER COLUMN run_id TYPE uuid USING run_id::uuid,
        ALTER COLUMN call_id TYPE uuid USING call_id::uuid,
        ADD COLUMN organization_id uuid,
        ADD COLUMN request_deadline_at TIMESTAMP WITH TIME ZONE,
        ADD COLUMN slot_held boolean NOT NULL DEFAULT false,
        ADD COLUMN reserved_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()`);
    await queryRunner.query(`
      UPDATE runtime.model_token_reservations AS reservation
      SET organization_id = budget.organization_id,
          slot_held = reservation.status IN ('reserved', 'usage_unknown')
      FROM runtime.model_token_budgets AS budget
      WHERE budget.run_id = reservation.run_id`);
    await queryRunner.query(`
      UPDATE runtime.model_token_budgets AS budget
      SET agent_reserved_tokens = totals.agent_reserved, agent_used_tokens = totals.agent_used,
          security_reserved_tokens = totals.security_reserved, security_used_tokens = totals.security_used,
          agent_calls = totals.agent_calls, security_calls = totals.security_calls,
          calls_in_flight = totals.in_flight
      FROM (
        SELECT run_id,
          COALESCE(sum(token_reservation) FILTER (WHERE purpose = 'agent' AND status <> 'settled'), 0) AS agent_reserved,
          COALESCE(sum(actual_tokens) FILTER (WHERE purpose = 'agent' AND status = 'settled'), 0) AS agent_used,
          COALESCE(sum(token_reservation) FILTER (WHERE purpose = 'security' AND status <> 'settled'), 0) AS security_reserved,
          COALESCE(sum(actual_tokens) FILTER (WHERE purpose = 'security' AND status = 'settled'), 0) AS security_used,
          count(*) FILTER (WHERE purpose = 'agent') AS agent_calls,
          count(*) FILTER (WHERE purpose = 'security') AS security_calls,
          count(*) FILTER (WHERE slot_held)::integer AS in_flight
        FROM runtime.model_token_reservations GROUP BY run_id
      ) AS totals
      WHERE totals.run_id = budget.run_id`);

    await queryRunner.query(`
      ALTER TABLE runtime.model_token_budgets
        ALTER COLUMN organization_id SET NOT NULL,
        ALTER COLUMN call_limit SET NOT NULL,
        ALTER COLUMN agent_call_limit SET NOT NULL,
        ALTER COLUMN security_call_limit SET NOT NULL,
        ALTER COLUMN request_timeout_ms SET NOT NULL,
        ALTER COLUMN max_concurrent_calls SET NOT NULL,
        ADD CONSTRAINT model_token_budgets_pkey PRIMARY KEY (run_id),
        ADD CONSTRAINT model_token_budgets_run_organization UNIQUE (run_id, organization_id),
        ADD CONSTRAINT model_token_budgets_run_fkey FOREIGN KEY (run_id, organization_id)
          REFERENCES runtime.runs (id, organization_id) ON DELETE RESTRICT,
        ADD CONSTRAINT model_token_budgets_agent_limit
          CHECK (agent_token_limit IS NULL OR (agent_token_limit > 0 AND agent_token_limit <= token_limit)),
        ADD CONSTRAINT model_token_budgets_security_limit
          CHECK (security_token_limit IS NULL OR (security_token_limit > 0 AND security_token_limit <= token_limit)),
        ADD CONSTRAINT model_token_budgets_purpose_counters CHECK (agent_reserved_tokens >= 0 AND agent_used_tokens >= 0
          AND security_reserved_tokens >= 0 AND security_used_tokens >= 0),
        ADD CONSTRAINT model_token_budgets_call_limits
          CHECK (call_limit > 0 AND agent_call_limit >= 0 AND security_call_limit >= 0),
        ADD CONSTRAINT model_token_budgets_call_counters CHECK (agent_calls >= 0 AND security_calls >= 0),
        ADD CONSTRAINT model_token_budgets_request_time CHECK (request_timeout_ms > 0),
        ADD CONSTRAINT model_token_budgets_concurrency CHECK (max_concurrent_calls > 0 AND calls_in_flight >= 0)`);
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_reservations
        ALTER COLUMN organization_id SET NOT NULL,
        ADD CONSTRAINT model_token_reservations_pkey PRIMARY KEY (run_id, call_id),
        ADD CONSTRAINT model_token_reservations_budget_fkey FOREIGN KEY (run_id, organization_id)
          REFERENCES runtime.model_token_budgets (run_id, organization_id) ON DELETE RESTRICT,
        ADD CONSTRAINT model_token_reservations_call_fkey FOREIGN KEY (call_id, organization_id, purpose)
          REFERENCES runtime.model_calls (id, organization_id, purpose) ON DELETE RESTRICT`);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_reservations
        DROP CONSTRAINT model_token_reservations_call_fkey,
        DROP CONSTRAINT model_token_reservations_budget_fkey,
        DROP CONSTRAINT model_token_reservations_pkey,
        DROP COLUMN organization_id,
        DROP COLUMN request_deadline_at,
        DROP COLUMN slot_held,
        DROP COLUMN reserved_at,
        ALTER COLUMN run_id TYPE text USING run_id::text,
        ALTER COLUMN call_id TYPE text USING call_id::text`);
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_budgets
        DROP CONSTRAINT model_token_budgets_run_fkey,
        DROP CONSTRAINT model_token_budgets_run_organization,
        DROP CONSTRAINT model_token_budgets_pkey,
        DROP CONSTRAINT model_token_budgets_agent_limit,
        DROP CONSTRAINT model_token_budgets_security_limit,
        DROP CONSTRAINT model_token_budgets_purpose_counters,
        DROP CONSTRAINT model_token_budgets_call_limits,
        DROP CONSTRAINT model_token_budgets_call_counters,
        DROP CONSTRAINT model_token_budgets_request_time,
        DROP CONSTRAINT model_token_budgets_concurrency,
        DROP COLUMN organization_id,
        DROP COLUMN agent_token_limit,
        DROP COLUMN security_token_limit,
        DROP COLUMN agent_reserved_tokens,
        DROP COLUMN agent_used_tokens,
        DROP COLUMN security_reserved_tokens,
        DROP COLUMN security_used_tokens,
        DROP COLUMN call_limit,
        DROP COLUMN agent_call_limit,
        DROP COLUMN security_call_limit,
        DROP COLUMN agent_calls,
        DROP COLUMN security_calls,
        DROP COLUMN request_timeout_ms,
        DROP COLUMN max_concurrent_calls,
        DROP COLUMN calls_in_flight,
        ALTER COLUMN run_id TYPE text USING run_id::text`);
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_budgets
        ADD CONSTRAINT model_token_budgets_pkey PRIMARY KEY (run_id),
        ADD CONSTRAINT model_token_budgets_run_id_check CHECK (run_id <> '')`);
    await queryRunner.query(`
      ALTER TABLE runtime.model_token_reservations
        ADD CONSTRAINT model_token_reservations_pkey PRIMARY KEY (run_id, call_id),
        ADD CONSTRAINT model_token_reservations_call_id_check CHECK (call_id <> ''),
        ADD CONSTRAINT model_token_reservations_run_id_fkey FOREIGN KEY (run_id)
          REFERENCES runtime.model_token_budgets (run_id)`);
    // Backfilled dispatch records stay: they describe calls that were made.
  }
}
