import { proxyUpstream } from "@/server/upstream-proxy";

export async function GET(request: Request) {
  return proxyUpstream(request, "/api/runs/options");
}
