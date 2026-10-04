// The sign-in page returns the visitor to the page the middleware sent them from (`callbackUrl`). The
// value comes from the address bar, so only a path inside this app is followed: an absolute URL, a
// protocol-relative `//host` or a backslash form would send the operator to another site.

const FALLBACK_PATH = "/";

export function safeCallbackPath(rawCallback: string | null | undefined): string {
  if (typeof rawCallback !== "string" || rawCallback === "") return FALLBACK_PATH;
  const isOwnPath =
    rawCallback.startsWith("/") && !rawCallback.startsWith("//") && !rawCallback.includes("\\");
  const hasControlCharacter = [...rawCallback].some((character) => character.charCodeAt(0) < 0x20);
  if (!isOwnPath || hasControlCharacter) return FALLBACK_PATH;
  // Back to the sign-in page would only loop.
  return rawCallback === "/login" || rawCallback.startsWith("/login?")
    ? FALLBACK_PATH
    : rawCallback;
}
