import { proxyUpstream } from "@/server/upstream-proxy";

// The frozen review material of one action (WEB-14); the API checks the reviewer role.
export async function GET(request: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return proxyUpstream(request, `/api/actions/${encodeURIComponent(id)}/review`);
}
