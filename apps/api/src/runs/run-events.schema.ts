import { z } from "zod";
import eventContract from "@workspace/contracts/schemas/safe-event.schema.json" with { type: "json" };
import pageContract from "@workspace/contracts/schemas/run-events-page.schema.json" with { type: "json" };
import type { SafeEvent, RunEventsPage } from "@workspace/contracts";

// Zod does not implement JSON Schema conditionals. Preserve X-12's sole conditional below.
const { if: conditional, then: consequence, ...eventShape } = eventContract;
void conditional;
void consequence;
const eventSchema = z
  .fromJSONSchema(eventShape as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const event = value as SafeEvent;
    if (event.actionId !== null && event.runId === null) {
      context.addIssue({ code: "custom", message: "An action event requires a run" });
    }
  });

export const RunEventsSchema = z
  .fromJSONSchema({
    ...pageContract,
    properties: {
      ...pageContract.properties,
      events: { ...pageContract.properties.events, items: eventShape },
    },
  } as Parameters<typeof z.fromJSONSchema>[0])
  .superRefine((value, context) => {
    const page = value as RunEventsPage;
    for (const event of page.events) {
      if (!eventSchema.safeParse(event).success) {
        context.addIssue({ code: "custom", message: "Invalid event" });
      }
    }
  });
