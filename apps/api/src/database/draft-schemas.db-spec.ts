// Database-backed tests of the DRAFT runtime and demo migrations (SH-16, SH-17, SH-24, SH-27,
// SH-44), run by `pnpm test:db api` only. Each run creates its own temporary database, migrates
// it, checks the constraints the roadmap's "Done when" items name, reverts every migration and
// drops the database, so the configured development database is never touched.
import "reflect-metadata";
import { randomUUID } from "node:crypto";
import { DataSource, QueryFailedError } from "typeorm";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { databaseEnvironmentSchema, parseEnvironment } from "../config/environment.js";
import { CreateRuntimeSchema1791041200000 } from "./migrations/1791041200000-CreateRuntimeSchema.js";
import { CreateDemoSchema1791041260000 } from "./migrations/1791041260000-CreateDemoSchema.js";
import { AddControlAssessmentsAndTiming1791041320000 } from "./migrations/1791041320000-AddControlAssessmentsAndTiming.js";
import { AddApprovalsAndReservations1791041380000 } from "./migrations/1791041380000-AddApprovalsAndReservations.js";
import { AddReportsAndOutbox1791041440000 } from "./migrations/1791041440000-AddReportsAndOutbox.js";
import { buildTypeOrmOptions } from "./typeorm-options.js";

const DRAFT_MIGRATIONS = [
  CreateRuntimeSchema1791041200000,
  CreateDemoSchema1791041260000,
  AddControlAssessmentsAndTiming1791041320000,
  AddApprovalsAndReservations1791041380000,
  AddReportsAndOutbox1791041440000,
];

const databaseEnvironment = parseEnvironment(databaseEnvironmentSchema, process.env);
const temporaryDatabaseName = `draft_schema_test_${randomUUID().replaceAll("-", "")}`;
const organizationId = randomUUID();
const otherOrganizationId = randomUUID();

// Connected to the configured database only to create and drop the temporary one.
let administrationDataSource: DataSource;
let dataSource: DataSource;

/** Runs SQL in the temporary database. */
async function execute(sql: string, parameters: unknown[] = []): Promise<void> {
  await dataSource.query(sql, parameters);
}

/** Runs a SELECT or an INSERT ... RETURNING and returns its rows. */
async function selectRows<Row>(sql: string, parameters: unknown[] = []): Promise<Row[]> {
  return dataSource.query<Row[]>(sql, parameters);
}

/** Runs an INSERT ... RETURNING id and returns that id. */
async function insertReturningId(sql: string, parameters: unknown[] = []): Promise<string> {
  const [row] = await selectRows<{ id: string }>(sql, parameters);
  if (!row) throw new Error("the INSERT returned no row");
  return row.id;
}

/** Asserts that the statement fails with the named constraint or trigger message. */
async function expectRejected(sql: string, parameters: unknown[], expectedMessage: string) {
  const failure = await execute(sql, parameters).then(
    () => null,
    (error: unknown) => error,
  );
  expect(failure, `expected a rejection mentioning ${expectedMessage}`).toBeInstanceOf(
    QueryFailedError,
  );
  expect((failure as QueryFailedError).message).toContain(expectedMessage);
}

/** A passport, its run and one action of the given tool. */
async function createRunWithAction(tool: string) {
  const passportId = await insertReturningId(
    `INSERT INTO runtime.passports (organization_id, actor_id, task_version, admission_catalog_revision_id, scope, limits, expires_at)
     VALUES ($1, $2, 'reconcile_atlas_v1', 1, '{}', '{}', now() + interval '15 minutes') RETURNING id`,
    [organizationId, randomUUID()],
  );
  const runId = await insertReturningId(
    `INSERT INTO runtime.runs (organization_id, passport_id, status) VALUES ($1, $2, 'running') RETURNING id`,
    [organizationId, passportId],
  );
  const actionId = await insertReturningId(
    `INSERT INTO runtime.actions (organization_id, run_id, step_number, tool, canonical_arguments,
       canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
     VALUES ($1, $2, 1, $3, '{}', 1, sha256('digest'::bytea), $4, 1, 'proposed') RETURNING id`,
    [organizationId, runId, tool, randomUUID()],
  );
  return { passportId, runId, actionId };
}

