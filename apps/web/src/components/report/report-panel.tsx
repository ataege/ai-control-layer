"use client";

import * as React from "react";
import type { ReportView } from "@workspace/contracts";
import { Button } from "@workspace/ui/components/button";
import { ErrorState } from "@workspace/ui/components/error-state";
import { LoadingState } from "@workspace/ui/components/loading-state";
import { getReport } from "@/lib/clients/reports-client";
import { describeReportFailure, type ReportFailure } from "./report-errors";
import { ReportContent } from "./report-content";

type PanelState =
  | { kind: "loading" }
  | { kind: "ready"; report: ReportView }
  | { kind: "failed"; failure: ReportFailure };

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
          : { kind: "failed", failure: describeReportFailure(result.error) },
      );
    });
    return () => controller.abort();
  }, [runId, reportId, attempt]);

  if (state.kind === "loading") return <LoadingState label="Loading the stored report" />;
  if (state.kind === "failed") {
    return (
      <ErrorState
        title={state.failure.title}
        description={state.failure.description}
        action={
          <Button
            variant="outline"
            onClick={() => {
              setState({ kind: "loading" });
              setAttempt((current) => current + 1);
            }}
          >
            Try again
          </Button>
        }
      />
    );
  }
  return <ReportContent report={state.report} />;
}
