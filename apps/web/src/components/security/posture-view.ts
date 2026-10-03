// The security posture dashboard's numbers, derived from the served SecuritySummary and from
// nothing else. Every count comes from stored records; durations are observed spans; no cost
// estimate exists in the summary, so none is shown. Pure functions, tested without a browser.

import type {
  AssessmentCount,
  DecisionCount,
  PhaseTiming,
  PurposeUsage,
  SecuritySummary,
} from "@workspace/contracts";

/** Who produced a record: the run's own agent path, or a judge's probe (GO-82). */
export type InputSource = "agent" | "judge";

export function inputSourceOf(row: { inputSource: "judge" | null }): InputSource {
  return row.inputSource === "judge" ? "judge" : "agent";
}

export const INPUT_SOURCE_LABELS: Record<InputSource, string> = {
  agent: "Agent run",
  judge: "Judge probe",
};

export interface SplitCount {
  agent: number;
  judge: number;
  total: number;
}

function emptySplit(): SplitCount {
  return { agent: 0, judge: 0, total: 0 };
}

function addToSplit(split: SplitCount, source: InputSource, count: number): void {
  split[source] += count;
  split.total += count;
}

/** The decision values an event can record (X-12); the dashboard's headline cards. */
export const HEADLINE_DECISIONS = [
  { decision: "allow", label: "Allowed" },
  { decision: "deny", label: "Blocked" },
  { decision: "redact", label: "Redacted" },
  { decision: "approval_required", label: "Sent to review" },
] as const;

/**
 * Reason codes that mean the security evaluation itself could not run or ran out of allowance.
 * These are failures of the control, not decisions about content.
 */
export const SECURITY_FAILURE_REASONS: readonly string[] = [
  "security_evaluator_unavailable",
  "security_allowance_exhausted",
];

export interface HeadlineCard {
  decision: string;
  label: string;
  counts: SplitCount;
}

export interface DecisionRow {
  eventType: string;
  decision: string | null;
  reasonCode: string | null;
  rejectionCause: string | null;
  source: InputSource;
  count: number;
}

export interface AssessmentRow {
  controlClass: "deterministic" | "semantic";
  controlId: string;
  outcome: string;
  verdictSource: "live" | "fixture" | null;
  source: InputSource;
  count: number;
}

export interface VerdictSourceTotals {
  /** Semantic assessments that produced a verdict from the live local model. */
  live: SplitCount;
  /** Semantic assessments answered by a labelled test fixture: tests handling, not detection. */
  fixture: SplitCount;
  /** Semantic assessments that made no model call (not applicable). */
  noModelCall: SplitCount;
  /** Semantic assessments that could not complete (outcome error): no verdict, whatever the source. */
  noVerdict: SplitCount;
}

export interface UsageRow extends PurposeUsage {
  /** Calls that finished with a known outcome. */
  resolved: number;
}

export interface TimingRow {
  phase: string;
  count: number;
  failed: number;
  median: string;
  p95: string;
  max: string;
}

export interface PostureView {
  organizationId: string;
  generatedAt: string;
  headline: HeadlineCard[];
  /** Decision events whose reason says the security evaluation failed. */
  securityFailures: SplitCount;
  /** Assessments by deterministic controls that blocked. */
  deterministicBlocks: SplitCount;
  /** Assessments (any control) that redacted. */
  redactions: SplitCount;
  decisions: DecisionRow[];
  assessments: AssessmentRow[];
  verdictSources: VerdictSourceTotals;
  runs: { status: string; count: number }[];
  runTotal: number;
  usage: UsageRow[];
  /** Calls whose usage could not be settled, with their reservations kept: never shown as zero. */
  unresolvedCalls: number;
  heldTokens: number;
  judgeSecurityCalls: number;
  timings: TimingRow[];
}

/** Microseconds as a short, honest duration: "850 µs", "12.4 ms", "1.92 s". */
export function formatMicroseconds(microseconds: number): string {
  if (microseconds < 1_000) return `${Math.round(microseconds)} µs`;
  if (microseconds < 1_000_000) return `${trim(microseconds / 1_000)} ms`;
  return `${trim(microseconds / 1_000_000)} s`;
}

function trim(value: number): string {
  return value >= 100 ? value.toFixed(0) : value >= 10 ? value.toFixed(1) : value.toFixed(2);
}

