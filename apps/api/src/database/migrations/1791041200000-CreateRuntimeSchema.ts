import type { MigrationInterface, QueryRunner } from "typeorm";

// DRAFT (SH-16), pending the Go implementer's approval: the first `runtime` migration. Go owns
// these tables, so there are no NestJS entities; the SQL is hand-written. Table names are the
// architecture's proposal; columns are limited to what the report and the X-08, X-09, X-11 and
// X-12 descriptions need. Contract fields that are not frozen (passport scope and limits, action
// arguments, status values) are stored as jsonb or text without a value list until they freeze.
// Approvals, reservations and usage (SH-27) and control assessments and timing (SH-44) come later.
//
// Every row carries `organization_id` (uuid, the id type of `app.organizations`). There is no
// foreign key into `app`: that schema belongs to NestJS and is not on main yet. Inside `runtime`,
// a child references its parent by (id, organization_id), so a row can never point at a parent of
// another organization.
export class CreateRuntimeSchema1791041200000 implements MigrationInterface {
  name = "CreateRuntimeSchema1791041200000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // First migration of the `runtime` schema; a later runtime migration must not create it again.
    await queryRunner.query(`CREATE SCHEMA IF NOT EXISTS "runtime"`);

    // The immutable grant for one run ("Trusted authority and passport invariants"). Scope and
    // limits follow the passport contract (X-08) once it freezes.
    await queryRunner.query(
      `CREATE TABLE "runtime"."passports" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "actor_id" uuid NOT NULL,
        "task_version" text NOT NULL,
        "admission_catalog_revision_id" bigint NOT NULL,
        "scope" jsonb NOT NULL,
        "limits" jsonb NOT NULL,
        "issued_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "expires_at" TIMESTAMP WITH TIME ZONE NOT NULL,
        CONSTRAINT "passports_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "passports_id_organization" UNIQUE ("id", "organization_id"),
        CONSTRAINT "passports_expiry_after_issue" CHECK ("expires_at" > "issued_at")
      )`,
    );
    // "Once admitted, the passport remains fixed": no UPDATE, from any service.
    await queryRunner.query(
      `CREATE FUNCTION "runtime"."reject_passport_update"() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN
        RAISE EXCEPTION 'runtime.passports rows are immutable';
      END;
      $$`,
    );
    await queryRunner.query(
      `CREATE TRIGGER "passports_immutable" BEFORE UPDATE ON "runtime"."passports"
       FOR EACH ROW EXECUTE FUNCTION "runtime"."reject_passport_update"()`,
    );

