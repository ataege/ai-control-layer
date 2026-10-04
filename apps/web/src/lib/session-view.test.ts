import { describe, expect, it } from "vitest";

import {
  isPublicPath,
  mustSignIn,
  sessionOutcome,
  signInFailureMessage,
  type OperatorSession,
} from "./session-view";

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
      "/security",
      "/security/export",
      "/runs/run_1",
      "/judge",
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
    expect(isPublicPath("/security")).toBe(false);
    expect(isPublicPath("/diagnostics/extra")).toBe(true);
    expect(isPublicPath("/diagnosticsx")).toBe(false);
  });
});

describe("signInFailureMessage", () => {
  it("says the credentials were wrong for a 401, never that a session ended", () => {
    expect(signInFailureMessage({ kind: "http", status: 401, body: undefined })).toBe(
      "The email or password is not correct.",
    );
  });

  it("uses the shared safe message for every other failure and never a server message", () => {
    const message = signInFailureMessage({
      kind: "http",
      status: 500,
      body: { error: { code: "internal_error", message: "stack trace secret" } },
    });
    expect(message).not.toContain("secret");
    expect(signInFailureMessage({ kind: "network", message: "x" })).toMatch(
      /could not reach the web server/,
    );
  });
});
