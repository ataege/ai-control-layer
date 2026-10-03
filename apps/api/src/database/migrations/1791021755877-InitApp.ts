import type { MigrationInterface, QueryRunner } from "typeorm";

export class InitApp1791021755877 implements MigrationInterface {
  name = "InitApp1791021755877";

  public async up(queryRunner: QueryRunner): Promise<void> {
    // The first `app` migration: it creates the schema and its down() drops it last. The foreign
    // keys to app.organizations are added by AddAppOrganizationForeignKeys1791029515064, after
    // IAMEntities creates that table (integration fix: they referenced it before it existed).
    await queryRunner.query(`CREATE SCHEMA IF NOT EXISTS "app"`);
    await queryRunner.query(
      `CREATE TABLE "app"."policy_versions" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "organization_id" uuid NOT NULL, "name" character varying NOT NULL, "rules" jsonb NOT NULL, "description" character varying, "created_at" TIMESTAMP NOT NULL DEFAULT now(), CONSTRAINT "PK_125d2970fc66a316f84af16812e" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."task_templates" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "organization_id" uuid NOT NULL, "name" character varying NOT NULL, "description" character varying, "created_at" TIMESTAMP NOT NULL DEFAULT now(), "updated_at" TIMESTAMP NOT NULL DEFAULT now(), CONSTRAINT "PK_a1347b5446b9e3158e2b72f58b2" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."tool_definitions" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "organization_id" uuid NOT NULL, "name" character varying NOT NULL, "schema" jsonb NOT NULL, "created_at" TIMESTAMP NOT NULL DEFAULT now(), CONSTRAINT "PK_1402cc35da3054fa5003e3704ca" PRIMARY KEY ("id"))`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP TABLE IF EXISTS "app"."tool_definitions" CASCADE`);
    await queryRunner.query(`DROP TABLE IF EXISTS "app"."task_templates" CASCADE`);
    await queryRunner.query(`DROP TABLE IF EXISTS "app"."policy_versions" CASCADE`);
    // Without CASCADE: this fails rather than dropping objects another migration left in `app`.
    await queryRunner.query(`DROP SCHEMA "app"`);
  }
}