/** Inserts a report for a create_report action and returns its id. */
async function createReport(template: "internal" | "vendor", content: string) {
  const { runId, actionId } = await createRunWithAction("create_report");
  const internal = template === "internal";
  return insertReturningId(
    `INSERT INTO demo.reports (organization_id, run_id, created_by_action_id, template, projection_rule,
       projection_policy_version, classification, destination_class, title, content, content_hash)
     VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'Report', $9, sha256(convert_to($9, 'UTF8'))) RETURNING id`,
    [
      organizationId,
      runId,
      actionId,
      internal ? "internal_investigation_v1" : "vendor_reconciliation_v1",
      internal ? null : "vendor_invoice_fields_v1",
      internal ? null : "p1",
      internal ? "internal_only" : "vendor_shareable",
      internal ? "internal_reviewers" : "registered_vendor_recipient",
      content,
    ],
  );
}

beforeAll(async () => {
  administrationDataSource = new DataSource(buildTypeOrmOptions(databaseEnvironment));
  await administrationDataSource.initialize();
  await administrationDataSource.query(`CREATE DATABASE ${temporaryDatabaseName}`);

  dataSource = new DataSource({
    ...buildTypeOrmOptions({ ...databaseEnvironment, POSTGRES_DB: temporaryDatabaseName }),
    migrations: DRAFT_MIGRATIONS,
  });
  await dataSource.initialize();
  await dataSource.runMigrations();
});

afterAll(async () => {
  if (dataSource?.isInitialized) await dataSource.destroy();
  if (administrationDataSource?.isInitialized) {
    await administrationDataSource.query(
      `DROP DATABASE IF EXISTS ${temporaryDatabaseName} WITH (FORCE)`,
    );
    await administrationDataSource.destroy();
  }
});

describe("draft runtime schema (SH-16)", () => {
  it("rejects a second action with the same stable identifier", async () => {
    const { runId, actionId } = await createRunWithAction("read_invoice");
    await expectRejected(
      `INSERT INTO runtime.actions (id, organization_id, run_id, step_number, tool, canonical_arguments,
         canonicalization_version, action_digest, idempotency_key, evaluated_catalog_revision_id, status)
       VALUES ($1, $2, $3, 2, 'read_invoice', '{}', 1, sha256('other'::bytea), $4, 1, 'proposed')`,
      [actionId, organizationId, runId, randomUUID()],
      "actions_pkey",
    );
  });

  it("keeps passports immutable against UPDATE and DELETE", async () => {
    const { passportId } = await createRunWithAction("read_invoice");
    await expectRejected(
      `UPDATE runtime.passports SET scope = '{"tools":["queue_report"]}' WHERE id = $1`,
      [passportId],
      "immutable",
    );
    await expectRejected(`DELETE FROM runtime.passports WHERE id = $1`, [passportId], "immutable");
  });

  it("records an event without a run but never an action event without its run", async () => {
    await execute(
      `INSERT INTO runtime.audit_events (organization_id, event_type) VALUES ($1, 'policy.reload_rejected')`,
      [organizationId],
    );
    const { actionId } = await createRunWithAction("read_invoice");
    await expectRejected(
      `INSERT INTO runtime.audit_events (organization_id, action_id, event_type) VALUES ($1, $2, 'action.denied')`,
      [organizationId, actionId],
      "audit_events_action_needs_run",
    );
  });

  it("rejects a reference to another organization's run", async () => {
    const { runId } = await createRunWithAction("read_invoice");
    await expectRejected(
      `INSERT INTO runtime.jobs (organization_id, run_id, kind, status) VALUES ($1, $2, 'agent_step', 'queued')`,
      [otherOrganizationId, runId],
      "jobs_run_fkey",
    );
  });
});

