import type { AssessmentRecord, SafeEvent } from "@workspace/contracts";
import { Badge } from "@workspace/ui/components/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { DecisionBadges } from "./decision-badges";
import { describeEvent, summarizeEvents, type EventKind, type EventView } from "./event-model";

const KIND_LABEL: Record<EventKind, string> = {
  state: "Run state",
  attempt: "Attempt",
  effect: "Completed effect",
  uncertain: "Outcome unknown",
  review: "Review",
  control: "Security check",
  guidance: "Guidance",
  unknown: "Unknown event",
};

const KIND_VARIANT: Record<EventKind, "default" | "secondary" | "outline" | "destructive"> = {
  state: "secondary",
  attempt: "outline",
  effect: "default",
  uncertain: "destructive",
  review: "secondary",
  control: "outline",
  guidance: "secondary",
  unknown: "destructive",
};

// A fixed UTC time of day keeps the server and browser renderings identical.
function timeOfDay(isoTime: string): string {
  const match = /T(\d{2}:\d{2}:\d{2})/.exec(isoTime);
  return match !== null && isoTime.endsWith("Z") ? `${match[1]} UTC` : isoTime;
}

function EventRow({ view }: { view: EventView }) {
  return (
    <li
      data-kind={view.kind}
      data-tone={view.tone}
      data-event-type={view.eventType}
      className="flex flex-col gap-1 border-b pb-3 last:border-0 last:pb-0"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex flex-wrap items-center gap-2">
          <Badge variant={KIND_VARIANT[view.kind]}>{KIND_LABEL[view.kind]}</Badge>
          <span className="text-sm font-medium">{view.title}</span>
        </span>
        <time className="text-xs text-muted-foreground" dateTime={view.occurredAt}>
          {timeOfDay(view.occurredAt)}
        </time>
      </div>
      {view.unrecognized ? (
        <p className="text-sm text-muted-foreground">
          This interface does not recognize this event type, so it shows the type and nothing else.
        </p>
      ) : null}
      {view.reasonCode !== null ? (
        <p className="text-sm">
          <span className="text-muted-foreground">Reason: </span>
          {view.reasonText} (<code className="text-xs">{view.reasonCode}</code>)
        </p>
      ) : null}
      {view.safeMessage !== null && view.safeMessage !== view.reasonText ? (
        <p className="text-sm text-muted-foreground">{view.safeMessage}</p>
      ) : null}
      {view.rule !== null ? (
        <p className="text-xs text-muted-foreground">Applicable rule: {view.rule}</p>
      ) : null}
      {view.effectText !== null ? (
        <p
          data-effect={view.kind === "effect" ? "completed" : "none"}
          className={
            view.kind === "effect" ? "text-sm font-medium" : "text-sm text-muted-foreground"
          }
        >
          {view.effectText}
        </p>
      ) : null}
      {view.correctionRoute !== null ? <p className="text-sm">{view.correctionRoute}</p> : null}
      {view.rejectionText !== null ? <p className="text-sm">{view.rejectionText}</p> : null}
      {view.replayLabel !== null ? (
        <p className="text-sm">
          <Badge variant="secondary">Labelled replay</Badge> {view.replayLabel}
        </p>
      ) : null}
      <DecisionBadges badges={view.badges} />
      {view.facts.length > 0 ? (
        <dl className="mt-1 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
          {view.facts.map((fact) => (
            <div key={fact.label} className="contents">
              <dt>{fact.label}</dt>
              <dd className="text-foreground">{fact.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}
    </li>
  );
}

export interface RunEventTimelineProps {
  /** The sanitized X-12 events of one run, in the order the API returned them. */
  events: readonly SafeEvent[];
  /** Stored control assessments of the same run; when given, badges name the exact controls. */
  assessments?: readonly AssessmentRecord[];
}

/**
 * WEB-09 and WEB-32: the run's events in order, with attempted operations kept apart from completed
 * effects, each denial showing its reason and rule, replays labelled, and the hybrid control decision
 * of each check as badges. Display only: it fetches nothing and computes no state of its own.
 */
export function RunEventTimeline({ events, assessments = [] }: RunEventTimelineProps) {
  const summary = summarizeEvents(events);
  const views = events.map((event) => describeEvent(event, assessments));
  return (
    <Card>
      <CardHeader>
        <CardTitle>What was attempted and what happened</CardTitle>
        <CardDescription>
          An attempt and its decision are not an effect: only the rows marked Completed effect
          changed anything.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <dl
          aria-label="Attempts and effects"
          className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm sm:grid-cols-4"
        >
          <div>
            <dt className="text-xs text-muted-foreground">Attempted operations</dt>
            <dd data-summary="attempted">{summary.attemptedOperations}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">Denied proposals (corrections)</dt>
            <dd data-summary="denied">{summary.deniedProposals}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">Completed effects</dt>
            <dd data-summary="effects">
              {summary.effects.reads} reads · {summary.effects.reportsStored} reports stored ·{" "}
              {summary.effects.messagesQueued} messages queued (simulated outbox)
            </dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">Outcome unknown</dt>
            <dd data-summary="uncertain">{summary.uncertain}</dd>
          </div>
        </dl>
        {views.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No events have been recorded for this run yet.
          </p>
        ) : (
          <ol className="flex flex-col gap-3">
            {views.map((view) => (
              <EventRow key={view.id} view={view} />
            ))}
          </ol>
        )}
      </CardContent>
    </Card>
  );
}
