import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import { describe, expect, it } from "vitest";

import {
  buildExportQuery,
  exportFileName,
  failureForStatus,
  fetchExportPage,
  getCatalogStatus,
  getSecuritySummary,
  isCatalogStatus,
  isSecuritySummary,
  probeExportAuthorization,
  summarizeJsonPage,
  type ExportParameters,
} from "./security-client";

function contractFixture(name: string): unknown {
  const path = createRequire(import.meta.url).resolve(`@workspace/contracts/fixtures/${name}`);
  return JSON.parse(readFileSync(path, "utf8")) as unknown;
}

function stubFetch(
  status: number,
  body: string,
  headers: Record<string, string> = { "content-type": "application/json" },
): { implementation: typeof fetch; requests: { url: string; accept: string | null }[] } {
  const requests: { url: string; accept: string | null }[] = [];
  const implementation: typeof fetch = (input, init) => {
    requests.push({
      url: String(input),
      accept: new Headers(init?.headers).get("accept"),
    });
    return Promise.resolve(new Response(body, { status, headers }));
  };
  return { implementation, requests };
}

const failingFetch: typeof fetch = () => Promise.reject(new TypeError("connection refused"));

const BASE: ExportParameters = { kind: "events", format: "json", after: "", limit: "100" };

describe("isSecuritySummary", () => {
  const fixture = contractFixture("security-summary.judge-split.json") as Record<string, unknown>;

  it("accepts the summary the gateway really serves", () => {
    expect(isSecuritySummary(fixture)).toBe(true);
  });

  it("rejects a summary with a missing section, a bad count or the wrong type", () => {
    const withoutTimings = { ...fixture };
    delete withoutTimings.timings;
    expect(isSecuritySummary(withoutTimings)).toBe(false);
    expect(isSecuritySummary({ ...fixture, judgeSecurityCalls: -1 })).toBe(false);
    expect(isSecuritySummary({ ...fixture, runs: [{ status: "completed", count: "2" }] })).toBe(
      false,
    );
    expect(isSecuritySummary({ ...fixture, organizationId: 7 })).toBe(false);
    expect(isSecuritySummary(null)).toBe(false);
    expect(isSecuritySummary([fixture])).toBe(false);
  });
});

describe("getSecuritySummary", () => {
  const body = JSON.stringify(contractFixture("security-summary.judge-split.json"));

  it("returns the summary with the request id", async () => {
    const { implementation, requests } = stubFetch(200, body, {
      "content-type": "application/json",
      "x-request-id": "req-1",
    });
    const result = await getSecuritySummary({ fetchImplementation: implementation });
    expect(result.ok && result.requestId).toBe("req-1");
    expect(requests).toEqual([{ url: "/api/security/summary", accept: "application/json" }]);
  });

  it.each([
    [401, "unauthorized"],
    [403, "forbidden"],
    [503, "unavailable"],
    [502, "unavailable"],
  ])("maps HTTP %i to %s", async (status, kind) => {
    const { implementation } = stubFetch(status, JSON.stringify({ error: { code: "x" } }));
    const result = await getSecuritySummary({ fetchImplementation: implementation });
    expect(!result.ok && result.failure.kind).toBe(kind);
  });

  it("reports an unreachable server and a body that does not fit the contract", async () => {
    const network = await getSecuritySummary({ fetchImplementation: failingFetch });
    expect(!network.ok && network.failure.kind).toBe("network");
    const wrong = stubFetch(200, JSON.stringify({ organizationId: "x" }));
    const invalid = await getSecuritySummary({ fetchImplementation: wrong.implementation });
    expect(!invalid.ok && invalid.failure.kind).toBe("invalid_response");
    const notJson = stubFetch(200, "<html>");
    const html = await getSecuritySummary({ fetchImplementation: notJson.implementation });
    expect(!html.ok && html.failure.kind).toBe("invalid_response");
  });

  it("never puts upstream text into the failure message", async () => {
    const { implementation } = stubFetch(
      503,
      JSON.stringify({ error: { code: "unavailable", message: "SECRET-DETAIL" } }),
    );
    const result = await getSecuritySummary({ fetchImplementation: implementation });
    expect(JSON.stringify(result)).not.toContain("SECRET-DETAIL");
  });
});

