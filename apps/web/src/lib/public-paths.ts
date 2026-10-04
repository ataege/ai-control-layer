// The paths served without a session (shared by the middleware and the navigation, so they cannot
// drift). A prefix matches the path itself or what is below it, never a longer name that merely starts
// with it: /loginx and /apix are not public.

const PUBLIC_PATH_PREFIXES = ["/login", "/api", "/_next", "/health", "/diagnostics"];

export function isPublicPath(pathname: string): boolean {
  return (
    pathname === "/favicon.ico" ||
    PUBLIC_PATH_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`))
  );
}
