// When the active-controls panel asks the API again (WEB-29). The panel must show a policy change
// that someone made from the terminal, without a click, so it asks on a modest interval while the
// page is open and stays quiet while the tab is hidden. Pure functions over an injected clock and
// visibility, tested without a browser.

/** How often the open panel asks again. */
export const CATALOG_POLL_MS = 5_000;
/** While a requested revision is not active yet, ask a little sooner. */
export const PENDING_POLL_MS = 3_000;

export function pollDelay(reloadPending: boolean): number {
  return reloadPending ? PENDING_POLL_MS : CATALOG_POLL_MS;
}

/** What the scheduler needs from the page; the browser's own is `browserPollEnvironment`. */
export interface PollEnvironment {
  setTimer(callback: () => void, delayMs: number): unknown;
  clearTimer(handle: unknown): void;
  isVisible(): boolean;
  /** Calls back once each time the page becomes visible; returns how to stop listening. */
  onVisible(callback: () => void): () => void;
}

/**
 * Calls `poll` once, `delayMs` from now, if the page is visible then. A hidden page is not asked:
 * the poll waits and runs the moment the page is visible again, so a change made meanwhile is
 * shown on return. Returns how to cancel; after cancelling, `poll` is never called.
 */
export function schedulePoll(
  environment: PollEnvironment,
  delayMs: number,
  poll: () => void,
): () => void {
  let timer: unknown;
  let stopWatching: (() => void) | undefined;
  let finished = false;

  const finish = () => {
    finished = true;
    if (timer !== undefined) environment.clearTimer(timer);
    stopWatching?.();
  };
  const fire = () => {
    if (finished) return;
    finish();
    poll();
  };
  const waitUntilVisible = () => {
    stopWatching = environment.onVisible(fire);
  };

  if (environment.isVisible()) {
    timer = environment.setTimer(() => {
      timer = undefined;
      // The tab was hidden while the timer ran: do not ask, wait for the page to return.
      if (environment.isVisible()) fire();
      else waitUntilVisible();
    }, delayMs);
  } else {
    waitUntilVisible();
  }
  return finish;
}

/** The real page: timers, `document.visibilityState` and the `visibilitychange` event. */
export const browserPollEnvironment: PollEnvironment = {
  setTimer: (callback, delayMs) => setTimeout(callback, delayMs),
  clearTimer: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
  isVisible: () => document.visibilityState !== "hidden",
  onVisible: (callback) => {
    const listener = () => {
      if (document.visibilityState === "visible") callback();
    };
    document.addEventListener("visibilitychange", listener);
    return () => document.removeEventListener("visibilitychange", listener);
  },
};
