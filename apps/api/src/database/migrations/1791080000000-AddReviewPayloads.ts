import type { MigrationInterface, QueryRunner } from "typeorm";

// W3 Go lane (action gate and approvals), GO-43. Go owns this table: no NestJS entity,
// hand-written SQL, no data.
//
// The exact material a reviewer approves ("the tool, canonical arguments, recipient, affected
// resources, relevant versions, exact outbound content, passport reference, policy version and
// expiry"), frozen before review. It is restricted review storage: the content and the exact
// recipient never go into runtime.audit_events. runtime.approvals.review_payload_reference holds
// this row's id; payload_digest is SHA-256 over the full canonical material, so a later change to
// any material field is detectable. The action's own digest stays untouched.
export class AddReviewPayloads1791080000000 implements MigrationInterface {
  name = "AddReviewPayloads1791080000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `CREATE TABLE "runtime"."review_payloads" (
        "id" uuid NOT NULL DEFAULT gen_random_uuid(),
        "organization_id" uuid NOT NULL,
        "run_id" uuid NOT NULL,
        "action_id" uuid NOT NULL,
        "payload" jsonb NOT NULL,
        "payload_digest" bytea NOT NULL,
        "canonicalization_version" integer NOT NULL,
        "expires_at" TIMESTAMP WITH TIME ZONE NOT NULL,
        "created_at" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
        CONSTRAINT "review_payloads_pkey" PRIMARY KEY ("id"),
        CONSTRAINT "review_payloads_one_per_action" UNIQUE ("action_id"),
        CONSTRAINT "review_payloads_digest_sha256" CHECK (octet_length("payload_digest") = 32),
        CONSTRAINT "review_payloads_canonicalization_version_positive" CHECK ("canonicalization_version" > 0),
        CONSTRAINT "review_payloads_expiry_after_creation" CHECK ("expires_at" > "created_at"),
        CONSTRAINT "review_payloads_run_fkey" FOREIGN KEY ("run_id", "organization_id")
          REFERENCES "runtime"."runs" ("id", "organization_id") ON DELETE RESTRICT,
        CONSTRAINT "review_payloads_action_fkey" FOREIGN KEY ("action_id", "organization_id")
          REFERENCES "runtime"."actions" ("id", "organization_id") ON DELETE RESTRICT
      )`,
    );
    // Frozen means frozen: no UPDATE and no row DELETE. TRUNCATE stays possible for
    // pnpm reset:demo, which runs as the owner; the service roles get neither DELETE nor TRUNCATE.
    await queryRunner.query(
      `CREATE FUNCTION "runtime"."reject_review_payload_change"() RETURNS trigger LANGUAGE plpgsql AS $$
      BEGIN
        RAISE EXCEPTION 'runtime.review_payloads rows are immutable';
      END;
      $$`,
    );
    await queryRunner.query(
      `CREATE TRIGGER "review_payloads_immutable" BEFORE UPDATE OR DELETE ON "runtime"."review_payloads"
       FOR EACH ROW EXECUTE FUNCTION "runtime"."reject_review_payload_change"()`,
    );
    // The gateway freezes and rechecks payloads. No read grant for the API here: how the review
    // screen reads the payload is the open item `read path`.
    await queryRunner.query(
      `DO $$ BEGIN
        IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'task_passport_gateway') THEN
          GRANT SELECT, INSERT ON "runtime"."review_payloads" TO "task_passport_gateway";
        END IF;
      END $$`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    // Dropping the table drops its trigger and the grants on it; the function goes after it.
    await queryRunner.query(`DROP TABLE "runtime"."review_payloads"`);
    await queryRunner.query(`DROP FUNCTION "runtime"."reject_review_payload_change"()`);
  }
}
