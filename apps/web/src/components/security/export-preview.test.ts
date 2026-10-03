import { describe, expect, it } from "vitest";

import { previewOfPage } from "./export-preview";

describe("previewOfPage", () => {
  it("indents a JSON page for reading without changing its content", () => {
    const body = JSON.stringify({ events: [{ eventId: "1" }], nextCursor: "v1.0.2.1" });
    const preview = previewOfPage(body);
    expect(preview).toContain('\n  "events": [');
    expect(JSON.parse(preview)).toEqual(JSON.parse(body));
  });

  it("cuts a long page and says so", () => {
    const preview = previewOfPage(JSON.stringify({ rows: "x".repeat(5_000) }), 100);
    expect(preview.length).toBeLessThan(110);
    expect(preview.endsWith("\n…")).toBe(true);
  });

  it("shows a CSV body as it is", () => {
    const csv = '"eventId","eventType"\r\n"1","run.queued"\r\n';
    expect(previewOfPage(csv)).toBe(csv);
  });
});
