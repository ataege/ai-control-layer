import { MigrationInterface, QueryRunner } from "typeorm";

export class IAMEntities1791029515063 implements MigrationInterface {
  name = "IAMEntities1791029515063";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `CREATE TABLE "app"."users" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "email" character varying NOT NULL, "name" character varying NOT NULL, "createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), "updatedAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "UQ_97672ac88f789774dd47f7c8be3" UNIQUE ("email"), CONSTRAINT "PK_a3ffb1c0c8416b9fc6f907b7433" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."organizations" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "name" character varying NOT NULL, "createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "UQ_9b7ca6d30b94fef571cff876884" UNIQUE ("name"), CONSTRAINT "PK_6b031fcd0863e3f6b44230163f9" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."memberships" ("id" uuid NOT NULL DEFAULT gen_random_uuid(), "userId" uuid NOT NULL, "organizationId" uuid NOT NULL, "roles" text array NOT NULL DEFAULT '{}', "createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "PK_25d28bd932097a9e90495ede7b4" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."password_hashes" ("userId" uuid NOT NULL, "hash" character varying NOT NULL, "createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), "updatedAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "PK_71aea4e4a394ab2106bab504f7b" PRIMARY KEY ("userId"))`,
    );
    await queryRunner.query(
      `CREATE TABLE "app"."sessions" ("id" text NOT NULL, "userId" uuid NOT NULL, "expiresAt" TIMESTAMP WITH TIME ZONE NOT NULL, "createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(), CONSTRAINT "PK_3238ef96f18b355b671619111bc" PRIMARY KEY ("id"))`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."memberships" ADD CONSTRAINT "FK_187d573e43b2c2aa3960df20b78" FOREIGN KEY ("userId") REFERENCES "app"."users"("id") ON DELETE NO ACTION ON UPDATE NO ACTION`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."memberships" ADD CONSTRAINT "FK_98d23786d647f0ccf477b3b2867" FOREIGN KEY ("organizationId") REFERENCES "app"."organizations"("id") ON DELETE NO ACTION ON UPDATE NO ACTION`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."password_hashes" ADD CONSTRAINT "FK_71aea4e4a394ab2106bab504f7b" FOREIGN KEY ("userId") REFERENCES "app"."users"("id") ON DELETE CASCADE ON UPDATE NO ACTION`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."sessions" ADD CONSTRAINT "FK_57de40bc620f456c7311aa3a1e6" FOREIGN KEY ("userId") REFERENCES "app"."users"("id") ON DELETE CASCADE ON UPDATE NO ACTION`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE "app"."sessions" DROP CONSTRAINT "FK_57de40bc620f456c7311aa3a1e6"`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."password_hashes" DROP CONSTRAINT "FK_71aea4e4a394ab2106bab504f7b"`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."memberships" DROP CONSTRAINT "FK_98d23786d647f0ccf477b3b2867"`,
    );
    await queryRunner.query(
      `ALTER TABLE "app"."memberships" DROP CONSTRAINT "FK_187d573e43b2c2aa3960df20b78"`,
    );
    await queryRunner.query(`DROP TABLE "app"."sessions"`);
    await queryRunner.query(`DROP TABLE "app"."password_hashes"`);
    await queryRunner.query(`DROP TABLE "app"."memberships"`);
    await queryRunner.query(`DROP TABLE "app"."organizations"`);
    await queryRunner.query(`DROP TABLE "app"."users"`);
  }
}
