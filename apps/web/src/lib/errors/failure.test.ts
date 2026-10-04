import { describe, expect, it } from "vitest";
import reasonCodeSchema from "@workspace/contracts/schemas/reason-code.schema.json";
import type { FetchJsonError } from "../fetch-json";
import { classifyFailure, failureFromReason, type FailureKind } from "./failure";
import { isReasonCode, REASON_FAILURES } from "./reason-failures";

function http(status: number, code?: string, message?: string): FetchJsonError {
  return {
    kind: "http",
    status,
    body: code === undefined ? undefined : { error: { code, message: message ?? "" } },
  };
}

const REASON_CODES = reasonCodeSchema.enum as string[];

describe("reason failures", () => {
  it("has the words for every X-13 reason code and for nothing else", () => {
    expect(REASON_CODES).toHaveLength(31);
    expect(Object.keys(REASON_FAILURES).sort()).toEqual([...REASON_CODES].sort());
    for (const code of REASON_CODES) expect(isReasonCode(code)).toBe(true);
    expect(isReasonCode("made_up")).toBe(false);
    expect(isReasonCode("toString")).toBe(false);
  });

  it("never names a run status: one code can end a run in different states", () => {
    for (const [code, reason] of Object.entries(REASON_FAILURES)) {
      expect(reason.title, code).not.toBe("");
      expect(reason.message, code).not.toBe("");
      expect(reason.message.toLowerCase(), code).not.toMatch(/paused|stopped|failed|completed/);
    }
  });

  it("states an X-13 reason with its own title and no retry", () => {
    for (const code of REASON_CODES) {
      const failure = failureFromReason(code as never);
      expect(failure).toMatchObject({ kind: "reason", reasonCode: code, action: "none" });
    }
  });

  it("puts a provider failure on the live model, apart from the working gateway", () => {
    for (const code of ["outcome_unknown", "security_evaluator_unavailable", "model_not_allowed"]) {
      expect(failureFromReason(code as never).scope).toBe("live_model");
    }
    expect(failureFromReason("outcome_unknown").unconfirmed).toBe(true);
    expect(failureFromReason("decision_unavailable").scope).toBe("gateway");
    expect(failureFromReason("report_export_restricted").scope).toBe("request");
  });
});

