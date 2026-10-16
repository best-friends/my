// SPDX-License-Identifier: AGPL-3.0-only
// The TanStack Query client: the only cache of server data (web-client.md S1). Routes reach it through
// the router context to call `ensureQueryData` in their loaders (S7); components use the hooks.
import { QueryClient } from "@tanstack/react-query";

/** What every route receives as its router context. */
export type RouterContext = { queryClient: QueryClient };

/** Creates the app's QueryClient; tests create a fresh one per test so that no cache leaks between them. */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Navigating back to a page within this time shows its cached data without a refetch; the
        // streaming API (S10) keeps timelines fresh in between.
        staleTime: 30_000,
      },
    },
  });
}
