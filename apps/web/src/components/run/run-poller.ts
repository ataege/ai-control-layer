import type { RunState, RunUsage, SafeEvent } from "@workspace/contracts";

import { getSafeMessage, ProductClient } from "@/lib/product-client";
import { isTerminalStatus, newEventsAfter } from "./run-page-model";

// One run's refresh logic (WEB-06), kept apart from the page so its two guarantees can be tested: the
// events cursor moves only past a page that was actually received, and a failed read keeps the last
// real events and reports the failure instead of inventing a state. Everything shown comes from a
// response; the browser holds only the cursor and the events already received.

/** The API's page size for events; a full page means there may be more. */
export const EVENTS_PAGE_LIMIT = 500;
export const MAXIMUM_EVENT_PAGES_PER_POLL = 5;

export interface RunRefresh {
  /** The persisted run state, when it was read this time; absent when that read failed. */
  run?: RunState;
  /** The usage ledger, when it was read this time. */
  usage?: RunUsage;
  /** Every event received so far, kept across a failed read. */
  events: SafeEvent[];
  /** The safe message of the first failed read of this refresh, or undefined when all succeeded. */
  error: string | undefined;
  /** False once the run is finished: a completed, failed or stopped run no longer changes. */
  keepPolling: boolean;
}

export function createRunPoller(runId: string, signal?: AbortSignal) {
  let cursor: string | undefined;
  let shown: SafeEvent[] = [];

  return {
    /**
     * One refresh: the run state first, then the events and the usage, so a finished state is never
     * shown without the events that led to it. Returns null when the page left and the reads were
     * cancelled, so a cancelled read is never reported as a failure.
     */
    async refresh(): Promise<RunRefresh | null> {
      const state = await ProductClient.getRun(runId, { signal });
      if (!state.ok) {
        if (state.error.kind === "aborted") return null;
        return { events: shown, error: getSafeMessage(state.error), keepPolling: true };
      }

      let failure: string | undefined;
      for (let pageNumber = 0; pageNumber < MAXIMUM_EVENT_PAGES_PER_POLL; pageNumber += 1) {
        const page = await ProductClient.getRunEvents(runId, cursor, { signal });
        if (!page.ok) {
          if (page.error.kind === "aborted") return null;
          // The cursor stays where it was, so the next poll asks for the same events again.
          failure = getSafeMessage(page.error);
          break;
        }
        const fresh = newEventsAfter(shown, page.data.events);
        if (fresh.length > 0) shown = [...shown, ...fresh];
        cursor = page.data.nextCursor;
        if (page.data.events.length < EVENTS_PAGE_LIMIT) break;
      }

      const usage = await ProductClient.getUsage(runId, { signal });
      if (!usage.ok && usage.error.kind === "aborted") return null;
      if (!usage.ok) failure = failure ?? getSafeMessage(usage.error);

      return {
        run: state.data,
        usage: usage.ok ? usage.data : undefined,
        events: shown,
        error: failure,
        keepPolling: !isTerminalStatus(state.data.status),
      };
    },
  };
}
