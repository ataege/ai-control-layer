import type { MigrationInterface, QueryRunner } from "typeorm";

// DRAFT (SH-44), pending the Go implementer's approval; builds on the SH-16 draft
// (1791041200000-CreateRuntimeSchema), which is not approved yet either. Model-purpose token usage
// is not here: it lives in the Go implementer's GO-06 ledger (runtime.model_token_reservations,
// with purpose and input, output and actual tokens per call), the one token authority. Go owns these tables:
// no NestJS entities, hand-written SQL, no data. Every row carries `organization_id` and points
// at its run by (run_id, organization_id).
//
// Value lists that report 1.2 settles are checked here (the two control classes, the two metered
// purposes, the live/fixture verdict source). Vocabularies that are still drafts (control ids and
// boundaries from the draft policy.yaml, outcomes, reason codes) are text until they freeze.
export class AddControlAssessmentsAndTiming1791041320000 implements MigrationInterface {
  name = "AddControlAssessmentsAndTiming1791041320000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // Targets for organization-safe references to a model call, and to a call of a given purpose.
    await queryRunner.query(
      `ALTER TABLE "runtime"."model_calls"
       ADD CONSTRAINT "model_calls_id_organization" UNIQUE ("id", "organization_id")`,
    );
    await queryRunner.query(
      `ALTER TABLE "runtime"."model_calls"
       ADD CONSTRAINT "model_calls_id_organization_purpose" UNIQUE ("id", "organization_id", "purpose")`,
    );

    // One row per control that ran in one evaluation ("Decision events connect organization,
    // run/action, metered purpose, active/admission catalog revisions, matched rule and feed
    // revision, outcome"). `evaluation_id` groups the controls of one evaluated interaction.
    // A semantic row must name its verdict source: "model-response fixtures and live semantic
    // checks are distinguished"; the verdict fields stay jsonb until
    // `classifier prompt and verdict schema` is decided.
    await queryRunner.query(
      `CREATE TABLE "runtime"."control_assessments" (
        "id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL,
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "evaluation_id" uuid NOT NULL,
        "action_id" uuid,
        "security_model_call_id" uuid,
        "security_purpose" text GENERATED ALWAYS AS ('security') STORED,
        "boundary" text NOT NULL,
        "control_class" text NOT NULL,
        "control_id" text NOT NULL,
        "outcome" text NOT NULL,
        "reason_code" text,
        "admission_catalog_revision_id" bigint NOT NULL,
        "evaluated_catalog_revision_id" bigint NOT NULL,
        "matched_rule_id" text,
        "feed_revision" text,
        "verdict_source" text,
        "verdict" jsonb,
        "assessed_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "control_assessments_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "control_assessments_class" CHECK ("control_class" IN ('deterministic', 'semantic')),
        CONSTRAINT "control_assessments_verdict_source"
          CHECK ("verdict_source" IS NULL OR "verdict_source" IN ('live', 'fixture')),
        CONSTRAINT "control_assessments_semantic_has_source"
          CHECK (("control_class" = 'semantic') = ("verdict_source" IS NOT NULL)),
        CONSTRAINT "control_assessments_verdict_only_semantic"
          CHECK ("verdict" IS NULL OR "control_class" = 'semantic'),
        CONSTRAINT "control_assessments_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "control_assessments_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "control_assessments_model_call_fkey"
          FOREIGN KEY ("security_model_call_id", "organization_id", "security_purpose")
          REFERENCES "runtime"."model_calls" ("id", "organization_id", "purpose") ON DELETE RESTRICT
      )`,
    );
    // The call behind a semantic verdict must be a `security` purpose call (the foreign key above,
    // through the constant `security_purpose`), and only a semantic row names one.
    await queryRunner.query(
      `ALTER TABLE "runtime"."control_assessments" ADD CONSTRAINT "control_assessments_security_call_semantic"
       CHECK ("security_model_call_id" IS NULL OR "control_class" = 'semantic')`,
    );
    await queryRunner.query(
      `CREATE INDEX "control_assessments_run_evaluation" ON "runtime"."control_assessments" ("run_id", "evaluation_id")`,
    );

    // One measured span per row, so each span attaches to what it measured ("Measure monotonic
    // durations for policy lookup, deterministic controls, semantic evaluator dispatch/response
    // and total gateway handling"; plus provider, approval wait and commit). Microseconds,
    // observed durations only, never estimates. `failed` counts errored spans.
    await queryRunner.query(
      `CREATE TABLE "runtime"."timing_records" (
        "id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL,
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "evaluation_id" uuid,
        "action_id" uuid,
        "model_call_id" uuid,
        "phase" text NOT NULL,
        "duration_microseconds" bigint NOT NULL,
        "failed" boolean NOT NULL DEFAULT false,
        "recorded_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "timing_records_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "timing_records_phase" CHECK ("phase" IN
          ('policy_lookup', 'deterministic', 'semantic', 'provider', 'approval_wait', 'commit', 'total')),
        CONSTRAINT "timing_records_duration_nonnegative" CHECK ("duration_microseconds" >= 0),
        CONSTRAINT "timing_records_provider_has_call"
          CHECK ("phase" <> 'provider' OR "model_call_id" IS NOT NULL),
        CONSTRAINT "timing_records_approval_wait_has_action"
          CHECK ("phase" <> 'approval_wait' OR "action_id" IS NOT NULL),
        CONSTRAINT "timing_records_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "timing_records_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "timing_records_model_call_fkey" FOREIGN KEY ("model_call_id", "organization_id")
          REFERENCES "runtime"."model_calls" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    await queryRunner.query(
      `CREATE INDEX "timing_records_phase_time" ON "runtime"."timing_records" ("phase", "recorded_at")`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE "runtime"."timing_records"`);
    await queryRunner.query(`DROP TABLE "runtime"."control_assessments"`);
    await queryRunner.query(
      `ALTER TABLE "runtime"."model_calls" DROP CONSTRAINT "model_calls_id_organization_purpose"`,
    );
    await queryRunner.query(
      `ALTER TABLE "runtime"."model_calls" DROP CONSTRAINT "model_calls_id_organization"`,
    );
  }
}
