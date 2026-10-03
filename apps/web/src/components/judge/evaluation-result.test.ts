import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ControlEvaluationResponse } from "@workspace/contracts";
import { EvaluationResult, FailureNotice } from "./evaluation-result";

const baseResponse: ControlEvaluationResponse = {
  evaluationId: "7b8c9d0e-1f2a-4b3c-9d4e-5f6a7b8c9d0e",
  runId: "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b",
  actionId: null,
  decision: "deny",
  reasonCode: "signature_match",
  safeMessage: "The content matched a known attack signature and was withheld.",
  alternativeTemplate: null,
  controls: [
    {
      boundary: "tool_result",
      controlClass: "deterministic",
      control: "signature_match",
      outcome: "block",
      reasonCode: "signature_match",
      ruleId: "prompt_ignore_previous_v1",
      feedRevision: "feed_v1",
    },
  ],
  semantic: null,
  content: null,
  catalog: { admissionRevisionId: 1, activeRevisionId: 1, feedRevisionId: 1 },
};

const render = (response: ControlEvaluationResponse) =>
  renderToStaticMarkup(createElement(EvaluationResult, { response, durationMs: 42 }));

describe("EvaluationResult", () => {
  it("shows the decision, reason, rule, evaluation id and the client-side timing label", () => {
    const html = render(baseResponse);
    expect(html).toContain("deny");
    expect(html).toContain("prompt_ignore_previous_v1");
    expect(html).toContain(baseResponse.evaluationId);
    expect(html).toContain("client-measured");
    expect(html).toContain("did not run");
  });

  it("labels a fixture semantic verdict as not live detection", () => {
    const html = render({
      ...baseResponse,
      semantic: {
        source: "fixture",
        riskCategory: "none",
        score: 0.1,
        reasonCode: "no_risk_found",
      },
    });
    expect(html).toContain("fixture (not live detection)");
    expect(html).not.toContain("live model");
  });

  it("labels a live semantic verdict as live", () => {
    const html = render({
      ...baseResponse,
      semantic: { source: "live", riskCategory: "none", score: 0.1, reasonCode: "no_risk_found" },
    });
    expect(html).toContain("live model");
  });

  it("shows redacted text only when the server returned it, escaped", () => {
    expect(render(baseResponse)).not.toContain("Redacted text");
    const html = render({
      ...baseResponse,
      decision: "redact",
      content: { text: "Use account [REDACTED:bank_account] <b>x</b>" },
    });
    expect(html).toContain("[REDACTED:bank_account]");
    expect(html).toContain("&lt;b&gt;");
  });

  it("explains run_not_active as a decision about the run", () => {
    expect(render({ ...baseResponse, reasonCode: "run_not_active" })).toContain(
      "start a new dedicated judge run",
    );
  });
});

describe("FailureNotice", () => {
  it.each([
    ["bad_request", "400"],
    ["unauthorized", "401"],
    ["run_not_found", "404"],
  ] as const)("names the %s state", (kind, marker) => {
    expect(renderToStaticMarkup(createElement(FailureNotice, { failure: { kind } }))).toContain(
      marker,
    );
  });

  it("states that a 503 is never an allow and shows the status", () => {
    const html = renderToStaticMarkup(
      createElement(FailureNotice, { failure: { kind: "unavailable", status: 503 }, status: 503 }),
    );
    expect(html).toContain("never an allow");
    expect(html).toContain("HTTP 503");
  });

  it("links an unauthorized state to sign in", () => {
    const html = renderToStaticMarkup(
      createElement(FailureNotice, { failure: { kind: "unauthorized" } }),
    );
    expect(html).toContain('href="/login?callbackUrl=/judge"');
  });
});
