// Contract consistency check: every fixture must satisfy its JSON schema, and one fixture
// per schema is pinned to a typed literal so the TS types cannot drift from the schemas.
// The Go gateway decodes the same fixtures in its own tests, so the three stay aligned.
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { Ajv2020, type ValidateFunction } from "ajv/dist/2020.js";

import type {
  ActionProposal,
  ApiReadinessResponse,
  ApprovalDecision,
  ApprovalResponse,
  AssessmentPage,
  AssessmentRecord,
  CatalogStatus,
  ControlEvaluationRequest,
  ControlEvaluationResponse,
  ErrorResponse,
  GatewayDiagnosticsResponse,
  GatewayPingResponse,
  LivenessResponse,
  OperatorContext,
  Passport,
  ReadinessResponse,
  ReasonCode,
  ReportView,
  ReviewView,
  RunEventsPage,
  RunState,
  RunUsage,
  SafeEvent,
  SecurityEventPage,
  SecuritySummary,
  StartRunRequest,
  StartRunResponse,
  StoredAction,
  TaskFormOptions,
  PolicyReloadRequest,
  PolicyReloadResponse,
  PolicyReloadErrorResponse,
  PolicyStatusResponse,
  SemanticVerdict,
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

const schemaFileNames = readdirSync(schemaDirectory).filter((name) =>
  name.endsWith(".schema.json"),
);

// One validator holds every schema, so a schema can reference another by its $id (a page of
// events refers to safe-event.schema.json instead of copying it).
function newSchemaValidator(): Ajv2020 {
  const validator = new Ajv2020({ allErrors: true, strict: true });
  for (const schemaFileName of schemaFileNames) {
    validator.addSchema(readJson(join(schemaDirectory, schemaFileName)) as object);
  }
  return validator;
}

function compiledSchema(validator: Ajv2020, schemaName: string): ValidateFunction {
  const validate = validator.getSchema(`${schemaName}.schema.json`);
  assert.ok(validate, `schema "${schemaName}" is not registered`);
  return validate;
}
const fixtureFileNames = readdirSync(fixtureDirectory).filter((name) => name.endsWith(".json"));

test("every schema compiles and has at least one fixture", () => {
  for (const schemaFileName of schemaFileNames) {
    const schemaName = schemaFileName.replace(".schema.json", "");
    compiledSchema(newSchemaValidator(), schemaName);
    const hasFixture = fixtureFileNames.some((name) => schemaNameOf(name) === schemaName);
    assert.ok(hasFixture, `schema "${schemaName}" has no fixture`);
  }
});

test("every fixture matches its schema", () => {
  for (const fixtureFileName of fixtureFileNames) {
    const validate = compiledSchema(newSchemaValidator(), schemaNameOf(fixtureFileName));
    const isValid = validate(readJson(join(fixtureDirectory, fixtureFileName)));
    assert.ok(isValid, `${fixtureFileName}: ${JSON.stringify(validate.errors)}`);
  }
});

function validatorFor(schemaName: string): (value: unknown) => boolean {
  const validate = compiledSchema(newSchemaValidator(), schemaName);
  return (value) => validate(value);
}

test("read contracts reject inconsistent combinations", () => {
  const vendorView = readJson(join(fixtureDirectory, "report-view.vendor.json")) as ReportView;
  const validateReportView = validatorFor("report-view");
  assert.equal(validateReportView(vendorView), true);
  // Only Internal only content is ever withheld, and withheld content is null.
  assert.equal(validateReportView({ ...vendorView, content: null, contentWithheld: true }), false);
  assert.equal(validateReportView({ ...vendorView, contentWithheld: true }), false);
  // The vendor template always names its projection; the internal one never does.
  assert.equal(validateReportView({ ...vendorView, projectionRule: null }), false);
  assert.equal(
    validateReportView({
      ...vendorView,
      template: "internal_investigation_v1",
      classification: "internal_only",
    }),
    false,
  );

  const usage = readJson(join(fixtureDirectory, "run-usage.ledger.json")) as RunUsage;
  const validateUsage = validatorFor("run-usage");
  assert.equal(validateUsage({ ...usage, modelCalls: [...usage.modelCalls].reverse() }), false);
  assert.equal(validateUsage({ ...usage, modelCalls: [usage.modelCalls[0]] }), false);

  const record = readJson(
    join(fixtureDirectory, "assessment-record.semantic-not-applicable.json"),
  ) as AssessmentRecord;
  const validateRecord = validatorFor("assessment-record");
  assert.equal(validateRecord(record), true);
  // An unclassified semantic record has no verdict; another outcome needs a source.
  assert.equal(validateRecord({ ...record, outcome: "pass" }), false);
  assert.equal(validateRecord({ ...record, matchedRuleId: "rule\u0007" }), false);
});

test("the catalog status contract rejects what Go never produces", () => {
  const status = readJson(join(fixtureDirectory, "catalog-status.active.json")) as CatalogStatus;
  const validateStatus = validatorFor("catalog-status");
  assert.equal(validateStatus(status), true);
  assert.equal(
    validateStatus(readJson(join(fixtureDirectory, "catalog-status.rejected-request.json"))),
    true,
  );
  // A policy or feed text field, an unknown control or a non-hex digest is never part of the read.
  assert.equal(validateStatus({ ...status, sourceText: "policy" }), false);
  assert.equal(
    validateStatus({ ...status, controls: [{ ...status.controls[0], controlId: "other" }] }),
    false,
  );
  assert.equal(validateStatus({ ...status, policyDigest: "not-a-digest" }), false);
  assert.equal(validateStatus({ ...status, lastError: { code: "x" } }), false);
});

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
  // rejectionCause is a fixed kind of a rejected final answer, never free text.
  assert.equal(
    validateEvent({
      ...event,
      maskedSummary: { ...event.maskedSummary, rejectionCause: "code_fence" },
    }),
    true,
  );
  assert.equal(
    validateEvent({
      ...event,
      maskedSummary: { ...event.maskedSummary, rejectionCause: "The answer said: fraud" as never },
    }),
    false,
  );

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
    replaySource: null,
  } satisfies StoredAction,
  runState: {
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    passportId: "7a1b2c3d-4e5f-4a6b-9c7d-8e9f0a1b2c3d",
    status: "paused",
    terminalReason: "allowance_exhausted",
    cancelRequestedAt: null,
    createdAt: "2026-10-03T12:00:00Z",
    updatedAt: "2026-10-03T12:07:30Z",
    resultReference: null,
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
      actorId: null,
      inputSource: null,
      evaluationId: null,
      rejectionCause: null,
    },
    occurredAt: "2026-10-03T11:59:00Z",
  } satisfies SafeEvent,
  operatorContext: {
    userId: "2c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f",
    organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
    roles: ["operator", "reviewer"],
  } satisfies OperatorContext,
  approvalDecision: { decision: "approve" } satisfies ApprovalDecision,
  controlEvaluationRequest: {
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    kind: "action_proposal",
    text: null,
    tool: "read_invoice",
    arguments: { invoice_id: "invoice_B01" },
  } satisfies ControlEvaluationRequest,
  controlEvaluationResponse: {
    evaluationId: "8c9d0e1f-2a3b-4c4d-8e5f-6a7b8c9d0e1f",
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    actionId: null,
    decision: "deny",
    reasonCode: "resource_out_of_scope",
    safeMessage: "The proposed action names a record outside the run's passport.",
    alternativeTemplate: null,
    controls: [],
    semantic: null,
    content: null,
    catalog: { admissionRevisionId: 1, activeRevisionId: 1, feedRevisionId: 1 },
  } satisfies ControlEvaluationResponse,
  runUsage: {
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    modelCalls: [
      {
        purpose: "agent",
        dispatched: 3,
        completed: 2,
        failed: 0,
        usageUnknown: 1,
        inFlight: 0,
        settledTokens: 300,
        heldTokens: 40,
        usageUnknownReservations: 1,
      },
      {
        purpose: "security",
        dispatched: 1,
        completed: 0,
        failed: 0,
        usageUnknown: 0,
        inFlight: 1,
        settledTokens: 0,
        heldTokens: 10,
        usageUnknownReservations: 0,
      },
    ],
    ledger: {
      paused: false,
      tokens: {
        limit: 20000,
        reserved: 50,
        used: 300,
      },
      agentTokens: {
        limit: 12000,
        reserved: 40,
        used: 300,
      },
      securityTokens: {
        limit: null,
        reserved: 10,
        used: 0,
      },
      calls: {
        limit: 24,
        agentLimit: 12,
        securityLimit: 12,
        agent: 3,
        security: 1,
      },
      requestTimeoutMilliseconds: 20000,
      maxConcurrentCalls: 2,
      callsInFlight: 2,
    },
    toolAttempts: {
      total: 4,
      succeeded: 1,
      failed: 1,
      aborted: 1,
      open: 1,
    },
  } satisfies RunUsage,
  runEventsPage: {
    events: [
      {
        eventId: "41",
        organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
        runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
        actionId: "9e8d7c6b-5a49-4382-a716-5f4e3d2c1b0a",
        eventType: "report.export_denied",
        decision: "deny",
        reasonCode: "report_export_restricted",
        catalogRevisionId: 1,
        maskedSummary: {
          purpose: null,
          admissionCatalogRevisionId: null,
          matchedRule: null,
          feedRevision: null,
          reportId: "3b532026-0d57-4c4c-8f0b-6f320ebd5d66",
          template: "internal_investigation_v1",
          classification: "internal_only",
          lineageCheck: "passed",
          effect: "none",
          replaySource: null,
          alternativeTemplate: "vendor_reconciliation_v1",
          safeMessage:
            "This report inherits an Internal only restriction and cannot be sent to the vendor.",
          actorId: null,
          inputSource: null,
          evaluationId: null,
          rejectionCause: null,
        },
        occurredAt: "2026-10-03T12:05:00Z",
      },
      {
        eventId: "42",
        organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
        runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
        actionId: "9e8d7c6b-5a49-4382-a716-5f4e3d2c1b0a",
        eventType: "report.safe_template_offered",
        decision: "deny",
        reasonCode: "report_export_restricted",
        catalogRevisionId: 1,
        maskedSummary: {
          purpose: null,
          admissionCatalogRevisionId: null,
          matchedRule: null,
          feedRevision: null,
          reportId: "3b532026-0d57-4c4c-8f0b-6f320ebd5d66",
          template: null,
          classification: null,
          lineageCheck: null,
          effect: "none",
          replaySource: null,
          alternativeTemplate: "vendor_reconciliation_v1",
          safeMessage:
            "This report inherits an Internal only restriction and cannot be sent to the vendor.",
          actorId: null,
          inputSource: null,
          evaluationId: null,
          rejectionCause: null,
        },
        occurredAt: "2026-10-03T12:05:00Z",
      },
    ],
    nextCursor: "42",
  } satisfies RunEventsPage,
  assessmentRecord: {
    assessmentId: "3",
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    evaluationId: "7b911223-f38a-48f3-91b5-fbb942f0229a",
    actionId: null,
    securityModelCallId: "ea6bd0bf-d14e-4109-9889-1f0f8aa7c26d",
    boundary: "model_input",
    controlClass: "semantic",
    controlId: "semantic_injection",
    outcome: "block",
    reasonCode: "semantic_injection_detected",
    admissionCatalogRevisionId: 1,
    evaluatedCatalogRevisionId: 1,
    matchedRuleId: null,
    feedRevision: null,
    verdictSource: "live",
    verdict: {
      risk_category: "instruction_injection",
      score: 0.97,
      reason_code: "instruction_override",
    },
    inputSource: "judge",
    assessedAt: "2026-10-03T12:10:00.5Z",
  } satisfies AssessmentRecord,
  assessmentPage: {
    records: [
      {
        assessmentId: "2",
        runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
        evaluationId: "6a0a71b2-9677-430a-8958-4fd79345d1f1",
        actionId: "9e8d7c6b-5a49-4382-a716-5f4e3d2c1b0a",
        securityModelCallId: null,
        boundary: "action_proposal",
        controlClass: "deterministic",
        controlId: "signature_match",
        outcome: "block",
        reasonCode: "signature_match",
        admissionCatalogRevisionId: 1,
        evaluatedCatalogRevisionId: 1,
        matchedRuleId: "sig_override_001",
        feedRevision: "feed_v1",
        verdictSource: null,
        verdict: null,
        inputSource: null,
        assessedAt: "2026-10-03T12:03:00.123456Z",
      },
      {
        assessmentId: "3",
        runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
        evaluationId: "7b911223-f38a-48f3-91b5-fbb942f0229a",
        actionId: null,
        securityModelCallId: "ea6bd0bf-d14e-4109-9889-1f0f8aa7c26d",
        boundary: "model_input",
        controlClass: "semantic",
        controlId: "semantic_injection",
        outcome: "block",
        reasonCode: "semantic_injection_detected",
        admissionCatalogRevisionId: 1,
        evaluatedCatalogRevisionId: 1,
        matchedRuleId: null,
        feedRevision: null,
        verdictSource: "live",
        verdict: {
          risk_category: "instruction_injection",
          score: 0.97,
          reason_code: "instruction_override",
        },
        inputSource: "judge",
        assessedAt: "2026-10-03T12:10:00.5Z",
      },
    ],
    nextCursor: "v1.1746.1750.3",
  } satisfies AssessmentPage,
  securityEventPage: {
    events: [
      {
        eventId: "57",
        organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
        runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
        actionId: null,
        eventType: "control.evaluated",
        decision: "deny",
        reasonCode: "semantic_injection_detected",
        catalogRevisionId: 1,
        maskedSummary: {
          purpose: "security",
          admissionCatalogRevisionId: 1,
          matchedRule: null,
          feedRevision: null,
          reportId: null,
          template: null,
          classification: null,
          lineageCheck: null,
          effect: "none",
          replaySource: null,
          alternativeTemplate: null,
          safeMessage: "The security check found instruction-like content and withheld it.",
          actorId: "2c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f",
          inputSource: "judge",
          evaluationId: "7b911223-f38a-48f3-91b5-fbb942f0229a",
          rejectionCause: null,
        },
        occurredAt: "2026-10-03T12:10:00.5Z",
      },
    ],
    nextCursor: "v1.1750.0.0",
  } satisfies SecurityEventPage,
  catalogStatus: {
    activeRevisionId: 1,
    requestedRevisionId: 1,
    validatedRevisionId: 1,
    policyDigest: "df00c9d6064542c1a5863abbb03d2a765a3b84b0385775b30bb2b50b6343c072",
    feedRevisionId: 1,
    feedRevision: "feed_v1",
    feedDigest: "c40e5df8ccf55a56908dc56f906173d5a9a72678fa2ff20170a5b09114c67244",
    feedRuleCount: 4,
    lastError: null,
    controls: [
      {
        controlId: "secret_pattern",
        controlClass: "deterministic",
        enabled: true,
        mode: "redact",
        threshold: null,
        boundaries: ["model_input", "tool_result"],
      },
      {
        controlId: "semantic_injection",
        controlClass: "semantic",
        enabled: true,
        mode: "block",
        threshold: 0.75,
        boundaries: ["model_input", "tool_result", "action_proposal"],
      },
      {
        controlId: "signature_match",
        controlClass: "deterministic",
        enabled: true,
        mode: null,
        threshold: null,
        boundaries: ["model_input", "tool_result", "action_proposal"],
      },
    ],
    disabledRules: [],
  } satisfies CatalogStatus,
  securitySummary: {
    organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
    generatedAt: "2026-10-03T12:30:00.25Z",
    runs: [
      {
        status: "awaiting_approval",
        count: 1,
      },
      {
        status: "completed",
        count: 2,
      },
    ],
    decisions: [
      {
        eventType: "action.denied",
        decision: "deny",
        reasonCode: "invalid_arguments",
        inputSource: null,
        rejectionCause: "code_fence",
        count: 1,
      },
      {
        eventType: "control.evaluated",
        decision: "deny",
        reasonCode: "semantic_injection_detected",
        inputSource: "judge",
        rejectionCause: null,
        count: 1,
      },
      {
        eventType: "report.export_denied",
        decision: "deny",
        reasonCode: "report_export_restricted",
        inputSource: null,
        rejectionCause: null,
        count: 1,
      },
    ],
    assessments: [
      {
        controlClass: "deterministic",
        controlId: "signature_match",
        outcome: "block",
        verdictSource: null,
        inputSource: null,
        count: 1,
      },
      {
        controlClass: "semantic",
        controlId: "semantic_injection",
        outcome: "block",
        verdictSource: "live",
        inputSource: "judge",
        count: 1,
      },
      {
        controlClass: "semantic",
        controlId: "semantic_injection",
        outcome: "pass",
        verdictSource: "fixture",
        inputSource: null,
        count: 4,
      },
    ],
    modelUsage: [
      {
        purpose: "agent",
        dispatched: 9,
        completed: 9,
        failed: 0,
        usageUnknown: 0,
        inFlight: 0,
        settledTokens: 6120,
        heldTokens: 0,
        usageUnknownReservations: 0,
      },
      {
        purpose: "security",
        dispatched: 6,
        completed: 6,
        failed: 0,
        usageUnknown: 0,
        inFlight: 0,
        settledTokens: 1310,
        heldTokens: 0,
        usageUnknownReservations: 0,
      },
    ],
    judgeSecurityCalls: 1,
    timings: [
      {
        phase: "deterministic",
        count: 15,
        failed: 0,
        medianMicroseconds: 77,
        p95Microseconds: 172,
        maxMicroseconds: 640,
      },
      {
        phase: "provider",
        count: 15,
        failed: 1,
        medianMicroseconds: 1919761,
        p95Microseconds: 1983987,
        maxMicroseconds: 2060859,
      },
    ],
  } satisfies SecuritySummary,
  reportView: {
    reportId: "f7c58bfd-6ed2-403c-abba-7f698d7aaea2",
    runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
    version: 1,
    template: "vendor_reconciliation_v1",
    templateVersion: 1,
    projectionRule: "vendor_invoice_fields_v1",
    projectionRuleVersion: 1,
    classification: "vendor_shareable",
    destinationClass: "registered_vendor_recipient",
    title: "Vendor reconciliation",
    contentHash: "500b7e182fb9fa4ed633aad1094763cb22109114986ab51e0d43e112684e9d15",
    content:
      "Vendor reconciliation (vendor_reconciliation_v1, projection vendor_invoice_fields_v1)\n\n- invoice_A01: external reference INV104, total EUR 1250.00, due 2026-10-31, duplicate reference: yes\n- invoice_A02: external reference INV104, total EUR 1250.00, due 2026-10-31, duplicate reference: yes\n",
    contentWithheld: false,
    lineage: [
      {
        sourceKind: "invoice",
        sourceId: "invoice_A01",
        sourceVersion: 1,
        classification: "vendor_shareable",
        consumedFields: [
          "invoice_id",
          "external_reference",
          "duplicate_reference",
          "currency",
          "total_minor_units",
          "due_on",
        ],
      },
      {
        sourceKind: "invoice",
        sourceId: "invoice_A02",
        sourceVersion: 1,
        classification: "vendor_shareable",
        consumedFields: [
          "invoice_id",
          "external_reference",
          "duplicate_reference",
          "currency",
          "total_minor_units",
          "due_on",
        ],
      },
    ],
  } satisfies ReportView,
  approvalResponse: {
    approvalId: "a9d8e64b-db06-46aa-bfbb-b3889db82366",
    actionId: "df036f8b-f6bb-4e9e-bf60-a6c50ae4c830",
    runId: "073514cb-6817-4cac-8af7-516e72952704",
    decision: "approve",
  } satisfies ApprovalResponse,
  reviewView: {
    canonicalization_version: 1,
    action_id: "df036f8b-f6bb-4e9e-bf60-a6c50ae4c830",
    run_id: "073514cb-6817-4cac-8af7-516e72952704",
    passport_id: "cf67a545-4d64-49c6-ae9d-4a945bae8a69",
    policy_revision_id: 1,
    tool: "queue_report",
    canonical_arguments: {
      report_id: "1ce31118-cb8b-46b6-91c2-cb09e9216b66",
      recipient_reference: "recipient:073514cb-6817-4cac-8af7-516e72952704:vendor_Atlas_88f6c992",
    },
    recipient: {
      reference: "recipient:073514cb-6817-4cac-8af7-516e72952704:vendor_Atlas_88f6c992",
      vendor_id: "vendor_Atlas_88f6c992",
      address: "reports-88f6c992@atlas.example.com",
    },
    report: {
      id: "1ce31118-cb8b-46b6-91c2-cb09e9216b66",
      version: 1,
      template: "vendor_reconciliation_v1",
      template_version: 1,
      projection_rule: "vendor_invoice_fields_v1",
      projection_rule_version: 1,
      classification: "vendor_shareable",
      content_hash: "b0afb95f7c9c4d9a057d8a4ca7892a79884c0fd431e2716bcc3e3d158a0dc6e3",
      content: "Vendor reconciliation 88f6c992",
      sources: [
        {
          kind: "invoice",
          id: "invoice_A01_88f6c992",
          version: 1,
          classification: "vendor_shareable",
          consumed_fields: [
            "invoice_id",
            "external_reference",
            "duplicate_reference",
            "currency",
            "total_minor_units",
            "due_on",
          ],
        },
      ],
      source_manifest_digest: "63a791578bdb66d0014e205811e20226d84a638102b2bc37c53292b8c360644e",
    },
    expires_at: "2026-10-03T22:23:29.02559Z",
  } satisfies ReviewView,
  policyReloadRequest: {} satisfies PolicyReloadRequest,
  policyReloadResponse: {
    status: "requested",
    requestedRevisionId: "2",
    fileDigest: "a".repeat(64),
    feedRevision: "feed_v1",
  } satisfies PolicyReloadResponse,
  policyStatusResponse: {
    requestedRevisionId: "2",
    validatedRevisionId: "2",
    activeRevisionId: "2",
    activeFeedRevisionId: "1",
    lastError: null,
  } satisfies PolicyStatusResponse,
  policyReloadErrorResponse: {
    error: {
      code: "policy_reload_rejected",
      message: "Policy validation failed",
      issues: [
        {
          path: "budgets.calls_agent",
          message: "agent and security calls exceed the shared ceiling",
        },
      ],
    },
    statusCode: 400,
    requestId: "reload-fixture",
    timestamp: "2026-10-04T00:00:00Z",
    path: "/api/policies/reload",
  } satisfies PolicyReloadErrorResponse,
  taskFormOptions: {
    templates: [{ id: "reconcile_atlas_v1", name: "Reconcile Atlas invoices" }],
    vendors: [{ id: "vendor_Atlas", name: "Atlas" }],
    invoices: [
      {
        id: "invoice_A01",
        number: "INV104",
        date: "2026-09-01",
        amount: 125000,
        currency: "EUR",
        vendorId: "vendor_Atlas",
      },
      {
        id: "invoice_A02",
        number: "INV104",
        date: "2026-09-08",
        amount: 125000,
        currency: "EUR",
        vendorId: "vendor_Atlas",
      },
    ],
    destinations: [{ id: "vendor_Atlas", name: "Atlas" }],
    approvalRequirements: [
      {
        id: "review_queue_report",
        description:
          "A reviewer approves the exact report and recipient before the report is queued.",
      },
    ],
    limits: { maxModelCalls: 24, maxTimeoutSeconds: 900 },
  } satisfies TaskFormOptions,
  semanticVerdict: {
    risk_category: "instruction_injection",
    score: 0.92,
    reason_code: "instruction_override",
  } satisfies SemanticVerdict,
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
  operatorContext: "operator-context.operator.json",
  approvalDecision: "approval-decision.approve.json",
  controlEvaluationRequest: "control-evaluation-request.action-proposal.json",
  controlEvaluationResponse: "control-evaluation-response.scope-deny.json",
  runUsage: "run-usage.ledger.json",
  runEventsPage: "run-events-page.export-denied.json",
  assessmentRecord: "assessment-record.semantic-judge.json",
  assessmentPage: "assessment-page.two-records.json",
  securityEventPage: "security-event-page.judge.json",
  catalogStatus: "catalog-status.active.json",
  securitySummary: "security-summary.judge-split.json",
  reportView: "report-view.vendor.json",
  approvalResponse: "approval-response.approve.json",
  reviewView: "review-view.queue-report.json",
  taskFormOptions: "task-form-options.atlas.json",
  policyReloadRequest: "policy-reload-request.empty.json",
  policyReloadResponse: "policy-reload-response.requested.json",
  policyStatusResponse: "policy-status-response.active.json",
  policyReloadErrorResponse: "policy-reload-error-response.invalid.json",
  semanticVerdict: "semantic-verdict.instruction-injection.json",
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

