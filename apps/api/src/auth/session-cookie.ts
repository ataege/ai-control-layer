import type { Request } from "express";

/** Name of the HttpOnly cookie that carries the opaque session id. */
export const SESSION_COOKIE_NAME = "session";

/** The session cookie value, or undefined when it is missing, empty or not a string. */
export function readSessionCookie(request: Request): string | undefined {
  // cookie-parser types the parsed cookies as any, so narrow them here.
  const cookies: unknown = request.cookies;
  if (typeof cookies !== "object" || cookies === null) return undefined;
  const sessionValue: unknown = (cookies as Record<string, unknown>)[SESSION_COOKIE_NAME];
  return typeof sessionValue === "string" && sessionValue !== "" ? sessionValue : undefined;
}
