"use client";

import * as React from "react";
import Link from "next/link";
import type { ApprovalResponse, ReviewView, RunState } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Badge } from "@workspace/ui/components/badge";
import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { ConfirmDialog } from "@workspace/ui/components/confirm-dialog";
import { ErrorState } from "@workspace/ui/components/error-state";
import { LoadingState } from "@workspace/ui/components/loading-state";

import { CancelRunButton } from "@/components/approval/cancel-run-button";
import { SimulatedOutboxLabel } from "@/components/labels";
import {
  decideApproval,
  getReview,
  getRunState,
  type ClientFailure,
} from "@/lib/clients/actions-client";

type LoadState =
  | { status: "loading" }
  | { status: "loaded"; review: ReviewView; run: RunState | null }
  | { status: "failed"; failure: ClientFailure };

type DecisionState =
  | { status: "idle" }
  | { status: "sending"; decision: "approve" | "reject" }
  | { status: "decided"; response: ApprovalResponse }
  | { status: "failed"; failure: ClientFailure };

const CLASSIFICATION_LABELS = {
  internal_only: "Internal only",
  vendor_shareable: "Vendor shareable",
} as const;

// Titles of a failed decision; an unconfirmed outcome may have been recorded, so it never says
// "not recorded".
const FAILURE_TITLES: Partial<Record<ClientFailure["kind"], string>> = {
  expired: "Expired: nothing was sent",
  changed: "Changed: nothing was sent",
  run_stopped: "Run stopped: nothing was sent",
  already_decided: "Already decided",
  unconfirmed: "Outcome unconfirmed",
  not_reviewer: "Not a reviewer",
  signed_out: "Signed out",
};

function formatTime(isoTime: string): string {
  const time = new Date(isoTime);
  return Number.isNaN(time.getTime()) ? isoTime : time.toLocaleString();
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid gap-1 sm:grid-cols-[12rem_1fr] sm:gap-4">
      <dt className="text-sm text-muted-foreground">{label}</dt>
      <dd className="text-sm break-all">{children}</dd>
    </div>
  );
}

/**
 * WEB-14: the exact frozen action a reviewer approves or rejects. The material is read from the
 * server each time the page opens; approve and reject send only this action's identifier and the
 * decision, and NestJS and Go decide.
 */
