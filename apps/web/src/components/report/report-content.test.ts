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

describe("classification and source trail", () => {
  it("shows the stored classification of a vendor report and of each source", () => {
    const html = render(vendorReport);
    expect(html).toContain("Vendor shareable");
    expect(html).not.toContain("Internal only");
    expect(html).toContain('data-classification="vendor_shareable"');
  });

  it("shows an Internal only report with its label even when its content is withheld", () => {
    const html = render(withheldReport);
    expect(html).toContain('data-classification="internal_only"');
    expect(html).toContain("Internal only");
    expect(html).toContain("Content withheld");
  });

  it("lists every source with its version, stored classification and consumed fields", () => {
    const html = render(withheldReport);
    for (const source of withheldReport.lineage) {
      expect(html).toContain(`${source.sourceKind} ${source.sourceId}`);
      expect(html).toContain(source.consumedFields.join(", "));
    }
    // The first source carries the internal note and so the restriction; the second does not.
    expect(html).toContain("internal_note");
    expect(html).toContain('data-classification="vendor_shareable"');
  });

  it("shows template and projection versions, the destination class and the content hash", () => {
    const html = render(vendorReport);
    expect(html).toContain("vendor_invoice_fields_v1 (version 1)");
    expect(html).toContain("Registered vendor recipient");
    expect(html).toContain(vendorReport.contentHash);
    const internalHtml = render(withheldReport);
    expect(internalHtml).toContain("None (the internal template reads sources directly)");
    expect(internalHtml).toContain("Internal reviewers");
  });

  it("reads the label from the stored field and never computes it from sources or the title", () => {
    // A title that claims another label and a source trail that disagrees change nothing.
    const html = render({
      ...vendorReport,
      title: "Internal only: renamed report",
      lineage: withheldReport.lineage,
    });
    const badge = html.match(/data-classification="([a-z_]+)"[^>]*>([^<]+)</);
    expect(badge?.[1]).toBe("vendor_shareable");
    expect(badge?.[2]).toBe("Vendor shareable");
  });
});

describe("describeReportFailure", () => {
  it("says a missing report was not found, without offering a retry", () => {
    const failure = describeReportFailure({ kind: "http", status: 404, body: undefined });
    expect(failure.title).toBe("Report not found");
    expect(failure.action).toBe("none");
  });

  it("shows a report outside its registered template as an error, not text", () => {
    const failure = describeReportFailure({ kind: "invalid_json", status: 200 });
    expect(failure.title).toBe("Report cannot be shown");
    expect(failure.description).toContain("registered template");
  });

  it("uses the shared states for every other failure", () => {
    const offline = describeReportFailure({ kind: "network", message: "ECONNREFUSED 10.0.0.1" });
    expect(offline.title).toBe("You appear to be offline");
    expect(offline.description).not.toContain("10.0.0.1");
    expect(describeReportFailure({ kind: "http", status: 401, body: undefined }).action).toBe(
      "sign_in",
    );
    const apiDown = describeReportFailure({
      kind: "http",
      status: 502,
      body: { error: { code: "upstream_unreachable" } },
    });
    expect(apiDown).toMatchObject({ kind: "upstream_unreachable", scope: "api" });
  });
});
