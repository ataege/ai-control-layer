import type { MigrationInterface, QueryRunner } from "typeorm";

// GO-11 with GO-29 (Go lane f3), approved by the lead on 2026-10-03: runtime.context_entries also
// stores the denial feedback the model receives after a denied proposal, kind 'correction'. Its
// content is only policy.BuildDenialFeedback's reason code, fixed safe message and permitted
// alternative, so a restarted worker rebuilds the same next request.
export class AllowContextCorrections1791100000000 implements MigrationInterface {
  name = "AllowContextCorrections1791100000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "runtime"."context_entries" DROP CONSTRAINT "context_entries_kind"`,
    );
    await queryRunner.query(
      `ALTER TABLE "runtime"."context_entries" ADD CONSTRAINT "context_entries_kind"
        CHECK ("kind" IN ('assistant_call', 'tool_result', 'correction'))`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    // Narrowing again fails while correction rows exist, rather than deleting stored context.
    await queryRunner.query(
      `ALTER TABLE "runtime"."context_entries" DROP CONSTRAINT "context_entries_kind"`,
    );
    await queryRunner.query(
      `ALTER TABLE "runtime"."context_entries" ADD CONSTRAINT "context_entries_kind"
        CHECK ("kind" IN ('assistant_call', 'tool_result'))`,
    );
  }
}
