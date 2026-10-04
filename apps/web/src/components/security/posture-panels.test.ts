import { readFileSync } from "node:fs";
import { createElement } from "react";
import { createRequire } from "node:module";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { SecuritySummary } from "@workspace/contracts";

import { HeadlineCards } from "./posture-panels";
import { buildPostureView } from "./posture-view";

function contractFixture(name: string): SecuritySummary {
  const path = createRequire(import.meta.url).resolve(`@workspace/contracts/fixtures/${name}`);
  return JSON.parse(readFileSync(path, "utf8")) as SecuritySummary;
}

const baseView = buildPostureView(contractFixture("security-summary.judge-split.json"));

function render(heldTokens: number): string {
  return renderToStaticMarkup(createElement(HeadlineCards, { view: { ...baseView, heldTokens } }));
}

describe("the reserved-token caption agrees with its count", () => {
  it("says '0 tokens stay', '1 token stays' and '2 tokens stay'", () => {
    expect(render(0)).toContain("0 tokens stay reserved");
    expect(render(1)).toContain("1 token stays reserved");
    expect(render(2)).toContain("2 tokens stay reserved");
  });

  it("keeps the thousands separator with the plural", () => {
    expect(render(1584)).toContain("1,584 tokens stay reserved");
  });
});
