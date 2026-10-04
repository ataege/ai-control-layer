"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import type { Passport, RunState, RunUsage, SafeEvent } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Button, buttonVariants } from "@workspace/ui/components/button";
import { Card, CardContent, CardHeader, CardTitle } from "@workspace/ui/components/card";
import { ErrorState } from "@workspace/ui/components/error-state";
import { LoadingState } from "@workspace/ui/components/loading-state";
import { PageHeader } from "@workspace/ui/components/page-header";
import { CancelRunButton } from "@/components/approval/cancel-run-button";
import { PassportPanel } from "@/components/passport/passport-panel";
import { ClassificationBadge } from "@/components/report/classification-badge";
import { ExportDenial, isExportDenial } from "@/components/report/export-denial";
import { LimitStopNotice } from "@/components/run/limit-stop-notice";
import { terminalSafeMessage } from "@/components/run/event-model";
import { RunEventTimeline } from "@/components/run/run-event-timeline";
import { RunStatePanel } from "@/components/run/run-state-panel";
import { RunUsagePanel } from "@/components/run/run-usage-panel";
import { awaitingActionId, reportsOfRun } from "@/components/run/run-page-model";
import { createRunPoller } from "@/components/run/run-poller";
import { fetchPassport } from "@/lib/clients/passport-client";
import { getSafeMessage } from "@/lib/product-client";

const POLL_INTERVAL_MS = 3000;

function reportHref(runId: string, reportId: string): string {
  return `/runs/${encodeURIComponent(runId)}/reports/${encodeURIComponent(reportId)}`;
}

/**
 * The run page. Everything on it is read from the real API through the web's own /api routes: the
 * persisted run state, the usage ledger, the sanitized events and the passport. Nothing is inferred
 * here; the components only display what the gateway stored.
 */
export default function RunPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [run, setRun] = useState<RunState | undefined>(undefined);
  const [usage, setUsage] = useState<RunUsage | null>(null);
  const [events, setEvents] = useState<SafeEvent[]>([]);
  const [passport, setPassport] = useState<Passport | null>(null);
  const [passportProblem, setPassportProblem] = useState<string | null>(null);
  const [error, setError] = useState<string | undefined>(undefined);
  const [isInitialLoading, setIsInitialLoading] = useState(true);

  useEffect(() => {
    const abortController = new AbortController();
    const { signal } = abortController;
    let isMounted = true;
    const poller = createRunPoller(id, signal);
    let pollTimeoutId: ReturnType<typeof setTimeout> | undefined;

    async function loadPassport() {
      const result = await fetchPassport(id, { signal });
      if (!isMounted) return;
      if (result.ok) {
        setPassport(result.data);
      } else if (result.error.kind !== "aborted") {
        setPassportProblem(getSafeMessage(result.error));
      }
    }

    // One refresh of the run (see run-poller.ts). Returns whether polling should continue.
    async function refresh(): Promise<boolean> {
      const result = await poller.refresh();
      if (!isMounted || result === null) return false;
      setEvents(result.events);
      if (result.usage !== undefined) setUsage(result.usage);
      if (result.run !== undefined) setRun(result.run);
      setError(result.error);
      setIsInitialLoading(false);
      return result.keepPolling;
    }

    async function poll() {
      let keepPolling = true;
      try {
        keepPolling = await refresh();
      } catch (caught: unknown) {
        if (isMounted && !(caught instanceof Error && caught.name === "AbortError")) {
          setError("An error occurred while loading the run.");
          setIsInitialLoading(false);
        }
      }
      if (isMounted && keepPolling) {
        pollTimeoutId = setTimeout(() => void poll(), POLL_INTERVAL_MS);
      }
    }

    void loadPassport();
    void poll();

    return () => {
      isMounted = false;
      abortController.abort();
      if (pollTimeoutId !== undefined) clearTimeout(pollTimeoutId);
    };
  }, [id]);

  if (isInitialLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <LoadingState />
      </div>
    );
  }

  // The first read failed completely: there is no run state to show.
  if (error !== undefined && run === undefined) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <ErrorState
          title="Could not load run"
          description={error}
          action={
            <Button variant="outline" onClick={() => window.location.reload()}>
              Retry
            </Button>
          }
        />
      </div>
    );
  }

  const waitingAction = run === undefined ? null : awaitingActionId(run, events);
  const reports = run === undefined ? [] : reportsOfRun(run, events);
  const exportDenials = events.filter(isExportDenial);

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <PageHeader
        title="Task run"
        description={`Run ${id}`}
        actions={
          // The button shows only the server-recorded state and offers nothing for a finished run.
          <CancelRunButton runId={id} runStatus={run?.status} onCancelled={setRun} />
        }
      />

      {/* A failed update keeps the real run and events on screen and says what failed. */}
      {error !== undefined ? (
        <Card className="border-destructive/50 bg-destructive/5 text-destructive">
          <CardContent className="pt-6">
            <p className="text-sm font-medium">Update failed</p>
            <p className="text-sm">{error}</p>
          </CardContent>
        </Card>
      ) : null}

      {run !== undefined ? (
        <>
          <RunStatePanel
            run={run}
            safeMessage={terminalSafeMessage(events)}
            reportHref={(reportId) => reportHref(id, reportId)}
          />
          {waitingAction !== null ? (
            <Alert data-testid="review-link">
              <AlertTitle>A reviewer must decide</AlertTitle>
              <AlertDescription className="flex flex-col gap-2">
                <p>
                  The run waits for a decision on one exact action. Nothing runs for it until a
                  reviewer approves it.
                </p>
                <Link
                  href={`/runs/${encodeURIComponent(id)}/review/${encodeURIComponent(waitingAction)}`}
                  className={buttonVariants({ variant: "outline" })}
                >
                  Review the action
                </Link>
              </AlertDescription>
            </Alert>
          ) : null}
          <LimitStopNotice run={run} usage={usage} safeMessage={terminalSafeMessage(events)} />
        </>
      ) : null}

      <div className="grid gap-6 md:grid-cols-2">
        <div className="space-y-6">
          {passport !== null ? (
            <PassportPanel passport={passport} />
          ) : (
            <Card data-testid="passport-unavailable">
              <CardHeader>
                <CardTitle>Task Passport</CardTitle>
              </CardHeader>
              <CardContent className="text-sm text-muted-foreground">
                {passportProblem === null
                  ? "Loading the passport."
                  : `The passport could not be read: ${passportProblem}`}
              </CardContent>
            </Card>
          )}
        </div>
        <div className="space-y-6">{usage !== null ? <RunUsagePanel usage={usage} /> : null}</div>
      </div>

      {reports.length > 0 ? (
        <Card data-testid="run-reports">
          <CardHeader>
            <CardTitle>Reports of this run</CardTitle>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2 text-sm">
              {reports.map((report) => (
                <li key={report.reportId} className="flex flex-wrap items-center gap-2">
                  <Link
                    href={reportHref(id, report.reportId)}
                    className="underline underline-offset-4"
                  >
                    {report.template ?? "Report"} <code className="text-xs">{report.reportId}</code>
                  </Link>
                  {report.classification !== null ? (
                    <ClassificationBadge classification={report.classification} />
                  ) : null}
                  {report.namedByFinalAnswer ? (
                    <span className="text-xs text-muted-foreground">named by the final answer</span>
                  ) : null}
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      ) : null}

      {exportDenials.map((event) => (
        <ExportDenial key={event.eventId} event={event} />
      ))}

      <RunEventTimeline events={events} />
    </div>
  );
}
