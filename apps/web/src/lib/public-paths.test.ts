import { describe, expect, it } from "vitest";

import { isPublicPath } from "./public-paths";

describe("isPublicPath", () => {
  it("serves each public prefix and what is below it", () => {
    for (const pathname of [
      "/login",
      "/login/",
      "/api/runs",
      "/api",
      "/_next/static/chunk.js",
      "/health/ready",
      "/diagnostics",
      "/diagnostics/gateway",
      "/favicon.ico",
    ]) {
      expect(isPublicPath(pathname)).toBe(true);
    }
  });

  it("does not serve a longer name that only starts with a public prefix", () => {
    for (const pathname of [
      "/loginx",
      "/apix",
      "/api-keys",
      "/healthx",
      "/diagnosticsx",
      "/_nextx",
      "/favicon.icox",
      "/login.html",
    ]) {
      expect(isPublicPath(pathname)).toBe(false);
    }
  });

  it("keeps every product page behind sign-in", () => {
    for (const pathname of [
      "/",
      "/tasks/new",
      "/security",
      "/security/export",
      "/runs/run_1",
      "/judge",
    ]) {
      expect(isPublicPath(pathname)).toBe(false);
    }
  });
});
