import { proxyUpstreamGet } from "@/server/upstream-proxy";

// Always answered at request time; never cached or prerendered.
export const dynamic = "force-dynamic";

export function GET(request: Request): Promise<Response> {
  return proxyUpstreamGet(request, "/api/health/live");
}
