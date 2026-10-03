import type { MigrationInterface, QueryRunner } from "typeorm";

// GO-06: Go alone mutates these balances. Run through the existing explicit migration command.
export class AddModelTokenBudgets1791043000000 implements MigrationInterface {
  name = "AddModelTokenBudgets1791043000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`CREATE SCHEMA IF NOT EXISTS "runtime"`);
    await queryRunner.query(`
      CREATE TABLE runtime.model_token_budgets (
        run_id text PRIMARY KEY CHECK (run_id <> ''),
        token_limit bigint NOT NULL CHECK (token_limit > 0),
        reserved_tokens bigint NOT NULL DEFAULT 0 CHECK (reserved_tokens >= 0),
        used_tokens bigint NOT NULL DEFAULT 0 CHECK (used_tokens >= 0),
        paused boolean NOT NULL DEFAULT false
      )
    `);
    await queryRunner.query(`
      CREATE TABLE runtime.model_token_reservations (
        run_id text NOT NULL REFERENCES runtime.model_token_budgets(run_id),
        call_id text NOT NULL CHECK (call_id <> ''),
        purpose text NOT NULL CHECK (purpose IN ('agent', 'security')),
        token_reservation bigint NOT NULL CHECK (token_reservation > 0),
        status text NOT NULL CHECK (status IN ('reserved', 'usage_unknown', 'settled')),
        input_tokens bigint CHECK (input_tokens >= 0),
        output_tokens bigint CHECK (output_tokens >= 0),
        actual_tokens bigint CHECK (actual_tokens >= 0),
        PRIMARY KEY (run_id, call_id),
        CONSTRAINT model_token_reservations_usage_state CHECK (
          (status = 'settled' AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL
            AND actual_tokens IS NOT NULL
            AND actual_tokens::numeric = input_tokens::numeric + output_tokens::numeric)
          OR (status IN ('reserved', 'usage_unknown') AND input_tokens IS NULL
            AND output_tokens IS NULL AND actual_tokens IS NULL)
        )
      )
    `);
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE runtime.model_token_reservations`);
    await queryRunner.query(`DROP TABLE runtime.model_token_budgets`);
    // Keep the schema: other runtime migrations may own objects inside it.
  }
}
