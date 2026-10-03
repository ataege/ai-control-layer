import type { SafeEvent } from "@workspace/contracts";
import { RunEventTimeline } from "@/components/run/run-event-timeline";

/**
 * Kept for importers of the earlier name. It shows the real sanitized events (X-12) through
 * RunEventTimeline, which keeps attempts apart from effects; it has no event shape of its own.
 */
export function RunTimeline({ events }: { events: readonly SafeEvent[] }) {
  return <RunEventTimeline events={events} />;
}
