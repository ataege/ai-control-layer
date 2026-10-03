import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { RunUsage } from "@workspace/contracts";
import usageLedger from "@workspace/contracts/fixtures/run-usage.ledger.json";
import usageNoLedger from "@workspace/contracts/fixtures/run-usage.no-ledger.json";
import { RunUsagePanel } from "./run-usage-panel";
import { describeUsage } from "./usage-model";

const asUsage = (value: unknown): RunUsage => value as RunUsage;
const ledgerUsage = asUsage(usageLedger);

function render(usage: RunUsage): string {
  return renderToStaticMarkup(createElement(RunUsagePanel, { usage }));
}

function withAgent(change: Partial<RunUsage["modelCalls"][number]>): RunUsage {
  return {
    ...ledgerUsage,
    modelCalls: ledgerUsage.modelCalls.map((row) =>
      row.purpose === "agent" ? { ...row, ...change } : row,
    ),
  };
}

describe("describeUsage keeps the figures apart", () => {
  it("separates reported tokens from reserved tokens and never adds them", () => {
    const view = describeUsage(ledgerUsage);
    expect(view.reportedTokens).toBe(300);
    expect(view.reservedTokens).toBe(50);
    const agent = view.purposes.find((row) => row.purpose === "agent");
    expect(agent).toMatchObject({ reportedTokens: 300, reservedTokens: 40, unknownCalls: 1 });
  });

  it("counts a request with unknown usage as uncertain, not as zero", () => {
    const view = describeUsage(ledgerUsage);
    expect(view.uncertain).toBe(true);
    expect(view.unknownCalls).toBe(1);
    expect(view.uncertainLabel?.id).toBe("uncertain_cost");
    expect(describeUsage(asUsage(usageNoLedger)).uncertain).toBe(false);
  });

  it("takes every limit from the run's ledger", () => {
    const changed = {
      ...ledgerUsage,
      ledger:
        ledgerUsage.ledger === null
          ? null
          : {
              ...ledgerUsage.ledger,
              tokens: { limit: 12345, reserved: 50, used: 300 },
              calls: { ...ledgerUsage.ledger.calls, limit: 7 },
            },
    };
    const view = describeUsage(changed);
    expect(view.ledger?.allowances[0]).toMatchObject({ limit: 12345, available: 11995 });
    expect(view.ledger?.calls[0]).toMatchObject({ limit: 7, used: 4 });
  });

  it("has no ledger view for a run without a ledger", () => {
    expect(describeUsage(asUsage(usageNoLedger)).ledger).toBeNull();
  });
});

describe("RunUsagePanel", () => {
  it("shows a retained reservation with missing reported usage as uncertain, not zero", () => {
    const html = render(
      withAgent({ settledTokens: 0, usageUnknown: 1, usageUnknownReservations: 1, heldTokens: 40 }),
    );
    expect(html).toContain('data-part="unknown-usage"');
    expect(html).toContain("Usage is uncertain");
    expect(html).toContain("not counted as used or as zero");
    expect(html).toContain("+ uncertain");
    expect(html).toContain(
      "Uncertain: an unresolved reservation, not a measured amount and not zero",
    );
  });

  it("shows no uncertainty notice when every request reported its usage", () => {
    const html = render(
      withAgent({ usageUnknown: 0, usageUnknownReservations: 0, completed: 3, heldTokens: 0 }),
    );
    expect(html).not.toContain('data-part="unknown-usage"');
    expect(html).not.toContain("+ uncertain");
  });

  it("renders the limits of the ledger, not constants", () => {
    const changed = asUsage({
      ...ledgerUsage,
      ledger: { ...ledgerUsage.ledger, tokens: { limit: 12345, reserved: 50, used: 300 } },
    });
    const html = render(changed);
    expect(html).toContain("12,345");
    expect(html).not.toContain("20,000");
    expect(render(ledgerUsage)).toContain("20,000");
  });

  it("states a separate agent limit and the lack of one for security", () => {
    const html = render(ledgerUsage);
    expect(html).toContain("12,000");
    expect(html).toContain("No separate limit");
  });

  it("never shows an amount of money: cost is not estimated for a local model", () => {
    const html = render(ledgerUsage);
    expect(html).toContain('data-part="cost"');
    expect(html).toContain("Estimated cost is not shown");
    expect(html).not.toMatch(/[$€£]|\bUSD\b|\bEUR\b/);
  });

  it("says plainly that a run without a ledger has nothing reserved or reported", () => {
    const html = render(asUsage(usageNoLedger));
    expect(html).toContain('data-part="no-ledger"');
    expect(html).toContain("nothing was reserved or reported");
    expect(html).not.toContain("Token allowance");
  });

  it("marks the request limits that are reached", () => {
    const base = ledgerUsage.ledger;
    expect(base).not.toBeNull();
    const changed = asUsage({
      ...ledgerUsage,
      ledger: { ...base, calls: { ...base?.calls, limit: 4 } },
    });
    expect(render(changed)).toContain("reached");
  });

  it("shows that a paused ledger reserves nothing more", () => {
    const changed = asUsage({ ...ledgerUsage, ledger: { ...ledgerUsage.ledger, paused: true } });
    expect(render(changed)).toContain('data-part="ledger-paused"');
  });
});