describe("failureForStatus", () => {
  it("separates sign-in, reviewer and request problems", () => {
    expect(failureForStatus(401).kind).toBe("unauthorized");
    expect(failureForStatus(403).kind).toBe("forbidden");
    expect(failureForStatus(400).kind).toBe("bad_request");
    expect(failureForStatus(404).kind).toBe("not_available");
    expect(failureForStatus(500)).toMatchObject({ kind: "unavailable", status: 500 });
  });
});

describe("buildExportQuery", () => {
  it("builds the query the API accepts", () => {
    expect(buildExportQuery(BASE)).toEqual({
      ok: true,
      query: "kind=events&format=json&limit=100",
    });
    expect(
      buildExportQuery({
        ...BASE,
        kind: "assessments",
        format: "csv",
        after: "v1.0.17.42",
        limit: "500",
      }),
    ).toEqual({ ok: true, query: "kind=assessments&format=csv&after=v1.0.17.42&limit=500" });
  });

  it.each(["0", "501", "-1", "1.5", "abc", "", "1000"])("refuses the limit %j", (limit) => {
    expect(buildExportQuery({ ...BASE, limit })).toMatchObject({ ok: false, field: "limit" });
  });

  it.each(["42", "v2.0.1.2", "v1.0.1", "v1.a.b.c", "v1.0.0.0.0", "../x"])(
    "refuses the cursor %j",
    (after) => {
      expect(buildExportQuery({ ...BASE, after })).toMatchObject({ ok: false, field: "after" });
    },
  );

  it("reports every wrong field at once, so one never hides another", () => {
    const result = buildExportQuery({ ...BASE, limit: "0", after: "nonsense" });
    expect(result).toMatchObject({ ok: false, field: "limit" });
    expect(!result.ok && Object.keys(result.problems).sort()).toEqual(["after", "limit"]);
    const onlyCursor = buildExportQuery({ ...BASE, after: "nonsense" });
    expect(!onlyCursor.ok && onlyCursor.problems).toEqual({
      after: expect.stringContaining("Paste the cursor"),
    });
  });

  it("trims the typed values", () => {
    expect(buildExportQuery({ ...BASE, after: "  v1.0.5.3 ", limit: " 25 " })).toEqual({
      ok: true,
      query: "kind=events&format=json&after=v1.0.5.3&limit=25",
    });
  });
});

describe("exportFileName", () => {
  it("names the page by kind, position and format without dots in the cursor", () => {
    expect(exportFileName(BASE)).toBe("security-events-from-start.json");
    expect(
      exportFileName({ ...BASE, kind: "assessments", format: "csv", after: "v1.0.17.42" }),
    ).toBe("security-assessments-after-v1-0-17-42.csv");
  });
});

describe("probeExportAuthorization", () => {
  it("asks for one record and treats 200 as authorized", async () => {
    const { implementation, requests } = stubFetch(
      200,
      JSON.stringify({ events: [], nextCursor: "v1.0.0.0" }),
    );
    expect(await probeExportAuthorization({ fetchImplementation: implementation })).toBe(
      "authorized",
    );
    expect(requests[0]?.url).toBe("/api/security/export?kind=events&format=json&limit=1");
  });

  it("hides the control on 403 and does not authorize on any other failure", async () => {
    const forbidden = stubFetch(403, JSON.stringify({ error: { code: "forbidden" } }));
    expect(await probeExportAuthorization({ fetchImplementation: forbidden.implementation })).toBe(
      "forbidden",
    );
    const signedOut = stubFetch(401, JSON.stringify({ error: { code: "unauthorized" } }));
    expect(await probeExportAuthorization({ fetchImplementation: signedOut.implementation })).toBe(
      "unauthorized",
    );
    const broken = stubFetch(503, JSON.stringify({}));
    expect(await probeExportAuthorization({ fetchImplementation: broken.implementation })).toBe(
      "unavailable",
    );
    expect(await probeExportAuthorization({ fetchImplementation: failingFetch })).toBe(
      "unavailable",
    );
  });
});

