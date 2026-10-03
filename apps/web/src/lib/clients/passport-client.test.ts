import { describe, expect, it } from "vitest";

import { fetchPassport, isPassport } from "./passport-client";

const validPassport = {
  passportId: "p1",
  runId: "r1",
  organizationId: "o1",
  actorId: "a1",
  taskVersion: "reconcile_atlas_v1",
  admissionCatalogRevisionId: 3,
  issuedAt: "2026-10-04T08:00:00Z",
  expiresAt: "2026-10-04T08:15:00Z",
  scope: {
    tools: ["read_invoice"],
    invoiceIds: ["INV-1"],
    vendorIds: ["V-1"],
    reportTemplates: ["internal_investigation_v1"],
    projectionRules: [],
    recipientReferences: ["recipient:r1:V-1"],
    allowedModels: ["local"],
    internalNoteReadable: true,
    approvalRequiredTools: ["queue_report"],
  },
  limits: {
    callsTotal: 10,
    callsAgent: 6,
    callsSecurity: 4,
    tokensTotal: 1000,
    tokensAgent: null,
    tokensSecurity: null,
    requestTimeoutSeconds: 30,
    localMaxConcurrency: 1,
    toolAttempts: 3,
    corrections: 2,
    runExpiryMinutes: 15,
  },
};

function respondWith(status: number, body: unknown): typeof fetch {
  return async () => new Response(JSON.stringify(body), { status });
}

describe("fetchPassport", () => {
  it("returns a well-formed passport", async () => {
    const result = await fetchPassport("r1", {
      fetchImplementation: respondWith(200, validPassport),
    });
    expect(result.ok && result.data.passportId).toBe("p1");
  });

  it("rejects a body whose scope lists are not arrays rather than rendering it", async () => {
    const broken = { ...validPassport, scope: { ...validPassport.scope, tools: null } };
    const result = await fetchPassport("r1", { fetchImplementation: respondWith(200, broken) });
    expect(result.ok).toBe(false);
    expect(isPassport(broken)).toBe(false);
  });

  it("passes a 404 through as an http failure", async () => {
    const result = await fetchPassport("r1", {
      fetchImplementation: respondWith(404, { error: { code: "not_found" } }),
    });
    expect(!result.ok && result.error.kind === "http" && result.error.status).toBe(404);
  });
});
