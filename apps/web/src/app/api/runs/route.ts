import { proxyUpstream } from "@/server/upstream-proxy";

// The start-run command takes its input from the body only: a query string from the browser is
// never forwarded to the API.
export async function POST(request: Request) {
  return proxyUpstream(request, "/api/runs");
}
