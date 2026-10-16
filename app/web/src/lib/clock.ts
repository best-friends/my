// SPDX-License-Identifier: AGPL-3.0-only
// The current time for rendering, as an external store (react.dev, useSyncExternalStore). Components stay
// pure (web-client.md R1): they read the time from the store instead of calling Date.now() while rendering,
// and every component that shows a relative time ("5 min. ago") renders again when the clock ticks.
import { useSyncExternalStore } from "react";

/** How often the clock ticks, in ms: relative times are shown to the minute at best. */
const tickInterval = 30_000;

/** The time of the last tick; read once when the first component asks, and dropped with the last subscriber. */
let now: number | undefined;
const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | undefined;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  timer ??= setInterval(() => {
    now = Date.now();
    for (const notify of listeners) notify();
  }, tickInterval);
  return () => {
    listeners.delete(listener);
    if (listeners.size > 0) return;
    clearInterval(timer);
    timer = undefined;
    now = undefined;
  };
}

function getSnapshot(): number {
  now ??= Date.now();
  return now;
}

/** Returns the current time in ms since the epoch, updated every 30 seconds. */
export function useNow(): number {
  return useSyncExternalStore(subscribe, getSnapshot);
}
