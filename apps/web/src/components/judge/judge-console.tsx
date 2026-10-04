"use client";

import * as React from "react";
import type { ControlBoundary, ControlEvaluationRequest, ToolName } from "@workspace/contracts";
import { Alert, AlertDescription, AlertTitle } from "@workspace/ui/components/alert";
import { Button } from "@workspace/ui/components/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@workspace/ui/components/card";
import { Input } from "@workspace/ui/components/input";
import { Label } from "@workspace/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@workspace/ui/components/select";
import { Textarea } from "@workspace/ui/components/textarea";
import { Loader2 } from "lucide-react";
import {
  BOUNDARIES,
  INACTIVE_RUN_STATUSES,
  JudgeClient,
  MAX_TEXT_BYTES,
  TOOLS,
  utf8ByteLength,
  buildEvaluationRequest,
  type EvaluationOutcome,
} from "@/lib/clients/judge-client";
import { getSafeMessage } from "@/lib/product-client";
import { EvaluationResult, FailureNotice } from "./evaluation-result";

const BOUNDARY_LABELS: Record<ControlBoundary, string> = {
  model_input: "Model input (text going to the model)",
  tool_result: "Tool result (text coming back from a tool)",
  action_proposal: "Action proposal (a tool call with arguments)",
};

const DEFAULT_ARGUMENTS = '{\n  "invoice_id": "invoice_A01"\n}';

