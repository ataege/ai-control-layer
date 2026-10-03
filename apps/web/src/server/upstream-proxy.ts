import "server-only";

import { randomUUID } from "node:crypto";

import { REQUEST_ID_HEADER, type ErrorResponse } from "@workspace/contracts";

/** The only upstream paths this app forwards. Each one has its own route file. */
export const UPSTREAM_PATHS = [
  "/api/health/live",
  "/api/health/ready",
  "/api/diagnostics/gateway",
] as const;

export type UpstreamPath = (typeof UPSTREAM_PATHS)[number];

// Longer than the API's own upstream timeouts, so its mapped status arrives first.
export const DEFAULT_UPSTREAM_TIMEOUT_MS = 10_000;

const REQUEST_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

type ProxyErrorCode =
  "configuration_error" | "upstream_unreachable" | "upstream_timeout" | "upstream_invalid_response";

interface ProxyOptions {
  /** Upper bound for the whole upstream exchange, including reading the body. */
  timeoutMs?: number;
}

class UpstreamConfigurationError extends Error {}

// Read at request time only: builds and module imports must work without any env.
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

// Accept a well-formed inbound ID, otherwise start a new correlation ID.
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
  // Only the code is logged: never the upstream URL or the underlying error.
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

/**
 * Forwards one allowlisted GET to the API and passes its status and JSON body through.
 * The target is built from server config and a fixed path; nothing from the caller's
 * URL, query or headers (except a validated request ID) reaches the upstream request.
 */
export async function proxyUpstreamGet(
  request: Request,
  upstreamPath: UpstreamPath,
  { timeoutMs = DEFAULT_UPSTREAM_TIMEOUT_MS }: ProxyOptions = {},
): Promise<Response> {
  const requestId = resolveRequestId(request);

  // Guards against a route passing a path that is not on the list.
  if (!UPSTREAM_PATHS.includes(upstreamPath)) {
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
      upstreamPath,
    );
  }

  try {
    const upstreamResponse = await fetch(`${upstreamBaseUrl}${upstreamPath}`, {
      method: "GET",
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(timeoutMs),
      headers: { accept: "application/json", [REQUEST_ID_HEADER]: requestId },
    });
    const upstreamBodyText = await upstreamResponse.text();

    if (!isJsonText(upstreamBodyText)) {
      return proxyErrorResponse(
        502,
        "upstream_invalid_response",
        "The API returned a response that is not JSON.",
        requestId,
        upstreamPath,
      );
    }

    const upstreamRequestId = upstreamResponse.headers.get(REQUEST_ID_HEADER);
    return new Response(upstreamBodyText, {
      status: upstreamResponse.status,
      headers: jsonHeaders(upstreamRequestId ?? requestId),
    });
  } catch (error) {
    const isTimeout = error instanceof Error && error.name === "TimeoutError";
    return isTimeout
      ? proxyErrorResponse(
          504,
          "upstream_timeout",
          "The API did not respond in time.",
          requestId,
          upstreamPath,
        )
      : proxyErrorResponse(
          502,
          "upstream_unreachable",
          "The API could not be reached.",
          requestId,
          upstreamPath,
        );
  }
}
