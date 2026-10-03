import "server-only";

import { randomUUID } from "node:crypto";
import { REQUEST_ID_HEADER, type ErrorResponse } from "@workspace/contracts";

/** The exact upstream paths this app forwards and drops queries for. */
export const UPSTREAM_PATHS = [
  "/api/health/live",
  "/api/health/ready",
  "/api/diagnostics/gateway",
] as const;
export const UPSTREAM_PATHS_EXTRA = ["/api/auth/me"];

export type UpstreamPath = (typeof UPSTREAM_PATHS)[number] | "/api/auth/me";

export const UPSTREAM_PREFIXES = [
  ...UPSTREAM_PATHS,
  "/api/runs",
  "/api/actions",
  "/api/auth",
  "/api/control",
  "/api/security",
] as const;

// Longer than the API's own upstream timeouts, so its mapped status arrives first.
export const DEFAULT_UPSTREAM_TIMEOUT_MS = 10_000;

const REQUEST_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

type ProxyErrorCode =
  "configuration_error" | "upstream_unreachable" | "upstream_timeout" | "upstream_invalid_response";

interface ProxyOptions {
  /** Upper bound for the whole upstream exchange, including reading the body. */
  timeoutMs?: number;
  /** Whether to buffer the response and validate it as JSON. Default true. */
  buffer?: boolean;
}

class UpstreamConfigurationError extends Error {}

function readUpstreamBaseUrl(): string {
  const configuredValue = process.env.API_UPSTREAM_URL?.trim();
  if (!configuredValue) {
    throw new UpstreamConfigurationError("API_UPSTREAM_URL is not set");
  }

  let parsedUrl: URL;
  try {
    parsedUrl = new URL(configuredValue);
  } catch {
    throw new UpstreamConfigurationError("API_UPSTREAM_URL is not a valid URL");
  }

  const isHttpUrl = parsedUrl.protocol === "http:" || parsedUrl.protocol === "https:";
  const hasCredentials = parsedUrl.username !== "" || parsedUrl.password !== "";
  if (!isHttpUrl || hasCredentials || parsedUrl.search !== "" || parsedUrl.hash !== "") {
    throw new UpstreamConfigurationError("API_UPSTREAM_URL must be a plain http(s) base URL");
  }

  return `${parsedUrl.origin}${parsedUrl.pathname.replace(/\/+$/, "")}`;
}

function resolveRequestId(request: Request): string {
  const inboundRequestId = request.headers.get(REQUEST_ID_HEADER);
  return inboundRequestId && REQUEST_ID_PATTERN.test(inboundRequestId)
    ? inboundRequestId
    : randomUUID();
}

function jsonHeaders(requestId: string): HeadersInit {
  return {
    "content-type": "application/json; charset=utf-8",
    "cache-control": "no-store",
    [REQUEST_ID_HEADER]: requestId,
  };
}

function proxyErrorResponse(
  statusCode: number,
  code: ProxyErrorCode,
  message: string,
  requestId: string,
  upstreamPath: string,
): Response {
  console.error(JSON.stringify({ level: "error", event: "proxy_failure", code, requestId }));

  const errorBody: ErrorResponse = {
    error: { code, message },
    statusCode,
    requestId,
    timestamp: new Date().toISOString(),
    path: upstreamPath,
  };
  return Response.json(errorBody, { status: statusCode, headers: jsonHeaders(requestId) });
}

function isJsonText(bodyText: string): boolean {
  try {
    JSON.parse(bodyText);
    return true;
  } catch {
    return false;
  }
}

function isPathAllowed(path: string): boolean {
  const pathname = path.split("?")[0] || "";
  return UPSTREAM_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(prefix + "/"),
  );
}

