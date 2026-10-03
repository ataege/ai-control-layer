// Typed JSON fetch for same-origin routes. It never throws for request failures:
// every outcome is returned as a value so callers must handle each case.

export const DEFAULT_FETCH_TIMEOUT_MS = 15_000;

const REQUEST_ID_HEADER_NAME = "x-request-id";

export type FetchJsonError =
  /** The request never produced a response (offline, DNS, connection refused). */
  | { kind: "network"; message: string }
  /** No complete response within `timeoutMs`. */
  | { kind: "timeout"; timeoutMs: number }
  /** Cancelled by the caller's AbortSignal. */
  | { kind: "aborted" }
  /** Non-2xx status. `body` is the parsed JSON body, or undefined when it was not JSON. */
  | { kind: "http"; status: number; body: unknown }
  /** 2xx status, but the body was not valid JSON. */
  | { kind: "invalid_json"; status: number };

interface FetchJsonMeta {
  /** Wall-clock time of the whole exchange as seen by the caller. */
  durationMs: number;
  /** Value of the x-request-id response header, when a response arrived. */
  requestId?: string;
}

export type FetchJsonSuccess<ResponseBody> = FetchJsonMeta & {
  ok: true;
  status: number;
  data: ResponseBody;
};

export type FetchJsonFailure = FetchJsonMeta & { ok: false; error: FetchJsonError };

export type FetchJsonResult<ResponseBody> = FetchJsonSuccess<ResponseBody> | FetchJsonFailure;

export interface FetchJsonOptions {
  timeoutMs?: number;
  /** Lets the caller cancel the request, for example when a component unmounts. */
  signal?: AbortSignal;
  /** Injectable for tests. */
  fetchImplementation?: typeof fetch;
}

function parseJsonText(bodyText: string): { parsed: true; value: unknown } | { parsed: false } {
  try {
    return { parsed: true, value: JSON.parse(bodyText) as unknown };
  } catch {
    return { parsed: false };
  }
}

/**
 * GETs a relative URL and parses the JSON response.
 * `ResponseBody` describes the expected 2xx body; it is not validated at runtime.
 */
export async function fetchJson<ResponseBody>(
  relativeUrl: string,
  {
    timeoutMs = DEFAULT_FETCH_TIMEOUT_MS,
    signal,
    fetchImplementation = fetch,
  }: FetchJsonOptions = {},
): Promise<FetchJsonResult<ResponseBody>> {
  // Same-origin only: this helper must not become a way to call arbitrary hosts.
  if (!relativeUrl.startsWith("/") || relativeUrl.startsWith("//")) {
    throw new TypeError("fetchJson only accepts relative URLs that start with a single slash");
  }

  const timeoutSignal = AbortSignal.timeout(timeoutMs);
  const combinedSignal = signal ? AbortSignal.any([signal, timeoutSignal]) : timeoutSignal;
  const startedAt = performance.now();
  const elapsedMs = () => Math.round(performance.now() - startedAt);
  let requestId: string | undefined;

  try {
    const response = await fetchImplementation(relativeUrl, {
      method: "GET",
      cache: "no-store",
      headers: { accept: "application/json" },
      signal: combinedSignal,
    });
    requestId = response.headers.get(REQUEST_ID_HEADER_NAME) ?? undefined;
    const bodyText = await response.text();
    const parsedBody = parseJsonText(bodyText);
    const meta: FetchJsonMeta = { durationMs: elapsedMs(), requestId };

    if (!response.ok) {
      const errorBody = parsedBody.parsed ? parsedBody.value : undefined;
      return {
        ok: false,
        error: { kind: "http", status: response.status, body: errorBody },
        ...meta,
      };
    }
    if (!parsedBody.parsed) {
      return { ok: false, error: { kind: "invalid_json", status: response.status }, ...meta };
    }
    return { ok: true, status: response.status, data: parsedBody.value as ResponseBody, ...meta };
  } catch (error) {
    const meta: FetchJsonMeta = { durationMs: elapsedMs(), requestId };
    // Check the signals rather than the error name: runtimes disagree on what they throw.
    if (signal?.aborted) {
      return { ok: false, error: { kind: "aborted" }, ...meta };
    }
    if (timeoutSignal.aborted) {
      return { ok: false, error: { kind: "timeout", timeoutMs }, ...meta };
    }
    const message = error instanceof Error ? error.message : "Request failed";
    return { ok: false, error: { kind: "network", message }, ...meta };
  }
}
