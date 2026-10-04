import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { FetchJsonError } from "@/lib/fetch-json";
import { classifyFailure, failureFromReason } from "@/lib/errors/failure";
import { FailureState, signInHref } from "./failure-state";

function render(
  error: FetchJsonError,
  props: { requestId?: string; onRetry?: () => void; command?: boolean } = {},
) {
  const failure = classifyFailure(error, { command: props.command });
  return renderToStaticMarkup(
    createElement(FailureState, { failure, requestId: props.requestId, onRetry: props.onRetry }),
  );
}

describe("FailureState", () => {
  it("sends an expired session to sign in and back to the page", () => {
    const html = render({ kind: "http", status: 401, body: undefined });
    expect(html).toContain("Your session has ended");
    expect(html).toContain('href="/login?callbackUrl=%2F"');
    expect(html).not.toContain("Try again");
    expect(signInHref("/runs/r1/reports/p1")).toBe(
      "/login?callbackUrl=%2Fruns%2Fr1%2Freports%2Fp1",
    );
  });

  it("offers a retry only for a failure that allows one, and only when the page can retry", () => {
    const offline: FetchJsonError = { kind: "network", message: "Failed to fetch" };
    expect(render(offline, { onRetry: () => undefined })).toContain("Try again");
    expect(render(offline)).not.toContain("Try again");
    const forbidden: FetchJsonError = { kind: "http", status: 403, body: undefined };
    expect(render(forbidden, { onRetry: () => undefined })).not.toContain("Try again");
  });

  it("says where it failed and shows the request id", () => {
    const html = render(
      { kind: "http", status: 502, body: { error: { code: "upstream_unreachable" } } },
      { requestId: "req-42" },
    );
    expect(html).toContain("The API could not be reached");
    expect(html).toContain("Where: The API");
    expect(html).toContain("Request ID: req-42");
    expect(html).toContain('data-failure-kind="upstream_unreachable"');
  });

  it("marks a command without an answer as unconfirmed and offers no retry", () => {
    const html = render(
      { kind: "timeout", timeoutMs: 15000 },
      { command: true, onRetry: () => undefined },
    );
    expect(html).toContain("Outcome unconfirmed.");
    expect(html).toContain("check the run before submitting it again");
    expect(html).not.toContain("Try again");
  });

  it("shows a live model failure apart from the working gateway", () => {
    const html = renderToStaticMarkup(
      createElement(FailureState, { failure: failureFromReason("security_evaluator_unavailable") }),
    );
    expect(html).toContain("Where: The live model (security_evaluator_unavailable)");
    expect(html).toContain("The gateway reported this itself");
    expect(html).toContain("labelled replays do not need the model");
  });

  it("does not show the live model note for a failure that is not the model's", () => {
    expect(render({ kind: "network", message: "x" })).not.toContain("labelled replays");
  });
});
