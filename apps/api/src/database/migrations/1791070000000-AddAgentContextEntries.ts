import type { MigrationInterface, QueryRunner } from "typeorm";

// GO-11 (Go lane f3), approved by the lead on 2026-10-03: the agent's model-visible working
// context, one append-only row per entry. A restarted worker rebuilds the next model request from
// these rows and never re-executes a completed action (GO-02: "Resume from its persisted result and
// continuation"; GO-07: a completed action returns its persisted minimized result).
//
// `content` holds only what the model actually saw: the tool call with the gate's canonical
// arguments, or the inspected tool result (the permitted JSON, or a withheld marker for blocked
// content). Never the raw adapter result. Go owns the table, so there is no NestJS entity.
export class AddAgentContextEntries1791070000000 implements MigrationInterface {
  name = "AddAgentContextEntries1791070000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `CREATE TABLE "runtime"."context_entries" (
        "id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL,
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "step_number" integer NOT NULL,
        "kind" text NOT NULL,
        "action_id" uuid,
        "content" jsonb NOT NULL,
        "inspection_outcome" text,
        "reason_code" text,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "context_entries_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "context_entries_one_kind_per_step" UNIQUE ("run_id", "step_number", "kind"),
        CONSTRAINT "context_entries_step_number_positive" CHECK ("step_number" > 0),
        CONSTRAINT "context_entries_kind" CHECK ("kind" IN ('assistant_call', 'tool_result')),
        CONSTRAINT "context_entries_inspection_outcome"
          CHECK ("inspection_outcome" IS NULL OR "inspection_outcome" IN ('pass', 'redacted', 'blocked')),
        CONSTRAINT "context_entries_tool_result_inspected"
          CHECK (("kind" = 'tool_result') = ("inspection_outcome" IS NOT NULL)),
        CONSTRAINT "context_entries_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "context_entries_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    // Append-only for the gateway: no UPDATE or DELETE. The role exists from CreateServiceRoles.
    await queryRunner.query(
      `GRANT SELECT, INSERT ON "runtime"."context_entries" TO "task_passport_gateway"`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `REVOKE ALL ON "runtime"."context_entries" FROM "task_passport_gateway"`,
    );
    await queryRunner.query(`DROP TABLE "runtime"."context_entries"`);
  }
}
