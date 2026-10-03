// Helpers for the smoke test's signed-in checks. The session cookie is a credential, so nothing here
// prints it; callers only pass it back in a request header.

/** The name of the cookie the API sets on sign-in (apps/api auth controller). */
export const SESSION_COOKIE_NAME = "session";

/**
 * The `name=value` pair to send back as a Cookie header, taken from a response's Set-Cookie headers,
 * or null when the response sets no session cookie or clears it (an empty value).
 */
export function sessionCookieHeader(setCookieHeaders) {
  for (const setCookieHeader of setCookieHeaders ?? []) {
    const nameAndValue = setCookieHeader.split(";")[0].trim();
    const separatorIndex = nameAndValue.indexOf("=");
    if (separatorIndex === -1) continue;
    const cookieName = nameAndValue.slice(0, separatorIndex);
    const cookieValue = nameAndValue.slice(separatorIndex + 1);
    if (cookieName === SESSION_COOKIE_NAME && cookieValue !== "") return nameAndValue;
  }
  return null;
}

/** True for a redirect whose Location is the sign-in page (what the middleware sends a visitor to). */
export function isRedirectToLogin(status, locationHeader) {
  if (status < 300 || status >= 400 || !locationHeader) return false;
  try {
    return new URL(locationHeader, "http://placeholder.invalid").pathname === "/login";
  } catch {
    return false;
  }
}
