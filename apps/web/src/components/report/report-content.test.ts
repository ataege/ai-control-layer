import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ReportView } from "@workspace/contracts";
import vendorFixture from "@workspace/contracts/fixtures/report-view.vendor.json";
import withheldFixture from "@workspace/contracts/fixtures/report-view.internal-withheld.json";
import { describeReportFailure } from "./report-errors";
import { ReportContent } from "./report-content";

const vendorReport = vendorFixture as ReportView;
const withheldReport = withheldFixture as ReportView;

function render(report: ReportView): string {
  return renderToStaticMarkup(createElement(ReportContent, { report }));
}

describe("ReportContent", () => {
  it("renders the stored content, title, identity and template of a vendor report", () => {
    const html = render(vendorReport);
    expect(html).toContain("Vendor reconciliation");
    expect(html).toContain("external reference INV104");
    expect(html).toContain("duplicate reference: yes");
    expect(html).toContain(vendorReport.reportId);
    expect(html).toContain("vendor_reconciliation_v1 (version 1)");
    expect(html).not.toContain("Content withheld");
  });

  it("shows withheld content as withheld and fills nothing in", () => {
    const html = render(withheldReport);
    expect(html).toContain("Content withheld");
    expect(html).toContain('data-testid="content-withheld"');
    expect(html).not.toContain('data-testid="report-content"');
  });

  it("never renders content that is flagged withheld, even if a body is present", () => {
    const tampered = { ...withheldReport, content: "SECRET INTERNAL NOTE" } as ReportView;
    const html = render(tampered);
    expect(html).toContain("Content withheld");
    expect(html).not.toContain("SECRET INTERNAL NOTE");
  });

  it("renders content as text, never as markup", () => {
    const html = render({ ...vendorReport, content: "<script>alert(1)</script>" });
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });
});

describe("describeReportFailure", () => {
  it("says a missing report was not found", () => {
    expect(describeReportFailure({ kind: "http", status: 404, body: undefined }).title).toBe(
      "Report not found",
    );
  });

  it("shows a report outside its registered template as an error, not text", () => {
    const failure = describeReportFailure({ kind: "invalid_json", status: 200 });
    expect(failure.title).toBe("Report cannot be shown");
    expect(failure.description).toContain("registered template");
  });

  it("names a network failure without exposing its detail", () => {
    const failure = describeReportFailure({ kind: "network", message: "ECONNREFUSED 10.0.0.1" });
    expect(failure.title).toBe("Report unavailable");
    expect(failure.description).not.toContain("10.0.0.1");
  });
});