export function JudgeConsole() {
  const [runId, setRunId] = React.useState("");
  const [runNotice, setRunNotice] = React.useState<string | null>(null);
  const [runError, setRunError] = React.useState<string | null>(null);
  const [isStartingRun, setIsStartingRun] = React.useState(false);
  const [runStatus, setRunStatus] = React.useState<string | null>(null);

  const [kind, setKind] = React.useState<ControlBoundary>("model_input");
  const [text, setText] = React.useState("");
  const [tool, setTool] = React.useState<ToolName | "">("");
  const [argumentsJson, setArgumentsJson] = React.useState(DEFAULT_ARGUMENTS);

  const [formError, setFormError] = React.useState<string | null>(null);
  const [isEvaluating, setIsEvaluating] = React.useState(false);
  const [outcome, setOutcome] = React.useState<EvaluationOutcome | null>(null);
  // The input that produced the shown outcome, labelled as the judge's own input.
  const [submitted, setSubmitted] = React.useState<ControlEvaluationRequest | null>(null);

  async function refreshRunStatus(id: string) {
    setRunStatus(await JudgeClient.getRunStatus(id));
  }

  async function startDedicatedRun() {
    setIsStartingRun(true);
    setRunError(null);
    setRunNotice(null);
    try {
      const startResult = await JudgeClient.startRun();
      if (!startResult.ok) {
        setRunError(getSafeMessage(startResult.error));
        return;
      }
      setRunId(startResult.data.runId);
      setRunNotice(
        "Judge run started: it has a passport but no agent. Evaluations spend this run's security allowance, not a demo run's.",
      );
      await refreshRunStatus(startResult.data.runId);
    } finally {
      setIsStartingRun(false);
    }
  }

  async function evaluate(event: React.FormEvent) {
    event.preventDefault();
    const built = buildEvaluationRequest({ runId, kind, text, tool, argumentsJson });
    if (!built.ok) {
      setFormError(built.message);
      return;
    }
    setFormError(null);
    setIsEvaluating(true);
    setOutcome(null);
    try {
      setSubmitted(built.request);
      setOutcome(await JudgeClient.evaluate(built.request));
      await refreshRunStatus(built.request.runId);
    } finally {
      setIsEvaluating(false);
    }
  }

  const needsText = kind !== "action_proposal";
  const needsTool = kind !== "model_input";

  return (
    <div className="mx-auto grid max-w-3xl gap-6">
      <Alert data-testid="judge-label">
        <AlertTitle>Judge evaluation</AlertTitle>
        <AlertDescription>
          Judge evaluation: decision only; nothing is stored as an action or executed.
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle>1. Choose a run</CardTitle>
          <CardDescription>
            Evaluations run against an admitted, still active run of your organization. A judge run
            has a passport but no agent: nothing runs on it and no model call is made for it.
            Evaluations spend its security allowance, never a demo run&apos;s. It stays valid for
            its passport&apos;s lifetime (about 15 minutes).
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="judge-run-id">Run id</Label>
            <Input
              id="judge-run-id"
              value={runId}
              onChange={(changeEvent) => setRunId(changeEvent.target.value)}
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              autoComplete="off"
              spellCheck={false}
            />
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={startDedicatedRun}
            disabled={isStartingRun}
            data-testid="judge-start-run"
          >
            {isStartingRun && <Loader2 className="mr-2 size-4 animate-spin" />}
            Start a dedicated judge run
          </Button>
          {runNotice && <p className="text-sm text-muted-foreground">{runNotice}</p>}
          {runStatus && (
            <p className="text-sm" data-testid="judge-run-status">
              Run status (from the gateway): <code>{runStatus}</code>
              {INACTIVE_RUN_STATUSES.includes(runStatus) &&
                " — a finished run refuses evaluations (run_not_active)."}
            </p>
          )}
          {runError && (
            <p className="text-sm text-destructive" role="alert">
              {runError}
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>2. Submit input</CardTitle>
          <CardDescription>
            The same gates as the agent path decide. The agent is never dispatched; the security
            evaluator may call the local model and spends the run&apos;s security allowance.
          </CardDescription>
        </CardHeader>
        <form onSubmit={evaluate}>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="judge-boundary">Boundary</Label>
              <Select value={kind} onValueChange={(value) => setKind(value as ControlBoundary)}>
                <SelectTrigger id="judge-boundary" data-testid="judge-boundary">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {BOUNDARIES.map((boundary) => (
                    <SelectItem key={boundary} value={boundary}>
                      {BOUNDARY_LABELS[boundary]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {needsTool && (
              <div className="space-y-2">
                <Label htmlFor="judge-tool">
                  {kind === "tool_result" ? "Tool the result is attributed to" : "Proposed tool"}
                </Label>
                <Select value={tool} onValueChange={(value) => setTool(value as ToolName)}>
                  <SelectTrigger id="judge-tool" data-testid="judge-tool">
                    <SelectValue placeholder="Select a tool" />
                  </SelectTrigger>
                  <SelectContent>
                    {TOOLS.map((toolName) => (
                      <SelectItem key={toolName} value={toolName}>
                        {toolName}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {needsText ? (
              <div className="space-y-2">
                <Label htmlFor="judge-text">
                  Text ({utf8ByteLength(text)}/{MAX_TEXT_BYTES} bytes)
                </Label>
                <Textarea
                  id="judge-text"
                  value={text}
                  onChange={(changeEvent) => setText(changeEvent.target.value)}
                  rows={6}
                  placeholder="Paste the untrusted text to evaluate"
                />
              </div>
            ) : (
              <div className="space-y-2">
                <Label htmlFor="judge-arguments">Arguments (JSON object, snake_case)</Label>
                <Textarea
                  id="judge-arguments"
                  value={argumentsJson}
                  onChange={(changeEvent) => setArgumentsJson(changeEvent.target.value)}
                  rows={5}
                  className="font-mono"
                  spellCheck={false}
                />
              </div>
            )}

            {formError && (
              <p className="text-sm text-destructive" role="alert" data-testid="judge-form-error">
                {formError}
              </p>
            )}
            <Button type="submit" disabled={isEvaluating} data-testid="judge-submit">
              {isEvaluating && <Loader2 className="mr-2 size-4 animate-spin" />}
              Evaluate
            </Button>
          </CardContent>
        </form>
      </Card>

      {(isEvaluating || outcome) && (
        <Card>
          <CardHeader>
            <CardTitle>3. Decision</CardTitle>
            {submitted && (
              <CardDescription>
                Your input ({submitted.kind}
                {submitted.tool ? `, ${submitted.tool}` : ""}) for run{" "}
                <code>{submitted.runId}</code>. Blocked text is never echoed back.
              </CardDescription>
            )}
          </CardHeader>
          <CardContent>
            {isEvaluating && (
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Loader2 className="size-4 animate-spin" />
                Waiting for the control layer. A live semantic check can take several seconds.
              </p>
            )}
            {outcome &&
              (outcome.ok ? (
                <EvaluationResult
                  response={outcome.response}
                  durationMs={outcome.durationMs}
                  requestId={outcome.requestId}
                />
              ) : (
                <FailureNotice
                  failure={outcome.failure}
                  status={
                    outcome.failure.kind === "unavailable" ? outcome.failure.status : undefined
                  }
                />
              ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
