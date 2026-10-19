// SPDX-License-Identifier: AGPL-3.0-only
// The server's rate limit (429 Too Many Requests with Retry-After) as the app shell handles it: the queries on the
// page that the limit refused wait until the time Retry-After names, and then load again by themselves; until then
// the layout tells the user when that will be. A query refused by the limit is not retried before then
// (shellQueryRetry in lib/query.ts), since every try spends more of the limit.
import { type Query, useQueryClient } from "@tanstack/react-query";
import { useEffect, useSyncExternalStore } from "react";
import { rateLimitedUntil } from "./errors";

/**
 * When a query that the rate limit refused may load again, while it is used on the page. It still waits while it loads
 * again (a new observer refetches it on mount), so that the bar does not flicker and announce itself again.
 */
function waitingUntil(query: Query): number | undefined {
  if (query.state.status !== "error" || query.getObserversCount() === 0) return undefined;
  return rateLimitedUntil(query.state.error);
}

/**
 * Loads the queries on the page that the rate limit refused again at the time the server named, and returns that
 * time (ms since the epoch; the latest one, when several queries wait), or null while no query waits. The layout
 * calls it once.
 */
export function useRateLimitRetry(): number | null {
  const queryClient = useQueryClient();
  const cache = queryClient.getQueryCache();
  // The query cache is a store outside React; the snapshot is a number, so it is the same while nothing changes.
  const until = useSyncExternalStore(
    (listener) => cache.subscribe(listener),
    () => latest(cache.getAll().map(waitingUntil)),
  );
  // An Effect, because the reason to load again is the time, not something the user did.
  useEffect(() => {
    if (until === null) return undefined;
    const timer = setTimeout(
      () => {
        void queryClient.refetchQueries({ type: "active", predicate: (query) => waitingUntil(query) !== undefined });
      },
      Math.max(0, until - Date.now()),
    );
    return () => {
      clearTimeout(timer);
    };
  }, [queryClient, until]);
  return until;
}

function latest(times: readonly (number | undefined)[]): number | null {
  let result: number | null = null;
  for (const time of times) if (time !== undefined && (result === null || time > result)) result = time;
  return result;
}
