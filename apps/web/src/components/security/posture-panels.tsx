import type * as React from "react";

import { Badge } from "@workspace/ui/components/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { EmptyState } from "@workspace/ui/components/empty-state";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table";
import { cn } from "@workspace/ui/lib/utils";

import { VerdictSourceLabel } from "@/components/labels";
import { LABELS } from "@/lib/labels";
import { pluralize } from "@/lib/plural";

import {
  INPUT_SOURCE_LABELS,
  type AssessmentRow,
  type DecisionRow,
  type InputSource,
  type PostureView,
  type SplitCount,
  type TimingRow,
  type UsageRow,
} from "./posture-view";

const NUMBER_FORMAT = new Intl.NumberFormat("en-US");

// A semantic check that made no model call (for example, no free-text arguments) has no verdict.
const NO_MODEL_CALL_LABEL = "No model call (not applicable)";
// A semantic check that errored produced no verdict, whatever its source label says.
const NO_VERDICT_LABEL = "Check did not complete (no verdict)";

export function formatCount(value: number): string {
  return NUMBER_FORMAT.format(value);
}

function Code({ children }: { children: React.ReactNode }) {
  return <code className="font-mono text-xs">{children}</code>;
}

function SourceBadge({ source }: { source: InputSource }) {
  return (
    <Badge variant={source === "judge" ? "secondary" : "outline"}>
      {INPUT_SOURCE_LABELS[source]}
    </Badge>
  );
}

function SplitCaption({ counts }: { counts: SplitCount }) {
  return (
    <p className="text-xs text-muted-foreground">
      Agent runs {formatCount(counts.agent)} · Judge probes {formatCount(counts.judge)}
    </p>
  );
}

function StatCard({
  label,
  value,
  caption,
  tone = "neutral",
}: {
  label: string;
  value: number;
  caption: React.ReactNode;
  tone?: "neutral" | "attention";
}) {
  return (
    <Card size="sm" className={cn(tone === "attention" && value > 0 && "ring-destructive/40")}>
      <CardHeader>
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl tabular-nums">{formatCount(value)}</CardTitle>
      </CardHeader>
      <CardContent>{caption}</CardContent>
    </Card>
  );
}

export function HeadlineCards({ view }: { view: PostureView }) {
  return (
    <section aria-labelledby="posture-headline" className="flex flex-col gap-3">
      <h2 id="posture-headline" className="text-base font-medium">
        Decisions at a glance
      </h2>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {view.headline.map((card) => (
          <StatCard
            key={card.decision}
            label={card.label}
            value={card.counts.total}
            caption={<SplitCaption counts={card.counts} />}
          />
        ))}
      </div>
      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard
          label="Blocked by deterministic controls"
          value={view.deterministicBlocks.total}
          caption={<SplitCaption counts={view.deterministicBlocks} />}
        />
        <StatCard
          label="Security evaluation failures"
          value={view.securityFailures.total}
          tone="attention"
          caption={
            <p className="text-xs text-muted-foreground">
              Evaluator unavailable or its allowance exhausted. A failure pauses or denies; it never
              allows.
            </p>
          }
        />
        <StatCard
          label="Calls with unknown usage"
          value={view.unresolvedCalls}
          tone="attention"
          caption={
            <p className="text-xs text-muted-foreground">
              {formatCount(view.heldTokens)}{" "}
              {pluralize(view.heldTokens, "token stays", "tokens stay")} reserved until resolved;
              unknown usage is never counted as zero.
            </p>
          }
        />
      </div>
      <p className="text-xs text-muted-foreground">
        Counts are decision events. One interaction can record several, so these are not distinct
        interactions.
      </p>
    </section>
  );
}

