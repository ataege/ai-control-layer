import { proxyUpstream } from "@/server/upstream-proxy";

// The active control catalog (revision, reload state, controls), as the gateway reports it. The API
// serves it for the verified operator; the page treats a missing route as "not available".
export async function GET(request: Request) {
  return proxyUpstream(request, "/api/policies/catalog");
}
