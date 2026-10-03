#!/usr/bin/env node
// End-to-end check of the operator flow through the web app (WEB-18, WEB-20, WEB-24), without a
// browser and without new dependencies: it talks to the web server's own routes and pages with
// fetch, like the browser would, and runs `pnpm reset:demo` between rounds.
//
// Per round: sign in, start the Atlas run, follow it to the approval request, read the exact
// action under review, approve it, wait for the run to complete, read the stored report and the
// security summary, check that a finished run refuses a judge evaluation (run_not_active), and
// load the run, report, security and judge pages. Then reset the demo data and repeat.
//
// Limits, stated plainly: it checks routes, statuses, contract fields and the server-rendered page
// shell. It does not execute client-side React, so it does not prove what a browser renders. The
// run is real (local model, real gateway); the agent's path can vary, so an unexpected run state
// fails the round with the state it reached.
//
// Run from the repository root with the stack up (`pnpm dev`):
//   node scripts/with-env.mjs node apps/web/scripts/e2e-flow.mjs
// Environment (all optional): E2E_WEB_URL (default http://localhost:$WEB_PORT), E2E_ROUNDS (2),
// E2E_RUN_TIMEOUT_MS (240000), E2E_SKIP_RESET=1, E2E_SKIP_PAGES=1 (API-only dry run, point
// E2E_WEB_URL at the API), DEMO_OPERATOR_EMAIL (demo-operator@example.com), DEMO_OPERATOR_PASSWORD.
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repositoryRoot = fileURLToPath(new URL("../../../", import.meta.url));
const webBaseUrl = (
  process.env.E2E_WEB_URL ?? `http://localhost:${process.env.WEB_PORT ?? "3000"}`
).replace(/\/+$/, "");
const roundCount = Number(process.env.E2E_ROUNDS ?? "2");
const runTimeoutMs = Number(process.env.E2E_RUN_TIMEOUT_MS ?? "240000");
const skipReset = process.env.E2E_SKIP_RESET === "1";
const skipPages = process.env.E2E_SKIP_PAGES === "1";
const operatorEmail = process.env.DEMO_OPERATOR_EMAIL ?? "demo-operator@example.com";
const operatorPassword = process.env.DEMO_OPERATOR_PASSWORD;

// The seeded Atlas scenario; the review requirement pauses the run at its report.
const START_RUN_BODY = {
  template: "reconcile_atlas_v1",
  vendorId: "vendor_Atlas",
  invoiceIds: ["invoice_A01", "invoice_A02"],
  destination: "vendor_Atlas",
  approvalRequirement: "review_queue_report",
};
const TERMINAL_STATUSES = ["completed", "failed", "stopped"];

class CheckFailure extends Error {}

function expect(condition, message) {
  if (!condition) throw new CheckFailure(message);
}

/** A tiny cookie jar: the web server sets one HttpOnly session cookie. */
class Session {
  cookie = "";

