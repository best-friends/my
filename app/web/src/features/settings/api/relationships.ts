// SPDX-License-Identifier: AGPL-3.0-only
// The lists of the accounts the viewer blocks, mutes, or has follow requests from (FR-302, FR-303), as
// infinite queries over the API's keyset pagination: each page's `next_cursor` is the `cursor` of the next.
// openapi-react-query has no infiniteQueryOptions, so these are TanStack Query's, on the same client.
// Their keys start like the client's own ([method, path]), so invalidating ["get", path] reaches them.
import { infiniteQueryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";

type Schemas = components["schemas"];

export type Account = Schemas["Account"];
export type MutedAccount = Schemas["MutedAccount"];

/** The page size of the lists. */
const pageSize = 40;

/** The paths of the lists, the first part of their query keys after the method. */
const listPaths = { blocks: "/blocks", mutes: "/mutes", followRequests: "/follow_requests" } as const;

/** The accounts the viewer blocks, newest first (GET /blocks). */
export function blocksQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", listPaths.blocks, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/blocks", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status, so this is only for the types.
      if (data === undefined) throw new Error(error.title);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The accounts the viewer mutes, with each mute's settings, newest first (GET /mutes). */
export function mutesQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", listPaths.mutes, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/mutes", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status, so this is only for the types.
      if (data === undefined) throw new Error(error.title);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The accounts whose follow requests await the viewer's approval, newest first (GET /follow_requests). */
export function followRequestsQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", listPaths.followRequests, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/follow_requests", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status, so this is only for the types.
      if (data === undefined) throw new Error(error.title);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

function pageQuery(cursor: string | undefined): { limit: number; cursor?: string } {
  return cursor === undefined ? { limit: pageSize } : { limit: pageSize, cursor };
}