export async function proxyUpstream(
  request: Request,
  upstreamPath: string,
  { timeoutMs = DEFAULT_UPSTREAM_TIMEOUT_MS, buffer = true }: ProxyOptions = {},
): Promise<Response> {
  const requestId = resolveRequestId(request);

  const normalizedUrl = new URL(upstreamPath, "http://localhost");
  const pathname = normalizedUrl.pathname;
  if (!isPathAllowed(pathname)) {
    return proxyErrorResponse(
      500,
      "configuration_error",
      "The requested path is not available.",
      requestId,
      "",
    );
  }

  let upstreamBaseUrl: string;
  try {
    upstreamBaseUrl = readUpstreamBaseUrl();
  } catch {
    return proxyErrorResponse(
      500,
      "configuration_error",
      "The web server is not configured to reach the API.",
      requestId,
      pathname,
    );
  }

  const upstreamUrl = `${upstreamBaseUrl}${pathname}${normalizedUrl.search}`;

  const headers = new Headers();
  headers.set("accept", "application/json");
  headers.set(REQUEST_ID_HEADER, requestId);

  // Forward session cookie if present
  const cookie = request.headers.get("cookie");
  if (cookie) {
    const sessionMatch = cookie.match(/(?:^|;\s*)(session=[^;]+)/);
    if (sessionMatch && sessionMatch[1]) {
      headers.set("cookie", sessionMatch[1]);
    }
  }

  // Forward content-type for POST
  if (request.headers.has("content-type")) {
    headers.set("content-type", request.headers.get("content-type")!);
  }

  const fetchOptions: RequestInit = {
    method: request.method,
    cache: "no-store",
    redirect: "manual",
    headers,
  };

  if (request.method !== "GET" && request.method !== "HEAD") {
    fetchOptions.body = request.body;
    // Need to use duplex: "half" for streaming bodies in Node.js fetch
    (fetchOptions as RequestInit & { duplex?: "half" }).duplex = "half";
  }

  let timeoutId: NodeJS.Timeout | undefined;
  if (buffer) {
    const controller = new AbortController();
    fetchOptions.signal = controller.signal;
    timeoutId = setTimeout(() => controller.abort(new Error("TimeoutError")), timeoutMs);
  }

  try {
    const upstreamResponse = await fetch(upstreamUrl, fetchOptions);
    if (timeoutId) clearTimeout(timeoutId);

    const upstreamRequestId = upstreamResponse.headers.get(REQUEST_ID_HEADER) ?? requestId;

    // Copy headers from upstream
    const responseHeaders = new Headers();
    responseHeaders.set("cache-control", "no-store");
    responseHeaders.set(REQUEST_ID_HEADER, upstreamRequestId);

    if (upstreamResponse.headers.has("content-type")) {
      responseHeaders.set("content-type", upstreamResponse.headers.get("content-type")!);
    }

    // Forward Set-Cookie headers
    const setCookieHeaders = upstreamResponse.headers.getSetCookie
      ? upstreamResponse.headers.getSetCookie()
      : [];
    if (setCookieHeaders.length === 0) {
      // Fallback
      const rawSetCookie = upstreamResponse.headers.get("set-cookie");
      if (rawSetCookie) setCookieHeaders.push(rawSetCookie);
    }
    for (const cookie of setCookieHeaders) {
      responseHeaders.append("set-cookie", cookie);
    }

    if (buffer) {
      const upstreamBodyText = await upstreamResponse.text();

      // Empty 2xx bodies are allowed
      const isSuccessEmpty = upstreamResponse.ok && upstreamBodyText === "";

      if (!isSuccessEmpty && !isJsonText(upstreamBodyText)) {
        return proxyErrorResponse(
          502,
          "upstream_invalid_response",
          "The API returned a response that is not JSON.",
          requestId,
          pathname,
        );
      }

      return new Response(upstreamBodyText, {
        status: upstreamResponse.status,
        headers: responseHeaders,
      });
    } else {
      // Stream response without buffering
      return new Response(upstreamResponse.body, {
        status: upstreamResponse.status,
        headers: responseHeaders,
      });
    }
  } catch (error) {
    if (timeoutId) clearTimeout(timeoutId);
    const isTimeout =
      error instanceof Error && (error.name === "TimeoutError" || error.message === "TimeoutError");
    return isTimeout
      ? proxyErrorResponse(
          504,
          "upstream_timeout",
          "The API did not respond in time.",
          requestId,
          pathname,
        )
      : proxyErrorResponse(
          502,
          "upstream_unreachable",
          "The API could not be reached.",
          requestId,
          pathname,
        );
  }
}

export async function proxyUpstreamGet(
  request: Request,
  upstreamPath: UpstreamPath,
  options?: ProxyOptions,
): Promise<Response> {
  // Existing GET handlers drop the query string
  return proxyUpstream(request, upstreamPath, { buffer: true, ...options });
}
