import type { MigrationInterface, QueryRunner } from "typeorm";

// IAMEntities1791029515063 created app.memberships without the uniqueness the Membership entity
// declares, so one user could hold two memberships in the same organization. The constraint keeps
// the name TypeORM derives for the entity, so the entity diff stays empty. It fails loudly on a
// database that already holds duplicate memberships.
//
// This enforces uniqueness of the (user, organization) pair, not one organization per user: the
// same user may hold a membership in a second organization. Which membership a session uses when a
// user has several is a separate rule (the oldest membership today, a known limit).
export class AddMembershipUniqueness1791160000000 implements MigrationInterface {
  name = "AddMembershipUniqueness1791160000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "app"."memberships" ADD CONSTRAINT "UQ_64893eb3c6fcaeaaee71a4d0ae1" UNIQUE ("userId", "organizationId")`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "app"."memberships" DROP CONSTRAINT "UQ_64893eb3c6fcaeaaee71a4d0ae1"`,
    );
  }
}
