import type { MigrationInterface, QueryRunner } from "typeorm";

// GO-73, catalog activation protocol (lead decision, 2026-10-03): NestJS imports a revision and
// sets only requested_revision_id; the gateway validates it with its own parsers and, as its
// acknowledgement, sets validated and active together with the bound feed, or records a safe
// last_error and keeps the last good active revision. The gateway therefore needs UPDATE on
// exactly those pointer columns, and nothing else on app.
const GATEWAY_ROLE = "task_passport_gateway";
const ACTIVATION_COLUMNS = [
  "validated_revision_id",
  "active_revision_id",
  "active_feed_revision_id",
  "last_error",
  "last_error_at",
  "updated_at",
];

export class GrantGatewayCatalogActivation1791120000000 implements MigrationInterface {
  name = "GrantGatewayCatalogActivation1791120000000";

  public async up(queryRunner: QueryRunner): Promise<void> {
    const columns = ACTIVATION_COLUMNS.map((column) => `"${column}"`).join(", ");
    await queryRunner.query(
      `GRANT UPDATE (${columns}) ON "app"."control_catalog_pointer" TO "${GATEWAY_ROLE}"`,
    );
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    const columns = ACTIVATION_COLUMNS.map((column) => `"${column}"`).join(", ");
    await queryRunner.query(
      `REVOKE UPDATE (${columns}) ON "app"."control_catalog_pointer" FROM "${GATEWAY_ROLE}"`,
    );
  }
}