export function ReviewPanel({ runId, actionId }: { runId: string; actionId: string }) {
  const [load, setLoad] = React.useState<LoadState>({ status: "loading" });
  const [decision, setDecision] = React.useState<DecisionState>({ status: "idle" });
  const [now, setNow] = React.useState(() => Date.now());

  // Bumped by "Try again" to read the review again.
  const [reloadCount, setReloadCount] = React.useState(0);

  React.useEffect(() => {
    const controller = new AbortController();
    // The run's state tells whether the review is still open; the frozen material stays
    // readable after a decision. Without the run state the server still refuses a stale decision.
    void Promise.all([
      getReview(actionId, { signal: controller.signal }),
      getRunState(runId, { signal: controller.signal }),
    ]).then(([review, run]) => {
      if (controller.signal.aborted) return;
      setLoad(
        review.ok
          ? { status: "loaded", review: review.data, run: run.ok ? run.data : null }
          : { status: "failed", failure: review.failure },
      );
    });
    return () => controller.abort();
  }, [actionId, runId, reloadCount]);

  // The expiry is shown live; the server still decides whether the approval has expired.
  React.useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 15_000);
    return () => clearInterval(timer);
  }, []);

  const send = async (choice: "approve" | "reject") => {
    setDecision({ status: "sending", decision: choice });
    const result = await decideApproval(actionId, choice);
    setDecision(
      result.ok
        ? { status: "decided", response: result.data }
        : { status: "failed", failure: result.failure },
    );
  };

  if (load.status === "loading") return <LoadingState label="Loading the action under review" />;
  if (load.status === "failed") {
    return (
      <ErrorState
        title={
          load.failure.kind === "not_reviewer"
            ? "Review not available to you"
            : "The review could not be loaded"
        }
        description={`${load.failure.message}${load.failure.status ? ` (HTTP ${load.failure.status})` : ""}`}
        action={
          <Button
            variant="outline"
            onClick={() => {
              setLoad({ status: "loading" });
              setReloadCount((count) => count + 1);
            }}
          >
            Try again
          </Button>
        }
      />
    );
  }

  const review = load.review;
  const expiresAt = new Date(review.expires_at).getTime();
  const expiredLocally = !Number.isNaN(expiresAt) && expiresAt <= now;
  const decided = decision.status === "decided";
  // Only a run awaiting approval has a decision to make; a finished or resumed run does not.
  const runNotWaiting = load.run !== null && load.run.status !== "awaiting_approval";
  const controlsDisabled =
    decision.status === "sending" || decided || expiredLocally || runNotWaiting;

  return (
    <div className="grid gap-6">
      <DecisionOutcome decision={decision} review={review} runId={runId} />

      <Card>
        <CardHeader>
          <CardTitle>Action under review</CardTitle>
          <CardDescription>
            Review is required before this report leaves the organization. You approve exactly the
            recipient and content below; any change after freezing needs a new review.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <dl className="grid gap-3">
            <Field label="Tool">
              <code>{review.tool}</code>
            </Field>
            <Field label="Action">
              <code>{review.action_id}</code>
            </Field>
            <Field label="Policy revision">{review.policy_revision_id}</Field>
            <Field label="Expires">
              {formatTime(review.expires_at)}{" "}
              {expiredLocally ? <Badge variant="destructive">Expired</Badge> : null}
            </Field>
          </dl>
        </CardContent>
      </Card>

      {review.recipient ? (
        <Card>
          <CardHeader>
            <CardTitle>Exact recipient</CardTitle>
            <CardDescription>
              The vendor&apos;s registered reporting address from trusted records.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="grid gap-3">
              <Field label="Address">
                <strong>{review.recipient.address}</strong>
              </Field>
              <Field label="Vendor">
                <code>{review.recipient.vendor_id}</code>
              </Field>
              <Field label="Reference">
                <code>{review.recipient.reference}</code>
              </Field>
            </dl>
          </CardContent>
        </Card>
      ) : null}

      {review.report ? (
        <Card>
          <CardHeader>
            <CardTitle>Report to be queued</CardTitle>
            <CardDescription>
              The stored, server-rendered content, exactly as it would be sent.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <dl className="grid gap-3">
              <Field label="Classification">
                <Badge
                  variant={
                    review.report.classification === "internal_only" ? "destructive" : "secondary"
                  }
                >
                  {CLASSIFICATION_LABELS[review.report.classification]}
                </Badge>
              </Field>
              <Field label="Report">
                <code>{review.report.id}</code> (version {review.report.version})
              </Field>
              <Field label="Template">
                <code>{review.report.template}</code> v{review.report.template_version}
              </Field>
              <Field label="Projection rule">
                {review.report.projection_rule ? (
                  <>
                    <code>{review.report.projection_rule}</code> v
                    {review.report.projection_rule_version}
                  </>
                ) : (
                  "None"
                )}
              </Field>
              <Field label="Content hash (SHA-256)">
                <code>{review.report.content_hash}</code>
              </Field>
            </dl>
            <pre className="max-h-96 overflow-auto rounded-md bg-muted p-4 text-sm whitespace-pre-wrap">
              {review.report.content}
            </pre>
          </CardContent>
        </Card>
      ) : null}

      {review.report ? (
        <Card>
          <CardHeader>
            <CardTitle>Source manifest</CardTitle>
            <CardDescription>
              The records the report was rendered from, with their versions and the fields used.
              Digest <code className="break-all">{review.report.source_manifest_digest}</code>
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ul className="grid gap-3">
              {review.report.sources.map((source) => (
                <li
                  key={`${source.kind}:${source.id}:${source.version}`}
                  className="rounded-md border p-3 text-sm"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <code>{source.id}</code>
                    <span className="text-muted-foreground">
                      {source.kind}, version {source.version}
                    </span>
                    <Badge
                      variant={
                        source.classification === "internal_only" ? "destructive" : "secondary"
                      }
                    >
                      {CLASSIFICATION_LABELS[source.classification]}
                    </Badge>
                  </div>
                  <p className="mt-2 text-muted-foreground">
                    Fields used: {source.consumed_fields.join(", ")}
                  </p>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>Arguments</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="overflow-auto rounded-md bg-muted p-4 text-sm">
              {JSON.stringify(review.canonical_arguments, null, 2)}
            </pre>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Decision</CardTitle>
          <CardDescription>
            {runNotWaiting && !decided
              ? `This review is closed: the run is ${load.run?.status.replace(/_/g, " ")}, so no decision is awaited.`
              : expiredLocally
                ? "This review has expired; the action needs a new review."
                : "Approve queues exactly this report for exactly this recipient in the simulated outbox. Reject queues nothing."}
          </CardDescription>
        </CardHeader>
        <CardFooter className="flex flex-wrap gap-3">
          <ConfirmDialog
            title="Approve this exact action?"
            description={
              review.recipient
                ? `The report will be queued in the simulated outbox for ${review.recipient.address}. This approval covers only this action.`
                : "This approval covers only this action."
            }
            confirmLabel="Approve"
            onConfirm={() => void send("approve")}
            trigger={
              <Button disabled={controlsDisabled}>
                {decision.status === "sending" && decision.decision === "approve"
                  ? "Approving…"
                  : "Approve"}
              </Button>
            }
          />
          <ConfirmDialog
            title="Reject this action?"
            description="Nothing will be sent. The run continues without this action."
            confirmLabel="Reject"
            confirmVariant="destructive"
            onConfirm={() => void send("reject")}
            trigger={
              <Button variant="outline" disabled={controlsDisabled}>
                {decision.status === "sending" && decision.decision === "reject"
                  ? "Rejecting…"
                  : "Reject"}
              </Button>
            }
          />
          <Button variant="ghost" asChild>
            <Link href={`/runs/${runId}`}>Back to the run</Link>
          </Button>
        </CardFooter>
        {load.run ? (
          <CardContent>
            <CancelRunButton runId={runId} runStatus={load.run.status} />
          </CardContent>
        ) : null}
      </Card>
    </div>
  );
}

function DecisionOutcome({
  decision,
  review,
  runId,
}: {
  decision: DecisionState;
  review: ReviewView;
  runId: string;
}) {
  if (decision.status === "decided") {
    const approved = decision.response.decision === "approve";
    return (
      <Alert>
        <AlertTitle className="flex flex-wrap items-center gap-2">
          {approved ? "Approved" : "Rejected"}
          {approved && review.recipient ? <SimulatedOutboxLabel /> : null}
        </AlertTitle>
        <AlertDescription>
          {approved
            ? `The approval is recorded for this exact action${
                review.recipient
                  ? `; when the run resumes, the report is queued in the simulated outbox for ${review.recipient.address} (nothing is delivered)`
                  : ""
              }.`
            : "The rejection is recorded. Nothing will be sent."}{" "}
          <Link className="underline" href={`/runs/${runId}`}>
            Follow the run
          </Link>
          .
        </AlertDescription>
      </Alert>
    );
  }
  if (decision.status === "failed") {
    return (
      <Alert variant="destructive">
        <AlertTitle>
          {FAILURE_TITLES[decision.failure.kind] ?? "The decision was not recorded"}
        </AlertTitle>
        <AlertDescription>
          {decision.failure.message}
          {decision.failure.status ? ` (HTTP ${decision.failure.status})` : ""}
        </AlertDescription>
      </Alert>
    );
  }
  return null;
}
