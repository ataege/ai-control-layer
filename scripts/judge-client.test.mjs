// Tests of the draft judge client against a local stand-in HTTP server. The stand-in only answers
// with fixed bodies shaped like the draft contract; it tests the client, not any control.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { after, before, test } from "node:test";

import { buildRequestBody, EVALUATE_PATH, main, UsageError } from "./judge-client.mjs";

const SESSION_COOKIE = "session=stand-in-session-value";
const DECISION_BODY = {
  evaluation_id: "eval_stand_in",
  run_id: "run_stand_in",
  decision: "deny",
  reason_code: "signature_match",
  safe_message: "withheld",
  controls: [
    { control: "signature_match", outcome: "block", rule_id: "prompt_ignore_previous_v1" },
  ],
  semantic: { source: "fixture" },
  catalog: { active_revision: "policy_v2", feed_revision: "feed_v1" },
};

let standInServer;
let apiUrl;
let nextResponse;
const receivedRequests = [];

before(async () => {
  standInServer = createServer((request, response) => {
    let requestText = "";
    request.on("data", (chunk) => (requestText += chunk));
    request.on("end", () => {
      receivedRequests.push({
        path: request.url,
        headers: request.headers,
        body: JSON.parse(requestText),
      });
      response.writeHead(nextResponse.status, { "content-type": "application/json" });
      response.end(JSON.stringify(nextResponse.body));
    });
  });
  await new Promise((resolveListening) => standInServer.listen(0, "127.0.0.1", resolveListening));
  apiUrl = `http://127.0.0.1:${standInServer.address().port}`;
});
after(() => standInServer.close());

/** Runs the CLI and captures what it prints. */
async function runClient(argv, environment = {}) {
  const printed = [];
  const originalLog = console.log;
  const originalError = console.error;
  console.log = (...parts) => printed.push(parts.join(" "));
  console.error = (...parts) => printed.push(parts.join(" "));
  try {
    const exitCode = await main(argv, environment);
    return { exitCode, output: printed.join("\n") };
  } finally {
    console.log = originalLog;
    console.error = originalError;
  }
}

test("a fixture case becomes a tool_result request with its text", () => {
  const { body, expectedOutcome } = buildRequestBody({
    run: "run_1",
    case: "indirect_ignore_previous_note_v1",
  });
  assert.equal(body.kind, "tool_result");
  assert.equal(body.tool, "read_invoice");
  assert.match(body.text, /Ignore previous instructions/);
  assert.equal(expectedOutcome, "block");
  assert.equal("expectedOutcome" in body, false);
});

test("the request body never carries identity fields", () => {
  const { body } = buildRequestBody({ run: "run_1", text: "hello", model: "qwen3.5:4b" });
  assert.deepEqual(Object.keys(body).sort(), ["kind", "model", "run_id", "text"]);
});

test("input sources are exclusive and --run is required", () => {
  assert.throws(() => buildRequestBody({ text: "x" }), UsageError);
  assert.throws(
    () => buildRequestBody({ run: "r", text: "x", case: "benign_vendor_record_v1" }),
    UsageError,
  );
  assert.throws(() => buildRequestBody({ run: "r", case: "no_such_case" }), UsageError);
  assert.throws(
    () => buildRequestBody({ run: "r", text: "x", kind: "action_proposal" }),
    UsageError,
  );
});

test("sends the session cookie, prints the decision and revision, never the cookie", async () => {
  nextResponse = { status: 200, body: DECISION_BODY };
  const { exitCode, output } = await runClient(
    ["--run", "run_stand_in", "--case", "indirect_ignore_previous_note_v1", "--api-url", apiUrl],
    { JUDGE_SESSION_COOKIE: SESSION_COOKIE },
  );
  assert.equal(exitCode, 0);
  const lastRequest = receivedRequests.at(-1);
  assert.equal(lastRequest.path, EVALUATE_PATH);
  assert.equal(lastRequest.headers.cookie, SESSION_COOKIE);
  assert.match(output, /decision: +deny \(signature_match\)/);
  assert.match(output, /active revision: +policy_v2/);
  assert.match(output, /FIXTURE, not a live verdict/);
  assert.match(output, /matches the expected block/);
  assert.equal(output.includes("stand-in-session-value"), false);
});

test("an error envelope is reported as no decision with a nonzero exit", async () => {
  nextResponse = {
    status: 401,
    body: { error: { code: "unauthorized", message: "Authentication required" }, statusCode: 401 },
  };
  const { exitCode, output } = await runClient(
    ["--run", "r", "--text", "hi", "--api-url", apiUrl],
    {
      JUDGE_SESSION_COOKIE: SESSION_COOKIE,
    },
  );
  assert.equal(exitCode, 1);
  assert.match(output, /No decision: HTTP 401 unauthorized/);
  assert.equal(/^decision:/m.test(output), false);
});

test("a 200 without a decision is not treated as one", async () => {
  nextResponse = { status: 200, body: { status: "ok" } };
  const { exitCode, output } = await runClient(
    ["--run", "r", "--text", "hi", "--api-url", apiUrl],
    {
      JUDGE_SESSION_COOKIE: SESSION_COOKIE,
    },
  );
  assert.equal(exitCode, 1);
  assert.match(output, /No decision/);
});

test("an unreachable API and a missing cookie fail without a decision", async () => {
  const unreachable = await runClient(
    ["--run", "r", "--text", "hi", "--api-url", "http://127.0.0.1:1"],
    {
      JUDGE_SESSION_COOKIE: SESSION_COOKIE,
    },
  );
  assert.equal(unreachable.exitCode, 1);
  assert.match(unreachable.output, /No decision: cannot reach/);

  const withoutCookie = await runClient(["--run", "r", "--text", "hi", "--api-url", apiUrl]);
  assert.equal(withoutCookie.exitCode, 2);
  assert.match(withoutCookie.output, /No operator session/);
});
