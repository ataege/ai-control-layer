import type { MigrationInterface, QueryRunner } from "typeorm";

// W3 Go lane (action gate and approvals), GO-44, approved by the lead (timestamp assigned). The
// gateway checks a reviewer's authority from trusted records, never from a signed claim alone:
// it reads app.memberships (the role "reviewer" for the verified user and organization). This is
// the Go read grant on the records behind operator and reviewer authority that SH-26 left pending
// on decisions 4 and 7. Read only: NestJS keeps the write.
export class GrantGatewayMembershipRead1791110000000 implements MigrationInterface {
  name = "GrantGatewayMembershipRead1791110000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `DO $$ BEGIN
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'task_passport_gateway') THEN
          GRANT USAGE ON SCHEMA "app" TO "task_passport_gateway";
          GRANT SELECT ON "app"."memberships" TO "task_passport_gateway";
        END IF;
      END $$`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    // USAGE on app stays: the gateway already had it for the control catalog (CreateServiceRoles).
    await queryRunner.query(
      `DO $$ BEGIN
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'task_passport_gateway') THEN
          REVOKE SELECT ON "app"."memberships" FROM "task_passport_gateway";
        END IF;
      END $$`,
    );
  }
}
