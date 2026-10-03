import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ReasonCode, RunState, RunUsage } from "@workspace/contracts";
import completed from "@workspace/contracts/fixtures/run-state.completed.json";
import pausedAllowance from "@workspace/contracts/fixtures/run-state.paused-allowance.json";
import stoppedCancelled from "@workspace/contracts/fixtures/run-state.stopped-cancelled.json";
import usageLedger from "@workspace/contracts/fixtures/run-usage.ledger.json";
import usageNoLedger from "@workspace/contracts/fixtures/run-usage.no-ledger.json";
import { LimitStopNotice } from "./limit-stop-notice";
import { limitStopKind } from "./usage-model";

const asRun = (value: unknown): RunState => value as RunState;
const asUsage = (value: unknown): RunUsage => value as RunUsage;

function render(run: RunState, usage: RunUsage | null, safeMessage: string | null = null): string {
  return renderToStaticMarkup(createElement(LimitStopNotice, { run, usage, safeMessage }));
}

function stoppedWith(status: "paused" | "stopped", reason: ReasonCode): RunState {
  return { ...asRun(pausedAllowance), status, terminalReason: reason };
}

describe("limitStopKind", () => {
  it("reads a limit stop from the persisted state alone", () => {
    expect(limitStopKind(asRun(pausedAllowance))).toBe("model_allowance");
    expect(limitStopKind(stoppedWith("paused", "security_allowance_exhausted"))).toBe(
      "security_allowance",
    );
    expect(limitStopKind(stoppedWith("stopped", "run_expired"))).toBe("time");
  });

  it("does not treat any other run as a limit stop", () => {
    expect(limitStopKind(asRun(stoppedCancelled))).toBeNull();
    expect(limitStopKind(asRun(completed))).toBeNull();
    expect(limitStopKind(stoppedWith("paused", "outcome_unknown"))).toBeNull();
    expect(limitStopKind({ ...asRun(pausedAllowance), status: "running" })).toBeNull();
    expect(limitStopKind({ ...asRun(pausedAllowance), terminalReason: null })).toBeNull();
  });
});

describe("LimitStopNotice", () => {
  it("shows the persisted terminal reason and the usage at the stop", () => {
    const html = render(
      asRun(pausedAllowance),
      asUsage(usageLedger),
      "The run reached one of its limits.",
    );
    expect(html).toContain('data-part="limit-stop"');
    expect(html).toContain("Paused at a limit");
    expect(html).toContain("allowance_exhausted");
    expect(html).toContain("The task&#x27;s allowance (model calls, tokens or time) is used up");
    expect(html).toContain("The run reached one of its limits.");
    expect(html).toContain("no further model request is sent for it");
    expect(html).toContain('data-part="usage-at-stop"');
    expect(html).toContain("Agent requests: 3 of 12");
    expect(html).toContain("20,000");
  });

  it("marks a request limit that is at its cap", () => {
    const usage = asUsage({
      ...usageLedger,
      ledger: { ...usageLedger.ledger, calls: { ...usageLedger.ledger.calls, agentLimit: 3 } },
    });
    const html = render(asRun(pausedAllowance), usage);
    expect(html).toContain("Agent requests: 3 of 3 (limit reached)");
  });

  it("says when every dispatched request is accounted for, and when one is not", () => {
    expect(render(asRun(pausedAllowance), asUsage(usageLedger))).toContain('data-part="accounted"');
    const broken = asUsage({
      ...usageLedger,
      modelCalls: usageLedger.modelCalls.map((row, index) =>
        index === 0 ? { ...row, dispatched: 9 } : row,
      ),
    });
    const html = render(asRun(pausedAllowance), broken);
    expect(html).toContain('data-part="unaccounted"');
    expect(html).toContain("Agent steps");
  });

  it("shows unknown usage at the stop as uncertain, not zero", () => {
    expect(render(asRun(pausedAllowance), asUsage(usageLedger))).toContain('data-part="uncertain"');
  });

  it("still shows the reason when the usage was not available", () => {
    const html = render(asRun(pausedAllowance), null);
    expect(html).toContain("allowance_exhausted");
    expect(html).toContain("The usage at the stop was not available to this page.");
  });

  it("says when there is no ledger instead of inventing usage", () => {
    const html = render(asRun(pausedAllowance), asUsage(usageNoLedger));
    expect(html).toContain("No allowance ledger exists for this run");
  });

  it("renders nothing for a run that did not stop on a limit", () => {
    expect(render(asRun(stoppedCancelled), asUsage(usageLedger))).toBe("");
    expect(render(asRun(completed), asUsage(usageLedger))).toBe("");
  });

  it("names a time stop as a stop at the time limit", () => {
    const html = render(stoppedWith("stopped", "run_expired"), asUsage(usageLedger));
    expect(html).toContain("Stopped at a limit");
    expect(html).toContain("time allowance passed");
  });
});
