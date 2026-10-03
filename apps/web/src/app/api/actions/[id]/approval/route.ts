import { proxyUpstream } from "@/server/upstream-proxy";

// Approve or reject the displayed action (WEB-14); the body is exactly {"decision": ...}.
export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxyUpstream(request, `/api/actions/${encodeURIComponent(id)}/approval`);
}