function compareText(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function decisionRows(decisions: DecisionCount[]): DecisionRow[] {
  return decisions
    .map((row) => ({
      eventType: row.eventType,
      decision: row.decision,
      reasonCode: row.reasonCode,
      rejectionCause: row.rejectionCause ?? null,
      source: inputSourceOf(row),
      count: row.count,
    }))
    .sort(
      (left, right) =>
        right.count - left.count ||
        compareText(left.eventType, right.eventType) ||
        compareText(left.decision ?? "", right.decision ?? "") ||
        compareText(left.reasonCode ?? "", right.reasonCode ?? "") ||
        compareText(left.source, right.source),
    );
}

function assessmentRows(assessments: AssessmentCount[]): AssessmentRow[] {
  return assessments
    .map((row) => ({
      controlClass: row.controlClass,
      controlId: row.controlId,
      outcome: row.outcome,
      verdictSource: row.verdictSource,
      source: inputSourceOf(row),
      count: row.count,
    }))
    .sort(
      (left, right) =>
        compareText(left.controlClass, right.controlClass) ||
        compareText(left.controlId, right.controlId) ||
        compareText(left.outcome, right.outcome) ||
        compareText(left.source, right.source) ||
        compareText(left.verdictSource ?? "", right.verdictSource ?? ""),
    );
}

function timingRows(timings: PhaseTiming[]): TimingRow[] {
  return [...timings]
    .sort((left, right) => compareText(left.phase, right.phase))
    .map((timing) => ({
      phase: timing.phase,
      count: timing.count,
      failed: timing.failed,
      median: formatMicroseconds(timing.medianMicroseconds),
      p95: formatMicroseconds(timing.p95Microseconds),
      max: formatMicroseconds(timing.maxMicroseconds),
    }));
}

/** Derives everything the dashboard shows from one served summary. */
export function buildPostureView(summary: SecuritySummary): PostureView {
  const headline: HeadlineCard[] = HEADLINE_DECISIONS.map(({ decision, label }) => ({
    decision,
    label,
    counts: emptySplit(),
  }));
  const securityFailures = emptySplit();
  for (const row of summary.decisions) {
    const source = inputSourceOf(row);
    const card = headline.find((candidate) => candidate.decision === row.decision);
    if (card) addToSplit(card.counts, source, row.count);
    if (row.reasonCode !== null && SECURITY_FAILURE_REASONS.includes(row.reasonCode)) {
      addToSplit(securityFailures, source, row.count);
    }
  }

  const deterministicBlocks = emptySplit();
  const redactions = emptySplit();
  const verdictSources: VerdictSourceTotals = {
    live: emptySplit(),
    fixture: emptySplit(),
    noModelCall: emptySplit(),
    noVerdict: emptySplit(),
  };
  for (const row of summary.assessments) {
    const source = inputSourceOf(row);
    if (row.controlClass === "deterministic" && row.outcome === "block") {
      addToSplit(deterministicBlocks, source, row.count);
    }
    if (row.outcome === "redact") addToSplit(redactions, source, row.count);
    if (row.controlClass === "semantic") {
      // An error has no verdict even when its source says live: the check did not complete.
      const bucket =
        row.outcome === "error"
          ? verdictSources.noVerdict
          : row.verdictSource === "live"
            ? verdictSources.live
            : row.verdictSource === "fixture"
              ? verdictSources.fixture
              : verdictSources.noModelCall;
      addToSplit(bucket, source, row.count);
    }
  }

  const usage: UsageRow[] = summary.modelUsage.map((row) => ({
    ...row,
    resolved: row.completed + row.failed,
  }));
  const runs = [...summary.runs].sort((left, right) => compareText(left.status, right.status));

  return {
    organizationId: summary.organizationId,
    generatedAt: summary.generatedAt,
    headline,
    securityFailures,
    deterministicBlocks,
    redactions,
    decisions: decisionRows(summary.decisions),
    assessments: assessmentRows(summary.assessments),
    verdictSources,
    runs,
    runTotal: runs.reduce((total, run) => total + run.count, 0),
    usage,
    unresolvedCalls: usage.reduce((total, row) => total + row.usageUnknown, 0),
    heldTokens: usage.reduce((total, row) => total + row.heldTokens, 0),
    judgeSecurityCalls: summary.judgeSecurityCalls,
    timings: timingRows(summary.timings),
  };
}
