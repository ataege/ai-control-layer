// Contract consistency check: every fixture must satisfy its JSON schema, and one fixture
// per schema is pinned to a typed literal so the TS types cannot drift from the schemas.
// The Go gateway decodes the same fixtures in its own tests, so the three stay aligned.
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { Ajv2020 } from "ajv/dist/2020.js";

import type {
  ActionProposal,
  ApiReadinessResponse,
  ErrorResponse,
  GatewayDiagnosticsResponse,
  GatewayPingResponse,
  LivenessResponse,
  Passport,
  ReadinessResponse,
  ReasonCode,
  RunState,
  SafeEvent,
  StartRunRequest,
  StartRunResponse,
  StoredAction,
} from "../src/index.ts";

const packageRoot = join(import.meta.dirname, "..");
const schemaDirectory = join(packageRoot, "schemas");
const fixtureDirectory = join(packageRoot, "fixtures");

function readJson(filePath: string): unknown {
  return JSON.parse(readFileSync(filePath, "utf8"));
}

// Fixture files are named "<schema-name>.<case>.json".
function schemaNameOf(fixtureFileName: string): string {
  return fixtureFileName.split(".")[0] ?? "";
}

const schemaValidator = new Ajv2020({ allErrors: true, strict: true });
const schemaFileNames = readdirSync(schemaDirectory).filter((name) =>
  name.endsWith(".schema.json"),
);
const fixtureFileNames = readdirSync(fixtureDirectory).filter((name) => name.endsWith(".json"));

test("every schema compiles and has at least one fixture", () => {
  for (const schemaFileName of schemaFileNames) {
    const schemaName = schemaFileName.replace(".schema.json", "");
    schemaValidator.compile(readJson(join(schemaDirectory, schemaFileName)) as object);
    const hasFixture = fixtureFileNames.some((name) => schemaNameOf(name) === schemaName);
    assert.ok(hasFixture, `schema "${schemaName}" has no fixture`);
  }
});

test("every fixture matches its schema", () => {
  for (const fixtureFileName of fixtureFileNames) {
    const schemaPath = join(schemaDirectory, `${schemaNameOf(fixtureFileName)}.schema.json`);
    const validate = new Ajv2020({ allErrors: true, strict: true }).compile(
      readJson(schemaPath) as object,
    );
    const isValid = validate(readJson(join(fixtureDirectory, fixtureFileName)));
    assert.ok(isValid, `${fixtureFileName}: ${JSON.stringify(validate.errors)}`);
  }
});

function validatorFor(schemaName: string): (value: unknown) => boolean {
  const validate = new Ajv2020({ strict: true }).compile(
    readJson(join(schemaDirectory, `${schemaName}.schema.json`)) as object,
  );
  return (value) => validate(value);
}

test("runtime contracts reject inconsistent combinations", () => {
  const runState = readJson(join(fixtureDirectory, "run-state.running.json")) as RunState;
  const validateRunState = validatorFor("run-state");
  assert.equal(validateRunState({ ...runState, terminalReason: "run_cancelled" }), false);
  assert.equal(validateRunState({ ...runState, status: "paused" }), false);
  assert.equal(validateRunState({ ...runState, status: "paused", terminalReason: "nope" }), false);

  const validateProposal = validatorFor("action-proposal");
  assert.equal(validateProposal({ tool: "read_vendor", arguments: { invoice_id: "a" } }), false);
  assert.equal(validateProposal({ tool: "read_invoice", arguments: { invoice_id: "" } }), false);
  assert.equal(validateProposal({ tool: "read_invoice", arguments: { invoice_id: 7 } }), false);
  assert.equal(
    validateProposal({
      tool: "create_report",
      arguments: { template: "internal_investigation_v1", source_invoice_ids: ["a", "a"] },
    }),
    false,
  );
  assert.equal(
    validateProposal({
      tool: "queue_report",
      arguments: { report_id: "NOT-A-UUID", recipient_reference: "recipient:x" },
    }),
    false,
  );

  const event = readJson(join(fixtureDirectory, "safe-event.export-denied.json")) as SafeEvent;
  const validateEvent = validatorFor("safe-event");
  assert.equal(validateEvent({ ...event, runId: null }), false);
  assert.equal(
    validateEvent({ ...event, maskedSummary: { ...event.maskedSummary, rawNote: "x" } }),
    false,
  );
  const summaryWithoutMessage: Record<string, unknown> = { ...event.maskedSummary };
  delete summaryWithoutMessage.safeMessage;
  assert.equal(validateEvent({ ...event, maskedSummary: summaryWithoutMessage }), false);

  const passport = readJson(join(fixtureDirectory, "passport.atlas.json")) as Passport;
  const validatePassport = validatorFor("passport");
  assert.equal(validatePassport({ ...passport, scope: { ...passport.scope, tools: [] } }), false);
  assert.equal(
    validatePassport({ ...passport, limits: { ...passport.limits, callsTotal: 0 } }),
    false,
  );
});

