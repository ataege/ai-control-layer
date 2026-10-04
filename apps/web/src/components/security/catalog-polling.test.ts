import { describe, expect, it } from "vitest";

import {
  CATALOG_POLL_MS,
  PENDING_POLL_MS,
  pollDelay,
  schedulePoll,
  type PollEnvironment,
} from "./catalog-polling";

/** A page with a manual clock and a switch for the tab's visibility. */
function fakePage() {
  let now = 0;
  let visible = true;
  let nextHandle = 1;
  const timers = new Map<number, { at: number; callback: () => void }>();
  const listeners = new Set<() => void>();
  const environment: PollEnvironment = {
    setTimer(callback, delayMs) {
      const handle = nextHandle++;
      timers.set(handle, { at: now + delayMs, callback });
      return handle;
    },
    clearTimer(handle) {
      timers.delete(handle as number);
    },
    isVisible: () => visible,
    onVisible(callback) {
      listeners.add(callback);
      return () => listeners.delete(callback);
    },
  };
  return {
    environment,
    advance(milliseconds: number) {
      const target = now + milliseconds;
      for (;;) {
        const due = [...timers.entries()]
          .filter(([, timer]) => timer.at <= target)
          .sort((left, right) => left[1].at - right[1].at)[0];
        if (!due) break;
        now = due[1].at;
        timers.delete(due[0]);
        due[1].callback();
      }
      now = target;
    },
    setVisible(next: boolean) {
      visible = next;
      if (next) [...listeners].forEach((listener) => listener());
    },
    openTimers: () => timers.size,
    openListeners: () => listeners.size,
  };
}

describe("pollDelay", () => {
  it("asks every 5 seconds, and sooner while a requested revision is pending", () => {
    expect(CATALOG_POLL_MS).toBe(5_000);
    expect(pollDelay(false)).toBe(CATALOG_POLL_MS);
    expect(pollDelay(true)).toBe(PENDING_POLL_MS);
    expect(PENDING_POLL_MS).toBeLessThan(CATALOG_POLL_MS);
  });
});

describe("schedulePoll", () => {
  it("polls once after the delay while the page is visible, and not before", () => {
    const page = fakePage();
    let polls = 0;
    schedulePoll(page.environment, CATALOG_POLL_MS, () => (polls += 1));
    page.advance(CATALOG_POLL_MS - 1);
    expect(polls).toBe(0);
    page.advance(1);
    expect(polls).toBe(1);
    page.advance(60_000);
    expect(polls).toBe(1); // one scheduled poll is one poll; the caller schedules the next
    expect(page.openTimers()).toBe(0);
    expect(page.openListeners()).toBe(0);
  });

  it("keeps polling when the caller schedules the next poll after each answer", () => {
    const page = fakePage();
    let polls = 0;
    const scheduleNext = () => schedulePoll(page.environment, CATALOG_POLL_MS, poll);
    const poll = () => {
      polls += 1;
      scheduleNext();
    };
    scheduleNext();
    page.advance(CATALOG_POLL_MS * 4);
    expect(polls).toBe(4);
  });

  it("does not ask while the tab is hidden, and asks as soon as it is visible again", () => {
    const page = fakePage();
    page.setVisible(false);
    let polls = 0;
    schedulePoll(page.environment, CATALOG_POLL_MS, () => (polls += 1));
    page.advance(10 * CATALOG_POLL_MS);
    expect(polls).toBe(0);
    page.setVisible(true);
    expect(polls).toBe(1); // a change made meanwhile is shown on return, without waiting a full interval
    page.setVisible(false);
    page.setVisible(true);
    expect(polls).toBe(1);
  });

  it("waits when the tab was hidden while the timer ran", () => {
    const page = fakePage();
    let polls = 0;
    schedulePoll(page.environment, CATALOG_POLL_MS, () => (polls += 1));
    page.advance(CATALOG_POLL_MS - 1);
    page.setVisible(false);
    page.advance(1); // the timer fires while hidden
    expect(polls).toBe(0);
    expect(page.openListeners()).toBe(1);
    page.setVisible(true);
    expect(polls).toBe(1);
    expect(page.openListeners()).toBe(0);
  });

  it("never polls after it is cancelled, hidden or visible", () => {
    const page = fakePage();
    let polls = 0;
    const cancel = schedulePoll(page.environment, CATALOG_POLL_MS, () => (polls += 1));
    cancel();
    page.advance(10 * CATALOG_POLL_MS);
    expect(polls).toBe(0);
    expect(page.openTimers()).toBe(0);

    page.setVisible(false);
    const cancelHidden = schedulePoll(page.environment, CATALOG_POLL_MS, () => (polls += 1));
    expect(page.openListeners()).toBe(1);
    cancelHidden();
    expect(page.openListeners()).toBe(0);
    page.setVisible(true);
    expect(polls).toBe(0);
  });
});