test("review and approval contracts reject what Go never produces", () => {
  const review = readJson(join(fixtureDirectory, "review-view.queue-report.json")) as ReviewView;
  const validateReview = validatorFor("review-view");
  assert.equal(validateReview(review), true);
  assert.equal(validateReview({ ...review, recipient: null }), false);
  assert.equal(validateReview({ ...review, report: null }), false);
  assert.equal(validateReview({ ...review, reviewer: "x" }), false);
  assert.equal(
    validateReview({ ...review, tool: "read_invoice", recipient: null, report: null }),
    true,
  );
  assert.equal(validateReview({ ...review, tool: "read_invoice" }), false);

  const approval = readJson(
    join(fixtureDirectory, "approval-response.approve.json"),
  ) as ApprovalResponse;
  const validateApproval = validatorFor("approval-response");
  assert.equal(validateApproval({ ...approval, decision: "approved" }), false);
  assert.equal(validateApproval({ ...approval, payload: {} }), false);
});

// NestJS-owned API-33 shapes. These literals pin types to the corresponding schemas/fixtures.
test("policy reload contracts reject browser authority and inconsistent responses", () => {
  const request: PolicyReloadRequest = {};
  const response: PolicyReloadResponse = {
    status: "requested",
    requestedRevisionId: "2",
    fileDigest: "a".repeat(64),
    feedRevision: "feed_v1",
  };
  const status: PolicyStatusResponse = {
    requestedRevisionId: "2",
    validatedRevisionId: "2",
    activeRevisionId: "2",
    activeFeedRevisionId: "1",
    lastError: null,
  };
  const failure: PolicyReloadErrorResponse = {
    error: {
      code: "policy_reload_rejected",
      message: "Policy validation failed",
      issues: [
        {
          path: "budgets.calls_agent",
          message: "agent and security calls exceed the shared ceiling",
        },
      ],
    },
    statusCode: 400,
    requestId: "reload-fixture",
    timestamp: "2026-10-04T00:00:00Z",
    path: "/api/policies/reload",
  };
  for (const [name, value, fixture] of [
    ["policy-reload-request", request, "empty"],
    ["policy-reload-response", response, "requested"],
    ["policy-status-response", status, "active"],
    ["policy-reload-error-response", failure, "invalid"],
  ] as const) {
    assert.equal(validatorFor(name)(value), true);
    assert.deepEqual(value, readJson(join(fixtureDirectory, `${name}.${fixture}.json`)));
  }
  assert.equal(validatorFor("policy-reload-request")({ userId: "forged" }), false);
  assert.equal(validatorFor("policy-reload-response")({ ...response, status: "active" }), false);
  assert.equal(
    validatorFor("policy-status-response")({ ...status, rawPolicy: "protected" }),
    false,
  );
});

test("the semantic verdict schema rejects what Go's ParseVerdict rejects", () => {
  const validateVerdict = validatorFor("semantic-verdict");
  const verdict = readJson(
    join(fixtureDirectory, "semantic-verdict.instruction-injection.json"),
  ) as SemanticVerdict;
  assert.equal(validateVerdict(verdict), true);
  assert.equal(validateVerdict({ ...verdict, explanation: "free text" }), false);
  assert.equal(validateVerdict({ ...verdict, risk_category: "prompt_injection" }), false);
  assert.equal(validateVerdict({ ...verdict, reason_code: "looks_bad" }), false);
  assert.equal(validateVerdict({ ...verdict, score: 90 }), false);
  assert.equal(validateVerdict({ ...verdict, score: -0.1 }), false);
  const withoutScore: Record<string, unknown> = { ...verdict };
  delete withoutScore.score;
  assert.equal(validateVerdict(withoutScore), false);
});