    // One run per passport (Figure 4: "The passport, run, and job are stored together").
    // Status values and the terminal reason follow the run state contract (X-11).
    await queryRunner.query(
      `CREATE TABLE "runtime"."runs" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "passport_id" uuid NOT NULL,
        "status" text NOT NULL,
        "terminal_reason" text,
        "cancel_requested_at" TIMESTAMP WITH TIME ZONE,
        "result_reference" text,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "updated_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "runs_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "runs_id_organization" UNIQUE ("id", "organization_id"),
        CONSTRAINT "runs_one_per_passport" UNIQUE ("passport_id"),
        CONSTRAINT "runs_passport_fkey" FOREIGN KEY ("passport_id", "organization_id")
          REFERENCES "runtime"."passports" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );

    // A proposed action, stored before any policy check (Figure 5). `id` is the stable action
    // identifier; `idempotency_key` is kept by every safe retry of the same action (GO-02).
    // One action per model step: (run_id, step_number) is unique.
    await queryRunner.query(
      `CREATE TABLE "runtime"."actions" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "step_number" integer NOT NULL,
        "tool" text NOT NULL,
        "canonical_arguments" jsonb NOT NULL,
        "canonicalization_version" integer NOT NULL,
        "action_digest" bytea NOT NULL,
        "idempotency_key" text NOT NULL,
        "evaluated_catalog_revision_id" bigint NOT NULL,
        "status" text NOT NULL,
        "expires_at" TIMESTAMP WITH TIME ZONE,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "updated_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "actions_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "actions_id_organization" UNIQUE ("id", "organization_id"),
        CONSTRAINT "actions_idempotency_key" UNIQUE ("idempotency_key"),
        CONSTRAINT "actions_one_per_step" UNIQUE ("run_id", "step_number"),
        CONSTRAINT "actions_step_number_positive" CHECK ("step_number" > 0),
        CONSTRAINT "actions_registered_tool"
          CHECK ("tool" IN ('read_invoice', 'read_vendor', 'create_report', 'queue_report')),
        CONSTRAINT "actions_digest_sha256" CHECK (octet_length("action_digest") = 32),
        CONSTRAINT "actions_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );

    // Durable jobs claimed with a lease ("A worker claims a durable job with a lease"); the lease
    // is released during an approval wait. `action_id` is the stored action a continuation resumes.
    await queryRunner.query(
      `CREATE TABLE "runtime"."jobs" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "kind" text NOT NULL,
        "action_id" uuid,
        "status" text NOT NULL,
        "available_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "lease_owner" text,
        "lease_expires_at" TIMESTAMP WITH TIME ZONE,
        "attempt_count" integer NOT NULL DEFAULT 0,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "updated_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "jobs_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "jobs_lease_pair"
          CHECK (("lease_owner" IS NULL) = ("lease_expires_at" IS NULL)),
        CONSTRAINT "jobs_attempt_count_nonnegative" CHECK ("attempt_count" >= 0),
        CONSTRAINT "jobs_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "jobs_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    await queryRunner.query(
      `CREATE INDEX "jobs_claim_order" ON "runtime"."jobs" ("status", "available_at")`,
    );

    // GO-02: a durable record committed before each model dispatch. It proves intent to dispatch,
    // not delivery. Missing token counts mean unknown usage, never zero. The reservation link is
    // added with the reservations (SH-27).
    await queryRunner.query(
      `CREATE TABLE "runtime"."model_calls" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "purpose" text NOT NULL,
        "model" text NOT NULL,
        "outcome" text,
        "input_tokens" integer,
        "output_tokens" integer,
        "dispatch_recorded_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "completed_at" TIMESTAMP WITH TIME ZONE,
        CONSTRAINT "model_calls_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "model_calls_purpose" CHECK ("purpose" IN ('agent', 'security')),
        CONSTRAINT "model_calls_tokens_nonnegative"
          CHECK (("input_tokens" IS NULL OR "input_tokens" >= 0)
             AND ("output_tokens" IS NULL OR "output_tokens" >= 0)),
        CONSTRAINT "model_calls_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );

    // GO-02: a durable record committed before each tool dispatch of an action. Safe retries add
    // another attempt for the same action (and reuse the action's idempotency key).
    await queryRunner.query(
      `CREATE TABLE "runtime"."execution_attempts" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "action_id" uuid NOT NULL,
        "attempt_number" integer NOT NULL,
        "outcome" text,
        "dispatch_recorded_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "completed_at" TIMESTAMP WITH TIME ZONE,
        CONSTRAINT "execution_attempts_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "execution_attempts_number_per_action" UNIQUE ("action_id", "attempt_number"),
        CONSTRAINT "execution_attempts_number_positive" CHECK ("attempt_number" > 0),
        CONSTRAINT "execution_attempts_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );

    // Sanitized decision events (X-12): masked metadata only, never raw notes, secrets or model
    // requests. `id` is the cursor. Identity values are assigned at insert, not commit, so a
    // reader paging by id must not skip in-flight rows; how the cursor handles that is Go's choice.
    await queryRunner.query(
      `CREATE TABLE "runtime"."audit_events" (
        "id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL,
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "action_id" uuid,
        "event_type" text NOT NULL,
        "decision" text,
        "reason_code" text,
        "catalog_revision_id" bigint,
        "masked_summary" jsonb NOT NULL DEFAULT '{}',
        "occurred_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "audit_events_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "audit_events_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "audit_events_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    await queryRunner.query(
      `CREATE INDEX "audit_events_run_cursor" ON "runtime"."audit_events" ("run_id", "id")`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE "runtime"."audit_events"`);
    await queryRunner.query(`DROP TABLE "runtime"."execution_attempts"`);
    await queryRunner.query(`DROP TABLE "runtime"."model_calls"`);
    await queryRunner.query(`DROP TABLE "runtime"."jobs"`);
    await queryRunner.query(`DROP TABLE "runtime"."actions"`);
    await queryRunner.query(`DROP TABLE "runtime"."runs"`);
    // Dropping the table drops its trigger; the function goes after it.
    await queryRunner.query(`DROP TABLE "runtime"."passports"`);
    await queryRunner.query(`DROP FUNCTION "runtime"."reject_passport_update"()`);
    // Without CASCADE: this fails rather than dropping objects a later migration added to `runtime`.
    await queryRunner.query(`DROP SCHEMA "runtime"`);
  }
}
