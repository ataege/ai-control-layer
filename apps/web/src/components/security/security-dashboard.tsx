"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { DownloadIcon, RefreshCwIcon } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Button } from "@workspace/ui/components/button";
import { ErrorState } from "@workspace/ui/components/error-state";
import { Skeleton } from "@workspace/ui/components/skeleton";

import { getSecuritySummary, type SecurityFailure } from "@/lib/clients/security-client";

import {
  ControlsPanel,
  DecisionsTable,
  HeadlineCards,
  RunsPanel,
  TimingsPanel,
  UsagePanel,
} from "./posture-panels";
import { buildPostureView, type PostureView } from "./posture-view";

type DashboardState =
  | { status: "loading" }
  | { status: "ready"; view: PostureView; requestId?: string; durationMs: number }
  | { status: "failed"; failure: SecurityFailure };

/** The security posture dashboard (WEB-30): one summary, as served, nothing invented. */
export function SecurityDashboard() {
  const [state, setState] = useState<DashboardState>({ status: "loading" });
  // Each refresh bumps the count; a request that settles records which count it answered, so
  // "refreshing" is derived and the effect never sets state synchronously.
  const [refreshCount, setRefreshCount] = useState(0);
  const [settledCount, setSettledCount] = useState(-1);
  const refreshing = settledCount !== refreshCount;
  const refresh = () => setRefreshCount((count) => count + 1);

  useEffect(() => {
    const controller = new AbortController();
    void getSecuritySummary({ signal: controller.signal }).then((result) => {
      if (controller.signal.aborted) return;
      setSettledCount(refreshCount);
      setState(
        result.ok
          ? {
              status: "ready",
              view: buildPostureView(result.summary),
              requestId: result.requestId,
              durationMs: result.durationMs,
            }
          : { status: "failed", failure: result.failure },
      );
    });
    return () => controller.abort();
  }, [refreshCount]);

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground" aria-live="polite">
          {state.status === "ready" ? (
            <>
              As of{" "}
              <time dateTime={state.view.generatedAt}>
                {formatGeneratedAt(state.view.generatedAt)}
              </time>
              {state.requestId ? <> · request {state.requestId}</> : null}
            </>
          ) : state.status === "loading" ? (
            "Reading the security summary…"
          ) : null}
        </p>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={refresh} disabled={refreshing}>
            <RefreshCwIcon aria-hidden="true" className={refreshing ? "animate-spin" : undefined} />
            Refresh
          </Button>
          <Button variant="outline" asChild>
            <Link href="/security/export">
              <DownloadIcon aria-hidden="true" />
              Audit export
            </Link>
          </Button>
        </div>
      </div>

      {state.status === "loading" ? <DashboardSkeleton /> : null}
      {state.status === "failed" ? (
        <FailureState failure={state.failure} onRetry={refresh} />
      ) : null}
      {state.status === "ready" ? <Dashboard view={state.view} /> : null}
    </div>
  );
}

function Dashboard({ view }: { view: PostureView }) {
  return (
    <>
      <Alert>
        <AlertTitle>What these figures are</AlertTitle>
        <AlertDescription>
          Every number is a count of records the gateway stored, read at the time above. Judge
          probes (inputs a judge submitted through the live test entry) are counted apart from agent
          runs. Durations are measured, not estimated, and no cost is shown.
        </AlertDescription>
      </Alert>
      <HeadlineCards view={view} />
      <DecisionsTable rows={view.decisions} />
      <ControlsPanel view={view} />
      <div className="grid gap-6 lg:grid-cols-2">
        <RunsPanel view={view} />
        <TimingsPanel rows={view.timings} />
      </div>
      <UsagePanel view={view} />
    </>
  );
}

function FailureState({ failure, onRetry }: { failure: SecurityFailure; onRetry: () => void }) {
  const title =
    failure.kind === "unauthorized"
      ? "Sign in to see the security posture"
      : failure.kind === "forbidden"
        ? "Not available to this account"
        : "The security posture could not be loaded";
  return (
    <ErrorState
      title={title}
      description={failure.message}
      action={
        failure.kind === "unauthorized" ? (
          <Button asChild>
            <Link href="/login?callbackUrl=/security">Sign in</Link>
          </Button>
        ) : failure.kind === "forbidden" ? null : (
          <Button variant="outline" onClick={onRetry}>
            Try again
          </Button>
        )
      }
    />
  );
}

function DashboardSkeleton() {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label="Loading the security posture">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className="h-24 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-64 rounded-xl" />
      <Skeleton className="h-48 rounded-xl" />
    </div>
  );
}

function formatGeneratedAt(generatedAt: string): string {
  const parsed = new Date(generatedAt);
  return Number.isNaN(parsed.getTime()) ? generatedAt : parsed.toLocaleString();
}
