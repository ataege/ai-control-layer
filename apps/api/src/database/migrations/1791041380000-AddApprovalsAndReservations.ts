import type { MigrationInterface, QueryRunner } from "typeorm";

// DRAFT (SH-27), pending the Go implementer's approval; builds on the SH-16 and SH-44 drafts.
// Go owns these tables: no NestJS entities, hand-written SQL, no data. Every row carries
// `organization_id` and references its parents by (id, organization_id). Actual usage stays in
// `runtime.model_usage` (SH-44); a reservation holds only what was reserved and its state.
export class AddApprovalsAndReservations1791041380000 implements MigrationInterface {
  name = "AddApprovalsAndReservations1791041380000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // Reference targets: an approval binds the action's exact digest; a tool reservation binds
    // an execution attempt of the same organization.
    await queryRunner.query(
      `ALTER TABLE "runtime"."actions"
       ADD CONSTRAINT "actions_id_organization_digest" UNIQUE ("id", "organization_id", "action_digest")`,
    );
    await queryRunner.query(
      `ALTER TABLE "runtime"."execution_attempts"
       ADD CONSTRAINT "execution_attempts_id_organization" UNIQUE ("id", "organization_id")`,
    );
    // An execution claim: at most one open (not completed) attempt per action, so two workers
    // cannot claim the same action at once ("Durable worker recovery requires identifiable
    // dispatched attempts").
    await queryRunner.query(
      `CREATE UNIQUE INDEX "execution_attempts_one_open_per_action" ON "runtime"."execution_attempts" ("action_id")
       WHERE "completed_at" IS NULL`,
    );

