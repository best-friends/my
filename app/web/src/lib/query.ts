// SPDX-License-Identifier: AGPL-3.0-only
// The TanStack Query client: the only cache of server data (web-client.md S1). Routes reach it through
// the router context to load what is not cached yet in their loaders (`queryClient.query` with `staleTime: "static"`, S7), with the API client and the session;
// components use the hooks.
import { type Query, QueryClient } from "@tanstack/react-query";
import type { Api } from "./api/client";
import type { Session } from "./auth/session";
import { NetworkError } from "./errors";

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
        // route's loader does not wait forever (web-client.md G17). The app counts as offline after a
        // NetworkError (lib/network.ts), so retrying one would only delay that message.
        networkMode: "always",
        retry: (failureCount, error) => !(error instanceof NetworkError) && failureCount < 3,
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

function refetchFailed(query: Query): "always" | false {
  return query.state.status === "error" ? "always" : false;
}
