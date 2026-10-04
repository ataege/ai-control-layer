"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { RefreshCwIcon } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Badge } from "@workspace/ui/components/badge";
import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { CopyButton } from "@workspace/ui/components/copy-button";
import { EmptyState } from "@workspace/ui/components/empty-state";
import { ErrorState } from "@workspace/ui/components/error-state";
import { Skeleton } from "@workspace/ui/components/skeleton";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@workspace/ui/components/table";
import type { CatalogStatus } from "@workspace/contracts";

import { getCatalogStatus, type SecurityFailure } from "@/lib/clients/security-client";

import { browserPollEnvironment, pollDelay, schedulePoll } from "./catalog-polling";
import {
  controlRows,
  describeReload,
  isReloadPending,
  shortDigest,
  type ReloadRejection,
  type ReloadState,
} from "./catalog-view";

type PanelState =
  | { status: "loading" }
  | { status: "ready"; catalog: CatalogStatus; requestId?: string }
  | { status: "failed"; failure: SecurityFailure };

/**
 * The active controls and policy revision (WEB-29): which revision the gateway enforces, whether a
 * requested change has taken effect or was rejected, and each control's setting. Everything comes
 * from the API's catalog status; nothing is shown that it does not carry.
 */
export function ActiveControls() {
  const [state, setState] = useState<PanelState>({ status: "loading" });
  // Each refresh (a click, or the background poll) bumps the count; a request that settles records
  // which count it answered, so "refreshing" is derived and the effect never sets state
  // synchronously. Only a click spins the button: the background poll is quiet.
  const [refreshCount, setRefreshCount] = useState(0);
  const [settledCount, setSettledCount] = useState(-1);
  const [clickedCount, setClickedCount] = useState(0);
  const refreshing = settledCount !== refreshCount && clickedCount === refreshCount;
  const refresh = () => {
    setClickedCount(refreshCount + 1);
    setRefreshCount((count) => count + 1);
  };

  useEffect(() => {
    const controller = new AbortController();
    let cancelPoll: (() => void) | undefined;
    void getCatalogStatus({ signal: controller.signal }).then((result) => {
      if (controller.signal.aborted) return;
      setSettledCount(refreshCount);
      if (result.ok) {
        setState({ status: "ready", catalog: result.status, requestId: result.requestId });
      } else {
        setState({ status: "failed", failure: result.failure });
      }
      // Ask again after every answer, so a policy change made from the terminal, a rejection or a
      // recovered outage shows up without a click. The poll is sooner while a revision is pending
      // and waits while the tab is hidden.
      const pending = result.ok && isReloadPending(result.status);
      cancelPoll = schedulePoll(browserPollEnvironment, pollDelay(pending), () =>
        setRefreshCount((count) => count + 1),
      );
    });
    return () => {
      controller.abort();
      cancelPoll?.();
    };
  }, [refreshCount]);

  return (
    <section aria-labelledby="active-controls-title">
      <Card>
        <CardHeader>
          <CardTitle id="active-controls-title" className="text-base">
            Active controls and policy revision
          </CardTitle>
          <CardDescription>
            What the gateway enforces now, and whether a requested policy change has taken effect.
          </CardDescription>
          <CardAction>
            <Button variant="outline" onClick={refresh} disabled={refreshing}>
              <RefreshCwIcon
                aria-hidden="true"
                className={refreshing ? "animate-spin" : undefined}
              />
              Refresh
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {state.status === "loading" ? (
            <Skeleton
              className="h-40 rounded-xl"
              role="status"
              aria-label="Loading the active controls"
            />
          ) : null}
          {state.status === "failed" ? <Failure failure={state.failure} onRetry={refresh} /> : null}
          {state.status === "ready" ? (
            <Catalog catalog={state.catalog} requestId={state.requestId} />
          ) : null}
        </CardContent>
      </Card>
    </section>
  );
}