test("schemas reject unknown fields and wrong enum values", () => {
  const validateLiveness = new Ajv2020().compile(
    readJson(join(schemaDirectory, "liveness.schema.json")) as object,
  );
  assert.equal(validateLiveness({ status: "ok", service: "gateway", extra: true }), false);
  assert.equal(validateLiveness({ status: "healthy", service: "gateway" }), false);
});

// Compile-time check: these literals must satisfy the TS types (`pnpm typecheck`).
// The tests below pin each literal to a real fixture file, so they cannot drift apart.
export const typedSamples = {
  liveness: { status: "ok", service: "gateway" } satisfies LivenessResponse,
  readiness: {
    status: "unavailable",
    service: "gateway",
    checks: { database: { status: "down", message: "database unreachable" } },
  } satisfies ReadinessResponse,
  apiReadiness: {
    status: "error",
    info: {},
    error: { database: { status: "down", message: "database unreachable" } },
    details: { database: { status: "down", message: "database unreachable" } },
  } satisfies ApiReadinessResponse,
  gatewayPing: { status: "ok", service: "gateway" } satisfies GatewayPingResponse,
  gatewayDiagnostics: {
    status: "degraded",
    requestId: "req-fixture-2",
    checks: {
      reachability: { status: "up", latencyMs: 4, upstreamStatus: 200 },
      databaseReadiness: { status: "down", latencyMs: 9, upstreamStatus: 503, reason: "not_ready" },
    },
  } satisfies GatewayDiagnosticsResponse,
  error: {
    error: { code: "unauthorized", message: "Missing or invalid service token." },
    statusCode: 401,
    requestId: "req-fixture-4",
    timestamp: "2026-01-01T00:00:00.000Z",
    path: "/internal/ping",
  } satisfies ErrorResponse,
  startRunRequest: {
    template: "reconcile_atlas_v1",
    invoiceIds: ["invoice_A01"],
    destination: "vendor_Atlas",
  } satisfies StartRunRequest,
  startRunResponse: {
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    passportId: "7a1b2c3d-4e5f-4a6b-9c7d-8e9f0a1b2c3d",
  } satisfies StartRunResponse,
  reasonCode: "report_export_restricted" satisfies ReasonCode,
  passport: {
    passportId: "7a1b2c3d-4e5f-4a6b-9c7d-8e9f0a1b2c3d",
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
    actorId: "2c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f",
    taskVersion: "reconcile_atlas_v1",
    admissionCatalogRevisionId: 1,
    issuedAt: "2026-10-03T12:00:00Z",
    expiresAt: "2026-10-03T12:15:00Z",
    scope: {
      tools: ["read_invoice", "read_vendor", "create_report", "queue_report"],
      invoiceIds: ["invoice_A01", "invoice_A02"],
      vendorIds: ["vendor_Atlas"],
      reportTemplates: ["internal_investigation_v1", "vendor_reconciliation_v1"],
      projectionRules: ["vendor_invoice_fields_v1"],
      recipientReferences: ["recipient:5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b:vendor_Atlas"],
      allowedModels: ["qwen3.5:4b"],
      internalNoteReadable: true,
      approvalRequiredTools: ["queue_report"],
    },
    limits: {
      callsTotal: 24,
      callsAgent: 12,
      callsSecurity: 12,
      tokensTotal: 20000,
      tokensAgent: null,
      tokensSecurity: null,
      requestTimeoutSeconds: 20,
      localMaxConcurrency: 2,
      toolAttempts: 12,
      corrections: 2,
      runExpiryMinutes: 15,
    },
  } satisfies Passport,
  actionProposal: {
    tool: "read_invoice",
    arguments: { invoice_id: "invoice_A01" },
  } satisfies ActionProposal,
  storedAction: {
    actionId: "9e8d7c6b-5a49-4382-a716-5f4e3d2c1b0a",
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    stepNumber: 1,
    proposal: { tool: "read_invoice", arguments: { invoice_id: "invoice_A01" } },
    canonicalizationVersion: 1,
    actionDigest: "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90",
    idempotencyKey: "9e8d7c6b-5a49-4382-a716-5f4e3d2c1b0a",
    evaluatedCatalogRevisionId: 1,
    status: "succeeded",
    expiresAt: null,
    createdAt: "2026-10-03T12:01:00.123Z",
  } satisfies StoredAction,
  runState: {
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    passportId: "7a1b2c3d-4e5f-4a6b-9c7d-8e9f0a1b2c3d",
    status: "paused",
    terminalReason: "allowance_exhausted",
    cancelRequestedAt: null,
    createdAt: "2026-10-03T12:00:00Z",
    updatedAt: "2026-10-03T12:07:30Z",
  } satisfies RunState,
  safeEvent: {
    eventId: "7",
    organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
    runId: null,
    actionId: null,
    eventType: "admission.rejected",
    decision: "deny",
    reasonCode: "resource_out_of_scope",
    catalogRevisionId: 1,
    maskedSummary: {
      purpose: null,
      admissionCatalogRevisionId: null,
      matchedRule: null,
      feedRevision: null,
      reportId: null,
      template: null,
      classification: null,
      lineageCheck: null,
      effect: "none",
      replaySource: null,
      alternativeTemplate: null,
      safeMessage: "Requested invoice is outside the task's authority.",
    },
    occurredAt: "2026-10-03T11:59:00Z",
  } satisfies SafeEvent,
};

