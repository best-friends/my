// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the admin screens (web-client.md S1): the health of remote servers
// (federation.md §7.3) with the history of their availability, and the dead jobs of the job queue. The
// lists follow the API's keyset pagination (each page's `next_cursor` is the `cursor` of the next) as TanStack
// Query infinite queries on the same client; their keys start like the client's own `[method, path]` and end
// with "pages", so that they never collide with a plain query of the same operation (web-client.md §10).
import { infiniteQueryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";

type Schemas = components["schemas"];

export type Permission = Schemas["Permission"];
export type InstanceHealth = Schemas["AdminInstanceHealth"];
export type InstanceState = Schemas["AdminInstanceState"];
export type AvailabilityEvent = Schemas["AdminAvailabilityEvent"];
export type DeadJob = Schemas["AdminDeadJob"];

/** The filters of the list of remote servers, in the order of its tabs; `all` sends no `state`. */
export const instanceFilters = ["all", "unavailable", "available"] as const;

export type InstanceFilter = (typeof instanceFilters)[number];

/** The page size of the lists. */
const pageSize = 40;

function pageQuery(cursor: string | undefined): { limit: number; cursor?: string } {
  return cursor === undefined ? { limit: pageSize } : { limit: pageSize, cursor };
}

/** The remote servers with delivery failures or marked unavailable, in domain order (GET /admin/instances). */
export function instancesQueryOptions(api: Api, filter: InstanceFilter) {
  const state = filter === "all" ? undefined : filter;
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/instances", { state }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const query = { ...pageQuery(pageParam), ...(state === undefined ? {} : { state }) };
      const { data, error } = await api.fetch.GET("/admin/instances", { params: { query }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The health of one remote server (GET /admin/instances/{domain}). */
export function instanceQueryOptions(api: Api, domain: string) {
  return api.query.queryOptions("get", "/admin/instances/{domain}", { params: { path: { domain } } });
}

/** When and why a remote server was marked unavailable or available, newest first (GET /admin/instances/{domain}/availability_events). */
export function availabilityEventsQueryOptions(api: Api, domain: string) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/instances/{domain}/availability_events", { domain }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/instances/{domain}/availability_events", {
        params: { path: { domain }, query: pageQuery(pageParam) },
        signal,
      });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/**
 * Marks a remote server available (POST /admin/instances/{domain}/mark_available), as the options of TanStack
 * Query's useMutation (the client's own useMutation is a hook read from an object, which React Compiler
 * cannot compile around; web-client.md §10). Returns the server's health after it.
 */
export function markAvailableMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/instances/{domain}/mark_available"] as const,
    mutationFn: async (domain: string): Promise<InstanceHealth> => {
      const { data, error } = await api.fetch.POST("/admin/instances/{domain}/mark_available", { params: { path: { domain } } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

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

/** The problem type of an admin operation that the viewer's roles do not allow (api/openapi.yaml, Problem). */
const missingPermissionType = "/api/v1/problems/missing-permission";

/** Whether `error` is the server's refusal of an admin operation for the viewer's lack of its permission. */
export function isMissingPermission(error: unknown): boolean {
  return error instanceof ApiError && error.type === missingPermissionType;
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
