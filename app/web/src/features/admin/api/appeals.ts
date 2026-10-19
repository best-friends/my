// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the review of appeals (FR-707): the appeals that local users made of the
// moderation actions on their accounts, awaiting a decision or decided, newest first; one appeal with its action;
// and upholding or overturning it with an explanation shown to the user. The lists follow the API's keyset
// pagination as infinite queries whose keys end with "pages" (web-client.md §10). A decision answers the appeal
// after it, with its action (lifted by an overturn), which replaces the cached appeal once the lists of appeals
// it changes are fetched again.
import { infiniteQueryOptions, type QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { adminAccountQueryOptions } from "./accounts";
import { pageQuery } from "./admin";
import { accountActionsQueryOptions } from "./reports";

type Schemas = components["schemas"];

export type Appeal = Schemas["AdminAppeal"];
export type AppealState = Schemas["ModerationAppealState"];
export type AppealPage = Schemas["AdminAppealPage"];

/** The tabs of the appeals screen, in their order: the appeals that await a decision, and those decided. */
export const appealFilters = ["pending", "decided"] as const;

export type AppealFilter = (typeof appealFilters)[number];

/** What a moderator decides on an appeal; each is the last segment of its operation's path. */
export type AppealDecision = "uphold" | "overturn";

/** The most characters of the explanation of a decision (DecideAppealRequest). */
export const appealReasonLimit = 2000;

/** The problem type of deciding an appeal that another moderator decided the other way first. */
const appealDecidedType = "/api/v1/problems/appeal-decided";

/** Whether `error` is the server's refusal of a decision because the appeal was decided the other way first. */
export function isAppealDecided(error: unknown): boolean {
  return error instanceof ApiError && error.type === appealDecidedType;
}

/**
 * The appeals of a tab, newest first (GET /admin/appeals). The API filters by one state, so the decided tab
 * reads every state and keeps the upheld and overturned ones: a page of pending appeals only is followed by the
 * next until one has a decided appeal or the list ends, so that a page is never empty while more follow.
 */
export function appealsQueryOptions(api: Api, filter: AppealFilter) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/appeals", { filter }, "pages"] as const,
    queryFn: async ({ pageParam, signal }): Promise<AppealPage> => {
      let cursor = pageParam;
      for (;;) {
        const query = filter === "pending" ? { ...pageQuery(cursor), state: "pending" as const } : pageQuery(cursor);
        const { data, error } = await api.fetch.GET("/admin/appeals", { params: { query }, signal });
        // The middleware throws an ApiError for every error status; this narrows the type.
        if (data === undefined) throw new ApiError(error);
        if (filter === "pending") return data;
        const items = data.items.filter((appeal) => appeal.state !== "pending");
        if (items.length > 0 || data.next_cursor === undefined) return { ...data, items };
        cursor = data.next_cursor;
      }
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** One appeal with its action (GET /admin/appeals/{id}). */
export function appealQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/admin/appeals/{id}", { params: { path: { id } } });
}

/** One decision: on which appeal, which, and the explanation to the user ("" for none). */
export type DecideAppeal = { id: string; decision: AppealDecision; reason: string };

/**
 * Upholds or overturns an appeal (POST /admin/appeals/{id}/uphold | overturn), as the options of TanStack
 * Query's useMutation (web-client.md §10). Returns the appeal after it, with its action.
 */
export function decideAppealMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/appeals/{id}"] as const,
    mutationFn: async ({ id, decision, reason }: DecideAppeal): Promise<Appeal> => {
      const init = { params: { path: { id } }, body: reason === "" ? {} : { public_reason: reason } };
      const { data, error } =
        decision === "uphold" ? await api.fetch.POST("/admin/appeals/{id}/uphold", init) : await api.fetch.POST("/admin/appeals/{id}/overturn", init);
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/**
 * Fetches again, at once, what a decision on an appeal of `accountId` changes: the lists of appeals (it moves
 * between the tabs), the moderators' counts, and that account with its actions (an overturn lifts one), even
 * those no page shows now (`refetchType: "all"`). A route loads them from the cache as they are (`staleTime:
 * "static"`, web-client.md S7), so a list that was only marked stale would show the appeal as it was until its
 * refetch on mount ends: a decided appeal among those awaiting review. Resolves once every fetch has ended; one
 * that fails leaves its query stale, to be fetched again when shown.
 */
export async function refreshAppealLists(queryClient: QueryClient, api: Api, accountId: string): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ predicate: (query) => query.queryKey[1] === "/admin/appeals" || query.queryKey[1] === "/admin/counts", refetchType: "all" }),
    queryClient.invalidateQueries({ queryKey: adminAccountQueryOptions(api, accountId).queryKey, refetchType: "all" }),
    queryClient.invalidateQueries({ queryKey: accountActionsQueryOptions(api, accountId).queryKey, refetchType: "all" }),
  ]);
}

/**
 * Fetches again what a decision changes (refreshAppealLists), then puts `appeal`, as the decision answered it,
 * into the cache: the appeal's page shows the decision once the list has it too.
 */
export async function storeAppeal(queryClient: QueryClient, api: Api, appeal: Appeal): Promise<void> {
  await refreshAppealLists(queryClient, api, appeal.account_id);
  queryClient.setQueryData(appealQueryOptions(api, appeal.id).queryKey, appeal);
}
