#!/usr/bin/env node
// Judge client (`pnpm judge`), SH-48. Submits one ad-hoc input, a fixtures/ case or an action
// proposal to the API's live test entry, POST /api/control/evaluate (X-91, packages/contracts:
// control-evaluation-request and -response), and prints the decision, the controls that ran and the
// active catalog revision. The request carries exactly the five X-91 fields (runId, kind, text,
// tool, arguments; null where a boundary does not use one); identity comes from the session, never
// from the body. The evaluation is a decision only: nothing is stored as an action or executed.
// Its only credential is the operator's session cookie; it never reads .env and holds no model,
// tool or service credential. The tests (scripts/judge-client.test.mjs) check the request body
// against the contract's schema; they do not make a live call.
import { readFileSync } from "node:fs";
import { parseArgs } from "node:util";

import { fromRepositoryRoot } from "./lib/repo-root.mjs";

export const DEFAULT_API_URL = "http://localhost:3001";
export const EVALUATE_PATH = "/api/control/evaluate";
const FIXTURE_FILES = ["fixtures/semantic-corpus.json", "fixtures/hostile-notes.json"];
const KINDS = ["model_input", "tool_result", "action_proposal"];
const TOOLS = ["read_invoice", "read_vendor", "create_report", "queue_report"];
const RUN_ID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
// The server limits the text in UTF-8 bytes.
const MAX_TEXT_BYTES = 4096;
// A fixture's expected outcome and the decision that matches it.
const DECISION_FOR_OUTCOME = { allow: "allow", block: "deny", redact: "redact" };

const USAGE = `Usage:
  pnpm judge --run <run id> --text "<text>" [--kind model_input|tool_result] [--tool <tool>]
  pnpm judge --run <run id> --case <fixture id> [--tool <tool>]
  pnpm judge --run <run id> --proposal <file.json>    (an action proposal: {"tool": ..., "arguments": {...}})

--run is the id (a UUID) of an admitted run that is still active; start a dedicated judge run
through POST /api/runs so a demo run's allowance is not spent. --tool is one of
read_invoice, read_vendor, create_report, queue_report (default read_invoice for a tool_result).

Options:
  --api-url <url>   NestJS API base URL (default ${DEFAULT_API_URL}, or JUDGE_API_URL)
  --cookie <value>  Cookie header with the operator session (or JUDGE_SESSION_COOKIE)
  --json            Print the raw response body
  --help            Show this text

The evaluation is a decision only; nothing is stored as an action or executed.
The cookie is the operator's own session. Sign in with
  curl -c cookies.txt -H 'content-type: application/json' \\
    -d '{"email":"...","password":"..."}' http://localhost:3001/api/auth/sign-in
and pass the "session" cookie as JUDGE_SESSION_COOKIE="session=<value>".`;

export class UsageError extends Error {}

/** Finds a case by id in the synthetic fixtures. */
export function findFixtureCase(caseId, fixtureFiles = FIXTURE_FILES) {
  for (const fixtureFile of fixtureFiles) {
    const fixture = JSON.parse(readFileSync(fromRepositoryRoot(fixtureFile), "utf8"));
    const match = [...(fixture.cases ?? []), ...(fixture.notes ?? [])].find(
      (entry) => entry.id === caseId,
    );
    if (match) return match;
  }
  throw new UsageError(`no fixture case "${caseId}" in ${fixtureFiles.join(" or ")}`);
}

/** Builds the X-91 request body, always with all five fields, and a fixture case's expected outcome. */
export function buildRequestBody(options) {
  if (!options.run) throw new UsageError("--run is required");
  const runId = options.run.trim().toLowerCase();
  if (!RUN_ID_PATTERN.test(runId)) throw new UsageError("--run must be a run id (a UUID)");
  const sources = ["text", "case", "proposal"].filter((name) => options[name] !== undefined);
  if (sources.length !== 1) throw new UsageError("give exactly one of --text, --case, --proposal");
  if (options.tool !== undefined && !TOOLS.includes(options.tool)) {
    throw new UsageError(`--tool must be one of ${TOOLS.join(", ")}`);
  }

  if (options.proposal !== undefined) {
    const proposal = JSON.parse(readFileSync(options.proposal, "utf8"));
    const argumentsAreObject =
      typeof proposal.arguments === "object" &&
      proposal.arguments !== null &&
      !Array.isArray(proposal.arguments);
    if (!TOOLS.includes(proposal.tool) || !argumentsAreObject) {
      throw new UsageError(
        'the proposal file must hold {"tool": "<registered tool>", "arguments": {...}}',
      );
    }
    return {
      body: {
        runId,
        kind: "action_proposal",
        text: null,
        tool: proposal.tool,
        arguments: proposal.arguments,
      },
      expectedOutcome: undefined,
    };
  }

  const fixtureCase = options.case === undefined ? null : findFixtureCase(options.case);
  const kind = options.kind ?? fixtureCase?.boundary ?? "model_input";
  if (!KINDS.includes(kind) || kind === "action_proposal") {
    throw new UsageError("--kind must be model_input or tool_result for text input");
  }
  const text = fixtureCase?.text ?? options.text;
  if (typeof text !== "string" || text.length === 0) throw new UsageError("the text is empty");
  if (Buffer.byteLength(text, "utf8") > MAX_TEXT_BYTES) {
    throw new UsageError(`the text is too long: ${MAX_TEXT_BYTES} bytes (UTF-8) at most`);
  }
  return {
    body: {
      runId,
      kind,
      text,
      tool: kind === "tool_result" ? (options.tool ?? "read_invoice") : null,
      arguments: null,
    },
    expectedOutcome: fixtureCase?.expected_outcome,
  };
}

