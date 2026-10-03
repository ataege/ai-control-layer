import { proxyUpstream } from "@/server/upstream-proxy";

// The active control catalog's status (revision, reload state, controls). The API serves it from the
// gateway for the verified operator; the page treats a missing route as "not available".
export async function GET(request: Request) {
  return proxyUpstream(request, "/api/policies/status");
}
