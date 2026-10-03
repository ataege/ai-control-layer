import { proxyUpstream } from "@/server/upstream-proxy";

export async function POST(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxyUpstream(request, `/api/actions/${id}/approval${new URL(request.url).search}`);
}
