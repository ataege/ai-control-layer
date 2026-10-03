#!/usr/bin/env node
// Judge client (`pnpm judge`), SH-48. DRAFT: written against the proposal in
// docs/contracts/control-evaluation-draft.md. The gateway serves X-91 (POST /internal/control/evaluate,
// GO-82); the NestJS live test entry this client calls (X-106, API-38) does not exist yet, and the
// request body still follows the draft rather than the frozen X-91 schema.
//
// Submits one ad-hoc input, a fixtures/ case or an action proposal to the NestJS live test entry
// and prints the decision and the active catalog revision. Its only credential is the operator's
// session cookie; it never reads .env and holds no model, tool or service credential.
import { readFileSync } from "node:fs";
import { parseArgs } from "node:util";

import { fromRepositoryRoot } from "./lib/repo-root.mjs";

export const DRAFT_NOTICE =
  "DRAFT judge client: the gateway serves POST /internal/control/evaluate (X-91), but the API's live test entry (X-106, API-38) that this client calls is not implemented yet, so every call ends without a decision.";
export const DEFAULT_API_URL = "http://localhost:3001";
export const EVALUATE_PATH = "/api/control/evaluate";
const FIXTURE_FILES = ["fixtures/semantic-corpus.json", "fixtures/hostile-notes.json"];
const KINDS = ["model_input", "tool_result", "action_proposal"];
// A fixture's expected outcome and the decision that matches it.
const DECISION_FOR_OUTCOME = { allow: "allow", block: "deny", redact: "redact" };

const USAGE = `Usage:
  pnpm judge --run <run_id> --text "<text>" [--kind model_input|tool_result] [--tool <tool>] [--model <alias>]
  pnpm judge --run <run_id> --case <fixture id> [--tool <tool>] [--model <alias>]
  pnpm judge --run <run_id> --proposal <file.json>    (an action proposal: {"tool": ..., "arguments": {...}})

Options:
  --api-url <url>   NestJS API base URL (default ${DEFAULT_API_URL}, or JUDGE_API_URL)
  --cookie <value>  Cookie header with the operator session (or JUDGE_SESSION_COOKIE)
  --json            Print the raw response body
  --help            Show this text

The cookie is the operator's own session. With the API branch as it stands, sign in with
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

/** Builds the X-91 request body (draft) and, for a fixture case, its expected outcome. */
export function buildRequestBody(options) {
  if (!options.run) throw new UsageError("--run is required");
  const sources = ["text", "case", "proposal"].filter((name) => options[name] !== undefined);
  if (sources.length !== 1) throw new UsageError("give exactly one of --text, --case, --proposal");

  if (options.proposal !== undefined) {
    const proposal = JSON.parse(readFileSync(options.proposal, "utf8"));
    if (typeof proposal.tool !== "string" || typeof proposal.arguments !== "object") {
      throw new UsageError('the proposal file must hold {"tool": "...", "arguments": {...}}');
    }
    return {
      body: {
        run_id: options.run,
        kind: "action_proposal",
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
  const body = { run_id: options.run, kind, text: fixtureCase?.text ?? options.text };
  if (kind === "tool_result") body.tool = options.tool ?? "read_invoice";
  if (kind === "model_input" && options.model) body.model = options.model;
  return { body, expectedOutcome: fixtureCase?.expected_outcome };
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

/** Human-readable summary lines of a decision response. */
export function describeDecision(body, expectedOutcome) {
  const lines = [
    `decision:         ${body.decision}${body.reason_code ? ` (${body.reason_code})` : ""}`,
    `message:          ${body.safe_message ?? ""}`,
    `active revision:  ${body.catalog?.active_revision ?? "unknown"} (feed ${body.catalog?.feed_revision ?? "unknown"})`,
    `evaluation:       ${body.evaluation_id ?? "unknown"}`,
  ];
  for (const control of body.controls ?? []) {
    const rule = control.rule_id ? ` rule ${control.rule_id}` : "";
    lines.push(`  control ${control.control}: ${control.outcome}${rule}`);
  }
  if (body.semantic) {
    // A fixture verdict is not semantic detection quality; say which one this was.
    lines.push(
      `  semantic verdict: ${body.semantic.source === "live" ? "live model" : "FIXTURE, not a live verdict"}`,
    );
  }
  if (body.content?.text) lines.push(`redacted text:    ${body.content.text}`);
  if (body.timings_ms) lines.push(`timings (ms):     ${JSON.stringify(body.timings_ms)}`);
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
        model: { type: "string" },
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
  console.error(DRAFT_NOTICE);

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
