import { proxyUpstream } from "@/server/upstream-proxy";

// The semantic evaluator calls a local model, so allow longer than the proxy default of 10 s.
const EVALUATE_TIMEOUT_MS = 45_000;

/** Judge evaluation (X-91): forwards the signed-in operator's request to the API unchanged. */
export async function POST(request: Request) {
  return proxyUpstream(request, "/api/control/evaluate", { timeoutMs: EVALUATE_TIMEOUT_MS });
}