describe("classifyFailure", () => {
  const cases: [string, FetchJsonError, FailureKind, string, string][] = [
    [
      "no response at all",
      { kind: "network", message: "Failed to fetch" },
      "offline",
      "browser",
      "retry",
    ],
    ["a client timeout", { kind: "timeout", timeoutMs: 15000 }, "timeout", "browser", "retry"],
    ["a cancelled request", { kind: "aborted" }, "cancelled", "browser", "retry"],
    [
      "a 2xx body that is not JSON",
      { kind: "invalid_json", status: 200 },
      "invalid_response",
      "web_server",
      "retry",
    ],
    ["an expired session", http(401, "unauthorized"), "session_expired", "session", "sign_in"],
    ["a 401 without a body", http(401), "session_expired", "session", "sign_in"],
    ["a forbidden request", http(403, "forbidden"), "forbidden", "request", "none"],
    ["a missing record", http(404, "not_found"), "not_found", "request", "none"],
    ["a conflict", http(409, "conflict"), "conflict", "request", "retry"],
    ["a bad request", http(400, "bad_request"), "bad_request", "request", "none"],
    ["an invalid input", http(400, "invalid_input"), "bad_request", "request", "none"],
    [
      "a payload that is too large",
      http(413, "payload_too_large"),
      "payload_too_large",
      "request",
      "none",
    ],
    [
      "the proxy without an API address",
      http(500, "configuration_error"),
      "configuration_error",
      "web_server",
      "none",
    ],
    [
      "an API that cannot be reached",
      http(502, "upstream_unreachable"),
      "upstream_unreachable",
      "api",
      "retry",
    ],
    [
      "an API answer that is not JSON",
      http(502, "upstream_invalid_response"),
      "upstream_invalid_response",
      "api",
      "retry",
    ],
    ["an API that is too slow", http(504, "upstream_timeout"), "upstream_timeout", "api", "retry"],
    [
      "a gateway the API cannot reach",
      http(503, "upstream_unavailable"),
      "upstream_unavailable",
      "gateway",
      "retry",
    ],
    [
      "a gateway call that failed",
      http(500, "server_error"),
      "upstream_unavailable",
      "gateway",
      "retry",
    ],
    [
      "a command with an unconfirmed outcome",
      http(504, "outcome_unconfirmed"),
      "outcome_unconfirmed",
      "gateway",
      "none",
    ],
    ["a server error", http(500, "internal_error"), "server_error", "api", "retry"],
    [
      "a feature that is not available",
      http(501, "not_implemented"),
      "not_implemented",
      "api",
      "none",
    ],
    ["an unknown 5xx", http(599), "server_error", "api", "retry"],
    ["an unknown 4xx", http(418), "unknown", "request", "retry"],
  ];

  it.each(cases)("gives %s its own state", (_name, error, kind, scope, action) => {
    expect(classifyFailure(error)).toMatchObject({ kind, scope, action });
  });

  it("reads an X-13 reason code from the API's error body", () => {
    const failure = classifyFailure(http(400, "limit_not_allowed"));
    expect(failure).toMatchObject({
      kind: "reason",
      reasonCode: "limit_not_allowed",
      statusCode: 400,
    });
    const provider = classifyFailure(http(409, "outcome_unknown"));
    expect(provider).toMatchObject({ scope: "live_model", unconfirmed: true });
  });

  it("gives every kind its own title and none reads as a success", () => {
    const titles = new Set<string>();
    for (const [, error, kind] of cases) {
      const failure = classifyFailure(error);
      expect(failure.kind).toBe(kind);
      titles.add(failure.title);
      expect(`${failure.title} ${failure.description}`).not.toMatch(
        /success|saved|started|created|queued|completed|all good/i,
      );
    }
    // Different kinds never share a state; "a 401 without a body" repeats the expired session.
    const distinctKinds = new Set(cases.map(([, , kind]) => kind));
    expect(titles.size).toBe(distinctKinds.size);
  });

  it("never shows a server message", () => {
    const failure = classifyFailure(http(404, "not_found", "secret-detail 10.0.0.1"));
    expect(`${failure.title} ${failure.description}`).not.toContain("secret-detail");
    const odd = classifyFailure({ kind: "network", message: "connect ECONNREFUSED 10.0.0.1" });
    expect(`${odd.title} ${odd.description}`).not.toContain("10.0.0.1");
  });

  it("makes a command that got no answer unconfirmed and offers no retry", () => {
    for (const error of [
      { kind: "network", message: "x" },
      { kind: "timeout", timeoutMs: 1 },
      http(504, "upstream_timeout"),
      http(500, "internal_error"),
    ] as FetchJsonError[]) {
      const read = classifyFailure(error);
      const command = classifyFailure(error, { command: true });
      expect(read.unconfirmed).toBe(false);
      expect(read.action).toBe("retry");
      expect(command.unconfirmed).toBe(true);
      expect(command.action).toBe("none");
      expect(command.description).toContain("check the run before submitting it again");
    }
  });

  it("leaves a command that certainly did not run as it is", () => {
    for (const error of [
      http(401, "unauthorized"),
      http(404, "not_found"),
      http(400, "bad_request"),
      http(502, "upstream_unreachable"),
      http(500, "configuration_error"),
    ]) {
      expect(classifyFailure(error, { command: true }).unconfirmed).toBe(false);
    }
    expect(classifyFailure(http(401, "unauthorized"), { command: true }).action).toBe("sign_in");
    expect(classifyFailure(http(502, "upstream_unreachable"), { command: true }).action).toBe(
      "retry",
    );
  });

  it("always reports the gateway's own unconfirmed answer as unconfirmed", () => {
    expect(classifyFailure(http(504, "outcome_unconfirmed")).unconfirmed).toBe(true);
  });

  it("lets a page word one kind itself without changing where it failed or what can be done", () => {
    const failure = classifyFailure(http(404, "not_found"), {
      overrides: { not_found: { title: "Report not found", description: "No such report here." } },
    });
    expect(failure).toMatchObject({
      kind: "not_found",
      title: "Report not found",
      description: "No such report here.",
      scope: "request",
      action: "none",
    });
  });
});
