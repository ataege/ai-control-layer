import { proxyUpstream } from "@/server/upstream-proxy";

// The sanitized audit export (reviewer role). The query (kind, format, after, limit) is forwarded
// unchanged and validated by the API. A CSV body is not JSON, so it is streamed instead of
// buffered: the buffering path would answer 502 for any non-JSON body.
export async function GET(request: Request) {
  const { search, searchParams } = new URL(request.url);
  return proxyUpstream(request, `/api/security/export${search}`, {
    buffer: searchParams.get("format") !== "csv",
  });
}
