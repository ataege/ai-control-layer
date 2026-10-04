"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Button, buttonVariants } from "@workspace/ui/components/button";
import { ErrorState } from "@workspace/ui/components/error-state";
import type { Failure } from "@/lib/errors/failure";
import type { FailureScope } from "@/lib/errors/reason-failures";

// Where a failure happened, in the operator's words.
export const SCOPE_LABELS: Record<FailureScope, string> = {
  browser: "Your browser",
  web_server: "The web server",
  api: "The API",
  gateway: "The gateway",
  live_model: "The live model",
  session: "Your session",
  request: "Your request",
};

// A failing live model is not a failing gateway: the gateway reported it, and the deterministic checks
// and labelled replays do not need the model.
const LIVE_MODEL_NOTE =
  "The gateway reported this itself. Only live model behavior is unavailable; deterministic checks and labelled replays do not need the model.";

/** The path to sign in again and come back to this page. */
export function signInHref(pathname: string | null): string {
  return `/login?callbackUrl=${encodeURIComponent(pathname ?? "/")}`;
}

interface FailureStateProps {
  failure: Failure;
  /** The request id of the failed exchange, so the operator can quote it. */
  requestId?: string;
  /** Offered only when the failure allows a retry. */
  onRetry?: () => void;
  className?: string;
}

/**
 * The truthful state of one failure: its own title and words, where it failed, whether the request
 * may still have taken effect, and the one thing the operator can do (retry, or sign in). It is
 * never shown as a success and never as an empty page.
 */
export function FailureState({ failure, requestId, onRetry, className }: FailureStateProps) {
  const pathname = usePathname();
  const action =
    failure.action === "sign_in" ? (
      <Link href={signInHref(pathname)} className={buttonVariants({ variant: "outline" })}>
        Sign in
      </Link>
    ) : failure.action === "retry" && onRetry !== undefined ? (
      <Button variant="outline" onClick={onRetry}>
        Try again
      </Button>
    ) : undefined;

  return (
    <ErrorState
      className={className}
      data-failure-kind={failure.kind}
      data-failure-scope={failure.scope}
      title={failure.title}
      description={
        <span className="flex flex-col gap-1">
          <span>{failure.description}</span>
          <span data-testid="failure-scope" className="text-xs">
            Where: {SCOPE_LABELS[failure.scope]}
            {failure.reasonCode !== null ? ` (${failure.reasonCode})` : ""}
          </span>
          {failure.scope === "live_model" ? (
            <span data-testid="failure-live-model" className="text-xs">
              {LIVE_MODEL_NOTE}
            </span>
          ) : null}
          {failure.unconfirmed ? (
            <span data-testid="failure-unconfirmed" className="text-xs font-medium">
              Outcome unconfirmed.
            </span>
          ) : null}
          {requestId !== undefined ? (
            <span data-testid="failure-request-id" className="font-mono text-xs">
              Request ID: {requestId}
            </span>
          ) : null}
        </span>
      }
      action={action}
    />
  );
}
