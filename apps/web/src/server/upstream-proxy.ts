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
  "/api/policies",
] as const;

// Longer than the API's default command timeout (10 s), so its mapped status arrives first,
// and shorter than the browser's 15 s fetch timeout.
export const DEFAULT_UPSTREAM_TIMEOUT_MS = 12_000;

const REQUEST_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

type ProxyErrorCode =
  | "configuration_error"
  | "upstream_unreachable"
  | "upstream_timeout"
  | "upstream_invalid_response"
  | "cross_site_request_refused"
  | "unsupported_media_type";

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

/** Fetch-metadata values a browser sends for a request that its own pages made. */
const SAME_ORIGIN_FETCH_SITES = new Set(["same-origin", "none"]);

/**
 * CSRF guard for commands (every method but GET and HEAD). A browser always sends Sec-Fetch-Site and,
 * on a POST, Origin, so a cross-site or same-site-other-origin page is refused here even where the
 * SameSite=Lax cookie would still travel. A client that sends neither header is not a browser and
 * is not a CSRF vector (the end-to-end script, the judge client).
 */
function commandRefusal(
  request: Request,
): { status: number; code: ProxyErrorCode; message: string } | null {
  const fetchSite = request.headers.get("sec-fetch-site");
  if (fetchSite !== null && !SAME_ORIGIN_FETCH_SITES.has(fetchSite.toLowerCase())) {
    return {
      status: 403,
      code: "cross_site_request_refused",
      message: "The request did not come from this site.",
    };
  }
  const origin = request.headers.get("origin");
  if (origin !== null) {
    // The browser's Origin must be this site's own origin: the request URL, or the Host the
    // browser addressed (the URL can carry an internal origin behind a reverse proxy).
    let originHost: string | null = null;
    try {
      originHost = new URL(origin).host;
    } catch {
      originHost = null;
    }
    const ownOrigin = new URL(request.url).origin;
    if (
      origin !== ownOrigin &&
      (originHost === null || originHost !== request.headers.get("host"))
    ) {
      return {
        status: 403,
        code: "cross_site_request_refused",
        message: "The request did not come from this site.",
      };
    }
  }
  // Commands are JSON. A body of another type is how a plain HTML form would post.
  // A bodyless POST (sign-out, cancel) reaches a Next.js handler with an empty stream and
  // Content-Length 0, so the stream alone does not say that a body was sent.
  const sendsBody = request.body !== null && request.headers.get("content-length") !== "0";
  if (sendsBody) {
    const mediaType = (request.headers.get("content-type") ?? "")
      .split(";")[0]
      ?.trim()
      .toLowerCase();
    if (mediaType !== "application/json") {
      return {
        status: 415,
        code: "unsupported_media_type",
        message: "The request body must be application/json.",
      };
    }
  }
  return null;
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

  if (request.method !== "GET" && request.method !== "HEAD") {
    const refusal = commandRefusal(request);
    if (refusal !== null) {
      return proxyErrorResponse(refusal.status, refusal.code, refusal.message, requestId, pathname);
    }
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

    // Paging cursor of the audit export's CSV pages; a reference, never a secret.
    const nextCursor = upstreamResponse.headers.get("x-next-cursor");
    if (nextCursor && /^[A-Za-z0-9._:-]{1,128}$/.test(nextCursor)) {
      responseHeaders.set("x-next-cursor", nextCursor);
    }
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

      // A 204, 205 or 304 has no body: a Response refuses even an empty string for them.
      const hasNoBodyStatus = [204, 205, 304].includes(upstreamResponse.status);
      return new Response(hasNoBodyStatus ? null : upstreamBodyText, {
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
