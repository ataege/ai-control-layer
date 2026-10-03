// Contract consistency check: every fixture must satisfy its JSON schema, and one fixture
// per schema is pinned to a typed literal so the TS types cannot drift from the schemas.
// The Go gateway decodes the same fixtures in its own tests, so the three stay aligned.
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { Ajv2020 } from "ajv/dist/2020.js";

import type {
  ApiReadinessResponse,
  ErrorResponse,
  GatewayDiagnosticsResponse,
  GatewayPingResponse,
  LivenessResponse,
  ReadinessResponse,
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
};

// Fixture file that each typed literal must equal, one per schema.
const fixtureFileOfSample: Record<keyof typeof typedSamples, string> = {
  liveness: "liveness.gateway.json",
  readiness: "readiness.unavailable.json",
  apiReadiness: "api-readiness.unavailable.json",
  gatewayPing: "gateway-ping.ok.json",
  gatewayDiagnostics: "gateway-diagnostics.degraded.json",
  error: "error.unauthorized.json",
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
