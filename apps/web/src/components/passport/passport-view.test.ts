import { describe, expect, it } from "vitest";

import { formatInstant, limitRows } from "./passport-view";

describe("formatInstant", () => {
  it("shows an RFC 3339 instant in UTC to the minute", () => {
    expect(formatInstant("2026-10-04T09:30:45Z")).toBe("2026-10-04 09:30 UTC");
    expect(formatInstant("2026-10-04T11:30:00+02:00")).toBe("2026-10-04 09:30 UTC");
  });

  it("keeps a value it cannot parse instead of inventing a time", () => {
    expect(formatInstant("not a time")).toBe("not a time");
  });
});

describe("limitRows", () => {
  const limits = {
    callsTotal: 1200,
    callsAgent: 800,
    callsSecurity: 400,
    tokensTotal: 100000,
    tokensAgent: null,
    tokensSecurity: 20000,
    requestTimeoutSeconds: 30,
    localMaxConcurrency: 2,
    toolAttempts: 3,
    corrections: 2,
    runExpiryMinutes: 15,
  };

  it("labels a missing purpose sub-limit as the shared total, never as zero", () => {
    const rows = Object.fromEntries(limitRows(limits).map((row) => [row.label, row.value]));
    expect(rows["Tokens (agent)"]).toBe("shared total only");
    expect(rows["Tokens (security)"]).toBe("20,000");
    expect(rows["Model calls (total)"]).toBe("1,200");
    expect(rows["Run expiry"]).toBe("15 min");
  });
});