describe("draft approvals and reservations (SH-27)", () => {
  it("consumes an approval once and claims an action once", async () => {
    const { actionId } = await createRunWithAction("queue_report");
    const attemptId = await insertReturningId(
      `INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number) VALUES ($1, $2, 1) RETURNING id`,
      [organizationId, actionId],
    );
    const approvalId = await insertReturningId(
      `INSERT INTO runtime.approvals (organization_id, action_id, action_digest, review_payload_reference, reviewer_id, decision, expires_at)
       VALUES ($1, $2, sha256('digest'::bytea), 'report:r1@1', $3, 'approved', now() + interval '10 minutes') RETURNING id`,
      [organizationId, actionId, randomUUID()],
    );
    const consume = `UPDATE runtime.approvals SET consumed_at = now(), consumed_by_attempt_id = $2 WHERE id = $1`;
    await execute(consume, [approvalId, attemptId]);
    await expectRejected(consume, [approvalId, attemptId], "already consumed");
    await expectRejected(
      `INSERT INTO runtime.execution_attempts (organization_id, action_id, attempt_number) VALUES ($1, $2, 2)`,
      [organizationId, actionId],
      "execution_attempts_one_open_per_action",
    );
  });

  it("binds an approval to the action's exact digest", async () => {
    const { actionId } = await createRunWithAction("queue_report");
    await expectRejected(
      `INSERT INTO runtime.approvals (organization_id, action_id, action_digest, review_payload_reference, reviewer_id, decision, expires_at)
       VALUES ($1, $2, sha256('changed'::bytea), 'x', $3, 'approved', now() + interval '10 minutes')`,
      [organizationId, actionId, randomUUID()],
      "approvals_action_fkey",
    );
  });

  it("holds a concurrency slot while usage is unknown", async () => {
    const { runId } = await createRunWithAction("read_invoice");
    const insertCall = `INSERT INTO runtime.model_calls (organization_id, run_id, purpose, model)
       VALUES ($1, $2, $3, 'qwen3.5:4b') RETURNING id`;
    const agentCallId = await insertReturningId(insertCall, [organizationId, runId, "agent"]);
    const securityCallId = await insertReturningId(insertCall, [organizationId, runId, "security"]);
    await execute(
      `INSERT INTO runtime.budget_reservations (organization_id, run_id, purpose, model_call_id, concurrency_slot, state, resolved_at)
       VALUES ($1, $2, 'agent', $3, 1, 'usage_unknown', now())`,
      [organizationId, runId, agentCallId],
    );
    await expectRejected(
      `INSERT INTO runtime.budget_reservations (organization_id, run_id, purpose, model_call_id, concurrency_slot)
       VALUES ($1, $2, 'security', $3, 1)`,
      [organizationId, runId, securityCallId],
      "budget_reservations_slot_in_use",
    );
  });
});

describe("draft demo schema, reports and outbox (SH-17, SH-24)", () => {
  it("keeps an invoice's vendor in the invoice's organization", async () => {
    await execute(
      `INSERT INTO demo.vendors (id, organization_id, name) VALUES ('vendor_test', $1, 'Test vendor')`,
      [organizationId],
    );
    await expectRejected(
      `INSERT INTO demo.invoices (id, organization_id, vendor_id, external_reference, currency, total_minor_units, issued_on, due_on)
       VALUES ('invoice_test', $1, 'vendor_test', 'INV1', 'EUR', 1, '2026-09-01', '2026-09-30')`,
      [otherOrganizationId],
      "invoices_vendor_fkey",
    );
  });

  it("refuses an Internal only report in the outbox and a second outbox row per action", async () => {
    const enqueue = `INSERT INTO demo.outbox_messages (organization_id, action_id, report_id, report_content_hash, recipient)
       VALUES ($1, $2, $3, sha256(convert_to($4, 'UTF8')), 'reports@atlas.example.com')`;

    const internalReportId = await createReport("internal", "internal body");
    const queueInternal = await createRunWithAction("queue_report");
    await expectRejected(
      enqueue,
      [organizationId, queueInternal.actionId, internalReportId, "internal body"],
      "outbox_messages_report_fkey",
    );

    const vendorReportId = await createReport("vendor", "vendor body");
    const queueVendor = await createRunWithAction("queue_report");
    const vendorRow = [organizationId, queueVendor.actionId, vendorReportId, "vendor body"];
    await execute(enqueue, vendorRow);
    await expectRejected(enqueue, vendorRow, "outbox_messages_one_per_action");
  });
});

describe("draft migrations round trip", () => {
  it("reverts every draft migration and leaves no runtime or demo schema", async () => {
    for (let reverted = 0; reverted < DRAFT_MIGRATIONS.length; reverted += 1) {
      await dataSource.undoLastMigration();
    }
    const schemas = await selectRows<{ nspname: string }>(
      `SELECT nspname FROM pg_namespace WHERE nspname IN ('runtime', 'demo')`,
    );
    expect(schemas).toEqual([]);
    const applied = await dataSource.runMigrations();
    expect(applied.map((migration) => migration.name)).toEqual(
      DRAFT_MIGRATIONS.map((migrationClass) => new migrationClass().name),
    );
  });
});
