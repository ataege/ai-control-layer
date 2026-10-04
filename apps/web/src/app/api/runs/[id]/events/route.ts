import { proxyUpstream } from "@/server/upstream-proxy";

// A polled JSON page, not a stream: it is buffered, validated as JSON and bounded by the proxy's
// upstream deadline like every other read.
export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxyUpstream(
    request,
    `/api/runs/${encodeURIComponent(id)}/events${new URL(request.url).search}`,
  );
}
