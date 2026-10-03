import type { MigrationInterface, QueryRunner } from "typeorm";

// W2 Go lane (tools and provenance), decisions by the lead's delegate on 3 October 2026:
// - `source classification storage`: a classification column on the demo invoice note field;
// - the vendor's trusted recipient: a registered reporting address on demo.vendors, returned to the
//   model only as an opaque reference;
// - `report storage`: report lineage lives in runtime.report_lineage.
// Go owns these tables: no NestJS entities, hand-written SQL, no data.
export class AddProvenanceColumnsAndLineage1791060000000 implements MigrationInterface {
  name = "AddProvenanceColumnsAndLineage1791060000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // The note's trusted classification, set exactly when the note is set. Same values as
    // demo.reports.classification.
    await queryRunner.query(
      `ALTER TABLE "demo"."invoices"
         ADD COLUMN "internal_note_classification" text,
         ADD CONSTRAINT "invoices_note_classification"
           CHECK ("internal_note_classification" IS NULL
                  OR "internal_note_classification" IN ('internal_only', 'vendor_shareable')),
         ADD CONSTRAINT "invoices_note_classification_with_note"
           CHECK (("internal_note" IS NULL) = ("internal_note_classification" IS NULL))`,
    );

    // The trusted destination of a vendor report; never taken from invoice prose or model output.
    await queryRunner.query(
      `ALTER TABLE "demo"."vendors"
         ADD COLUMN "registered_reporting_address" text,
         ADD CONSTRAINT "vendors_reporting_address_format"
           CHECK ("registered_reporting_address" IS NULL
                  OR "registered_reporting_address" ~ '^[^@[:space:]]+@[^@[:space:]]+$')`,
    );

    // Lineage references its report within the organization.
    await queryRunner.query(
      `ALTER TABLE "demo"."reports" ADD CONSTRAINT "reports_id_organization" UNIQUE ("id", "organization_id")`,
    );

    // One row per trusted source a report consumed, written by Go in the same transaction as the
    // report ("so an artifact cannot exist without its restrictions"). The template and projection
    // versions are the Go-registered constants of internal/provenance.
    await queryRunner.query(
      `CREATE TABLE "runtime"."report_lineage" (
        "id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL,
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "report_id" uuid NOT NULL,
        "source_kind" text NOT NULL,
        "source_id" text NOT NULL,
        "source_version" integer NOT NULL,
        "source_classification" text NOT NULL,
        "consumed_fields" text[] NOT NULL,
        "template" text NOT NULL,
        "template_version" integer NOT NULL,
        "projection_rule" text,
        "projection_rule_version" integer,
        "recorded_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "report_lineage_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "report_lineage_one_per_source" UNIQUE ("report_id", "source_kind", "source_id"),
        CONSTRAINT "report_lineage_source_kind" CHECK ("source_kind" IN ('invoice', 'vendor')),
        CONSTRAINT "report_lineage_source_version_positive" CHECK ("source_version" > 0),
        CONSTRAINT "report_lineage_source_classification"
          CHECK ("source_classification" IN ('internal_only', 'vendor_shareable')),
        CONSTRAINT "report_lineage_consumed_fields_present" CHECK (cardinality("consumed_fields") > 0),
        CONSTRAINT "report_lineage_template"
          CHECK ("template" IN ('internal_investigation_v1', 'vendor_reconciliation_v1')),
        CONSTRAINT "report_lineage_template_version_positive" CHECK ("template_version" > 0),
        CONSTRAINT "report_lineage_projection_pair"
          CHECK (("projection_rule" IS NULL) = ("projection_rule_version" IS NULL)),
        CONSTRAINT "report_lineage_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "report_lineage_report_fkey" FOREIGN KEY ("report_id", "organization_id")
          REFERENCES "demo"."reports" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    await queryRunner.query(
      `CREATE INDEX "report_lineage_report" ON "runtime"."report_lineage" ("report_id")`,
    );
    // Lineage is as immutable as its report: no UPDATE and no row DELETE. TRUNCATE stays possible
    // for pnpm reset:demo, which runs as the owner.
    await queryRunner.query(
      `CREATE FUNCTION "runtime"."reject_lineage_change"() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN
        RAISE EXCEPTION 'runtime.report_lineage rows are immutable';
      END;
      $$`,
    );
    await queryRunner.query(
      `CREATE TRIGGER "report_lineage_immutable" BEFORE UPDATE OR DELETE ON "runtime"."report_lineage"
       FOR EACH ROW EXECUTE FUNCTION "runtime"."reject_lineage_change"()`,
    );

    // The gateway role writes lineage with its report (CreateServiceRoles1791050000000: every table
    // is granted explicitly). Skipped where that role does not exist.
    await queryRunner.query(
      `DO $$ BEGIN
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'task_passport_gateway') THEN
          GRANT SELECT, INSERT ON "runtime"."report_lineage" TO "task_passport_gateway";
        END IF;
      END $$`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    // Dropping the table drops its trigger and grants; the function goes after it.
    await queryRunner.query(`DROP TABLE "runtime"."report_lineage"`);
    await queryRunner.query(`DROP FUNCTION "runtime"."reject_lineage_change"()`);
    await queryRunner.query(
      `ALTER TABLE "demo"."reports" DROP CONSTRAINT "reports_id_organization"`,
    );
    await queryRunner.query(
      `ALTER TABLE "demo"."vendors"
         DROP CONSTRAINT "vendors_reporting_address_format",
         DROP COLUMN "registered_reporting_address"`,
    );
    await queryRunner.query(
      `ALTER TABLE "demo"."invoices"
         DROP CONSTRAINT "invoices_note_classification_with_note",
         DROP CONSTRAINT "invoices_note_classification",
         DROP COLUMN "internal_note_classification"`,
    );
  }
}