describe("fetchExportPage", () => {
  const jsonBody = JSON.stringify({
    events: [{ eventId: "5" }, { eventId: "6" }],
    nextCursor: "v1.0.9.6",
  });

  it("keeps a JSON page's body exactly as served and reads its count and cursor", async () => {
    const { implementation, requests } = stubFetch(200, jsonBody, {
      "content-type": "application/json",
      "x-request-id": "req-2",
    });
    const result = await fetchExportPage(BASE, { fetchImplementation: implementation });
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.page).toMatchObject({
      body: jsonBody,
      recordCount: 2,
      nextCursor: "v1.0.9.6",
      requestId: "req-2",
    });
    expect(requests[0]).toEqual({
      url: "/api/security/export?kind=events&format=json&limit=100",
      accept: "application/json",
    });
  });

  it("reads assessment pages by their records key", async () => {
    const body = JSON.stringify({ records: [{ assessmentId: "1" }], nextCursor: "v1.0.3.1" });
    const { implementation } = stubFetch(200, body);
    const result = await fetchExportPage(
      { ...BASE, kind: "assessments" },
      { fetchImplementation: implementation },
    );
    expect(result.ok && result.page.recordCount).toBe(1);
  });

  it("refuses a JSON page of the wrong kind or without a cursor", async () => {
    const wrongKind = stubFetch(200, jsonBody);
    const mismatch = await fetchExportPage(
      { ...BASE, kind: "assessments" },
      { fetchImplementation: wrongKind.implementation },
    );
    expect(!mismatch.ok && mismatch.failure.kind).toBe("invalid_response");
    const noCursor = stubFetch(200, JSON.stringify({ events: [] }));
    const missing = await fetchExportPage(BASE, { fetchImplementation: noCursor.implementation });
    expect(!missing.ok && missing.failure.kind).toBe("invalid_response");
  });

  it("downloads CSV as served and checks its content type", async () => {
    const csv = '"eventId","eventType"\r\n"5","run.queued"\r\n';
    const good = stubFetch(200, csv, { "content-type": "text/csv; charset=utf-8" });
    const result = await fetchExportPage(
      { ...BASE, format: "csv" },
      { fetchImplementation: good.implementation },
    );
    expect(result.ok && result.page.body).toBe(csv);
    expect(good.requests[0]?.accept).toBe("text/csv");
    const notCsv = stubFetch(200, csv, { "content-type": "application/json" });
    const refused = await fetchExportPage(
      { ...BASE, format: "csv" },
      { fetchImplementation: notCsv.implementation },
    );
    expect(!refused.ok && refused.failure.kind).toBe("invalid_response");
  });

  it("answers 403 for a viewer without the reviewer role and never fetches on a bad query", async () => {
    const forbidden = stubFetch(403, JSON.stringify({ error: { code: "forbidden" } }));
    const result = await fetchExportPage(BASE, { fetchImplementation: forbidden.implementation });
    expect(!result.ok && result.failure.kind).toBe("forbidden");
    const spy = stubFetch(200, jsonBody);
    const bad = await fetchExportPage(
      { ...BASE, limit: "0" },
      { fetchImplementation: spy.implementation },
    );
    expect(!bad.ok && bad.failure.kind).toBe("bad_request");
    expect(spy.requests).toHaveLength(0);
  });

  it("reports an unreachable server", async () => {
    const result = await fetchExportPage(BASE, { fetchImplementation: failingFetch });
    expect(!result.ok && result.failure.kind).toBe("network");
  });
});

describe("summarizeJsonPage", () => {
  it("reads events and records, and rejects anything else", () => {
    expect(
      summarizeJsonPage(JSON.stringify({ events: [], nextCursor: "v1.0.0.0" }), "events"),
    ).toEqual({
      recordCount: 0,
      nextCursor: "v1.0.0.0",
    });
    expect(summarizeJsonPage("not json", "events")).toBeNull();
    expect(summarizeJsonPage(JSON.stringify([]), "events")).toBeNull();
    expect(summarizeJsonPage(JSON.stringify({ events: [], nextCursor: 1 }), "events")).toBeNull();
  });
});

