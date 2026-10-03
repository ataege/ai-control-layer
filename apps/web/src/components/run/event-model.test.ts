import { describe, expect, it } from "vitest";
import type { AssessmentRecord, SafeEvent } from "@workspace/contracts";
import exportDenied from "@workspace/contracts/fixtures/safe-event.export-denied.json";
import admissionRejected from "@workspace/contracts/fixtures/safe-event.admission-rejected.json";
import eventsPage from "@workspace/contracts/fixtures/run-events-page.export-denied.json";
import semanticJudge from "@workspace/contracts/fixtures/assessment-record.semantic-judge.json";
import semanticNotApplicable from "@workspace/contracts/fixtures/assessment-record.semantic-not-applicable.json";
import { decisionBadges, describeEvent, summarizeEvents, terminalSafeMessage } from "./event-model";

// The JSON fixtures are typed as plain strings; the contract types are the narrower ones.
const asEvent = (value: unknown): SafeEvent => value as SafeEvent;
const asAssessment = (value: unknown): AssessmentRecord => value as AssessmentRecord;

// An indexed read is possibly undefined; a missing one is a broken fixture, not a passing test.
function must<T>(value: T | undefined): T {
  if (value === undefined) {
    throw new Error("expected a value");
  }
  return value;
}

function event(
  overrides: Partial<SafeEvent> & Pick<SafeEvent, "eventType">,
  summary: Partial<SafeEvent["maskedSummary"]> = {},
): SafeEvent {
  const base = asEvent(exportDenied);
  return {
    ...base,
    eventId: String(Math.random()),
    actionId: null,
    decision: null,
    reasonCode: null,
    ...overrides,
    maskedSummary: {
      ...base.maskedSummary,
      reportId: null,
      template: null,
      classification: null,
      lineageCheck: null,
      effect: null,
      alternativeTemplate: null,
      safeMessage: null,
      replaySource: null,
      ...summary,
    },
  };
}

describe("describeEvent: attempts apart from effects", () => {
  it("shows the export denial as an attempt that changed nothing, with its rule and the way forward", () => {
    const view = describeEvent(asEvent(exportDenied));
    expect(view.kind).toBe("attempt");
    expect(view.tone).toBe("deny");
    expect(view.effectText).toBe("No change was made.");
    expect(view.reasonCode).toBe("report_export_restricted");
    expect(view.rule).toContain("Internal only restriction");
    expect(view.correctionRoute).toContain("vendor reconciliation report");
    expect(view.facts).toContainEqual({
      label: "Label (set by the gateway)",
      value: "Internal only",
    });
  });

  it("never renders a denied attempt as an effect, whatever its type says", () => {
    const view = describeEvent(
      event(
        { eventType: "action.succeeded", decision: "deny", reasonCode: "destination_not_allowed" },
        { effect: "outbox_message_queued" },
      ),
    );
    expect(view.kind).toBe("attempt");
    expect(view.effectText).toBeNull();
  });

  it("shows a completed effect with what it did, labelling the outbox as simulated", () => {
    const view = describeEvent(
      event(
        { eventType: "action.succeeded", decision: "allow" },
        { effect: "outbox_message_queued" },
      ),
    );
    expect(view.kind).toBe("effect");
    expect(view.effectText).toContain("simulated outbox");
    expect(view.effectText).toContain("nothing was delivered");
  });

  it("always shows the label of a replayed proposal", () => {
    const view = describeEvent(
      event(
        { eventType: "action.denied", decision: "deny", reasonCode: "destination_not_allowed" },
        { replaySource: "labelled_replay:hostile_note_redirect_recipient_v1" },
      ),
    );
    expect(view.replayLabel).toContain(
      "Replay (labelled_replay:hostile_note_redirect_recipient_v1)",
    );
    expect(view.replayLabel).toContain("not an action the model generated");
  });

  it("shows an unknown event type as unknown instead of dropping it", () => {
    const view = describeEvent(event({ eventType: "action.teleported" as SafeEvent["eventType"] }));
    expect(view.kind).toBe("unknown");
    expect(view.unrecognized).toBe(true);
    expect(view.title).toBe("Unknown event (action.teleported)");
  });

  it("shows an unrecognized reason code as such", () => {
    const view = describeEvent(
      event({
        eventType: "action.denied",
        decision: "deny",
        reasonCode: "brand_new_reason" as SafeEvent["reasonCode"],
      }),
    );
    expect(view.reasonText).toBe("Unrecognized reason (brand_new_reason)");
  });

  it("shows the fixed cause of a rejected final answer and never any text", () => {
    const view = describeEvent(
      event(
        { eventType: "action.denied", decision: "deny", reasonCode: "invalid_arguments" },
        { rejectionCause: "code_fence" },
      ),
    );
    expect(view.rejectionText).toContain("code fence");
  });

  it("describes every event of a stored page", () => {
    const page = (eventsPage as unknown as { events: SafeEvent[] }).events;
    expect(page.map((item) => describeEvent(item).unrecognized)).not.toContain(true);
    expect(page.length).toBeGreaterThan(1);
    expect(describeEvent(asEvent(admissionRejected)).kind).toBe("state");
  });
});

