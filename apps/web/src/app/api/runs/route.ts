import { proxyUpstream } from "@/server/upstream-proxy";

export async function POST(request: Request) {
  return proxyUpstream(request, `/api/runs${new URL(request.url).search}`);
}
