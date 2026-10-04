import { readFileSync } from "node:fs";
import type { NestExpressApplication } from "@nestjs/platform-express";
import { getRepositoryToken } from "@nestjs/typeorm";
import state from "@workspace/contracts/fixtures/run-state.completed.json" with { type: "json" };
import usage from "@workspace/contracts/fixtures/run-usage.ledger.json" with { type: "json" };
import events from "@workspace/contracts/fixtures/run-events-page.export-denied.json" with { type: "json" };
import report from "@workspace/contracts/fixtures/report-view.vendor.json" with { type: "json" };
import review from "@workspace/contracts/fixtures/review-view.queue-report.json" with { type: "json" };
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import request from "supertest";
import { z } from "zod";
import { createTestApp } from "../testing/create-test-app.js";
import { GatewayClientService } from "../gateway-client/gateway-client.service.js";
import { AUTH_PROVIDER } from "../auth/auth.types.js";
import { Membership } from "../identity/entities/membership.entity.js";
import { ActionsController } from "../actions/actions.controller.js";
import { RunsController } from "./runs.controller.js";

// API-24, field minimization on the public activity views (critical check "Field minimization").
// The Go side is a labelled fixture: these specs prove what the API relays and refuses for the
// protected values of the synthetic policy fixture, not what Go decides.
const demoRecords = JSON.parse(
  readFileSync(new URL("../../../../fixtures/demo-records.json", import.meta.url), "utf8"),
) as {
  invoices: { id: string; internal_note: string | null }[];
  vendors: { id: string; registered_reporting_address: string | null }[];
};
const internalNote = demoRecords.invoices.find((invoice) => invoice.internal_note)!.internal_note!;
const registeredAddress = demoRecords.vendors.find(
  (vendor) => vendor.registered_reporting_address,
)!.registered_reporting_address!;
// Whole values plus distinctive fragments, so a re-worded or truncated leak is still found.
const PROTECTED_VALUES = [
  internalNote,
  "Investigation note",
  "Accounts payable has not confirmed",
  registeredAddress,
];

/** The protected values found in a serialized body (empty when it is clean). */
function leakedValues(serialized: string): string[] {
  return PROTECTED_VALUES.filter((value) => serialized.toLowerCase().includes(value.toLowerCase()));
}

describe("Field minimization on the activity views", () => {
  let app: NestExpressApplication;
  let roles: string[];
  let organizationId: string;
  let upstream: unknown;
  let upstreamFailure: { statusCode: number; code: string } | undefined;
  const runId = state.runId;
  const ownerOrganization = events.events[0]!.organizationId;
  const getRead = vi.fn((_path: string, _requestId: string, schema: z.ZodTypeAny) => {
    if (upstreamFailure) {
      return { success: false, reason: "bad_request", ...upstreamFailure };
    }
    const parsed = schema.safeParse(upstream);
    return parsed.success
      ? { success: true, data: parsed.data }
      : { success: false, reason: "invalid_response" };
  });
  const context = () => ({
    userId: "c28e2545-2de6-41b9-9be6-d21ae01e4901",
    organizationId,
    roles,
  });

  beforeAll(async () => {
    app = await createTestApp(
      {
        controllers: [RunsController, ActionsController],
        providers: [
          { provide: GatewayClientService, useValue: { getRead } },
          {
            provide: AUTH_PROVIDER,
            useValue: { authenticate: () => Promise.resolve({ subjectId: context().userId }) },
          },
          { provide: getRepositoryToken(Membership), useValue: { findOne: () => context() } },
        ],
      },
      { mockAuth: false },
    );
  });
  afterAll(async () => {
    await app.close();
  });
  beforeEach(() => {
    roles = ["operator"];
    organizationId = ownerOrganization;
    upstream = undefined;
    upstreamFailure = undefined;
    getRead.mockClear();
  });

  const get = (path: string) => request(app.getHttpServer()).get(path).set("Cookie", "session=x");

  it("serves a full run's views with no protected value", async () => {
    // Each labelled fixture belongs to its own run and organization.
    const views: [string, string, unknown][] = [
      [`/api/runs/${state.runId}`, ownerOrganization, state],
      [`/api/runs/${usage.runId}/usage`, ownerOrganization, usage],
      [`/api/runs/${events.events[0]!.runId}/events`, events.events[0]!.organizationId, events],
      [`/api/runs/${report.runId}/reports/${report.reportId}`, ownerOrganization, report],
    ];
    for (const [path, viewOrganizationId, body] of views) {
      organizationId = viewOrganizationId;
      upstream = structuredClone(body);
      const response = await get(path).expect(200);
      expect(leakedValues(JSON.stringify(response.body)), path).toEqual([]);
      expect(response.body).toEqual(body);
    }
  });

  it("finds a protected value when one is present (the scan is not vacuous)", () => {
    expect(leakedValues(JSON.stringify({ safeMessage: internalNote }))).toContain(internalNote);
    expect(leakedValues("a re-worded Accounts payable has not confirmed it")).toEqual([
      "Accounts payable has not confirmed",
    ]);
    expect(leakedValues(`sent to ${registeredAddress.toUpperCase()}`)).toEqual([registeredAddress]);
    expect(leakedValues(JSON.stringify(report))).toEqual([]);
  });

  it("refuses an upstream view that carries a protected field or key, without echoing it", async () => {
    const hostile: [string, unknown][] = [
      [`/api/runs/${runId}`, { ...state, internalNote }],
      [`/api/runs/${runId}/usage`, { ...usage, internalNote }],
      [
        `/api/runs/${events.events[0]!.runId}/events`,
        {
          ...events,
          events: events.events.map((event) => ({
            ...event,
            maskedSummary: { ...event.maskedSummary, internalNote },
          })),
        },
      ],
      [
        `/api/runs/${report.runId}/reports/${report.reportId}`,
        { ...report, registeredReportingAddress: registeredAddress },
      ],
    ];
    for (const [path, body] of hostile) {
      upstream = body;
      const response = await get(path);
      expect(response.status, path).toBeGreaterThanOrEqual(500);
      expect(leakedValues(response.text), path).toEqual([]);
    }
  });

  it("never relays an upstream error code or message that carries a protected value", async () => {
    upstreamFailure = { statusCode: 422, code: internalNote };
    const paths = [
      `/api/runs/${runId}`,
      `/api/runs/${runId}/usage`,
      `/api/runs/${events.events[0]!.runId}/events`,
      `/api/runs/${report.runId}/reports/${report.reportId}`,
    ];
    for (const path of paths) {
      const response = await get(path);
      expect(response.status, path).toBeGreaterThanOrEqual(400);
      expect(leakedValues(response.text), path).toEqual([]);
    }
  });

  it("carries the exact review content only on the reviewer read", async () => {
    upstream = structuredClone(review);
    const path = `/api/actions/${review.action_id}/review`;

    const refused = await get(path).expect(403);
    expect(getRead).not.toHaveBeenCalled();
    expect(refused.text).not.toContain(review.recipient.address);
    expect(refused.text).not.toContain(review.report.content_hash);

    roles = ["operator", "reviewer"];
    const served = (await get(path).expect(200)).body as typeof review;
    expect(served.recipient.address).toBe(review.recipient.address);
    expect(served.canonical_arguments).toEqual(review.canonical_arguments);
    expect(getRead).toHaveBeenCalledTimes(1);

    // The same address is not on any other activity view of the run.
    for (const body of [state, usage, events, report]) {
      expect(JSON.stringify(body)).not.toContain(review.recipient.address);
    }
  });
});
