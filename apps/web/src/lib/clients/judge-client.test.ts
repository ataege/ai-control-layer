import { describe, expect, it, vi, afterEach } from "vitest";
import type { ControlEvaluationResponse } from "@workspace/contracts";
import { postJson, type FetchJsonResult } from "../fetch-json";
import {
  JudgeClient,
  buildEvaluationRequest,
  interpretEvaluation,
  type JudgeFormInput,
} from "./judge-client";

vi.mock("../fetch-json", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../fetch-json")>();
  return { ...actual, postJson: vi.fn() };
});

const RUN_ID = "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b";

const baseInput: JudgeFormInput = {
  runId: RUN_ID,
  kind: "model_input",
  text: "Summarize the duplicate references.",
  tool: "",
  argumentsJson: "{}",
};

const decision: ControlEvaluationResponse = {
  evaluationId: "7b8c9d0e-1f2a-4b3c-9d4e-5f6a7b8c9d0e",
  runId: RUN_ID,
  actionId: null,
  decision: "deny",
  reasonCode: "run_not_active",
  safeMessage: "The run is no longer active.",
  alternativeTemplate: null,
  controls: [],
  semantic: null,
  content: null,
  catalog: { admissionRevisionId: 1, activeRevisionId: 1, feedRevisionId: null },
};

const httpFailure = (status: number, body?: unknown): FetchJsonResult<unknown> => ({
  ok: false,
  error: { kind: "http", status, body },
  durationMs: 5,
});

describe("buildEvaluationRequest", () => {
  it("sends model_input with null tool and arguments", () => {
    expect(buildEvaluationRequest(baseInput)).toEqual({
      ok: true,
      request: {
        runId: RUN_ID,
        kind: "model_input",
        text: baseInput.text,
        tool: null,
        arguments: null,
      },
    });
  });

  it("accepts multibyte text of exactly 4096 bytes and says too long beyond it", () => {
    expect(buildEvaluationRequest({ ...baseInput, text: "é".repeat(2048) }).ok).toBe(true);
    const refused = buildEvaluationRequest({ ...baseInput, text: "é".repeat(2049) });
    expect(!refused.ok && refused.message).toContain("too long");
  });

  it("drops a stale tool from a model_input request", () => {
    const built = buildEvaluationRequest({ ...baseInput, tool: "read_invoice" });
    expect(built.ok && built.request.tool).toBeNull();
  });

  it("sends tool_result with its tool and null arguments", () => {
    const built = buildEvaluationRequest({
      ...baseInput,
      kind: "tool_result",
      tool: "read_vendor",
    });
    expect(built).toEqual({
      ok: true,
      request: {
        runId: RUN_ID,
        kind: "tool_result",
        text: baseInput.text,
        tool: "read_vendor",
        arguments: null,
      },
    });
  });

  it("sends action_proposal with null text and parsed arguments", () => {
    const built = buildEvaluationRequest({
      ...baseInput,
      kind: "action_proposal",
      text: "ignored leftover text",
      tool: "read_invoice",
      argumentsJson: '{"invoice_id":"invoice_B01"}',
    });
    expect(built).toEqual({
      ok: true,
      request: {
        runId: RUN_ID,
        kind: "action_proposal",
        text: null,
        tool: "read_invoice",
        arguments: { invoice_id: "invoice_B01" },
      },
    });
  });

  it.each([
    ["a run id that is not a UUID", { runId: "run-1" }],
    ["empty text", { text: "" }],
    ["text over the limit", { text: "x".repeat(4097) }],
    ["multibyte text within 4096 characters but over 4096 bytes", { text: "é".repeat(2049) }],
    [
      "action arguments over the size limit",
      {
        kind: "action_proposal" as const,
        tool: "read_invoice" as const,
        argumentsJson: JSON.stringify({ note: "x".repeat(4100) }),
      },
    ],
    ["a tool_result without a tool", { kind: "tool_result" as const }],
    [
      "an action_proposal without a tool",
      { kind: "action_proposal" as const, argumentsJson: "{}" },
    ],
    [
      "action arguments that are not JSON",
      { kind: "action_proposal" as const, tool: "read_invoice" as const, argumentsJson: "{nope" },
    ],
    [
      "action arguments that are an array",
      { kind: "action_proposal" as const, tool: "read_invoice" as const, argumentsJson: "[1]" },
    ],
    [
      "action arguments that are null",
      { kind: "action_proposal" as const, tool: "read_invoice" as const, argumentsJson: "null" },
    ],
  ])("refuses %s", (_label, override) => {
    expect(buildEvaluationRequest({ ...baseInput, ...override }).ok).toBe(false);
  });
});

describe("interpretEvaluation", () => {
  it("treats a run_not_active deny as a decision, not a failure", () => {
    const outcome = interpretEvaluation({ ok: true, status: 200, data: decision, durationMs: 9 });
    expect(outcome.ok && outcome.response.reasonCode).toBe("run_not_active");
  });

  it.each([
    [400, "bad_request"],
    [401, "unauthorized"],
    [404, "run_not_found"],
  ])("maps HTTP %i to %s", (status, kind) => {
    const outcome = interpretEvaluation(httpFailure(status));
    expect(!outcome.ok && outcome.failure.kind).toBe(kind);
  });

  it("maps the proxy's configuration_error to not_routed", () => {
    const outcome = interpretEvaluation(
      httpFailure(500, { error: { code: "configuration_error" } }),
    );
    expect(!outcome.ok && outcome.failure.kind).toBe("not_routed");
  });

  it("maps 503 to unavailable and keeps the status", () => {
    const outcome = interpretEvaluation(httpFailure(503));
    expect(!outcome.ok && outcome.failure).toEqual({ kind: "unavailable", status: 503 });
  });

  it("never shows a malformed 2xx body as a decision", () => {
    for (const data of [
      { decision: "allow" },
      { ...decision, actionId: "act-1" },
      { ...decision, decision: "grant" },
    ]) {
      const outcome = interpretEvaluation({ ok: true, status: 200, data, durationMs: 1 });
      expect(!outcome.ok && outcome.failure.kind).toBe("invalid_response");
    }
  });

  it("maps a timeout and a network error to their own states", () => {
    const timeout = interpretEvaluation({
      ok: false,
      error: { kind: "timeout", timeoutMs: 1 },
      durationMs: 1,
    });
    const network = interpretEvaluation({
      ok: false,
      error: { kind: "network", message: "down" },
      durationMs: 1,
    });
    expect(!timeout.ok && timeout.failure.kind).toBe("timeout");
    expect(!network.ok && network.failure.kind).toBe("network");
  });
});

describe("JudgeClient.evaluate", () => {
  afterEach(() => vi.clearAllMocks());

  it("posts the request to the same-origin evaluate route", async () => {
    vi.mocked(postJson).mockResolvedValueOnce({
      ok: true,
      status: 200,
      data: decision,
      durationMs: 12,
      requestId: "req-1",
    });
    const request = {
      runId: RUN_ID,
      kind: "model_input" as const,
      text: "hi",
      tool: null,
      arguments: null,
    };
    const outcome = await JudgeClient.evaluate(request);
    expect(postJson).toHaveBeenCalledWith("/api/control/evaluate", request, expect.any(Object));
    expect(outcome.ok && outcome.requestId).toBe("req-1");
  });
});
