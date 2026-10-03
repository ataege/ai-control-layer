// Bounded HTTP request (GET unless told otherwise) used by the smoke checks. Redirects are not followed,
// so a redirect is seen as the status the server sent. Never throws: failures are returned as data.

/**
 * @returns {Promise<{ reached: boolean, status?: number, headers?: Headers, setCookies?: string[],
 *   bodyText?: string, bodyJson?: unknown, failure?: string }>}
 */
export async function probeHttp(
  url,
  { headers = {}, timeoutMs = 5000, method = "GET", body } = {},
) {
  try {
    const response = await fetch(url, {
      method,
      headers,
      body,
      redirect: "manual",
      signal: AbortSignal.timeout(timeoutMs),
    });
    const bodyText = await response.text();
    return {
      reached: true,
      status: response.status,
      headers: response.headers,
      setCookies: response.headers.getSetCookie(),
      bodyText,
      bodyJson: parseJsonOrUndefined(bodyText),
    };
  } catch (probeError) {
    const timedOut = probeError?.name === "TimeoutError" || probeError?.name === "AbortError";
    return {
      reached: false,
      failure: timedOut ? `no response within ${timeoutMs} ms` : "connection failed",
    };
  }
}

function parseJsonOrUndefined(text) {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}
