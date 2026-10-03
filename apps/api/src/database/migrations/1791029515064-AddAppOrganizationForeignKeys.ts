import type { MigrationInterface, QueryRunner } from "typeorm";

// Integration fix: InitApp1791021755877 sorts before IAMEntities1791029515063, which creates
// app.organizations, so InitApp's organization foreign keys could not be created there on a fresh
// database. They are added here, right after IAMEntities, with their original names and rules.
export class AddAppOrganizationForeignKeys1791029515064 implements MigrationInterface {
  name = "AddAppOrganizationForeignKeys1791029515064";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "app"."policy_versions" ADD CONSTRAINT "FK_policy_org" FOREIGN KEY ("organization_id") REFERENCES "app"."organizations"("id") ON DELETE CASCADE`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."task_templates" ADD CONSTRAINT "FK_task_org" FOREIGN KEY ("organization_id") REFERENCES "app"."organizations"("id") ON DELETE CASCADE`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."tool_definitions" ADD CONSTRAINT "FK_tool_org" FOREIGN KEY ("organization_id") REFERENCES "app"."organizations"("id") ON DELETE CASCADE`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TABLE "app"."tool_definitions" DROP CONSTRAINT "FK_tool_org"`);
    await queryRunner.query(`ALTER TABLE "app"."task_templates" DROP CONSTRAINT "FK_task_org"`);
    await queryRunner.query(`ALTER TABLE "app"."policy_versions" DROP CONSTRAINT "FK_policy_org"`);
  }
}