describe("catalog status", () => {
  const activeFixture = contractFixture("catalog-status.active.json") as Record<string, unknown>;
  const rejectedFixture = contractFixture("catalog-status.rejected-request.json");
  const body = JSON.stringify(activeFixture);

  it("accepts both statuses the gateway really serves", () => {
    expect(isCatalogStatus(activeFixture)).toBe(true);
    expect(isCatalogStatus(rejectedFixture)).toBe(true);
  });

  it("rejects a status outside the contract", () => {
    expect(isCatalogStatus(null)).toBe(false);
    expect(isCatalogStatus({ ...activeFixture, controls: "none" })).toBe(false);
    expect(isCatalogStatus({ ...activeFixture, activeRevisionId: "1" })).toBe(false);
    expect(isCatalogStatus({ ...activeFixture, lastError: { code: "x" } })).toBe(false);
    expect(isCatalogStatus({ ...activeFixture, disabledRules: [1] })).toBe(false);
    const controls = activeFixture.controls as Record<string, unknown>[];
    expect(
      isCatalogStatus({ ...activeFixture, controls: [{ ...controls[0], controlId: "made_up" }] }),
    ).toBe(false);
    expect(
      isCatalogStatus({
        ...activeFixture,
        controls: [{ ...controls[0], boundaries: ["anywhere"] }],
      }),
    ).toBe(false);
    expect(
      isCatalogStatus({ ...activeFixture, controls: [{ ...controls[0], mode: "allow" }] }),
    ).toBe(false);
  });

  it("returns the status with its request id", async () => {
    const { implementation, requests } = stubFetch(200, body, {
      "content-type": "application/json",
      "x-request-id": "req-9",
    });
    const result = await getCatalogStatus({ fetchImplementation: implementation });
    expect(result.ok && result.requestId).toBe("req-9");
    expect(requests).toEqual([{ url: "/api/policies/catalog", accept: "application/json" }]);
  });

  it("says not available when the route is not served, and never calls that an outage", async () => {
    const missing = stubFetch(404, JSON.stringify({ error: { code: "not_found" } }));
    const notFound = await getCatalogStatus({ fetchImplementation: missing.implementation });
    expect(!notFound.ok && notFound.failure.kind).toBe("not_available");
    // The web proxy's own refusal of a path it does not forward (yet).
    const refused = stubFetch(
      500,
      JSON.stringify({ error: { code: "configuration_error", message: "x" } }),
    );
    const proxyRefusal = await getCatalogStatus({ fetchImplementation: refused.implementation });
    expect(!proxyRefusal.ok && proxyRefusal.failure.kind).toBe("not_available");
  });

  it("shows an unenforceable revision (503) and other server errors as unavailable", async () => {
    const down = stubFetch(503, JSON.stringify({ error: { code: "unavailable" } }));
    const unavailable = await getCatalogStatus({ fetchImplementation: down.implementation });
    expect(!unavailable.ok && unavailable.failure.kind).toBe("unavailable");
    const broken = stubFetch(500, JSON.stringify({ error: { code: "internal_error" } }));
    const other = await getCatalogStatus({ fetchImplementation: broken.implementation });
    expect(!other.ok && other.failure.kind).toBe("unavailable");
  });

  it("maps sign-in and role refusals and a body outside the contract", async () => {
    const signedOut = stubFetch(401, JSON.stringify({ error: { code: "unauthorized" } }));
    const unauthorized = await getCatalogStatus({ fetchImplementation: signedOut.implementation });
    expect(!unauthorized.ok && unauthorized.failure.kind).toBe("unauthorized");
    const forbidden = stubFetch(403, JSON.stringify({ error: { code: "forbidden" } }));
    const refusal = await getCatalogStatus({ fetchImplementation: forbidden.implementation });
    expect(!refusal.ok && refusal.failure.kind).toBe("forbidden");
    const wrong = stubFetch(200, JSON.stringify({ activeRevisionId: 1 }));
    const invalid = await getCatalogStatus({ fetchImplementation: wrong.implementation });
    expect(!invalid.ok && invalid.failure.kind).toBe("invalid_response");
    const network = await getCatalogStatus({ fetchImplementation: failingFetch });
    expect(!network.ok && network.failure.kind).toBe("network");
  });
});