  async request(path, { method = "GET", body, redirect = "manual" } = {}) {
    const response = await fetch(`${webBaseUrl}${path}`, {
      method,
      redirect,
      headers: {
        accept: "application/json",
        ...(body === undefined ? {} : { "content-type": "application/json" }),
        ...(this.cookie ? { cookie: this.cookie } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    for (const setCookie of response.headers.getSetCookie()) {
      const pair = setCookie.split(";")[0];
      if (pair?.startsWith("session=")) this.cookie = pair;
    }
    const text = await response.text();
    let json;
    try {
      json = JSON.parse(text);
    } catch {
      json = undefined;
    }
    return { status: response.status, json, text, response };
  }
}

const results = [];

async function step(round, name, action) {
  const startedAt = performance.now();
  try {
    const value = await action();
    results.push({ round, name, ok: true });
    console.log(`  PASS  ${name} (${Math.round(performance.now() - startedAt)} ms)`);
    return value;
  } catch (error) {
    results.push({ round, name, ok: false, detail: error.message });
    console.log(`  FAIL  ${name}: ${error.message}`);
    throw error;
  }
}

async function waitFor(description, read, isDone, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let last;
  while (Date.now() < deadline) {
    last = await read();
    if (isDone(last)) return last;
    await new Promise((resolve) => setTimeout(resolve, 2000));
  }
  throw new CheckFailure(
    `timed out waiting for ${description}; last seen: ${JSON.stringify(last)}`,
  );
}

async function runRound(round) {
  console.log(`\nRound ${round} of ${roundCount}`);
  const session = new Session();

  await step(round, "sign in as the seeded demo operator", async () => {
    expect(operatorPassword, "DEMO_OPERATOR_PASSWORD is not set (run with scripts/with-env.mjs)");
    const result = await session.request("/api/auth/sign-in", {
      method: "POST",
      body: { email: operatorEmail, password: operatorPassword },
    });
    expect(result.status === 200 || result.status === 201, `sign-in answered ${result.status}`);
    expect(session.cookie, "sign-in set no session cookie");
  });

  const { runId } = await step(round, "start the Atlas run", async () => {
    const result = await session.request("/api/runs", { method: "POST", body: START_RUN_BODY });
    expect(result.status === 201 || result.status === 200, `start run answered ${result.status}`);
    expect(typeof result.json?.runId === "string", "start run returned no runId");
    return { runId: result.json.runId };
  });

  const approval = await step(round, "follow the run to its approval request", async () => {
    const state = await waitFor(
      "the run to await approval",
      async () => (await session.request(`/api/runs/${runId}`)).json,
      (run) => run?.status === "awaiting_approval" || TERMINAL_STATUSES.includes(run?.status),
      runTimeoutMs,
    );
    expect(
      state.status === "awaiting_approval",
      `the run ended as ${state.status} before approval`,
    );
    const events = await session.request(`/api/runs/${runId}/events?limit=500`);
    expect(events.status === 200, `events answered ${events.status}`);
    const requested = events.json?.events?.find(
      (event) => event.eventType === "approval.requested",
    );
    expect(requested?.actionId, "no approval.requested event with an actionId");
    return { actionId: requested.actionId, reportId: requested.maskedSummary?.reportId };
  });

  await step(round, "read the exact action under review", async () => {
    const review = await session.request(`/api/actions/${approval.actionId}/review`);
    expect(review.status === 200, `review answered ${review.status}`);
    expect(review.json?.tool === "queue_report", `reviewed tool is ${review.json?.tool}`);
    expect(review.json?.run_id === runId, "the review belongs to another run");
    expect(
      review.json?.report?.id === approval.reportId,
      "the reviewed report differs from the one in the event",
    );
  });

  await step(round, "approve the exact action", async () => {
    const decision = await session.request(`/api/actions/${approval.actionId}/approval`, {
      method: "POST",
      body: { decision: "approve" },
    });
    expect(
      decision.status === 200 || decision.status === 201,
      `approval answered ${decision.status}`,
    );
  });

  await step(round, "the run completes after approval", async () => {
    const state = await waitFor(
      "the run to finish",
      async () => (await session.request(`/api/runs/${runId}`)).json,
      (run) => TERMINAL_STATUSES.includes(run?.status),
      runTimeoutMs,
    );
    expect(state.status === "completed", `the run ended as ${state.status}`);
  });

  await step(round, "read the stored report", async () => {
    const report = await session.request(`/api/runs/${runId}/reports/${approval.reportId}`);
    expect(report.status === 200, `report answered ${report.status}`);
    expect(report.json?.classification, "the report carries no stored classification");
  });

  await step(round, "read the security summary", async () => {
    const summary = await session.request("/api/security/summary");
    expect(summary.status === 200, `security summary answered ${summary.status}`);
    expect(summary.json && typeof summary.json === "object", "the summary is not an object");
  });

  await step(round, "a finished run refuses a judge evaluation (run_not_active)", async () => {
    const evaluation = await session.request("/api/control/evaluate", {
      method: "POST",
      body: {
        runId,
        kind: "model_input",
        text: "Summarize the duplicates.",
        tool: null,
        arguments: null,
      },
    });
    expect(evaluation.status === 200, `evaluate answered ${evaluation.status}`);
    expect(
      evaluation.json?.decision === "deny" && evaluation.json?.reasonCode === "run_not_active",
      `expected a run_not_active deny, got ${evaluation.json?.decision}/${evaluation.json?.reasonCode}`,
    );
    expect(evaluation.json?.actionId === null, "an evaluation must never create an action");
  });

  if (!skipPages) {
    const pages = [
      `/runs/${runId}`,
      `/runs/${runId}/reports/${approval.reportId}`,
      "/security",
      "/judge",
    ];
    for (const page of pages) {
      await step(
        round,
        `page ${page.replace(runId, "{run}").replace(approval.reportId, "{report}")}`,
        async () => {
          const result = await session.request(page);
          expect(result.status === 200, `answered ${result.status} (a redirect means no session)`);
          expect(result.text.includes("<html"), "the response is not an HTML page");
        },
      );
    }
  }
}

function resetDemoData(round) {
  return step(round, "reset the demo data (pnpm reset:demo)", async () => {
    const reset = spawnSync("pnpm", ["reset:demo"], { cwd: repositoryRoot, encoding: "utf8" });
    expect(
      reset.status === 0,
      `pnpm reset:demo exited ${reset.status}: ${reset.stderr || reset.stdout}`.slice(0, 600),
    );
  });
}

let failed = false;
try {
  for (let round = 1; round <= roundCount; round += 1) {
    await runRound(round);
    if (!skipReset) await resetDemoData(round);
  }
} catch {
  failed = true;
}

const failedSteps = results.filter((result) => !result.ok);
console.log(
  `\n${results.length - failedSteps.length} passed, ${failedSteps.length} failed` +
    (failed && failedSteps.length === 0 ? " (stopped on an error)" : ""),
);
process.exit(failed || failedSteps.length > 0 ? 1 : 0);
