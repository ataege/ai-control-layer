import { proxyUpstream } from "@/server/upstream-proxy";

// Starts a judge run: a passport and a run with no agent. The body only; a query string from the
// browser is never forwarded. A static segment, so it is never read as a run id.
export async function POST(request: Request) {
  return proxyUpstream(request, "/api/runs/judge");
}
