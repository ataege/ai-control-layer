import { proxyUpstreamGet } from "@/server/upstream-proxy";

export async function GET(request: Request) {
  return proxyUpstreamGet(request, "/api/auth/me");
}