    // "Approval would authorize one immutable action": one decision per action, bound to the
    // action's digest (the foreign key fails if it differs), with an expiry, consumed once by the
    // execution attempt that claims it ("An atomic execution claim consumes the approval once").
    // Where the frozen review payload lives is open (`review payload read`); the reference is opaque.
    await queryRunner.query(
      `CREATE TABLE "runtime"."approvals" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "action_id" uuid NOT NULL,
        "action_digest" bytea NOT NULL,
        "review_payload_reference" text NOT NULL,
        "reviewer_id" uuid NOT NULL,
        "decision" text NOT NULL,
        "decided_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "expires_at" TIMESTAMP WITH TIME ZONE NOT NULL,
        "consumed_at" TIMESTAMP WITH TIME ZONE,
        "consumed_by_attempt_id" uuid,
        CONSTRAINT "approvals_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "approvals_one_per_action" UNIQUE ("action_id"),
        CONSTRAINT "approvals_one_per_attempt" UNIQUE ("consumed_by_attempt_id"),
        CONSTRAINT "approvals_decision" CHECK ("decision" IN ('approved', 'rejected')),
        CONSTRAINT "approvals_expiry_after_decision" CHECK ("expires_at" > "decided_at"),
        CONSTRAINT "approvals_consumption_pair"
          CHECK (("consumed_at" IS NULL) = ("consumed_by_attempt_id" IS NULL)),
        CONSTRAINT "approvals_only_approved_consumed"
          CHECK ("consumed_at" IS NULL OR "decision" = 'approved'),
        CONSTRAINT "approvals_action_fkey" FOREIGN KEY ("action_id", "organization_id", "action_digest")
          REFERENCES "runtime"."actions" ("id", "organization_id", "action_digest") ON DELETE RESTRICT,
        CONSTRAINT "approvals_attempt_fkey" FOREIGN KEY ("consumed_by_attempt_id", "organization_id")
          REFERENCES "runtime"."execution_attempts" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    // The only permitted change is the single consumption: setting consumed_at and the attempt
    // once, on an approval that has not expired. Everything else is frozen.
    await queryRunner.query(
      `CREATE FUNCTION "runtime"."guard_approval_update"() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN
        IF OLD."consumed_at" IS NOT NULL THEN
          RAISE EXCEPTION 'approval % is already consumed', OLD."id";
        END IF;
        IF (NEW."id", NEW."organization_id", NEW."action_id", NEW."action_digest",
            NEW."review_payload_reference", NEW."reviewer_id", NEW."decision", NEW."decided_at",
            NEW."expires_at")
           IS DISTINCT FROM
           (OLD."id", OLD."organization_id", OLD."action_id", OLD."action_digest",
            OLD."review_payload_reference", OLD."reviewer_id", OLD."decision", OLD."decided_at",
            OLD."expires_at") THEN
          RAISE EXCEPTION 'approval % is immutable except for its consumption', OLD."id";
        END IF;
        IF NEW."consumed_at" IS NOT NULL AND NEW."consumed_at" >= OLD."expires_at" THEN
          RAISE EXCEPTION 'approval % expired before consumption', OLD."id";
        END IF;
        RETURN NEW;
      END;
      $$`,
    );
    await queryRunner.query(
      `CREATE TRIGGER "approvals_guard" BEFORE UPDATE ON "runtime"."approvals"
       FOR EACH ROW EXECUTE FUNCTION "runtime"."guard_approval_update"()`,
    );

    // One reservation per dispatch, committed before it ("Reserve allowance before agent and
    // guard dispatch"). A model reservation counts against the shared ceiling and its purpose's
    // sub-budget; a tool reservation covers one tool attempt. Settlement compares with
    // runtime.model_usage. `unresolved` keeps the allowance when usage or completion is unknown
    // ("retains unresolved reservations rather than treating consumption as zero").
    await queryRunner.query(
      `CREATE TABLE "runtime"."budget_reservations" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "purpose" text NOT NULL,
        "model_call_id" uuid,
        "execution_attempt_id" uuid,
        "reserved_input_tokens" integer,
        "reserved_output_tokens" integer,
        "concurrency_slot" smallint,
        "state" text NOT NULL DEFAULT 'reserved',
        "reserved_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "resolved_at" TIMESTAMP WITH TIME ZONE,
        CONSTRAINT "budget_reservations_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "budget_reservations_purpose" CHECK ("purpose" IN ('agent', 'security', 'tool')),
        CONSTRAINT "budget_reservations_state" CHECK ("state" IN ('reserved', 'settled', 'unresolved')),
        CONSTRAINT "budget_reservations_resolved_pair"
          CHECK (("state" = 'reserved') = ("resolved_at" IS NULL)),
        CONSTRAINT "budget_reservations_dispatch_link" CHECK (
          ("purpose" = 'tool' AND "execution_attempt_id" IS NOT NULL AND "model_call_id" IS NULL)
          OR ("purpose" <> 'tool' AND "model_call_id" IS NOT NULL AND "execution_attempt_id" IS NULL)),
        CONSTRAINT "budget_reservations_model_tokens" CHECK (
          "purpose" = 'tool'
          OR ("reserved_input_tokens" >= 0 AND "reserved_output_tokens" > 0)),
        CONSTRAINT "budget_reservations_slot_positive"
          CHECK ("concurrency_slot" IS NULL OR "concurrency_slot" > 0),
        CONSTRAINT "budget_reservations_one_per_model_call" UNIQUE ("model_call_id"),
        CONSTRAINT "budget_reservations_one_per_attempt" UNIQUE ("execution_attempt_id"),
        CONSTRAINT "budget_reservations_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "budget_reservations_model_call_fkey"
          FOREIGN KEY ("model_call_id", "organization_id", "purpose")
          REFERENCES "runtime"."model_calls" ("id", "organization_id", "purpose") ON DELETE RESTRICT,
        CONSTRAINT "budget_reservations_attempt_fkey"
          FOREIGN KEY ("execution_attempt_id", "organization_id")
          REFERENCES "runtime"."execution_attempts" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    // A slot stays held while the request may still be running: reserved, or unresolved because
    // "a client timeout does not prove inference stopped". The slot limit itself is the
    // passport's concurrency ceiling, checked by Go.
    await queryRunner.query(
      `CREATE UNIQUE INDEX "budget_reservations_slot_in_use" ON "runtime"."budget_reservations" ("run_id", "concurrency_slot")
       WHERE "concurrency_slot" IS NOT NULL AND "state" IN ('reserved', 'unresolved')`,
    );
    await queryRunner.query(
      `CREATE INDEX "budget_reservations_run_purpose_state" ON "runtime"."budget_reservations" ("run_id", "purpose", "state")`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE "runtime"."budget_reservations"`);
    // Dropping the table drops its trigger; the function goes after it.
    await queryRunner.query(`DROP TABLE "runtime"."approvals"`);
    await queryRunner.query(`DROP FUNCTION "runtime"."guard_approval_update"()`);
    await queryRunner.query(`DROP INDEX "runtime"."execution_attempts_one_open_per_action"`);
    await queryRunner.query(
      `ALTER TABLE "runtime"."execution_attempts" DROP CONSTRAINT "execution_attempts_id_organization"`,
    );
    await queryRunner.query(
      `ALTER TABLE "runtime"."actions" DROP CONSTRAINT "actions_id_organization_digest"`,
    );
  }
}
