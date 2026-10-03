import { z } from "zod";
import stateContract from "@workspace/contracts/schemas/run-state.schema.json" with { type: "json" };
import usageContract from "@workspace/contracts/schemas/run-usage.schema.json" with { type: "json" };
import type { RunState } from "@workspace/contracts";

const { if: conditional, then: consequence, else: alternative, ...stateShape } = stateContract;
void conditional;
void consequence;
void alternative;
// Restore the shared contract's conditional, which Zod's JSON Schema converter cannot express.
export const RunStateSchema = z
  .fromJSONSchema(stateShape as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const state = value as RunState;
    const needsReason = ["paused", "failed", "stopped"].includes(state.status);
    if (needsReason === (state.terminalReason === null)) {
      context.addIssue({ code: "custom", message: "Invalid terminal reason for run status" });
    }
    if (
      state.resultReference &&
      new Set(state.resultReference.reportIds).size !== state.resultReference.reportIds.length
    ) {
      context.addIssue({ code: "custom", message: "Duplicate report reference" });
    }
  });
export const RunUsageSchema = z.fromJSONSchema(
  usageContract as Parameters<typeof z.fromJSONSchema>[0],
);
