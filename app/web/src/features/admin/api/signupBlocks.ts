// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the sign-up blocks (FR-719): the blocks on sign-ups by the domain of the email
// address, one email address (by its hash) or a range of client addresses, listed newest first, added and
// lifted by a viewer with manage_accounts. The list follows the API's keyset pagination as a TanStack Query
// infinite query whose key ends with "pages" (web-client.md §10).
import { infiniteQueryOptions, type QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { refreshQueries } from "../../../lib/query";
import { pageQuery } from "./admin";

type Schemas = components["schemas"];

export type SignupBlock = Schemas["AdminSignupBlock"];
export type SignupBlockKind = Schemas["AdminSignupBlockKind"];
export type SignupBlockSeverity = Schemas["AdminSignupBlockSeverity"];
export type SignupBlockState = Schemas["AdminSignupBlockState"];
export type NewSignupBlock = Schemas["CreateSignupBlockRequest"];

/** The kinds of sign-up block, in the order of the form: the domain of the email address first, the most used. */
export const signupBlockKinds = ["email_domain", "email_hash", "ip_range"] as const satisfies readonly SignupBlockKind[];

/** The severities of a sign-up block, from the lighter to the heavier. */
export const signupBlockSeverities = ["require_approval", "reject"] as const satisfies readonly SignupBlockSeverity[];

/** The most characters of a sign-up block's value and comment (CreateSignupBlockRequest). */
export const signupBlockValueLimit = 320;
export const signupBlockCommentLimit = 1000;

/** The filters of the list of sign-up blocks, in the order of its tabs; `active` sends `active=true`. */
export const signupBlockFilters = ["active", "all"] as const;

export type SignupBlockFilter = (typeof signupBlockFilters)[number];

/** The sign-up blocks, newest first (GET /admin/signup_blocks): those in effect, or all of them. */
export function signupBlocksQueryOptions(api: Api, filter: SignupBlockFilter) {
  const active = filter === "active" ? true : undefined;
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/signup_blocks", { active }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const query = { ...pageQuery(pageParam), ...(active === undefined ? {} : { active }) };
      const { data, error } = await api.fetch.GET("/admin/signup_blocks", { params: { query }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** Whether a query is one of the lists of sign-up blocks, of any filter. */
export function isSignupBlocksQuery(queryKey: readonly unknown[]): boolean {
  return queryKey[1] === "/admin/signup_blocks";
}

/** Whether a list of sign-up blocks keeps only those in effect, which a lifted block leaves. */
export function isActiveSignupBlocksQuery(queryKey: readonly unknown[]): boolean {
  const params = queryKey[2];
  return isSignupBlocksQuery(queryKey) && typeof params === "object" && params !== null && "active" in params && params.active === true;
}

/** Fetches again every cached list of sign-up blocks (refreshQueries): a new block is in each. */
export function refreshSignupBlockLists(queryClient: QueryClient): Promise<void> {
  return refreshQueries(queryClient, { predicate: (query) => isSignupBlocksQuery(query.queryKey) });
}

/** Adds a sign-up block (POST /admin/signup_blocks), as the options of useMutation. */
export function createSignupBlockMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/signup_blocks"] as const,
    mutationFn: async (body: NewSignupBlock): Promise<SignupBlock> => {
      const { data, error } = await api.fetch.POST("/admin/signup_blocks", { body });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Lifts a sign-up block (POST /admin/signup_blocks/{id}/lift), as the options of useMutation. */
export function liftSignupBlockMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/signup_blocks/{id}/lift"] as const,
    mutationFn: async (id: string): Promise<SignupBlock> => {
      const { data, error } = await api.fetch.POST("/admin/signup_blocks/{id}/lift", { params: { path: { id } } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}
