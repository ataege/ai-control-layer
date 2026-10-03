import { describe, expect, it, vi } from "vitest";
import type { ReportView } from "@workspace/contracts";
import vendorFixture from "@workspace/contracts/fixtures/report-view.vendor.json";
import withheldFixture from "@workspace/contracts/fixtures/report-view.internal-withheld.json";
import { getReport, isReportView, REGISTERED_TEMPLATES } from "./reports-client";

const vendorReport = vendorFixture as ReportView;
const withheldReport = withheldFixture as ReportView;

function respond(status: number, body: unknown): typeof fetch {
  return vi.fn(
    async () => new Response(JSON.stringify(body), { status }),
  ) as unknown as typeof fetch;
}

describe("isReportView", () => {
  it("accepts the shared fixtures, the vendor report and the withheld Internal only one", () => {
    expect(isReportView(vendorReport)).toBe(true);
    expect(isReportView(withheldReport)).toBe(true);
  });

  it("registers exactly the two report templates", () => {
    expect([...REGISTERED_TEMPLATES]).toEqual([
      "internal_investigation_v1",
      "vendor_reconciliation_v1",
    ]);
  });

  it("refuses a report outside the registered templates", () => {
    expect(isReportView({ ...vendorReport, template: "free_text_summary_v1" })).toBe(false);
  });

  it("refuses content that disagrees with the withheld flag", () => {
    expect(isReportView({ ...vendorReport, contentWithheld: true })).toBe(false);
    expect(isReportView({ ...vendorReport, content: null })).toBe(false);
    expect(isReportView({ ...withheldReport, content: "filled in" })).toBe(false);
  });

  it("refuses a report without a source trail, with a bad hash or with a computed label", () => {
    expect(isReportView({ ...vendorReport, lineage: [] })).toBe(false);
    expect(isReportView({ ...vendorReport, contentHash: "abc" })).toBe(false);
    expect(isReportView({ ...vendorReport, classification: "public" })).toBe(false);
    expect(isReportView({ ...vendorReport, lineage: [{ sourceKind: "email" }] })).toBe(false);
  });

  it("refuses values that are not objects", () => {
    for (const value of [null, undefined, "report", 7, [vendorReport]]) {
      expect(isReportView(value)).toBe(false);
    }
  });
});

describe("getReport", () => {
  it("reads one report through the same-origin route with both ids encoded", async () => {
    const fetchImplementation = respond(200, vendorReport);
    const result = await getReport("run 1", "report/2", { fetchImplementation });
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.data).toEqual(vendorReport);
    expect(vi.mocked(fetchImplementation).mock.calls[0]?.[0]).toBe(
      "/api/runs/run%201/reports/report%2F2",
    );
  });

  it("turns a body that is not a stored report into an invalid_json failure", async () => {
    const result = await getReport("r", "p", {
      fetchImplementation: respond(200, { ...vendorReport, template: "other" }),
    });
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toMatchObject({ kind: "invalid_json", status: 200 });
  });

  it("passes the API's refusal through, for example another organization's report", async () => {
    const result = await getReport("r", "p", {
      fetchImplementation: respond(404, {
        error: { code: "not_found", message: "Report not found." },
      }),
    });
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toMatchObject({ kind: "http", status: 404 });
  });
});
