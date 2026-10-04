import type {
  ControlBoundary,
  StartRunRequest,
  ControlEvaluationRequest,
  ControlEvaluationResponse,
  ToolName,
} from "@workspace/contracts";
import { fetchJson, postJson, type FetchJsonResult } from "../fetch-json";

export const EVALUATE_PATH = "/api/control/evaluate";
// The server limits the text in UTF-8 bytes, not UTF-16 units, so the client counts bytes too.
export const MAX_TEXT_BYTES = 4096;
export const MAX_ARGUMENTS_BYTES = 4096;

const textEncoder = new TextEncoder();
export const utf8ByteLength = (value: string): number => textEncoder.encode(value).length;

/**
 * The seeded Atlas scenario (fixtures/demo-records.json), started as the judge's own run so an
 * evaluation spends this run's security allowance and never a demo run's. The review requirement
 * pauses the run at its report, which keeps it active for further evaluations.
 */
export const JUDGE_RUN_REQUEST: StartRunRequest = {
  template: "reconcile_atlas_v1",
  vendorId: "vendor_Atlas",
  invoiceIds: ["invoice_A01", "invoice_A02"],
  destination: "vendor_Atlas",
  approvalRequirement: "review_queue_report",
};

/** A run in one of these states is refused by the evaluator with run_not_active. */
export const INACTIVE_RUN_STATUSES = ["completed", "failed", "stopped"];

export const BOUNDARIES: readonly ControlBoundary[] = [
  "model_input",
  "tool_result",
  "action_proposal",
];
export const TOOLS: readonly ToolName[] = [
  "read_invoice",
  "read_vendor",
  "create_report",
  "queue_report",
];
const DECISIONS = ["allow", "deny", "redact", "approval_required"];

const RUN_ID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** What the judge typed into the form, before it becomes an X-91 request. */
export interface JudgeFormInput {
  runId: string;
  kind: ControlBoundary;
  text: string;
  tool: ToolName | "";
  /** Raw JSON text of the proposed tool's snake_case arguments (action_proposal only). */
  argumentsJson: string;
}

export type BuildRequestResult =
  { ok: true; request: ControlEvaluationRequest } | { ok: false; message: string };

/** Builds exactly the five X-91 fields, with null for the ones a boundary does not use. */
export function buildEvaluationRequest(input: JudgeFormInput): BuildRequestResult {
  const runId = input.runId.trim().toLowerCase();
  if (!RUN_ID_PATTERN.test(runId)) {
    return { ok: false, message: "The run id must be a UUID (lowercase hex, 8-4-4-4-12)." };
  }

  if (input.kind === "action_proposal") {
    if (input.tool === "") return { ok: false, message: "Choose the proposed tool." };
    if (utf8ByteLength(input.argumentsJson) > MAX_ARGUMENTS_BYTES) {
      return {
        ok: false,
        message: `The arguments are too long: ${MAX_ARGUMENTS_BYTES} bytes at most.`,
      };
    }
    let parsedArguments: unknown;
    try {
      parsedArguments = JSON.parse(input.argumentsJson);
    } catch {
      return { ok: false, message: "The arguments must be valid JSON." };
    }
    if (
      typeof parsedArguments !== "object" ||
      parsedArguments === null ||
      Array.isArray(parsedArguments)
    ) {
      return { ok: false, message: "The arguments must be a JSON object." };
    }
    return {
      ok: true,
      request: {
        runId,
        kind: "action_proposal",
        text: null,
        tool: input.tool,
        arguments: parsedArguments as Record<string, unknown>,
      },
    };
  }

  if (input.text.length === 0) return { ok: false, message: "Enter the text to evaluate." };
  if (utf8ByteLength(input.text) > MAX_TEXT_BYTES) {
    return {
      ok: false,
      message: `The text is too long: ${MAX_TEXT_BYTES} bytes at most (UTF-8; accented and non-Latin characters take 2 to 4 bytes each).`,
    };
  }
  if (input.kind === "tool_result" && input.tool === "") {
    return { ok: false, message: "Choose the tool this result is attributed to." };
  }
  return {
    ok: true,
    request: {
      runId,
      kind: input.kind,
      text: input.text,
      tool: input.kind === "tool_result" && input.tool !== "" ? input.tool : null,
      arguments: null,
    },
  };
}

