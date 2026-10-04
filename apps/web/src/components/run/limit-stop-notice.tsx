import type { RunState, RunUsage } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Badge } from "@workspace/ui/components/badge";
import { countOf, pluralize } from "@/lib/plural";
import { reasonLabel } from "./labels";
import {
  describeUsage,
  formatCount,
  limitStopKind,
  type LimitStopKind,
  type UsageView,
} from "./usage-model";

export interface LimitStopNoticeProps {
  /** The persisted run state: the terminal reason is read from it and nowhere else. */
  run: RunState;
  /** The run's usage at the stop; without it the reason is still shown. */
  usage: RunUsage | null;
  /** The recorded explanation from `terminalSafeMessage(events)`. */
  safeMessage?: string | null;
}

const KIND_TEXT: Record<LimitStopKind, string> = {
  model_allowance: "The run reached its allowance of model requests or tokens.",
  security_allowance: "The security check's allowance is used up, so nothing more was released.",
  time: "The run's time allowance passed.",
};

function accounting(view: UsageView): { accounted: boolean; unaccounted: string[] } {
  const unaccounted = view.purposes.filter((row) => !row.accounted).map((row) => row.label);
  return { accounted: unaccounted.length === 0, unaccounted };
}

/**
 * WEB-17: the display of a limit-triggered stop: the persisted terminal reason, which limits the run's
 * own ledger shows at their cap, and the usage at the stop. It renders nothing for a run that did not
 * stop on a limit. "No further model request is sent" follows from the run being paused or stopped; it
 * is not a measurement, and the usage is as of the latest read.
 */
export function LimitStopNotice({ run, usage, safeMessage = null }: LimitStopNoticeProps) {
  const kind = limitStopKind(run);
  if (kind === null) {
    return null;
  }
  const view = usage === null ? null : describeUsage(usage);
  const ledger = view?.ledger ?? null;
  const reachedCalls = ledger?.calls.filter((row) => row.reached) ?? [];
  const checked = view === null ? null : accounting(view);
  return (
    <Alert data-part="limit-stop" data-kind={kind} data-status={run.status}>
      <AlertTitle className="flex flex-wrap items-center gap-2">
        {run.status === "paused" ? "Paused at a limit" : "Stopped at a limit"}
        <Badge variant="outline">{run.terminalReason}</Badge>
      </AlertTitle>
      <AlertDescription className="flex flex-col gap-2">
        <p data-part="reason">
          <span className="font-medium text-foreground">Recorded reason: </span>
          {reasonLabel(run.terminalReason)} (<code className="text-xs">{run.terminalReason}</code>)
        </p>
        <p>{KIND_TEXT[kind]}</p>
        {safeMessage !== null ? (
          <p data-part="recorded-explanation">
            <span className="font-medium text-foreground">Recorded explanation: </span>
            {safeMessage}
          </p>
        ) : null}
        <p>
          The run is {run.status}: no further model request is sent for it, and a request is only
          sent when its reserved estimate fits in the allowance that is left.
        </p>
        {view === null || ledger === null ? (
          <p data-part="usage-missing">
            {view === null
              ? "The usage at the stop was not available to this page."
              : "No allowance ledger exists for this run, so there is no usage to show."}
          </p>
        ) : (
          <div data-part="usage-at-stop" className="flex flex-col gap-1">
            <span className="font-medium text-foreground">Usage at the stop (latest read)</span>
            <ul className="list-disc pl-5">
              {ledger.calls.map((row) => (
                <li key={row.name} data-reached={row.reached}>
                  {row.name}: {row.used} of {row.limit}
                  {row.reached ? " (limit reached)" : ""}
                </li>
              ))}
              {ledger.allowances.map((row) => (
                <li key={row.name}>
                  {row.name}: {formatCount(row.reported)} reported
                  {row.reserved > 0 ? ` + ${formatCount(row.reserved)} reserved` : ""}
                  {row.limit !== null
                    ? ` of ${formatCount(row.limit)} (${formatCount(row.available ?? 0)} available)`
                    : " (no separate limit)"}
                </li>
              ))}
            </ul>
            {reachedCalls.length === 0 && kind === "model_allowance" ? (
              <span>No request limit is at its cap.</span>
            ) : null}
            {checked !== null && checked.accounted ? (
              <span data-part="accounted">
                Every dispatched request is accounted for: each has an outcome, is still running or
                holds a reservation.
              </span>
            ) : (
              <span data-part="unaccounted" className="font-medium text-destructive">
                Unaccounted requests for: {checked?.unaccounted.join(", ")}.
              </span>
            )}
            {view.uncertain ? (
              <span data-part="uncertain">
                {countOf(view.unknownCalls, "request")}{" "}
                {pluralize(view.unknownCalls, "has", "have")} unknown usage;{" "}
                {pluralize(view.unknownCalls, "its", "their")} reservation stays held and the usage
                is uncertain, not zero.
              </span>
            ) : null}
          </div>
        )}
      </AlertDescription>
    </Alert>
  );
}
