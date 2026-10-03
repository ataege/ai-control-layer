import { proxyUpstream } from "@/server/upstream-proxy";

// Request cancellation of a run (WEB-15): stops future work, reverses nothing already done.
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxyUpstream(request, `/api/runs/${encodeURIComponent(id)}/cancel`);
}
