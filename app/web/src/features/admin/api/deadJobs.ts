// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the dead jobs of the job queue (web-client.md S1), whose list follows the API's
// keyset pagination as a TanStack Query infinite query, keyed like the client's own `[method, path]` with
// "pages" at the end (web-client.md §10).
import { infiniteQueryOptions, type QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { pageQuery } from "../../moderation";

export type DeadJob = components["schemas"]["AdminDeadJob"];

/** The jobs that failed for good, most recently failed first (GET /admin/dead_jobs). */
export function deadJobsQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/dead_jobs", "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/dead_jobs", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** Runs a dead job again (POST /admin/dead_jobs/{id}/retry), as the options of useMutation. */
export function retryDeadJobMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/dead_jobs/{id}/retry"] as const,
    mutationFn: async (id: string): Promise<void> => {
      // The middleware throws for every error status; a 204 has no body to read.
      await api.fetch.POST("/admin/dead_jobs/{id}/retry", { params: { path: { id } } });
    },
  };
}

/** Takes the dead job `id`, enqueued again, off the cached list (ADR-0045); returns it as the list had it. */
export function removeRetriedDeadJob(api: Api, queryClient: QueryClient, id: string): DeadJob | undefined {
  const listKey = deadJobsQueryOptions(api).queryKey;
  const job = queryClient.getQueryData(listKey)?.pages.flatMap((page) => page.items).find((item) => item.id === id);
  queryClient.setQueryData(listKey, (data) => data && { ...data, pages: data.pages.map((page) => ({ ...page, items: page.items.filter((item) => item.id !== id) })) });
  return job;
}
