import { describe, expect, it } from "vitest";

import { safeCallbackPath } from "./safe-callback";

describe("safeCallbackPath", () => {
  it("follows a path inside the app, with its query", () => {
    expect(safeCallbackPath("/tasks/new")).toBe("/tasks/new");
    expect(safeCallbackPath("/runs/run_1?tab=events")).toBe("/runs/run_1?tab=events");
  });

  it("falls back to the home page for anything that could leave the app", () => {
    for (const hostile of [
      "https://evil.example/login",
      "//evil.example",
      "/\\evil.example",
      "javascript:alert(1)",
      "evil.example",
      "/ok\nLocation: https://evil.example",
    ]) {
      expect(safeCallbackPath(hostile)).toBe("/");
    }
  });

  it("falls back when there is no callback, and does not loop back to sign-in", () => {
    expect(safeCallbackPath(null)).toBe("/");
    expect(safeCallbackPath(undefined)).toBe("/");
    expect(safeCallbackPath("")).toBe("/");
    expect(safeCallbackPath("/login")).toBe("/");
    expect(safeCallbackPath("/login?callbackUrl=%2F")).toBe("/");
  });
});