export function DecisionsTable({ rows }: { rows: DecisionRow[] }) {
  return (
    <Panel
      id="posture-decisions"
      title="Decision events"
      description="By event type, decision, reason and who produced them: the agent's own run or a judge's probe."
    >
      {rows.length === 0 ? (
        <EmptyState
          title="No decision events yet"
          description="Nothing has been recorded for this organization."
        />
      ) : (
        <Table>
          <TableCaption className="sr-only">
            Decision events by type, decision, reason and source
          </TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Event</TableHead>
              <TableHead>Decision</TableHead>
              <TableHead>Reason</TableHead>
              <TableHead>Source</TableHead>
              <TableHead className="text-right">Count</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={`${row.eventType}|${row.decision}|${row.reasonCode}|${row.rejectionCause}|${row.source}`}
              >
                <TableCell>
                  <Code>{row.eventType}</Code>
                </TableCell>
                <TableCell>
                  {row.decision ? (
                    <Code>{row.decision}</Code>
                  ) : (
                    <span className="text-muted-foreground">none</span>
                  )}
                </TableCell>
                <TableCell>
                  {row.reasonCode ? (
                    <Code>{row.reasonCode}</Code>
                  ) : (
                    <span className="text-muted-foreground">none</span>
                  )}
                  {row.rejectionCause ? (
                    <p className="text-xs text-muted-foreground">
                      rejected final answer: <Code>{row.rejectionCause}</Code>
                    </p>
                  ) : null}
                </TableCell>
                <TableCell>
                  <SourceBadge source={row.source} />
                </TableCell>
                <TableCell className="text-right tabular-nums">{formatCount(row.count)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Panel>
  );
}

export function ControlsPanel({ view }: { view: PostureView }) {
  const { live, fixture, noModelCall, noVerdict } = view.verdictSources;
  return (
    <Panel
      id="posture-controls"
      title="Assessments by control"
      description="Every control assessment the gateway stored, grouped by control, outcome and verdict source."
    >
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label={LABELS.liveVerdict.short}
          value={live.total}
          caption={<SplitCaption counts={live} />}
        />
        <StatCard
          label={LABELS.fixtureVerdict.full}
          value={fixture.total}
          caption={<SplitCaption counts={fixture} />}
        />
        <StatCard
          label={NO_MODEL_CALL_LABEL}
          value={noModelCall.total}
          caption={<SplitCaption counts={noModelCall} />}
        />
        <StatCard
          label={NO_VERDICT_LABEL}
          value={noVerdict.total}
          tone="attention"
          caption={<SplitCaption counts={noVerdict} />}
        />
      </div>
      <p className="text-xs text-muted-foreground">
        Semantic assessments only. A fixture verdict comes from a labelled test double: it shows
        that the gateway handles a verdict, not how well a model detects anything.
      </p>
      {view.assessments.length === 0 ? (
        <EmptyState
          title="No assessments yet"
          description="No control has assessed anything for this organization."
        />
      ) : (
        <AssessmentsTable rows={view.assessments} />
      )}
      <p className="text-xs text-muted-foreground">
        Matched rule names are in the sanitized audit export, not in this summary.
      </p>
    </Panel>
  );
}

function AssessmentsTable({ rows }: { rows: AssessmentRow[] }) {
  return (
    <Table>
      <TableCaption className="sr-only">
        Assessments by control, outcome and verdict source
      </TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Control</TableHead>
          <TableHead>Outcome</TableHead>
          <TableHead>Verdict source</TableHead>
          <TableHead>Source</TableHead>
          <TableHead className="text-right">Count</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow
            key={`${row.controlClass}|${row.controlId}|${row.outcome}|${row.verdictSource}|${row.source}`}
          >
            <TableCell>
              <div className="flex flex-wrap items-center gap-2">
                <Code>{row.controlId}</Code>
                <Badge variant="outline">{row.controlClass}</Badge>
              </div>
            </TableCell>
            <TableCell>
              <Code>{row.outcome}</Code>
            </TableCell>
            <TableCell>
              {row.verdictSource && row.outcome !== "error" ? (
                <VerdictSourceLabel verdictSource={row.verdictSource} />
              ) : (
                <span className="text-muted-foreground">
                  {row.outcome === "error" ? "no verdict" : "none"}
                </span>
              )}
            </TableCell>
            <TableCell>
              <SourceBadge source={row.source} />
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatCount(row.count)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

export function RunsPanel({ view }: { view: PostureView }) {
  return (
    <Panel id="posture-runs" title="Runs" description="Run status as stored.">
      {view.runs.length === 0 ? (
        <EmptyState title="No runs yet" description="No run exists for this organization." />
      ) : (
        <ul className="flex flex-wrap gap-2">
          {view.runs.map((run) => (
            <li key={run.status}>
              <Badge variant="outline">
                {run.status.replaceAll("_", " ")} · {formatCount(run.count)}
              </Badge>
            </li>
          ))}
          <li>
            <Badge variant="secondary">total · {formatCount(view.runTotal)}</Badge>
          </li>
        </ul>
      )}
    </Panel>
  );
}

export function UsagePanel({ view }: { view: PostureView }) {
  return (
    <Panel
      id="posture-usage"
      title="Model usage"
      description="Dispatched calls and provider-reported tokens by purpose. Agent and security consumption are metered apart."
    >
      <UsageTable rows={view.usage} />
      <p className="text-xs text-muted-foreground">
        {formatCount(view.judgeSecurityCalls)} of the security-purpose calls are linked to judge
        probes. Tokens are the provider&apos;s reported counts; the summary carries no price, so no
        cost is shown or estimated.
      </p>
    </Panel>
  );
}

function UsageTable({ rows }: { rows: UsageRow[] }) {
  return (
    <Table>
      <TableCaption className="sr-only">Model usage by purpose</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Purpose</TableHead>
          <TableHead className="text-right">Dispatched</TableHead>
          <TableHead className="text-right">Completed</TableHead>
          <TableHead className="text-right">Failed</TableHead>
          <TableHead className="text-right">Usage unknown</TableHead>
          <TableHead className="text-right">In flight</TableHead>
          <TableHead className="text-right">Settled tokens</TableHead>
          <TableHead className="text-right">Held tokens</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow key={row.purpose}>
            <TableCell>
              <Code>{row.purpose}</Code>
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatCount(row.dispatched)}</TableCell>
            <TableCell className="text-right tabular-nums">{formatCount(row.completed)}</TableCell>
            <TableCell className="text-right tabular-nums">{formatCount(row.failed)}</TableCell>
            <TableCell className="text-right tabular-nums">
              {row.usageUnknown > 0 ? (
                <Badge variant="destructive">{formatCount(row.usageUnknown)}</Badge>
              ) : (
                formatCount(row.usageUnknown)
              )}
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatCount(row.inFlight)}</TableCell>
            <TableCell className="text-right tabular-nums">
              {formatCount(row.settledTokens)}
            </TableCell>
            <TableCell className="text-right tabular-nums">{formatCount(row.heldTokens)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

export function TimingsPanel({ rows }: { rows: TimingRow[] }) {
  return (
    <Panel
      id="posture-timings"
      title="Measured durations"
      description="Spans the gateway observed, kept apart by phase: deterministic checks and model time are separate rows."
    >
      {rows.length === 0 ? (
        <EmptyState
          title="No timings recorded yet"
          description="No span has been measured for this organization."
        />
      ) : (
        <Table>
          <TableCaption className="sr-only">Observed durations by phase</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Phase</TableHead>
              <TableHead className="text-right">Samples</TableHead>
              <TableHead className="text-right">Failed</TableHead>
              <TableHead className="text-right">Median</TableHead>
              <TableHead className="text-right">p95</TableHead>
              <TableHead className="text-right">Max</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.phase}>
                <TableCell>
                  <Code>{row.phase}</Code>
                </TableCell>
                <TableCell className="text-right tabular-nums">{formatCount(row.count)}</TableCell>
                <TableCell className="text-right tabular-nums">{formatCount(row.failed)}</TableCell>
                <TableCell className="text-right tabular-nums">{row.median}</TableCell>
                <TableCell className="text-right tabular-nums">{row.p95}</TableCell>
                <TableCell className="text-right tabular-nums">{row.max}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <p className="text-xs text-muted-foreground">
        Durations are measured, never estimated, and say nothing about cost.
      </p>
    </Panel>
  );
}

function Panel({
  id,
  title,
  description,
  children,
}: {
  id: string;
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    // min-w-0: a grid item may shrink below its table's width; the table scrolls inside its card.
    <section aria-labelledby={id} className="min-w-0">
      <Card>
        <CardHeader>
          <CardTitle id={id} className="text-base">
            {title}
          </CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">{children}</CardContent>
      </Card>
    </section>
  );
}
