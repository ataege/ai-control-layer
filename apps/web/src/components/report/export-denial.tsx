import Link from "next/link";
import { ShieldAlertIcon } from "lucide-react";
import type { SafeEvent } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { ReplayLabel } from "@/components/labels";

/** The event this explanation is for: the gateway's export denial for an inherited restriction. */
export function isExportDenial(event: SafeEvent): boolean {
  return (
    event.eventType === "report.export_denied" && event.reasonCode === "report_export_restricted"
  );
}

/**
 * Explains a `report.export_denied` event (reason `report_export_restricted`): the rule, that the
 * recipient itself was permitted, and the vendor report the task continues with. It is a
 * denial, never an approval request, so it has no action to take. It renders nothing for any other
 * event, and uses only what the event carries; a replayed proposal carries the shared replay label.
 */
export function ExportDenial({ event }: { event: SafeEvent }) {
  if (!isExportDenial(event)) return null;
  const summary = event.maskedSummary;
  const restrictedReportHref =
    event.runId !== null && summary.reportId !== null
      ? `/runs/${encodeURIComponent(event.runId)}/reports/${encodeURIComponent(summary.reportId)}`
      : null;

  return (
    <Alert data-testid="export-denial" data-reason="report_export_restricted">
      <ShieldAlertIcon aria-hidden="true" />
      <AlertTitle>Export denied: the report inherits an Internal only restriction</AlertTitle>
      <AlertDescription className="space-y-2">
        <p>
          <strong>Rule.</strong> A report built from a source with an Internal only restriction
          inherits it. It cannot be sent to a vendor, whatever its title says and even to a
          recipient the task may otherwise use.
        </p>
        <p>
          <strong>The recipient was permitted.</strong> The task&apos;s registered recipient passed
          its checks; the denial comes from the report&apos;s own stored restriction, not from the
          destination.
        </p>
        {summary.alternativeTemplate !== null ? (
          <p data-testid="export-denial-continuation">
            <strong>The task continues with a vendor report.</strong> A separate report is rendered
            from approved database fields only, with the template{" "}
            <code className="font-mono text-xs">{summary.alternativeTemplate}</code>. It carries no
            internal notes.
          </p>
        ) : (
          <p data-testid="export-denial-continuation">
            <strong>No alternative was offered</strong> for this run.
          </p>
        )}
        <p>
          This is a denial, not an approval request: the restriction cannot be overridden by a
          reviewer.
        </p>
        {summary.safeMessage !== null ? (
          <p className="text-muted-foreground">Gateway message: {summary.safeMessage}</p>
        ) : null}
        {/* The shared label (WEB-13): it renders nothing for a model proposal. */}
        <ReplayLabel replaySource={summary.replaySource} />
        {restrictedReportHref !== null ? (
          <p>
            <Link href={restrictedReportHref} className="underline underline-offset-4">
              View the restricted report
            </Link>
          </p>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}
