"use client";

import * as React from "react";
import type { ReportView } from "@workspace/contracts";
import { LoadingState } from "@workspace/ui/components/loading-state";
import { FailureState } from "@/components/errors";
import { getReport } from "@/lib/clients/reports-client";
import type { Failure } from "@/lib/errors/failure";
import { describeReportFailure } from "./report-errors";
import { ReportContent } from "./report-content";

type PanelState =
  | { kind: "loading" }
  | { kind: "ready"; report: ReportView }
  | { kind: "failed"; failure: Failure; requestId?: string };

/** Loads one stored report through the same-origin route and shows it, or why it cannot be shown. */
export function ReportPanel({ runId, reportId }: { runId: string; reportId: string }) {
  const [attempt, setAttempt] = React.useState(0);
  const [state, setState] = React.useState<PanelState>({ kind: "loading" });

  React.useEffect(() => {
    const controller = new AbortController();
    void getReport(runId, reportId, { signal: controller.signal }).then((result) => {
      if (controller.signal.aborted) return;
      setState(
        result.ok
          ? { kind: "ready", report: result.data }
          : {
              kind: "failed",
              failure: describeReportFailure(result.error),
              requestId: result.requestId,
            },
      );
    });
    return () => controller.abort();
  }, [runId, reportId, attempt]);

  if (state.kind === "loading") return <LoadingState label="Loading the stored report" />;
  if (state.kind === "failed") {
    return (
      <FailureState
        failure={state.failure}
        requestId={state.requestId}
        onRetry={() => {
          setState({ kind: "loading" });
          setAttempt((current) => current + 1);
        }}
      />
    );
  }
  return <ReportContent report={state.report} />;
}
