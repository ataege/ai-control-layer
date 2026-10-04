// Tests of the judge client. The request body is validated against the frozen X-91 schema in
// packages/contracts, and the responses are the contract's own fixtures, so the client and the
// contract cannot drift apart unnoticed. A local stand-in HTTP server stands in for the API; this
// tests the client, not any control, and makes no live call.
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, test } from "node:test";

import { buildRequestBody, EVALUATE_PATH, main, UsageError } from "./judge-client.mjs";
import { fromRepositoryRoot } from "./lib/repo-root.mjs";

// Ajv is a dependency of packages/contracts, where the contract's own tests use it.
const requireFromContracts = createRequire(
  fromRepositoryRoot("packages", "contracts", "package.json"),
);
const { Ajv2020 } = requireFromContracts("ajv/dist/2020.js");

const contractFile = (...segments) =>
  JSON.parse(readFileSync(fromRepositoryRoot("packages", "contracts", ...segments), "utf8"));
const validateRequest = new Ajv2020({ allErrors: true }).compile(
  contractFile("schemas", "control-evaluation-request.schema.json"),
);
const assertMatchesContract = (body) =>
  assert.ok(
    validateRequest(body),
    `the request body breaks the X-91 schema: ${JSON.stringify(validateRequest.errors)}`,
  );

const SESSION_COOKIE = "session=stand-in-session-value";
const RUN_ID = "5f0c1a2b-3c4d-4e5f-8a9b-0c1d2e3f4a5b";
const requestFixtures = {
  model_input: contractFile("fixtures", "control-evaluation-request.model-input.json"),
  tool_result: contractFile("fixtures", "control-evaluation-request.tool-result.json"),
  action_proposal: contractFile("fixtures", "control-evaluation-request.action-proposal.json"),
};
const DECISION_BODY = contractFile("fixtures", "control-evaluation-response.signature-deny.json");

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

test("the request for each boundary satisfies the X-91 schema and equals the contract's fixture", () => {
  const modelInput = buildRequestBody({
    run: RUN_ID,
    text: requestFixtures.model_input.text,
  }).body;
  const toolResult = buildRequestBody({
    run: RUN_ID,
    kind: "tool_result",
    tool: "read_invoice",
    text: requestFixtures.tool_result.text,
  }).body;
  const proposalDirectory = mkdtempSync(join(tmpdir(), "judge-proposal-"));
  try {
    const proposalPath = join(proposalDirectory, "proposal.json");
    writeFileSync(
      proposalPath,
      JSON.stringify({
        tool: requestFixtures.action_proposal.tool,
        arguments: requestFixtures.action_proposal.arguments,
      }),
    );
    const actionProposal = buildRequestBody({ run: RUN_ID, proposal: proposalPath }).body;
    for (const [kind, body] of Object.entries({
      model_input: modelInput,
      tool_result: toolResult,
      action_proposal: actionProposal,
    })) {
      assertMatchesContract(body);
      assert.deepEqual(body, requestFixtures[kind], `the ${kind} body differs from the fixture`);
    }
  } finally {
    rmSync(proposalDirectory, { recursive: true, force: true });
  }
});

test("the schema check has teeth: the old draft body and unknown fields are refused", () => {
  assert.equal(validateRequest({ run_id: RUN_ID, kind: "model_input", text: "x" }), false);
  assert.equal(validateRequest({ ...requestFixtures.model_input, model: "qwen3.5:4b" }), false);
  assert.equal(
    validateRequest({ runId: RUN_ID, kind: "model_input", text: "x", tool: null }),
    false,
    "a body that omits the null fields must fail",
  );
});

test("a fixture case becomes a tool_result request with its text", () => {
  const { body, expectedOutcome } = buildRequestBody({
    run: RUN_ID,
    case: "indirect_ignore_previous_note_v1",
  });
  assertMatchesContract(body);
  assert.equal(body.kind, "tool_result");
  assert.equal(body.tool, "read_invoice");
  assert.equal(body.arguments, null);
  assert.match(body.text, /Ignore previous instructions/);
  assert.equal(expectedOutcome, "block");
  assert.equal("expectedOutcome" in body, false);
});

test("every fixture case builds a request the schema accepts", () => {
  for (const file of ["semantic-corpus.json", "hostile-notes.json"]) {
    const fixture = JSON.parse(readFileSync(fromRepositoryRoot("fixtures", file), "utf8"));
    for (const entry of [...(fixture.cases ?? []), ...(fixture.notes ?? [])]) {
      assertMatchesContract(buildRequestBody({ run: RUN_ID, case: entry.id }).body);
    }
  }
});

test("the body has exactly the five X-91 fields and never carries identity", () => {
  const { body } = buildRequestBody({ run: RUN_ID, text: "hello" });
  assert.deepEqual(Object.keys(body).sort(), ["arguments", "kind", "runId", "text", "tool"]);
});

test("input sources are exclusive and the run, tool and text are checked before sending", () => {
  assert.throws(() => buildRequestBody({ text: "x" }), UsageError);
  assert.throws(() => buildRequestBody({ run: "run_1", text: "x" }), UsageError, "a non-UUID run");
  assert.throws(
    () => buildRequestBody({ run: RUN_ID, text: "x", case: "benign_vendor_record_v1" }),
    UsageError,
  );
  assert.throws(() => buildRequestBody({ run: RUN_ID, case: "no_such_case" }), UsageError);
  assert.throws(
    () => buildRequestBody({ run: RUN_ID, text: "x", kind: "action_proposal" }),
    UsageError,
  );
  assert.throws(() => buildRequestBody({ run: RUN_ID, text: "x", tool: "drop_table" }), UsageError);
  assert.throws(() => buildRequestBody({ run: RUN_ID, text: "" }), UsageError);
  // 2049 two-byte characters fit 4096 UTF-16 units but not 4096 UTF-8 bytes.
  assert.throws(() => buildRequestBody({ run: RUN_ID, text: "é".repeat(2049) }), /too long/);
  assert.ok(buildRequestBody({ run: RUN_ID, text: "é".repeat(2048) }).body);
});