function Failure({ failure, onRetry }: { failure: SecurityFailure; onRetry: () => void }) {
  if (failure.kind === "not_available") {
    return (
      <EmptyState
        title="Not available yet"
        description="The API does not serve the active control catalog yet, so no revision or control setting is shown. Nothing here is estimated; this panel fills in when the API provides it."
        action={
          <Button variant="outline" onClick={onRetry}>
            Check again
          </Button>
        }
      />
    );
  }
  const title =
    failure.kind === "unauthorized"
      ? "Sign in to see the active controls"
      : failure.kind === "forbidden"
        ? "Not available to this account"
        : failure.kind === "unavailable"
          ? "The active controls could not be read"
          : "The active controls could not be shown";
  const description =
    failure.kind === "unavailable"
      ? "The gateway could not report its active revision, which can mean that no enforceable revision is active. This is an outage, not an empty catalog."
      : failure.message;
  return (
    <ErrorState
      title={title}
      description={description}
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

function Catalog({ catalog, requestId }: { catalog: CatalogStatus; requestId?: string }) {
  const reload = describeReload(catalog);
  const rows = controlRows(catalog);
  return (
    <>
      <ReloadBanner reload={reload} />
      <dl className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Fact label="Active revision" value={numberOrNone(catalog.activeRevisionId)} />
        <Fact label="Requested revision" value={numberOrNone(catalog.requestedRevisionId)} />
        <Fact label="Validated revision" value={numberOrNone(catalog.validatedRevisionId)} />
        <Fact
          label="Signature feed"
          value={
            catalog.feedRevision === null
              ? "none bound"
              : `${catalog.feedRevision} (revision ${numberOrNone(catalog.feedRevisionId)}, ${numberOrNone(catalog.feedRuleCount)} rules)`
          }
        />
        <Digest label="Policy digest (SHA-256)" digest={catalog.policyDigest} />
        <Digest label="Feed digest (SHA-256)" digest={catalog.feedDigest} />
        <Fact
          label="Disabled feed rules"
          value={catalog.disabledRules.length === 0 ? "none" : catalog.disabledRules.join(", ")}
        />
      </dl>
      {rows.length === 0 ? (
        <EmptyState
          title="No controls to show"
          description="No revision has been activated, so the gateway reports no control settings."
        />
      ) : (
        <Table>
          <TableCaption className="sr-only">Controls in the active catalog revision</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Control</TableHead>
              <TableHead>State</TableHead>
              <TableHead>Response</TableHead>
              <TableHead>Threshold</TableHead>
              <TableHead>Applied at</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.controlId}>
                <TableCell>
                  <div className="flex flex-wrap items-center gap-2">
                    <code className="font-mono text-xs">{row.controlId}</code>
                    <Badge variant="outline">{row.controlClass}</Badge>
                  </div>
                </TableCell>
                <TableCell>
                  <Badge variant={row.enabled ? "default" : "secondary"}>
                    {row.enabled ? "Enabled" : "Disabled"}
                  </Badge>
                </TableCell>
                <TableCell>{row.response}</TableCell>
                <TableCell className="tabular-nums">{row.threshold}</TableCell>
                <TableCell>
                  {row.boundaries.length === 0 ? (
                    <span className="text-muted-foreground">nowhere</span>
                  ) : (
                    <div className="flex flex-wrap gap-1">
                      {row.boundaries.map((boundary) => (
                        <code key={boundary} className="font-mono text-xs">
                          {boundary}
                        </code>
                      ))}
                    </div>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <p className="text-xs text-muted-foreground">
        Settings are read from the revision the gateway enforces. No policy text, feed text or
        signature pattern is exposed here.
        {requestId ? <> Request {requestId}.</> : null}
      </p>
    </>
  );
}

function ReloadBanner({ reload }: { reload: ReloadState }) {
  if (reload.kind === "in_effect") {
    return (
      <Alert>
        <AlertTitle>Revision {reload.activeRevisionId} is in force</AlertTitle>
        <AlertDescription>
          It is the latest revision requested, and no change is waiting or rejected.
        </AlertDescription>
      </Alert>
    );
  }
  if (reload.kind === "pending") {
    return (
      <Alert role="status">
        <AlertTitle>Revision {reload.requestedRevisionId} is requested, not active yet</AlertTitle>
        <AlertDescription>
          Revision {reload.activeRevisionId} stays in force until the gateway validates revision{" "}
          {reload.requestedRevisionId}. This panel asks again every few seconds.
        </AlertDescription>
      </Alert>
    );
  }
  if (reload.kind === "rejected") {
    return (
      <Alert variant="destructive" role="alert">
        <AlertTitle>
          The last change was rejected; revision {reload.activeRevisionId} stays in force
        </AlertTitle>
        <AlertDescription>
          <Rejection rejection={reload.rejection} />
        </AlertDescription>
      </Alert>
    );
  }
  return (
    <Alert variant="destructive" role="alert">
      <AlertTitle>No revision is active, so nothing is enforceable yet</AlertTitle>
      <AlertDescription>
        {reload.rejection ? (
          <Rejection rejection={reload.rejection} />
        ) : (
          "Import a policy and activate it."
        )}
      </AlertDescription>
    </Alert>
  );
}

function Rejection({ rejection }: { rejection: ReloadRejection }) {
  return (
    <span>
      <code className="font-mono text-xs">{rejection.code}</code>: {rejection.message} Stage{" "}
      <code className="font-mono text-xs">{rejection.stage}</code>
      {rejection.revisionId > 0 ? <>, revision {rejection.revisionId}</> : null}.
    </span>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1 rounded-lg border px-3 py-2">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="text-sm font-medium break-words">{value}</dd>
    </div>
  );
}

function Digest({ label, digest }: { label: string; digest: string | null }) {
  return (
    <div className="flex flex-col gap-1 rounded-lg border px-3 py-2">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="flex items-center justify-between gap-2 text-sm font-medium">
        <code className="font-mono text-xs" title={digest ?? undefined}>
          {shortDigest(digest)}
        </code>
        {digest ? <CopyButton value={digest} label="Copy" iconOnly size="icon-xs" /> : null}
      </dd>
    </div>
  );
}

function numberOrNone(value: number | null): string {
  return value === null ? "none" : String(value);
}
