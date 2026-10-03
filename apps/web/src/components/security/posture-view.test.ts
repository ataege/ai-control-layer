import { readFileSync } from "node:fs";
import { createRequire } from "node:module";

import type { SecuritySummary } from "@workspace/contracts";
import { describe, expect, it } from "vitest";

import { buildPostureView, formatMicroseconds } from "./posture-view";

// The contract fixture is the shape the gateway really serves (generated from Go's encoding).
function contractFixture(name: string): SecuritySummary {
  const path = createRequire(import.meta.url).resolve(`@workspace/contracts/fixtures/${name}`);
  return JSON.parse(readFileSync(path, "utf8")) as SecuritySummary;
}

// Synthetic test data (not served by anything): counts chosen to exercise every derived number.
function syntheticSummary(): SecuritySummary {
  return {
    organizationId: "0b9a3c2e-5d4f-4a61-9b7e-3f2d1c0a9e01",
    generatedAt: "2026-10-04T10:00:00Z",
    runs: [
      { status: "stopped", count: 2 },
      { status: "completed", count: 5 },
    ],
    decisions: [
      row("action.allowed", "allow", null, null, 12),
      row("control.evaluated", "allow", null, "judge", 3),
      row("action.denied", "deny", "resource_out_of_scope", null, 2),
      row("control.evaluated", "deny", "semantic_injection_detected", "judge", 4),
      row("control.evaluated", "redact", "content_redacted", "judge", 1),
      row("action.denied", "deny", "security_evaluator_unavailable", null, 2),
      row("action.denied", "deny", "security_allowance_exhausted", null, 1),
      row("approval.requested", "approval_required", "approval_required", null, 2),
      row("approval.decided", "approved", null, null, 1),
    ],
    assessments: [
      assess("deterministic", "signature_match", "block", null, "judge", 2),
      assess("deterministic", "signature_match", "block", null, null, 1),
      assess("deterministic", "secret_pattern", "redact", null, null, 3),
      assess("semantic", "semantic_injection", "pass", "live", null, 6),
      assess("semantic", "semantic_injection", "pass", "fixture", null, 4),
      assess("semantic", "semantic_injection", "not_applicable", null, null, 9),
      assess("semantic", "semantic_injection", "error", "live", "judge", 2),
    ],
    modelUsage: [
      usage("agent", { dispatched: 9, completed: 8, failed: 0, usageUnknown: 1, held: 1584 }),
      usage("security", { dispatched: 6, completed: 6, failed: 0, usageUnknown: 0, held: 0 }),
    ],
    judgeSecurityCalls: 4,
    timings: [
      {
        phase: "provider",
        count: 4,
        failed: 1,
        medianMicroseconds: 1_919_761,
        p95Microseconds: 1_983_987,
        maxMicroseconds: 2_060_859,
      },
      {
        phase: "deterministic",
        count: 20,
        failed: 0,
        medianMicroseconds: 77,
        p95Microseconds: 12_400,
        maxMicroseconds: 640,
      },
    ],
  };
}

function row(
  eventType: string,
  decision: string | null,
  reasonCode: string | null,
  inputSource: "judge" | null,
  count: number,
) {
  return {
    eventType,
    decision,
    reasonCode,
    inputSource,
    rejectionCause: null,
    count,
  } as SecuritySummary["decisions"][number];
}

function assess(
  controlClass: "deterministic" | "semantic",
  controlId: string,
  outcome: string,
  verdictSource: "live" | "fixture" | null,
  inputSource: "judge" | null,
  count: number,
): SecuritySummary["assessments"][number] {
  return { controlClass, controlId, outcome, verdictSource, inputSource, count };
}

function usage(
  purpose: "agent" | "security",
  figures: {
    dispatched: number;
    completed: number;
    failed: number;
    usageUnknown: number;
    held: number;
  },
): SecuritySummary["modelUsage"][number] {
  return {
    purpose,
    dispatched: figures.dispatched,
    completed: figures.completed,
    failed: figures.failed,
    usageUnknown: figures.usageUnknown,
    inFlight: 0,
    settledTokens: 1000,
    heldTokens: figures.held,
    usageUnknownReservations: figures.usageUnknown,
  };
}

