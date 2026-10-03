import { proxyUpstream } from "@/server/upstream-proxy";

export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxyUpstream(
    request,
    `/api/runs/${encodeURIComponent(id)}/passport${new URL(request.url).search}`,
  );
}
