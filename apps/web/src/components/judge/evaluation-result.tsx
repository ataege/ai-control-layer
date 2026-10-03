import type { ControlEvaluationResponse } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Badge } from "@workspace/ui/components/badge";
import { CopyButton } from "@workspace/ui/components/copy-button";
import type { EvaluationFailure } from "@/lib/clients/judge-client";

type BadgeVariant = "default" | "secondary" | "destructive" | "outline";

const DECISION_VARIANT: Record<ControlEvaluationResponse["decision"], BadgeVariant> = {
  allow: "default",
  deny: "destructive",
  redact: "secondary",
  approval_required: "outline",
};

const OUTCOME_VARIANT: Record<string, BadgeVariant> = {
  pass: "default",
  block: "destructive",
  error: "destructive",
  redact: "secondary",
  not_applicable: "outline",
};

/** Plain-language state for each way an evaluation can fail; a deny is shown as a decision instead. */
export const FAILURE_MESSAGES: Record<EvaluationFailure["kind"], { title: string; body: string }> =
  {
    bad_request: {
      title: "400: the API rejected the request",
      body: "The input does not match the X-91 request contract for this boundary. Check the text, tool and arguments.",
    },
    unauthorized: {
      title: "401: you are not signed in",
      body: "The session is missing or expired. Sign in again, then retry.",
    },
    run_not_found: {
      title: "404: run not found",
      body: "No run with this id exists in your organization. Start a dedicated judge run or paste another id.",
    },
    not_routed: {
      title: "The web app does not forward /api/control yet",
      body: "The web server refused the path before reaching the API (configuration_error). This is a web routing gap, not a decision.",
    },
    unavailable: {
      title: "503: the control layer is unavailable",
      body: "The API or gateway could not produce a decision. Nothing was evaluated; this is never an allow.",
    },
    timeout: {
      title: "The evaluation timed out",
      body: "No decision arrived in time. The outcome is unconfirmed; retry.",
    },
    network: {
      title: "The server could not be reached",
      body: "The request never produced a response.",
    },
    invalid_response: {
      title: "The response was not a valid decision",
      body: "The reply did not match the X-91 response contract, so it is not shown as a decision.",
    },
  };

export function FailureNotice({
  failure,
  status,
}: {
  failure: EvaluationFailure;
  status?: number;
}) {
  const message = FAILURE_MESSAGES[failure.kind];
  return (
    <Alert variant="destructive" role="alert" data-testid="judge-failure">
      <AlertTitle>{message.title}</AlertTitle>
      <AlertDescription>
        {message.body}
        {failure.kind === "unauthorized" && (
          <>
            {" "}
            <a className="underline" href="/login?callbackUrl=/judge">
              Go to sign in
            </a>
          </>
        )}
        {failure.kind === "unavailable" && status !== undefined && ` (HTTP ${status})`}
      </AlertDescription>
    </Alert>
  );
}

interface EvaluationResultProps {
  response: ControlEvaluationResponse;
  durationMs: number;
  requestId?: string;
}

/** Shows one X-91 decision exactly as the control layer returned it. */
export function EvaluationResult({ response, durationMs, requestId }: EvaluationResultProps) {
  const { semantic } = response;
  return (
    <div className="space-y-4" data-testid="judge-result">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={DECISION_VARIANT[response.decision]} data-testid="judge-decision">
          {response.decision}
        </Badge>
        {response.reasonCode && (
          <Badge variant="outline" data-testid="judge-reason-code">
            {response.reasonCode}
          </Badge>
        )}
        {response.reasonCode === "run_not_active" && (
          <span className="text-sm text-muted-foreground">
            This run is finished or cancelled; start a new dedicated judge run.
          </span>
        )}
      </div>
      <p className="text-sm">{response.safeMessage}</p>

      {response.content && (
        <div className="space-y-1">
          <p className="text-sm font-medium">Redacted text (server output)</p>
          <pre
            className="rounded-md border bg-muted p-3 text-sm break-words whitespace-pre-wrap"
            data-testid="judge-redacted-text"
          >
            {response.content.text}
          </pre>
        </div>
      )}

      {response.alternativeTemplate && (
        <p className="text-sm">
          Alternative template: <code>{response.alternativeTemplate}</code>
        </p>
      )}

      <div className="space-y-1">
        <p className="text-sm font-medium">Controls that ran</p>
        <div className="overflow-x-auto rounded-md border">
          <table className="w-full text-sm" data-testid="judge-controls">
            <thead className="bg-muted/50 text-left">
              <tr>
                <th className="px-3 py-2">Boundary</th>
                <th className="px-3 py-2">Class</th>
                <th className="px-3 py-2">Control</th>
                <th className="px-3 py-2">Outcome</th>
                <th className="px-3 py-2">Reason / rule</th>
              </tr>
            </thead>
            <tbody>
              {response.controls.map((control, index) => (
                <tr key={`${control.control}-${index}`} className="border-t">
                  <td className="px-3 py-2">{control.boundary}</td>
                  <td className="px-3 py-2">{control.controlClass}</td>
                  <td className="px-3 py-2">{control.control}</td>
                  <td className="px-3 py-2">
                    <Badge variant={OUTCOME_VARIANT[control.outcome] ?? "outline"}>
                      {control.outcome}
                    </Badge>
                  </td>
                  <td className="px-3 py-2">
                    {[control.reasonCode, control.ruleId, control.feedRevision]
                      .filter(Boolean)
                      .join(" · ") || "none"}
                  </td>
                </tr>
              ))}
              {response.controls.length === 0 && (
                <tr>
                  <td className="px-3 py-2 text-muted-foreground" colSpan={5}>
                    No control ran for this evaluation.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="space-y-1">
        <p className="text-sm font-medium">Semantic evaluator</p>
        {semantic ? (
          <p className="text-sm" data-testid="judge-semantic">
            <Badge variant={semantic.source === "live" ? "default" : "secondary"}>
              {semantic.source === "live" ? "live model" : "fixture (not live detection)"}
            </Badge>{" "}
            category <code>{semantic.riskCategory}</code>, score {semantic.score}, reason{" "}
            <code>{semantic.reasonCode}</code>
          </p>
        ) : (
          <p className="text-sm text-muted-foreground" data-testid="judge-semantic">
            The semantic evaluator did not run for this decision.
          </p>
        )}
      </div>

      <dl className="grid gap-x-4 gap-y-1 text-sm sm:grid-cols-[auto_1fr]">
        <dt className="text-muted-foreground">Round trip (browser to decision)</dt>
        <dd data-testid="judge-duration">
          {durationMs} ms (client-measured; the API returns no server timings)
        </dd>
        <dt className="text-muted-foreground">Evaluation id</dt>
        <dd className="flex items-center gap-2">
          <code data-testid="judge-evaluation-id">{response.evaluationId}</code>
          <CopyButton value={response.evaluationId} />
        </dd>
        {requestId && (
          <>
            <dt className="text-muted-foreground">Request id</dt>
            <dd>
              <code>{requestId}</code>
            </dd>
          </>
        )}
        <dt className="text-muted-foreground">Catalog</dt>
        <dd>
          admitted revision {response.catalog.admissionRevisionId}, active revision{" "}
          {response.catalog.activeRevisionId}, feed {response.catalog.feedRevisionId ?? "none"}
        </dd>
      </dl>
    </div>
  );
}
