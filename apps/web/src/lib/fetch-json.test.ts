import { describe, expect, it, vi } from "vitest";

import { fetchJson, postJson } from "@/lib/fetch-json";

// Never resolves on its own; rejects the way fetch does when its signal aborts.
const hangingFetch: typeof fetch = (_input, init) =>
  new Promise((_resolve, reject) => {
    init?.signal?.addEventListener("abort", () => reject(init.signal?.reason));
  });

describe("fetchJson", () => {
  it("returns parsed data, status and the request ID on success", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () =>
      Response.json({ status: "ok" }, { headers: { "x-request-id": "request-1" } }),
    );

    const result = await fetchJson<{ status: string }>("/api/health/live", {
      fetchImplementation: fetchMock,
    });

    expect(result).toMatchObject({
      ok: true,
      status: 200,
      data: { status: "ok" },
      requestId: "request-1",
    });
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: "GET", cache: "no-store" });
  });

  it("reports a timeout when no response arrives in time", async () => {
    const result = await fetchJson("/api/health/live", {
      timeoutMs: 20,
      fetchImplementation: hangingFetch,
    });

    expect(result).toMatchObject({ ok: false, error: { kind: "timeout", timeoutMs: 20 } });
  });

  it("reports an abort separately from a timeout", async () => {
    const abortController = new AbortController();
    const pendingResult = fetchJson("/api/health/live", {
      signal: abortController.signal,
      fetchImplementation: hangingFetch,
    });
    abortController.abort();

    expect(await pendingResult).toMatchObject({ ok: false, error: { kind: "aborted" } });
  });

  it("returns the status and parsed body of an HTTP error", async () => {
    const errorBody = { error: { code: "upstream_unreachable", message: "unreachable" } };
    const result = await fetchJson("/api/health/ready", {
      fetchImplementation: async () =>
        Response.json(errorBody, { status: 502, headers: { "x-request-id": "request-2" } }),
    });

    expect(result).toMatchObject({
      ok: false,
      requestId: "request-2",
      error: { kind: "http", status: 502, body: errorBody },
    });
  });

  it("keeps the status of an HTTP error whose body is not JSON", async () => {
    const result = await fetchJson("/api/health/ready", {
      fetchImplementation: async () => new Response("<html>bad gateway</html>", { status: 502 }),
    });

    expect(result).toMatchObject({
      ok: false,
      error: { kind: "http", status: 502, body: undefined },
    });
  });

  it("reports a network failure", async () => {
    const result = await fetchJson("/api/health/live", {
      fetchImplementation: async () => {
        throw new TypeError("fetch failed");
      },
    });

    expect(result).toMatchObject({
      ok: false,
      error: { kind: "network", message: "fetch failed" },
    });
    expect(result.requestId).toBeUndefined();
  });

  it("reports invalid JSON in a successful response with body", async () => {
    const result = await fetchJson("/api/health/live", {
      fetchImplementation: async () => new Response("not json", { status: 200 }),
    });

    expect(result).toMatchObject({ ok: false, error: { kind: "invalid_json", status: 200 } });
  });

  it("treats an empty 2xx body as success", async () => {
    const result = await fetchJson("/api/health/live", {
      fetchImplementation: async () => new Response(null, { status: 204 }),
    });

    expect(result).toMatchObject({ ok: true, status: 204, data: undefined });
  });

  it("postJson works and sends the body", async () => {
    const fetchMock = vi.fn<typeof fetch>(async () =>
      Response.json({ status: "ok" }, { headers: { "x-request-id": "request-post" } }),
    );

    const result = await postJson<{ status: string }>(
      "/api/actions",
      { cmd: "do" },
      {
        fetchImplementation: fetchMock,
      },
    );

    expect(result).toMatchObject({
      ok: true,
      status: 200,
      data: { status: "ok" },
      requestId: "request-post",
    });

    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ cmd: "do" }),
    });
  });

  it("rejects absolute and protocol-relative URLs", async () => {
    const fetchMock = vi.fn<typeof fetch>();

    await expect(
      fetchJson("https://example.com/data", { fetchImplementation: fetchMock }),
    ).rejects.toThrow(TypeError);
    await expect(
      fetchJson("//example.com/data", { fetchImplementation: fetchMock }),
    ).rejects.toThrow(TypeError);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
