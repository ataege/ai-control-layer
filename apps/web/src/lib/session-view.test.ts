import { describe, expect, it } from "vitest";

import { isPublicPath, mustSignIn, sessionOutcome, type OperatorSession } from "./session-view";

const operator: OperatorSession = {
  name: "Development Demonstration Operator",
  email: "demo-operator@example.com",
  organizationId: "org_1",
};
const meta = { durationMs: 1 };

describe("sessionOutcome", () => {
  it("is signed in with the operator the server returned", () => {
    expect(sessionOutcome({ ...meta, ok: true, status: 200, data: operator })).toEqual({
      status: "signed_in",
      operator,
    });
  });

  it("is signed out only when the server answered 401", () => {
    expect(
      sessionOutcome({ ...meta, ok: false, error: { kind: "http", status: 401, body: undefined } }),
    ).toEqual({ status: "signed_out" });
  });

  it("is unavailable, not signed out, when the session could not be read", () => {
    for (const error of [
      { kind: "http" as const, status: 404, body: undefined },
      { kind: "http" as const, status: 500, body: undefined },
      { kind: "network" as const, message: "offline" },
      { kind: "timeout" as const, timeoutMs: 1 },
      { kind: "invalid_json" as const, status: 200 },
    ]) {
      expect(sessionOutcome({ ...meta, ok: false, error })).toEqual({ status: "unavailable" });
    }
  });
});

describe("mustSignIn", () => {
  it("sends a signed-out visitor from a product page to sign in", () => {
    for (const pathname of [
      "/",
      "/tasks/new",
      "/components",
      "/runs/run_1",
      "/judge",
      "/security",
    ]) {
      expect(mustSignIn({ status: "signed_out" }, pathname)).toBe(true);
    }
  });

  it("leaves the public pages alone, and never signs out an unreadable session", () => {
    expect(mustSignIn({ status: "signed_out" }, "/diagnostics")).toBe(false);
    expect(mustSignIn({ status: "signed_out" }, "/login")).toBe(false);
    expect(mustSignIn({ status: "unavailable" }, "/")).toBe(false);
    expect(mustSignIn({ status: "signed_in", operator }, "/")).toBe(false);
  });

  it("matches the middleware's public paths exactly, so /components is not public", () => {
    expect(isPublicPath("/components")).toBe(false);
    expect(isPublicPath("/diagnostics/extra")).toBe(true);
    expect(isPublicPath("/diagnosticsx")).toBe(false);
  });
});