describe("summarizeEvents", () => {
  it("counts attempted operations apart from completed effects, once per action", () => {
    const events = [
      event({ eventType: "action.allowed", decision: "allow", actionId: "a" }),
      event(
        { eventType: "action.succeeded", decision: "allow", actionId: "a" },
        { effect: "read" },
      ),
      event({ eventType: "action.allowed", decision: "allow", actionId: "b" }),
      event(
        { eventType: "report.created", decision: "allow", actionId: "b" },
        { effect: "report_created" },
      ),
      event(
        { eventType: "action.succeeded", decision: "allow", actionId: "b" },
        { effect: "report_created" },
      ),
      event(
        {
          eventType: "report.export_denied",
          decision: "deny",
          reasonCode: "report_export_restricted",
          actionId: "c",
        },
        { effect: "none" },
      ),
      event({ eventType: "action.denied", decision: "deny", reasonCode: "invalid_arguments" }),
    ];
    const summary = summarizeEvents(events);
    expect(summary.attemptedOperations).toBe(4);
    expect(summary.deniedProposals).toBe(2);
    expect(summary.effects).toEqual({ reads: 1, reportsStored: 1, messagesQueued: 0 });
  });

  it("shows no effect for a run whose only send was denied", () => {
    const summary = summarizeEvents([asEvent(exportDenied)]);
    expect(summary.effects).toEqual({ reads: 0, reportsStored: 0, messagesQueued: 0 });
    expect(summary.attemptedOperations).toBe(1);
  });

  it("counts replays, unknown outcomes and unrecognized events", () => {
    const summary = summarizeEvents([
      event({ eventType: "action.unknown", actionId: "d" }),
      event(
        { eventType: "action.denied", decision: "deny", actionId: "e" },
        { replaySource: "labelled_replay:x" },
      ),
      event({ eventType: "action.teleported" as SafeEvent["eventType"] }),
    ]);
    expect(summary).toMatchObject({ uncertain: 1, replays: 1, unrecognizedEvents: 1 });
  });
});

describe("decisionBadges: hybrid decisions", () => {
  const evaluated = (evaluationId: string, overrides: Partial<SafeEvent> = {}) =>
    event({ eventType: "control.evaluated", decision: "deny", ...overrides }, { evaluationId });

  it("names the live semantic block and says the text was withheld", () => {
    const assessment = asAssessment(semanticJudge);
    const badge = must(decisionBadges(evaluated(assessment.evaluationId), [assessment])[0]);
    expect(badge).toMatchObject({ family: "semantic", result: "blocked", source: "live" });
    expect(badge.text).toContain("live model");
    expect(badge.consequence).toContain("never shown to the agent");
  });

  it("labels a fixture verdict as a fixture, not as detection quality", () => {
    const assessment = { ...asAssessment(semanticJudge), verdictSource: "fixture" as const };
    const badge = must(decisionBadges(evaluated(assessment.evaluationId), [assessment])[0]);
    expect(badge.source).toBe("fixture");
    expect(badge.text).toContain("fixture verdict, not detection quality");
  });

  it("shows a semantic check that had nothing to classify as not applicable, with no source", () => {
    const assessment = asAssessment(semanticNotApplicable);
    const badge = must(
      decisionBadges(evaluated(assessment.evaluationId, { decision: "allow" }), [assessment])[0],
    );
    expect(badge).toMatchObject({ family: "semantic", result: "not_applicable", source: null });
  });

  it("separates the signature control from the other deterministic controls", () => {
    const base = asAssessment(semanticJudge);
    const records = [
      {
        ...base,
        controlClass: "deterministic" as const,
        controlId: "signature_match",
        outcome: "block",
        verdictSource: null,
        verdict: null,
        matchedRuleId: "sig-7",
        feedRevision: "feed_v1",
      },
      {
        ...base,
        controlClass: "deterministic" as const,
        controlId: "field_limit",
        outcome: "pass",
        verdictSource: null,
        verdict: null,
      },
    ];
    const badges = decisionBadges(evaluated(base.evaluationId), records);
    expect(badges.map((badge) => [badge.family, badge.result])).toEqual([
      ["signature", "blocked"],
      ["deterministic", "passed"],
    ]);
    expect(must(badges[0]).detail).toContain("sig-7");
  });

  it("falls back to the one control a reason code names, never a guessed one", () => {
    const signature = decisionBadges(
      event({ eventType: "control.evaluated", decision: "deny", reasonCode: "signature_match" }),
      [],
    );
    expect(signature).toHaveLength(1);
    expect(must(signature[0])).toMatchObject({
      family: "signature",
      result: "blocked",
      source: null,
    });
    const redacted = decisionBadges(
      event({ eventType: "control.evaluated", decision: "redact", reasonCode: "content_redacted" }),
      [],
    );
    expect(must(redacted[0])).toMatchObject({ family: "unidentified", result: "redacted" });
    expect(decisionBadges(event({ eventType: "action.allowed", decision: "allow" }), [])).toEqual(
      [],
    );
  });

  it("shows a guard failure as a visible pause with nothing released", () => {
    const badge = must(
      decisionBadges(
        event({
          eventType: "control.evaluated",
          decision: "deny",
          reasonCode: "security_evaluator_unavailable",
        }),
        [],
      )[0],
    );
    expect(badge).toMatchObject({ family: "semantic", result: "unavailable" });
    expect(badge.consequence).toContain("Nothing was released");
  });
});

describe("terminalSafeMessage", () => {
  it("returns the message of the latest terminal event, as stored", () => {
    const events = [
      event({ eventType: "run.paused" }, { safeMessage: "first" }),
      event({ eventType: "action.allowed" }, { safeMessage: "not terminal" }),
      event({ eventType: "run.stopped" }, { safeMessage: "last" }),
      event({ eventType: "run.completed" }),
    ];
    expect(terminalSafeMessage(events)).toBe("last");
    expect(terminalSafeMessage([event({ eventType: "run.started" })])).toBeNull();
  });
});
