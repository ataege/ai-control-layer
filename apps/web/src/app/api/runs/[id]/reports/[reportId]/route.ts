import { proxyUpstream } from "@/server/upstream-proxy";

// The stored report of one run: forwarded to the API, which reads it from the gateway scoped to the
// signed-in operator's organization. Both path segments are encoded; nothing else is forwarded.
export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string; reportId: string }> },
) {
  const { id, reportId } = await params;
  return proxyUpstream(
    request,
    `/api/runs/${encodeURIComponent(id)}/reports/${encodeURIComponent(reportId)}`,
  );
}
