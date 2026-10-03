import { describe, expect, it } from "vitest";

import {
  cancelRun,
  decideApproval,
  describeFailure,
  getReview,
  getRunState,
} from "./actions-client";

const actionId = "df036f8b-f6bb-4e9e-bf60-a6c50ae4c830";
const runId = "073514cb-6817-4cac-8af7-516e72952704";

interface RecordedRequest {
  url: string;
  method: string;
  body: string | undefined;
}

// A fetch stand-in that records the request and answers with one fixed response.
function fakeFetch(status: number, body: unknown, recorded: RecordedRequest[]): typeof fetch {
  return (async (input: RequestInfo | URL, init?: RequestInit) => {
    recorded.push({
      url: String(input),
      method: init?.method ?? "GET",
      body: typeof init?.body === "string" ? init.body : undefined,
    });
    return new Response(JSON.stringify(body), {
      status,
      headers: { "content-type": "application/json" },
    });
  }) as typeof fetch;
}

function errorBody(code: string) {
  return { error: { code, message: "Gateway request failed" }, statusCode: 409 };
}

describe("decideApproval", () => {
  it("sends only the decision for the displayed action", async () => {
    const recorded: RecordedRequest[] = [];
    const result = await decideApproval(actionId, "approve", {
      fetchImplementation: fakeFetch(
        200,
        {
          approvalId: "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
          actionId,
          runId,
          decision: "approve",
        },
        recorded,
      ),
    });
    expect(result.ok).toBe(true);
    expect(recorded).toHaveLength(1);
    expect(recorded[0]?.url).toBe(`/api/actions/${actionId}/approval`);
    expect(recorded[0]?.method).toBe("POST");
    // Exactly the decision: no content, recipient or action fields leave the browser.
    expect(JSON.parse(recorded[0]?.body ?? "null")).toStrictEqual({ decision: "approve" });
  });

  it("never reports an expired, changed or stopped outcome as approved", async () => {
    const cases: [string, string][] = [
      ["approval_expired", "expired"],
      ["action_changed", "changed"],
      ["run_cancelled", "run_stopped"],
      ["conflict", "already_decided"],
    ];
    for (const [code, kind] of cases) {
      const result = await decideApproval(actionId, "approve", {
        fetchImplementation: fakeFetch(409, errorBody(code), []),
      });
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.failure.kind).toBe(kind);
        expect(result.failure.message).not.toMatch(/approved/i);
      }
    }
  });

  it("treats an answer about another action or decision as unconfirmed", async () => {
    const result = await decideApproval(actionId, "approve", {
      fetchImplementation: fakeFetch(
        200,
        { approvalId: "x", actionId, runId, decision: "reject" },
        [],
      ),
    });
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.failure.kind).toBe("unconfirmed");
  });

  it("sends nothing for an identifier that is not a record id", async () => {
    const recorded: RecordedRequest[] = [];
    const result = await decideApproval("../runs", "approve", {
      fetchImplementation: fakeFetch(200, {}, recorded),
    });
    expect(result.ok).toBe(false);
    expect(recorded).toHaveLength(0);
  });
});

describe("describeFailure", () => {
  it("maps every error status the review page shows", () => {
    const http = (status: number, code?: string) =>
      describeFailure({
        kind: "http",
        status,
        body: code ? { error: { code } } : undefined,
      }).kind;
    expect(http(403, "forbidden")).toBe("not_reviewer");
    expect(http(404, "not_found")).toBe("not_found");
    expect(http(401, "unauthorized")).toBe("signed_out");
    expect(http(504, "outcome_unconfirmed")).toBe("unconfirmed");
    expect(http(503, "decision_unavailable")).toBe("unavailable");
    expect(http(400, "bad_request")).toBe("rejected_request");
    expect(describeFailure({ kind: "timeout", timeoutMs: 15000 }).kind).toBe("unconfirmed");
    expect(describeFailure({ kind: "network", message: "offline" }).kind).toBe("unavailable");
    // A command whose connection was lost may have been carried out: never "nothing was recorded".
    expect(describeFailure({ kind: "network", message: "offline" }, { command: true }).kind).toBe(
      "unconfirmed",
    );
  });
});

describe("getReview", () => {
  it("refuses a review of another action", async () => {
    const result = await getReview(actionId, {
      fetchImplementation: fakeFetch(
        200,
        { action_id: "11111111-2222-4333-8444-555555555555" },
        [],
      ),
    });
    expect(result.ok).toBe(false);
  });

  it("reads the review from the same-origin route", async () => {
    const recorded: RecordedRequest[] = [];
    const result = await getReview(actionId, {
      fetchImplementation: fakeFetch(200, { action_id: actionId }, recorded),
    });
    expect(result.ok).toBe(true);
    expect(recorded[0]?.url).toBe(`/api/actions/${actionId}/review`);
    expect(recorded[0]?.method).toBe("GET");
  });
});

describe("cancelRun", () => {
  it("posts an empty command and returns the recorded run state", async () => {
    const recorded: RecordedRequest[] = [];
    const result = await cancelRun(runId, {
      fetchImplementation: fakeFetch(
        200,
        { runId, status: "stopped", terminalReason: "run_cancelled" },
        recorded,
      ),
    });
    expect(result.ok).toBe(true);
    expect(recorded[0]?.url).toBe(`/api/runs/${runId}/cancel`);
    expect(JSON.parse(recorded[0]?.body ?? "null")).toStrictEqual({});
  });

  it("names an unknown run, not a missing review", async () => {
    const result = await cancelRun(runId, {
      fetchImplementation: fakeFetch(404, { error: { code: "not_found" } }, []),
    });
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.failure.message).toBe("No run was found.");
  });
});

describe("getRunState", () => {
  it("reads the run from the same-origin route and refuses another run's state", async () => {
    const recorded: RecordedRequest[] = [];
    const own = await getRunState(runId, {
      fetchImplementation: fakeFetch(200, { runId, status: "awaiting_approval" }, recorded),
    });
    expect(own.ok).toBe(true);
    expect(recorded[0]?.url).toBe(`/api/runs/${runId}`);
    const other = await getRunState(runId, {
      fetchImplementation: fakeFetch(200, { runId: actionId, status: "completed" }, []),
    });
    expect(other.ok).toBe(false);
  });
});