/** Sends the request to the live test entry; resolves { status, body } or throws on a network error. */
export async function submitEvaluation({ apiUrl, cookie, requestBody }) {
  const response = await fetch(new URL(EVALUATE_PATH, apiUrl), {
    method: "POST",
    headers: { "content-type": "application/json", cookie },
    body: JSON.stringify(requestBody),
    signal: AbortSignal.timeout(60_000),
  });
  const responseText = await response.text();
  let body = null;
  try {
    body = JSON.parse(responseText);
  } catch {
    // Not JSON: reported as an unexpected response below.
  }
  return { status: response.status, body, responseText };
}

/** Human-readable summary lines of an X-91 decision response. */
export function describeDecision(body, expectedOutcome) {
  const lines = [
    `decision:         ${body.decision}${body.reasonCode ? ` (${body.reasonCode})` : ""}`,
    `message:          ${body.safeMessage ?? ""}`,
    `active revision:  ${body.catalog?.activeRevisionId ?? "unknown"} (feed ${body.catalog?.feedRevisionId ?? "unknown"}, admitted under ${body.catalog?.admissionRevisionId ?? "unknown"})`,
    `evaluation:       ${body.evaluationId ?? "unknown"}`,
  ];
  for (const control of body.controls ?? []) {
    const detail = [control.reasonCode, control.ruleId ? `rule ${control.ruleId}` : null]
      .filter(Boolean)
      .join(", ");
    lines.push(
      `  control ${control.control} (${control.controlClass}): ${control.outcome}${detail ? ` (${detail})` : ""}`,
    );
  }
  if (body.semantic) {
    // A fixture verdict is not semantic detection quality; say which one this was.
    const verdict = body.semantic.source === "live" ? "live model" : "FIXTURE, not a live verdict";
    lines.push(
      `  semantic verdict: ${verdict}, ${body.semantic.riskCategory}, score ${body.semantic.score}`,
    );
  }
  if (body.content?.text) lines.push(`redacted text:    ${body.content.text}`);
  if (expectedOutcome) {
    const expectedDecision = DECISION_FOR_OUTCOME[expectedOutcome];
    const verdict = body.decision === expectedDecision ? "matches" : "DIFFERS FROM";
    lines.push(
      `fixture label:    ${verdict} the expected ${expectedOutcome} (a test label, not detection quality)`,
    );
  }
  return lines;
}

/** Runs the CLI; resolves the exit code. 0 means a decision was returned, whatever it was. */
export async function main(argv, environment = process.env) {
  let parsed;
  try {
    parsed = parseArgs({
      args: argv,
      options: {
        run: { type: "string" },
        text: { type: "string" },
        case: { type: "string" },
        proposal: { type: "string" },
        kind: { type: "string" },
        tool: { type: "string" },
        "api-url": { type: "string" },
        cookie: { type: "string" },
        json: { type: "boolean" },
        help: { type: "boolean" },
      },
    }).values;
  } catch (parseError) {
    console.error(`${parseError.message}\n\n${USAGE}`);
    return 2;
  }
  if (parsed.help) {
    console.log(USAGE);
    return 0;
  }

  let requestBody;
  let expectedOutcome;
  try {
    ({ body: requestBody, expectedOutcome } = buildRequestBody(parsed));
  } catch (buildError) {
    console.error(`${buildError.message}\n\n${USAGE}`);
    return 2;
  }
  const cookie = parsed.cookie ?? environment.JUDGE_SESSION_COOKIE;
  if (!cookie) {
    console.error("No operator session: pass --cookie or set JUDGE_SESSION_COOKIE.");
    return 2;
  }
  const apiUrl = parsed["api-url"] ?? environment.JUDGE_API_URL ?? DEFAULT_API_URL;

  let result;
  try {
    result = await submitEvaluation({ apiUrl, cookie, requestBody });
  } catch (networkError) {
    console.error(`No decision: cannot reach ${apiUrl}${EVALUATE_PATH} (${networkError.message}).`);
    return 1;
  }
  if (result.status !== 200 || !result.body?.decision) {
    const errorText = result.body?.error
      ? `${result.body.error.code}: ${result.body.error.message}`
      : result.responseText.slice(0, 200);
    console.error(`No decision: HTTP ${result.status} ${errorText}`);
    return 1;
  }
  console.log(
    parsed.json
      ? JSON.stringify(result.body, null, 2)
      : describeDecision(result.body, expectedOutcome).join("\n"),
  );
  return 0;
}

if (import.meta.main) process.exit(await main(process.argv.slice(2)));
