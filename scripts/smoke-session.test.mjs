// Tests of the smoke test's session helpers: picking the session cookie out of Set-Cookie headers and
// recognising the middleware's redirect to the sign-in page.
import assert from "node:assert/strict";
import { test } from "node:test";

import { isRedirectToLogin, sessionCookieHeader } from "./lib/smoke-session.mjs";

test("takes the session cookie's name and value and drops its attributes", () => {
  assert.equal(
    sessionCookieHeader([
      "session=abc123; Path=/; Expires=Tue, 01 Jan 2030 00:00:00 GMT; HttpOnly",
    ]),
    "session=abc123",
  );
});

test("finds the session cookie among other cookies", () => {
  assert.equal(
    sessionCookieHeader(["theme=dark; Path=/", "session=abc123; Path=/; HttpOnly"]),
    "session=abc123",
  );
});

test("returns null when no session is set, or it is cleared", () => {
  assert.equal(sessionCookieHeader([]), null);
  assert.equal(sessionCookieHeader(undefined), null);
  assert.equal(sessionCookieHeader(["theme=dark"]), null);
  assert.equal(
    sessionCookieHeader(["session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT"]),
    null,
  );
  assert.equal(sessionCookieHeader(["sessionx=abc"]), null);
});

test("keeps an equals sign inside the cookie value", () => {
  assert.equal(sessionCookieHeader(["session=ab=cd; Path=/"]), "session=ab=cd");
});

test("recognises a redirect to the sign-in page, with or without the callback query", () => {
  assert.equal(isRedirectToLogin(307, "http://localhost:3000/login?callbackUrl=%2F"), true);
  assert.equal(isRedirectToLogin(307, "/login"), true);
  assert.equal(isRedirectToLogin(302, "/login"), true);
});

test("does not accept a redirect elsewhere, a missing location or a non-redirect", () => {
  assert.equal(isRedirectToLogin(307, "http://localhost:3000/elsewhere"), false);
  assert.equal(isRedirectToLogin(307, "/login-help"), false);
  assert.equal(isRedirectToLogin(307, null), false);
  assert.equal(isRedirectToLogin(200, "/login"), false);
});
