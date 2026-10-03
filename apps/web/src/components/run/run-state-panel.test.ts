import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ReasonCode, RunState, RunStatus } from "@workspace/contracts";
import completed from "@workspace/contracts/fixtures/run-state.completed.json";
import pausedAllowance from "@workspace/contracts/fixtures/run-state.paused-allowance.json";
import running from "@workspace/contracts/fixtures/run-state.running.json";
import stoppedCancelled from "@workspace/contracts/fixtures/run-state.stopped-cancelled.json";
import { describeRunState } from "./labels";
import { RunStatePanel } from "./run-state-panel";

const asRun = (value: unknown): RunState => value as RunState;

function run(status: string, terminalReason: ReasonCode | null = null): RunState {
  return { ...asRun(running), status: status as RunStatus, terminalReason, resultReference: null };
}

function render(state: RunState, safeMessage: string | null = null): string {
  return renderToStaticMarkup(createElement(RunStatePanel, { run: state, safeMessage }));
}

const ALL_STATUSES: readonly [RunStatus, ReasonCode | null][] = [
  ["queued", null],
  ["running", null],
  ["awaiting_approval", null],
  ["paused", "allowance_exhausted"],
  ["completed", null],
  ["failed", "decision_unavailable"],
  ["stopped", "run_cancelled"],
];

describe("describeRunState: every state of the contract", () => {
  it.each(ALL_STATUSES)("has a business view for %s", (status, reason) => {
    const view = describeRunState(run(status, reason));
    expect(view.status).toBe(status);
    expect(view.tone).not.toBe("unknown");
    expect(view.title).not.toContain("Unknown");
    expect(view.summary.length).toBeGreaterThan(20);
    expect(view.next.length).toBeGreaterThan(10);
    expect(view.contractProblem).toBeNull();
  });

  it("shows an unknown status as unknown and never as success", () => {
    const view = describeRunState(run("teleported"));
    expect(view.tone).toBe("unknown");
    expect(view.outcomeKind).toBe("unknown");
    expect(view.title).toBe("Unknown state (teleported)");
    expect(view.contractProblem).toContain("teleported");
    expect(render(run("teleported"))).not.toContain("Completed");
  });

  it("tells a justified interruption, an uncertain result, a stop and a failure apart", () => {
    expect(describeRunState(run("paused", "allowance_exhausted")).outcomeKind).toBe("interrupted");
    expect(describeRunState(run("paused", "outcome_unknown")).outcomeKind).toBe("uncertain");
    expect(describeRunState(run("stopped", "run_cancelled")).outcomeKind).toBe("stopped");
    expect(describeRunState(run("failed", "decision_unavailable")).outcomeKind).toBe("failed");
    expect(describeRunState(run("awaiting_approval")).outcomeKind).toBe("waiting");
    expect(describeRunState(run("completed")).outcomeKind).toBe("succeeded");
  });

  it("names the unknown outcome as operator attention with the allowance held, not zero", () => {
    const view = describeRunState(run("paused", "outcome_unknown"));
    expect(view.title).toBe("Operator attention required");
    expect(view.summary).toContain("uncertain, not zero");
  });

  it("always shows the terminal reason of a paused, failed or stopped run", () => {
    for (const [status, reason] of ALL_STATUSES.filter(([, code]) => code !== null)) {
      const html = render(run(status, reason));
      expect(html).toContain('data-part="reason"');
      expect(html).toContain(String(reason));
    }
  });

  it("says that no reason was recorded instead of inventing one", () => {
    const view = describeRunState(run("stopped", null));
    expect(view.reasonText).toBe("No reason was recorded");
    expect(view.contractProblem).toContain("no terminal reason");
  });

  it("shows no reason for a run that has none", () => {
    expect(describeRunState(run("running")).reasonText).toBeNull();
    expect(render(run("awaiting_approval"))).not.toContain('data-part="reason"');
  });
});

describe("RunStatePanel on the contract fixtures", () => {
  it("shows a waiting run as waiting, with nothing run for the action yet", () => {
    const html = render(run("awaiting_approval"));
    expect(html).toContain('data-outcome="waiting"');
    expect(html).toContain("Nothing has run for it yet.");
  });

  it("shows a stopped run's explanation: its reason and the recorded message", () => {
    const html = render(asRun(stoppedCancelled), "The run was cancelled by an operator.");
    expect(html).toContain('data-state="stopped"');
    expect(html).toContain("run_cancelled");
    expect(html).toContain("An operator cancelled the run");
    expect(html).toContain("Recorded explanation");
    expect(html).toContain("The run was cancelled by an operator.");
  });

  it("shows the paused-at-allowance run as a justified interruption with its reason", () => {
    const html = render(asRun(pausedAllowance));
    expect(html).toContain('data-outcome="interrupted"');
    expect(html).toContain("justified interruption");
    expect(html).toContain("allowance_exhausted");
  });

  it("shows a completed run with the reports its final answer named, and no reason", () => {
    const html = render(asRun(completed));
    expect(html).toContain('data-outcome="succeeded"');
    expect(html).toContain("4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3");
    expect(html).toContain("5e6f7081-92a3-4b4c-9d6e-7f8091a2b3c4");
    expect(html).not.toContain('data-part="reason"');
  });

  it("links the named reports when it is given where to open them", () => {
    const html = renderToStaticMarkup(
      createElement(RunStatePanel, {
        run: asRun(completed),
        reportHref: (id: string) => `/reports/${id}`,
      }),
    );
    expect(html).toContain('href="/reports/4d5e6f70-8192-4a3b-8c5d-6e7f8091a2b3"');
  });
});
