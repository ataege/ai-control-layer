import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { JudgeConsole } from "./judge-console";

describe("JudgeConsole text", () => {
  const html = renderToStaticMarkup(createElement(JudgeConsole));

  it("says a judge run has a passport but no agent and that evaluations spend its security allowance", () => {
    const text = html.replaceAll(/<[^>]+>/g, " ").replaceAll(/\s+/g, " ");
    expect(text).toContain("A judge run has a passport but no agent");
    expect(text).toContain("no model call is made for it");
    expect(text).toContain("Evaluations spend its security allowance");
  });

  it("keeps the decision-only label", () => {
    expect(html).toContain(
      "Judge evaluation: decision only; nothing is stored as an action or executed.",
    );
  });

  it("no longer claims the agent model is merely not dispatched on an ordinary run", () => {
    expect(html).not.toContain("The agent model is never dispatched");
  });
});
