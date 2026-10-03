import type { MigrationInterface, QueryRunner } from "typeorm";

// DRAFT (SH-17), pending the Go implementer's approval: the first `demo` migration, the synthetic
// vendors and invoices of the Atlas scenario (invoice_A01, invoice_A02, invoice_B01, vendor Atlas).
// Go's tool adapters own these tables, so there are no NestJS entities; the SQL is hand-written.
// No data is inserted here: the records come from the explicit seed command (SH-18).
//
// - Record versions: an integer `version` per record, starting at 1. How a recheck compares
//   versions stays open (`record versions`).
// - The internal investigation note is an authorized field of a synthetic invoice. Its
//   classification column is left out until `source classification storage` is decided.
// - Ids are text, so the seed can use the scenario's readable ids. A record id is a reference,
//   not authorization: every query also needs the verified organization.
// - Every row has `organization_id` (uuid, as `app.organizations`); no foreign key into `app`.
export class CreateDemoSchema1791041260000 implements MigrationInterface {
  name = "CreateDemoSchema1791041260000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // First migration of the `demo` schema; a later demo migration must not create it again.
    await queryRunner.query(`CREATE SCHEMA IF NOT EXISTS "demo"`);

    await queryRunner.query(
      `CREATE TABLE "demo"."vendors" (
        "id" text NOT NULL,
        "organization_id" uuid NOT NULL,
        "version" integer NOT NULL DEFAULT 1,
        "name" text NOT NULL,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "updated_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "vendors_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "vendors_id_organization" UNIQUE ("id", "organization_id"),
        CONSTRAINT "vendors_version_positive" CHECK ("version" > 0)
      )`,
    );

    // Money is an integer amount in minor units plus an ISO 4217 code: no floating point (report,
    // "Exact action approval versioning and execution rechecks"). The external reference is not
    // unique: the scenario's two invoices share INV104 on purpose.
    await queryRunner.query(
      `CREATE TABLE "demo"."invoices" (
        "id" text NOT NULL,
        "organization_id" uuid NOT NULL,
        "vendor_id" text NOT NULL,
        "version" integer NOT NULL DEFAULT 1,
        "external_reference" text NOT NULL,
        "currency" character(3) NOT NULL,
        "total_minor_units" bigint NOT NULL,
        "issued_on" date NOT NULL,
        "due_on" date NOT NULL,
        "internal_note" text,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        "updated_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "invoices_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "invoices_version_positive" CHECK ("version" > 0),
        CONSTRAINT "invoices_currency_code" CHECK ("currency" ~ '^[A-Z]{3}$'),
        CONSTRAINT "invoices_total_nonnegative" CHECK ("total_minor_units" >= 0),
        CONSTRAINT "invoices_due_not_before_issue" CHECK ("due_on" >= "issued_on"),
        CONSTRAINT "invoices_vendor_fkey" FOREIGN KEY ("vendor_id", "organization_id")
          REFERENCES "demo"."vendors" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    await queryRunner.query(
      `CREATE INDEX "invoices_organization_vendor" ON "demo"."invoices" ("organization_id", "vendor_id")`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE "demo"."invoices"`);
    await queryRunner.query(`DROP TABLE "demo"."vendors"`);
    // Without CASCADE: this fails rather than dropping objects a later migration added to `demo`.
    await queryRunner.query(`DROP SCHEMA "demo"`);
  }
}
