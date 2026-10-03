import type { Passport } from "@workspace/contracts";

/** RFC 3339 UTC instant as "YYYY-MM-DD HH:MM UTC"; an unparseable value is shown as stored. */
export function formatInstant(instant: string): string {
  const parsed = new Date(instant);
  if (Number.isNaN(parsed.getTime())) return instant;
  return `${parsed.toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

export interface LimitRow {
  label: string;
  value: string;
}

/** A null purpose sub-limit means only the shared token total applies. */
function purposeTokens(limit: number | null): string {
  return limit === null ? "shared total only" : limit.toLocaleString("en-US");
}

/** The passport's hard ceilings as labelled rows, in a fixed order. */
export function limitRows(limits: Passport["limits"]): LimitRow[] {
  const count = (value: number) => value.toLocaleString("en-US");
  return [
    { label: "Model calls (total)", value: count(limits.callsTotal) },
    { label: "Model calls (agent)", value: count(limits.callsAgent) },
    { label: "Model calls (security)", value: count(limits.callsSecurity) },
    { label: "Tokens (total)", value: count(limits.tokensTotal) },
    { label: "Tokens (agent)", value: purposeTokens(limits.tokensAgent) },
    { label: "Tokens (security)", value: purposeTokens(limits.tokensSecurity) },
    { label: "Request timeout", value: `${count(limits.requestTimeoutSeconds)} s` },
    { label: "Tool attempts", value: count(limits.toolAttempts) },
    { label: "Corrections", value: count(limits.corrections) },
    { label: "Run expiry", value: `${count(limits.runExpiryMinutes)} min` },
  ];
}
