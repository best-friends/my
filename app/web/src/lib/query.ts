// SPDX-License-Identifier: AGPL-3.0-only
// The TanStack Query client: the only cache of server data (web-client.md S1). Routes reach it through
// the router context to load what is not cached yet in their loaders (`queryClient.query` with `staleTime: "static"`, S7), with the API client and the session;
// components use the hooks.
import { type Query, QueryClient } from "@tanstack/react-query";
import type { Api } from "./api/client";
import type { Session } from "./auth/session";
import { ApiError, NetworkError } from "./errors";

/** What every route receives as its router context. */
export type RouterContext = { queryClient: QueryClient; api: Api; session: Session };

/** Creates the app's QueryClient; tests create a fresh one per test so that no cache leaks between them. */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Navigating back to a page within this time shows its cached data without a refetch; the
        // streaming API (S10, lib/sse) brings the home timeline's new posts and the unread count in between.
        staleTime: 30_000,
        // Offline, a query fails at once with a NetworkError that says so, instead of pausing until the network
        // is back: a region shows that it cannot load while offline rather than a spinner that never ends, and a
        // route's loader does not wait forever (web-client.md G17). A NetworkError is retried at once, so
        // that the message is not delayed by the backoff; loaders (`queryClient.query`) do not retry at all.
        // `retry` keeps its default: setting one here would make the loaders retry too.
        networkMode: "always",
        retryDelay: (failureCount, error) => (error instanceof NetworkError ? 0 : Math.min(1000 * 2 ** failureCount, 30_000)),
        // Back online, only the queries that failed fetch again (G17); the others wait until they are stale
        // and used. A query that must not refetch then (a timeline, whose top would change) says so itself.
        refetchOnReconnect: refetchFailed,
      },
      mutations: {
        // Offline, a write fails at once instead of pausing, so that what the user sent stays in the form or
        // the outbox to send again (G17, web-client.md §12).
        networkMode: "always",
      },
    },
  });
}

/**
 * The `retry` of the queries the app shell reads on every page (the viewer, the preferences, the instance, the
 * notification marker), which show their last value or a default while they fail: another try finds the same
 * answer to a request the server refused (4xx), and a 429 only spends more of the rate limit before Retry-After
 * (lib/rateLimit.ts loads it again then). Anything else is tried once more, so that a failure shows in about a
 * second instead of after the default three tries (about seven seconds).
 */
export function shellQueryRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status < 500) return false;
  return failureCount < 1;
}

function refetchFailed(query: Query): "always" | false {
  return query.state.status === "error" ? "always" : false;
}