// Fixture file that each typed literal must equal, one per schema.
const fixtureFileOfSample: Record<keyof typeof typedSamples, string> = {
  liveness: "liveness.gateway.json",
  readiness: "readiness.unavailable.json",
  apiReadiness: "api-readiness.unavailable.json",
  gatewayPing: "gateway-ping.ok.json",
  gatewayDiagnostics: "gateway-diagnostics.degraded.json",
  error: "error.unauthorized.json",
  startRunRequest: "start-run-request.minimal.json",
  startRunResponse: "start-run-response.created.json",
  reasonCode: "reason-code.report-export-restricted.json",
  passport: "passport.atlas.json",
  actionProposal: "action-proposal.read-invoice.json",
  storedAction: "stored-action.read-allowed.json",
  runState: "run-state.paused-allowance.json",
  safeEvent: "safe-event.admission-rejected.json",
};

test("typed samples are identical to their fixtures", () => {
  for (const [sampleName, fixtureFileName] of Object.entries(fixtureFileOfSample)) {
    assert.deepEqual(
      typedSamples[sampleName as keyof typeof typedSamples],
      readJson(join(fixtureDirectory, fixtureFileName)),
      `${sampleName} drifted from ${fixtureFileName}`,
    );
  }
});

test("every schema has a typed sample", () => {
  const schemaNamesWithSample = new Set(Object.values(fixtureFileOfSample).map(schemaNameOf));
  for (const schemaFileName of schemaFileNames) {
    const schemaName = schemaFileName.replace(".schema.json", "");
    assert.ok(schemaNamesWithSample.has(schemaName), `schema "${schemaName}" has no typed sample`);
  }
});
