import { proxyUpstream } from "@/server/upstream-proxy";

// The organization's security summary; the API reads it from the gateway for the verified operator.
export async function GET(request: Request) {
  return proxyUpstream(request, "/api/security/summary");
}
