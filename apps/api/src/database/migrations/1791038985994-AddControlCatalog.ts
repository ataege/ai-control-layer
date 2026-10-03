import type { MigrationInterface, QueryRunner } from "typeorm";

// The control catalog (API-31, SH-43): immutable policy and signature-feed revisions and the single
// active-version pointer. Generated from the entities in src/policies; the schema creation and the
// immutability triggers are added by hand because `migration:generate` emits neither.
export class AddControlCatalog1791038985994 implements MigrationInterface {
  name = "AddControlCatalog1791038985994";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // InitApp1791021755877 creates the `app` schema; IF NOT EXISTS keeps this harmless.
    await queryRunner.query(`CREATE SCHEMA IF NOT EXISTS "app"`);
    await queryRunner.query(
      `CREATE TABLE "app"."control_catalog_revisions" ("id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL, "schema_version" integer NOT NULL, "source_file_name" text NOT NULL, "source_text" text NOT NULL, "file_digest" text NOT NULL, "content" jsonb NOT NULL, "import_source" text NOT NULL, "imported_by" text, "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "control_catalog_revisions_import_actor" CHECK (("import_source" = 'command' AND "imported_by" IS NULL) OR ("import_source" = 'reload' AND "imported_by" IS NOT NULL)), CONSTRAINT "control_catalog_revisions_file_digest_format" CHECK ("file_digest" ~ '^[0-9a-f]{64}$'), CONSTRAINT "PK_1c49045dc6262df94895b3c6682" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."signature_feed_revisions" ("id" bigint GENERATED ALWAYS AS IDENTITY NOT NULL, "issuer" text NOT NULL, "revision" text NOT NULL, "source_file_name" text NOT NULL, "source_text" text NOT NULL, "file_digest" text NOT NULL, "content" jsonb NOT NULL, "import_source" text NOT NULL, "imported_by" text, "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "signature_feed_revisions_issuer_revision" UNIQUE ("issuer", "revision"), CONSTRAINT "signature_feed_revisions_import_actor" CHECK (("import_source" = 'command' AND "imported_by" IS NULL) OR ("import_source" = 'reload' AND "imported_by" IS NOT NULL)), CONSTRAINT "signature_feed_revisions_file_digest_format" CHECK ("file_digest" ~ '^[0-9a-f]{64}$'), CONSTRAINT "PK_a089b9c090ec22876f0f32365e1" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."control_catalog_pointer" ("id" smallint NOT NULL, "requested_revision_id" bigint, "validated_revision_id" bigint, "active_revision_id" bigint, "active_feed_revision_id" bigint, "last_error" jsonb, "last_error_at" TIMESTAMP WITH TIME ZONE, "updated_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "control_catalog_pointer_error_pair" CHECK (("last_error" IS NULL AND "last_error_at" IS NULL) OR ("last_error" IS NOT NULL AND "last_error_at" IS NOT NULL)), CONSTRAINT "control_catalog_pointer_single_row" CHECK ("id" = 1), CONSTRAINT "PK_773c7ede5008b9cbaa072310bc0" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" ADD CONSTRAINT "FK_77b620dd2189053f187ae450d65" FOREIGN KEY ("requested_revision_id") REFERENCES "app"."control_catalog_revisions"("id") ON DELETE RESTRICT ON UPDATE NO ACTION`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" ADD CONSTRAINT "FK_e9bcefb15758dfcb00c93c2b597" FOREIGN KEY ("validated_revision_id") REFERENCES "app"."control_catalog_revisions"("id") ON DELETE RESTRICT ON UPDATE NO ACTION`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" ADD CONSTRAINT "FK_adf6505e5cbb964f36e6ba7b56d" FOREIGN KEY ("active_revision_id") REFERENCES "app"."control_catalog_revisions"("id") ON DELETE RESTRICT ON UPDATE NO ACTION`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" ADD CONSTRAINT "FK_a5f886778fdd4140ba9aa0672d3" FOREIGN KEY ("active_feed_revision_id") REFERENCES "app"."signature_feed_revisions"("id") ON DELETE RESTRICT ON UPDATE NO ACTION`,
    );

    // Revisions are immutable: reject every change to a stored row, including TRUNCATE.
    await queryRunner.query(
      `CREATE FUNCTION "app"."reject_revision_change"() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'revisions in %.% are immutable', TG_TABLE_SCHEMA, TG_TABLE_NAME USING ERRCODE = 'restrict_violation'; END $$`,
    );
    for (const revisionTable of ["control_catalog_revisions", "signature_feed_revisions"]) {
      await queryRunner.query(
        `CREATE TRIGGER "${revisionTable}_immutable_rows" BEFORE UPDATE OR DELETE ON "app"."${revisionTable}" FOR EACH ROW EXECUTE FUNCTION "app"."reject_revision_change"()`,
      );
      await queryRunner.query(
        `CREATE TRIGGER "${revisionTable}_no_truncate" BEFORE TRUNCATE ON "app"."${revisionTable}" FOR EACH STATEMENT EXECUTE FUNCTION "app"."reject_revision_change"()`,
      );
    }
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" DROP CONSTRAINT "FK_a5f886778fdd4140ba9aa0672d3"`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" DROP CONSTRAINT "FK_adf6505e5cbb964f36e6ba7b56d"`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" DROP CONSTRAINT "FK_e9bcefb15758dfcb00c93c2b597"`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."control_catalog_pointer" DROP CONSTRAINT "FK_77b620dd2189053f187ae450d65"`,
    );
    await queryRunner.query(`DROP TABLE "app"."control_catalog_pointer"`);
    // Dropping the tables drops their triggers; the function goes after them.
    await queryRunner.query(`DROP TABLE "app"."signature_feed_revisions"`);
    await queryRunner.query(`DROP TABLE "app"."control_catalog_revisions"`);
    await queryRunner.query(`DROP FUNCTION "app"."reject_revision_change"()`);
    // The `app` schema belongs to InitApp1791021755877, the first app migration; it is dropped there.
  }
}