test("the draft --model option is gone", async () => {
  const { exitCode, output } = await runClient(["--run", RUN_ID, "--text", "x", "--model", "m"], {
    JUDGE_SESSION_COOKIE: SESSION_COOKIE,
  });
  assert.equal(exitCode, 2);
  assert.match(output, /Usage:/);
});

test("sends a schema-valid body and the session cookie; prints the X-91 decision, never the cookie", async () => {
  nextResponse = { status: 200, body: DECISION_BODY };
  const { exitCode, output } = await runClient(
    ["--run", RUN_ID, "--case", "indirect_ignore_previous_note_v1", "--api-url", apiUrl],
    { JUDGE_SESSION_COOKIE: SESSION_COOKIE },
  );
  assert.equal(exitCode, 0);
  const lastRequest = receivedRequests.at(-1);
  assert.equal(lastRequest.path, EVALUATE_PATH);
  assert.equal(lastRequest.headers.cookie, SESSION_COOKIE);
  assertMatchesContract(lastRequest.body);
  assert.match(output, /decision: +deny \(signature_match\)/);
  assert.match(output, /active revision: +2 \(feed 1, admitted under 1\)/);
  assert.match(
    output,
    /control signature_match \(deterministic\): block \(signature_match, rule prompt_ignore_previous_v1\)/,
  );
  assert.match(output, /evaluation: +6a7b8c9d-0e1f-4a2b-8c3d-4e5f6a7b8c9d/);
  assert.match(output, /matches the expected block/);
  assert.equal(output.includes("stand-in-session-value"), false);
  assert.equal(output.includes("DRAFT"), false);
});

test("a redact decision prints the server's redacted text and a fixture verdict says so", async () => {
  nextResponse = {
    status: 200,
    body: contractFile("fixtures", "control-evaluation-response.semantic-redact.json"),
  };
  const { exitCode, output } = await runClient(
    ["--run", RUN_ID, "--text", "x", "--api-url", apiUrl],
    { JUDGE_SESSION_COOKIE: SESSION_COOKIE },
  );
  assert.equal(exitCode, 0);
  assert.match(output, /redacted text: +Use account \[REDACTED:bank_account\]/);
  assert.match(output, /semantic verdict: live model/);

  nextResponse = {
    status: 200,
    body: { ...nextResponse.body, semantic: { ...nextResponse.body.semantic, source: "fixture" } },
  };
  const fixtureRun = await runClient(["--run", RUN_ID, "--text", "x", "--api-url", apiUrl], {
    JUDGE_SESSION_COOKIE: SESSION_COOKIE,
  });
  assert.match(fixtureRun.output, /FIXTURE, not a live verdict/);
});

test("an error envelope is reported as no decision with a nonzero exit", async () => {
  nextResponse = {
    status: 401,
    body: { error: { code: "unauthorized", message: "Authentication required" }, statusCode: 401 },
  };
  const { exitCode, output } = await runClient(
    ["--run", RUN_ID, "--text", "hi", "--api-url", apiUrl],
    { JUDGE_SESSION_COOKIE: SESSION_COOKIE },
  );
  assert.equal(exitCode, 1);
  assert.match(output, /No decision: HTTP 401 unauthorized/);
  assert.equal(/^decision:/m.test(output), false);
});

test("a 200 without a decision is not treated as one", async () => {
  nextResponse = { status: 200, body: { status: "ok" } };
  const { exitCode, output } = await runClient(
    ["--run", RUN_ID, "--text", "hi", "--api-url", apiUrl],
    { JUDGE_SESSION_COOKIE: SESSION_COOKIE },
  );
  assert.equal(exitCode, 1);
  assert.match(output, /No decision/);
});

test("an unreachable API and a missing cookie fail without a decision", async () => {
  const unreachable = await runClient(
    ["--run", RUN_ID, "--text", "hi", "--api-url", "http://127.0.0.1:1"],
    { JUDGE_SESSION_COOKIE: SESSION_COOKIE },
  );
  assert.equal(unreachable.exitCode, 1);
  assert.match(unreachable.output, /No decision: cannot reach/);

  const withoutCookie = await runClient(["--run", RUN_ID, "--text", "hi", "--api-url", apiUrl]);
  assert.equal(withoutCookie.exitCode, 2);
  assert.match(withoutCookie.output, /No operator session/);
});

test("a malformed proposal file is rejected", () => {
  const proposalDirectory = mkdtempSync(join(tmpdir(), "judge-proposal-"));
  try {
    for (const proposal of [
      { tool: "read_invoice" },
      { tool: "read_invoice", arguments: [1] },
      { tool: "drop_table", arguments: {} },
    ]) {
      const path = join(proposalDirectory, "bad.json");
      writeFileSync(path, JSON.stringify(proposal));
      assert.throws(() => buildRequestBody({ run: RUN_ID, proposal: path }), UsageError);
    }
  } finally {
    rmSync(proposalDirectory, { recursive: true, force: true });
  }
});