function isEvaluationResponse(data: unknown): data is ControlEvaluationResponse {
  if (typeof data !== "object" || data === null) return false;
  const candidate = data as Record<string, unknown>;
  return (
    typeof candidate.evaluationId === "string" &&
    candidate.actionId === null &&
    typeof candidate.decision === "string" &&
    DECISIONS.includes(candidate.decision) &&
    Array.isArray(candidate.controls)
  );
}

/** How a failed evaluation is shown; every case is a distinct state in the console. */
export type EvaluationFailure =
  | { kind: "bad_request" }
  | { kind: "unauthorized" }
  | { kind: "run_not_found" }
  | { kind: "not_routed" }
  | { kind: "unavailable"; status: number }
  | { kind: "timeout" }
  | { kind: "network" }
  | { kind: "invalid_response" };

export type EvaluationOutcome =
  | { ok: true; response: ControlEvaluationResponse; durationMs: number; requestId?: string }
  | { ok: false; failure: EvaluationFailure; durationMs: number; requestId?: string };

/** Maps the transport result to a decision or a named failure; a deny is a decision, not a failure. */
export function interpretEvaluation(result: FetchJsonResult<unknown>): EvaluationOutcome {
  const meta = { durationMs: result.durationMs, requestId: result.requestId };
  if (result.ok) {
    return isEvaluationResponse(result.data)
      ? { ok: true, response: result.data, ...meta }
      : { ok: false, failure: { kind: "invalid_response" }, ...meta };
  }
  const { error } = result;
  if (error.kind === "timeout") return { ok: false, failure: { kind: "timeout" }, ...meta };
  if (error.kind === "network" || error.kind === "aborted") {
    return { ok: false, failure: { kind: "network" }, ...meta };
  }
  if (error.kind === "invalid_json") {
    return { ok: false, failure: { kind: "invalid_response" }, ...meta };
  }
  if (error.status === 400) return { ok: false, failure: { kind: "bad_request" }, ...meta };
  if (error.status === 401) return { ok: false, failure: { kind: "unauthorized" }, ...meta };
  if (error.status === 404) return { ok: false, failure: { kind: "run_not_found" }, ...meta };
  // The web proxy answers 500 configuration_error while "/api/control" is not an allowed prefix.
  const errorCode = (error.body as { error?: { code?: string } } | undefined)?.error?.code;
  if (error.status === 500 && errorCode === "configuration_error") {
    return { ok: false, failure: { kind: "not_routed" }, ...meta };
  }
  return { ok: false, failure: { kind: "unavailable", status: error.status }, ...meta };
}

export class JudgeClient {
  /** Starts the judge's dedicated run through the existing start-run route. */
  static async startRun(): Promise<FetchJsonResult<{ runId: string }>> {
    const result = await postJson<{ runId?: unknown }>("/api/runs", JUDGE_RUN_REQUEST);
    if (result.ok && typeof result.data?.runId !== "string") {
      return {
        ok: false,
        error: { kind: "invalid_json", status: result.status },
        durationMs: result.durationMs,
        requestId: result.requestId,
      };
    }
    return result as FetchJsonResult<{ runId: string }>;
  }

  /** The run's state as the gateway reports it, or null when it cannot be read. */
  static async getRunStatus(runId: string): Promise<string | null> {
    const result = await fetchJson<{ status?: unknown }>(`/api/runs/${encodeURIComponent(runId)}`);
    return result.ok && typeof result.data?.status === "string" ? result.data.status : null;
  }

  static async evaluate(request: ControlEvaluationRequest): Promise<EvaluationOutcome> {
    // The semantic evaluator runs a local model, so allow longer than the default 15 s.
    const result = await postJson<unknown>(EVALUATE_PATH, request, { timeoutMs: 60_000 });
    return interpretEvaluation(result);
  }
}
