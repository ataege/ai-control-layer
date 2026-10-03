import type { MigrationInterface, QueryRunner } from "typeorm";

// DRAFT (SH-24, table part), pending the Go implementer's approval; builds on the SH-16 and
// SH-17 drafts. Go's tool adapters own these tables: no NestJS entities, hand-written SQL, no
// data. Left out on purpose: the lineage tables (`report storage` is open) and the report view
// (`stored report read` is open).
//
// Classifications and templates are the report's fixed prototype set: Internal only and Vendor
// shareable; internal_investigation_v1 and vendor_reconciliation_v1.
export class AddReportsAndOutbox1791041440000 implements MigrationInterface {
  name = "AddReportsAndOutbox1791041440000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // One immutable artifact per create_report action ("A uniqueness constraint tied to action ID
    // prevents a repeated successful attempt from creating a fresh artifact"). The content hash is
    // SHA-256 of the stored UTF-8 content, checked by the database. A title is display only.
    await queryRunner.query(
      `CREATE TABLE "demo"."reports" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "created_by_action_id" uuid NOT NULL,
        "version" integer NOT NULL DEFAULT 1,
        "template" text NOT NULL,
        "projection_rule" text,
        "projection_policy_version" text,
        "classification" text NOT NULL,
        "destination_class" text NOT NULL,
        "title" text NOT NULL,
        "content" text NOT NULL,
        "content_hash" bytea NOT NULL,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "reports_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "reports_one_per_action" UNIQUE ("created_by_action_id"),
        CONSTRAINT "reports_id_organization_hash_classification"
          UNIQUE ("id", "organization_id", "content_hash", "classification"),
        CONSTRAINT "reports_version_positive" CHECK ("version" > 0),
        CONSTRAINT "reports_template"
          CHECK ("template" IN ('internal_investigation_v1', 'vendor_reconciliation_v1')),
        CONSTRAINT "reports_classification"
          CHECK ("classification" IN ('internal_only', 'vendor_shareable')),
        -- "The internal template is always Internal only and retains source restrictions."
        CONSTRAINT "reports_internal_template_internal_only"
          CHECK ("template" <> 'internal_investigation_v1' OR "classification" = 'internal_only'),
        -- The vendor template renders a named projection; the internal one has none.
        CONSTRAINT "reports_projection_only_vendor" CHECK (
          ("template" = 'vendor_reconciliation_v1')
          = ("projection_rule" IS NOT NULL AND "projection_policy_version" IS NOT NULL)),
        CONSTRAINT "reports_content_hash_sha256"
          CHECK ("content_hash" = sha256(convert_to("content", 'UTF8'))),
        CONSTRAINT "reports_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "reports_action_fkey" FOREIGN KEY ("created_by_action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );

    // The simulated outbox: a database record, never a sent message. One row per queue_report
    // action ("a repeated approved action cannot create a second row"), bound to the exact
    // report content hash that was reviewed. As a last line behind Go's export check, the
    // database accepts only Vendor shareable reports here: the constant column below must match
    // the report's classification.
    await queryRunner.query(
      `CREATE TABLE "demo"."outbox_messages" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "action_id" uuid NOT NULL,
        "report_id" uuid NOT NULL,
        "report_content_hash" bytea NOT NULL,
        "required_classification" text GENERATED ALWAYS AS ('vendor_shareable') STORED,
        "recipient" text NOT NULL,
        "queued_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "outbox_messages_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "outbox_messages_one_per_action" UNIQUE ("action_id"),
        CONSTRAINT "outbox_messages_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "outbox_messages_report_fkey"
          FOREIGN KEY ("report_id", "organization_id", "report_content_hash", "required_classification")
          REFERENCES "demo"."reports" ("id", "organization_id", "content_hash", "classification")
          ON DELETE RESTRICT
      )`,
    );
    await queryRunner.query(
      `COMMENT ON TABLE "demo"."outbox_messages" IS 'Simulated outbox: synthetic demonstration records; no message is ever sent.'`,
    );

    // Reports and queued messages never change after insert ("The report and this metadata
    // become one immutable artifact").
    await queryRunner.query(
      `CREATE FUNCTION "demo"."reject_artifact_update"() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN
        RAISE EXCEPTION '%.% rows are immutable', TG_TABLE_SCHEMA, TG_TABLE_NAME;
      END;
      $$`,
    );
    await queryRunner.query(
      `CREATE TRIGGER "reports_immutable" BEFORE UPDATE ON "demo"."reports"
       FOR EACH ROW EXECUTE FUNCTION "demo"."reject_artifact_update"()`,
    );
    await queryRunner.query(
      `CREATE TRIGGER "outbox_messages_immutable" BEFORE UPDATE ON "demo"."outbox_messages"
       FOR EACH ROW EXECUTE FUNCTION "demo"."reject_artifact_update"()`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    // Dropping the tables drops their triggers; the function goes after them.
    await queryRunner.query(`DROP TABLE "demo"."outbox_messages"`);
    await queryRunner.query(`DROP TABLE "demo"."reports"`);
    await queryRunner.query(`DROP FUNCTION "demo"."reject_artifact_update"()`);
  }
}
