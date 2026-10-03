import type { MigrationInterface, QueryRunner } from "typeorm";

// c1 Go lane (hybrid security), approved by the lead (timestamp assigned). The semantic action
// check makes no model call when every argument of a proposal is a constrained value (identifier,
// template, uuid, run-scoped reference): it records a semantic-class control assessment with outcome
// `not_applicable` and no verdict source, because there is no verdict, live or fixture, to label.
// The original check demanded a verdict source on every semantic row. The relaxed check demands it
// unless the outcome is `not_applicable`; a deterministic row still carries none.
//
// Reverting restores the original check, which fails while such rows exist: remove or reclassify
// them first (this table keeps evidence, so the migration never rewrites it).
export class AllowUnclassifiedSemanticNotApplicable1791150000000 implements MigrationInterface {
  name = "AllowUnclassifiedSemanticNotApplicable1791150000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "runtime"."control_assessments"
         DROP CONSTRAINT "control_assessments_semantic_has_source",
         ADD CONSTRAINT "control_assessments_semantic_has_source"
           CHECK (("control_class" = 'semantic' AND ("verdict_source" IS NOT NULL OR "outcome" = 'not_applicable'))
                  OR ("control_class" <> 'semantic' AND "verdict_source" IS NULL))`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "runtime"."control_assessments"
         DROP CONSTRAINT "control_assessments_semantic_has_source",
         ADD CONSTRAINT "control_assessments_semantic_has_source"
           CHECK (("control_class" = 'semantic') = ("verdict_source" IS NOT NULL))`,
    );
  }
}
