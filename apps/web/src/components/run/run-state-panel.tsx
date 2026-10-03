import type { RunState } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Badge } from "@workspace/ui/components/badge";
import { describeRunState } from "./labels";

export interface RunStatePanelProps {
  /** The persisted run state (X-11); the browser never infers a state of its own. */
  run: RunState;
  /** The recorded explanation of the run's last paused, failed, stopped or completed event. */
  safeMessage?: string | null;
  /** Where a named report can be opened; without it the report ids are shown as text. */
  reportHref?: (reportId: string) => string;
}

/**
 * WEB-10: the run's waiting or terminal state in business language with its recorded reason. It
 * tells apart a justified interruption (paused at its allowance, waiting for a reviewer) from an
 * inability to complete (failed), a deliberate stop, and an uncertain result that needs an operator.
 * An unknown state is shown as unknown, never as success.
 */
export function RunStatePanel({ run, safeMessage = null, reportHref }: RunStatePanelProps) {
  const view = describeRunState(run, safeMessage);
  return (
    <Alert
      variant={view.tone === "failed" || view.tone === "unknown" ? "destructive" : "default"}
      data-state={view.status}
      data-tone={view.tone}
      data-outcome={view.outcomeKind}
    >
      <AlertTitle className="flex flex-wrap items-center gap-2">
        {view.title}
        <Badge variant="outline">{view.status}</Badge>
      </AlertTitle>
      <AlertDescription className="flex flex-col gap-2">
        <p>{view.summary}</p>
        {view.reasonText !== null ? (
          <p data-part="reason">
            <span className="font-medium text-foreground">Why: </span>
            {view.reasonText}
            {view.reasonCode !== null ? (
              <>
                {" "}
                (<code className="text-xs">{view.reasonCode}</code>)
              </>
            ) : null}
          </p>
        ) : null}
        {view.safeMessage !== null ? (
          <p data-part="recorded-explanation">
            <span className="font-medium text-foreground">Recorded explanation: </span>
            {view.safeMessage}
          </p>
        ) : null}
        <p data-part="next">
          <span className="font-medium text-foreground">What happens next: </span>
          {view.next}
        </p>
        {view.reportIds.length > 0 ? (
          <div data-part="reports">
            <span className="font-medium text-foreground">Reports named by the final answer:</span>
            <ul className="mt-1 list-disc pl-5">
              {view.reportIds.map((reportId) => (
                <li key={reportId}>
                  {reportHref !== undefined ? (
                    <a className="underline" href={reportHref(reportId)}>
                      <code className="text-xs">{reportId}</code>
                    </a>
                  ) : (
                    <code className="text-xs">{reportId}</code>
                  )}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        {view.contractProblem !== null ? (
          <p data-part="contract-problem" className="font-medium text-destructive">
            {view.contractProblem}
          </p>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}
