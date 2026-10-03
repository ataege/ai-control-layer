"use client";

import * as React from "react";
import type { RunState } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Button } from "@workspace/ui/components/button";
import { ConfirmDialog } from "@workspace/ui/components/confirm-dialog";

import { cancelRun, type ClientFailure } from "@/lib/clients/actions-client";

import { CANCELLATION_EXPLANATION, cancelledStateText, isFinishedRun } from "./cancel-run-text";

type CancelState =
  | { status: "idle" }
  | { status: "sending" }
  | { status: "done"; run: RunState }
  | { status: "failed"; failure: ClientFailure };

/**
 * WEB-15: the cancel control for a run. It asks for confirmation with the limitation spelled
 * out, then shows the state the server recorded. `onCancelled` lets the run page refresh its own
 * view; already committed effects stay listed there.
 */
export function CancelRunButton({
  runId,
  runStatus,
  onCancelled,
}: {
  runId: string;
  /** The run's current status; a finished run offers no cancel. */
  runStatus?: RunState["status"];
  onCancelled?: (run: RunState) => void;
}) {
  const [state, setState] = React.useState<CancelState>({ status: "idle" });

  if (runStatus && isFinishedRun(runStatus)) return null;

  const confirm = async () => {
    setState({ status: "sending" });
    const result = await cancelRun(runId);
    if (result.ok) {
      setState({ status: "done", run: result.data });
      onCancelled?.(result.data);
    } else {
      setState({ status: "failed", failure: result.failure });
    }
  };

  return (
    <div className="grid gap-3">
      <ConfirmDialog
        title="Cancel this run?"
        description={CANCELLATION_EXPLANATION}
        confirmLabel="Cancel run"
        cancelLabel="Keep running"
        confirmVariant="destructive"
        onConfirm={() => void confirm()}
        trigger={
          <Button
            variant="destructive"
            disabled={state.status === "sending" || state.status === "done"}
          >
            {state.status === "sending" ? "Cancelling…" : "Cancel run"}
          </Button>
        }
      />
      {state.status === "done" ? (
        <Alert>
          <AlertTitle>Cancellation recorded</AlertTitle>
          <AlertDescription>
            {cancelledStateText(state.run)} Work already done is not reversed and stays listed.
          </AlertDescription>
        </Alert>
      ) : null}
      {state.status === "failed" ? (
        <Alert variant="destructive">
          <AlertTitle>
            {state.failure.kind === "unconfirmed" ? "Outcome unconfirmed" : "Not cancelled"}
          </AlertTitle>
          <AlertDescription>
            {state.failure.message}
            {state.failure.status ? ` (HTTP ${state.failure.status})` : ""}
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  );
}
