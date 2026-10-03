// Bounded HTTP GET used by the smoke checks. Never throws: failures are returned as data.

/**
 * @returns {Promise<{ reached: boolean, status?: number, headers?: Headers, bodyText?: string,
 *   bodyJson?: unknown, failure?: string }>}
 */
export async function probeHttp(url, { headers = {}, timeoutMs = 5000 } = {}) {
  try {
    const response = await fetch(url, {
      headers,
      redirect: "manual",
      signal: AbortSignal.timeout(timeoutMs),
    });
    const bodyText = await response.text();
    return {
      reached: true,
      status: response.status,
      headers: response.headers,
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