describe("buildPostureView on the contract fixture", () => {
  const summary = contractFixture("security-summary.judge-split.json");
  const view = buildPostureView(summary);

  it("keeps every served number and splits agent from judge", () => {
    const blocked = view.headline.find((card) => card.decision === "deny");
    // The fixture has three deny events: two from the agent path, one judge probe.
    expect(blocked?.counts).toEqual({ agent: 2, judge: 1, total: 3 });
    expect(view.headline.find((card) => card.decision === "allow")?.counts.total).toBe(0);
    expect(view.judgeSecurityCalls).toBe(summary.judgeSecurityCalls);
    expect(view.runTotal).toBe(3);
    expect(view.decisions.reduce((total, decision) => total + decision.count, 0)).toBe(3);
    expect(view.assessments.reduce((total, assessment) => total + assessment.count, 0)).toBe(6);
  });

  it("separates live verdicts from fixture verdicts", () => {
    expect(view.verdictSources.live).toEqual({ agent: 0, judge: 1, total: 1 });
    expect(view.verdictSources.fixture).toEqual({ agent: 4, judge: 0, total: 4 });
    expect(view.verdictSources.noModelCall.total).toBe(0);
  });

  it("reports deterministic blocks and no unresolved usage when the summary has none", () => {
    expect(view.deterministicBlocks.total).toBe(1);
    expect(view.unresolvedCalls).toBe(0);
    expect(view.heldTokens).toBe(0);
  });

  it("formats measured durations without inventing any", () => {
    expect(view.timings.map((timing) => timing.phase)).toEqual(["deterministic", "provider"]);
    expect(view.timings[0]).toMatchObject({ median: "77 µs", p95: "172 µs", max: "640 µs" });
    expect(view.timings[1]).toMatchObject({ median: "1.92 s", failed: 1 });
  });
});

describe("buildPostureView on synthetic counts", () => {
  const view = buildPostureView(syntheticSummary());

  it("sums each headline decision across event types, split by input source", () => {
    const byDecision = Object.fromEntries(
      view.headline.map((card) => [card.decision, card.counts]),
    );
    expect(byDecision.allow).toEqual({ agent: 12, judge: 3, total: 15 });
    expect(byDecision.deny).toEqual({ agent: 5, judge: 4, total: 9 });
    expect(byDecision.redact).toEqual({ agent: 0, judge: 1, total: 1 });
    expect(byDecision.approval_required).toEqual({ agent: 2, judge: 0, total: 2 });
  });

  it("counts security failures only for the two evaluator-failure reasons", () => {
    expect(view.securityFailures).toEqual({ agent: 3, judge: 0, total: 3 });
  });

  it("counts deterministic blocks and redactions from assessments", () => {
    expect(view.deterministicBlocks).toEqual({ agent: 1, judge: 2, total: 3 });
    expect(view.redactions).toEqual({ agent: 3, judge: 0, total: 3 });
  });

  it("puts every semantic assessment in exactly one verdict-source bucket", () => {
    const { live, fixture, noModelCall, noVerdict } = view.verdictSources;
    expect(live.total + fixture.total + noModelCall.total + noVerdict.total).toBe(6 + 4 + 9 + 2);
    expect(noModelCall.total).toBe(9);
  });

  it("never counts an errored check as a live verdict, whatever its source label says", () => {
    expect(view.verdictSources.live).toEqual({ agent: 6, judge: 0, total: 6 });
    expect(view.verdictSources.noVerdict).toEqual({ agent: 0, judge: 2, total: 2 });
  });

  it("surfaces unknown usage and its held tokens instead of folding them into zero", () => {
    expect(view.unresolvedCalls).toBe(1);
    expect(view.heldTokens).toBe(1584);
    expect(view.usage[0]).toMatchObject({ purpose: "agent", resolved: 8, usageUnknown: 1 });
  });

  it("orders decision rows by count, then by name, deterministically", () => {
    expect(view.decisions[0]).toMatchObject({ eventType: "action.allowed", count: 12 });
    expect(view.decisions.map((decision) => decision.count)).toEqual(
      [...view.decisions.map((decision) => decision.count)].sort((left, right) => right - left),
    );
    expect(view.runs.map((run) => run.status)).toEqual(["completed", "stopped"]);
  });
});

describe("formatMicroseconds", () => {
  it("chooses a unit by magnitude", () => {
    expect(formatMicroseconds(0)).toBe("0 µs");
    expect(formatMicroseconds(999)).toBe("999 µs");
    expect(formatMicroseconds(1_000)).toBe("1.00 ms");
    expect(formatMicroseconds(12_400)).toBe("12.4 ms");
    expect(formatMicroseconds(250_000)).toBe("250 ms");
    expect(formatMicroseconds(1_919_761)).toBe("1.92 s");
  });
});
