import { NextRequest } from "next/server";
import { describe, expect, it } from "vitest";

import { middleware } from "./middleware";

const requestFor = (pathname: string, cookie?: string) =>
  new NextRequest(`http://localhost:3000${pathname}`, cookie ? { headers: { cookie } } : undefined);

const redirectsToLogin = (pathname: string, cookie?: string) => {
  const response = middleware(requestFor(pathname, cookie));
  const location = response.headers.get("location");
  return response.status === 307 && location !== null && new URL(location).pathname === "/login";
};

describe("the sign-in gate", () => {
  it("sends a visitor without a session from a product page to /login, keeping where they were", () => {
    for (const pathname of ["/", "/tasks/new", "/security", "/runs/run_1", "/judge"]) {
      expect(redirectsToLogin(pathname)).toBe(true);
    }
    const location = middleware(requestFor("/tasks/new")).headers.get("location") ?? "";
    expect(new URL(location).searchParams.get("callbackUrl")).toBe("/tasks/new");
  });

  it("does not let a name that merely starts with a public path through", () => {
    for (const pathname of ["/loginx", "/apix", "/healthx", "/diagnosticsx", "/_nextx"]) {
      expect(redirectsToLogin(pathname)).toBe(true);
    }
  });

  it("serves the public paths, and what is below them, without a session", () => {
    for (const pathname of [
      "/login",
      "/diagnostics",
      "/diagnostics/x",
      "/api/auth/sign-in",
      "/health/live",
    ]) {
      expect(redirectsToLogin(pathname)).toBe(false);
    }
  });

  it("lets a request with a session cookie through (the API decides whether it is valid)", () => {
    expect(redirectsToLogin("/", "session=anything")).toBe(false);
  });
});
