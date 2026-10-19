// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the moderators' report queue (FR-701, FR-710, NFR-MOD-4): the reports open or
// resolved, newest first; one report with its notes and what the moderators had about its target; taking it,
// leaving it, writing a note, resolving and reopening it; and taking a moderation action on its target account
// (FR-703) that resolves it. The lists follow the API's keyset pagination as infinite queries whose keys end
// with "pages" (web-client.md §10). Every operation on one report answers the report as it is after it, which
// replaces the cached report.
import { infiniteQueryOptions, type QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { pageQuery } from "./admin";

type Schemas = components["schemas"];

export type Report = Schemas["AdminReport"];
export type ReportDetail = Schemas["AdminReportDetail"];
export type ReportState = Schemas["AdminReportState"];
export type ReportOutcome = Schemas["AdminReportOutcome"];
export type ReportCategory = Report["category"];
export type AccountAction = Schemas["AdminAccountAction"];
export type AccountActionKind = Schemas["AdminAccountActionKind"];
export type AccountActionCategory = Schemas["AdminAccountActionCategory"];
export type AccountActionState = Schemas["AdminAccountActionState"];
export type NewAccountAction = Schemas["CreateAccountActionRequest"];

/** The tabs of the report queue, in their order: the reports that await the moderators, and those handled. */
export const reportFilters = ["open", "resolved"] as const satisfies readonly ReportState[];

export type ReportFilter = (typeof reportFilters)[number];

/** The categories of a report and of the reason for an account action, in the order of the form. */
export const reportCategories = ["spam", "legal", "violation", "other"] as const satisfies readonly ReportCategory[];

/** The most characters of a note on a report (CreateReportNoteRequest). */
export const reportNoteLimit = 2000;

/** The most characters of an account action's public reason and private note (CreateAccountActionRequest). */
export const accountActionTextLimit = 5000;

/** The reports in `state`, newest first (GET /admin/reports). */
export function reportsQueryOptions(api: Api, state: ReportFilter) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/reports", { state }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/reports", { params: { query: { ...pageQuery(pageParam), state } }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** One report with its notes and the history of its target (GET /admin/reports/{id}). */
export function reportQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/admin/reports/{id}", { params: { path: { id } } });
}

/** An account by its ID (GET /accounts/{id}): the moderator who handles a report or wrote a note. */
export function accountByIdQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/accounts/{id}", { params: { path: { id } } });
}

/** The instance, whose rules a report cites (GET /instance). */
export function instanceInfoQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/instance");
}

/** The moderation actions on an account, newest first, ended ones included (GET /admin/accounts/{id}/actions). */
export function accountActionsQueryOptions(api: Api, accountId: string) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/accounts/{id}/actions", { id: accountId }, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/accounts/{id}/actions", { params: { path: { id: accountId }, query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/**
 * Fetches again, at once, what a change of a report changes: the lists of reports (it may move between the tabs)
 * and the moderators' counts of open reports, even those no page shows now (`refetchType: "all"`). A route loads
 * its list from the cache as it is (`staleTime: "static"`, web-client.md S7), so a list that was only marked
 * stale would show the report as it was until its refetch on mount ends: a resolved report in the open tab.
 * Resolves once every fetch has ended; one that fails leaves its list stale, to be fetched again when shown.
 */
export function refreshReportLists(queryClient: QueryClient): Promise<void> {
  return queryClient.invalidateQueries({
    predicate: (query) => query.queryKey[1] === "/admin/reports" || query.queryKey[1] === "/admin/counts",
    refetchType: "all",
  });
}

/**
 * Fetches again the lists that a change of a report changes (refreshReportLists), then puts `report`, as the
 * operation answered it, into the cache: the page of the report shows the change once the queue has it too.
 */
export async function storeReport(queryClient: QueryClient, api: Api, report: ReportDetail): Promise<void> {
  await refreshReportLists(queryClient);
  queryClient.setQueryData(reportQueryOptions(api, report.id).queryKey, report);
}

/** Puts `report` after a new note into the cache. A note changes no row of the lists of reports. */
export function storeReportNote(queryClient: QueryClient, api: Api, report: ReportDetail): void {
  queryClient.setQueryData(reportQueryOptions(api, report.id).queryKey, report);
}

/** What can be done to a report that takes no input. */
type ReportOperation = "assign_to_self" | "unassign" | "reopen";

/** One change of a report: the operation and the report. */
export type ChangeReport = { id: string; operation: ReportOperation } | { id: string; operation: "resolve"; outcome: ReportOutcome };

/**
 * Takes a report, leaves it, reopens it, or resolves it (POST /admin/reports/{id}/assign_to_self | unassign |
 * reopen | resolve), as the options of TanStack Query's useMutation (web-client.md §10). Returns the report after it.
 */
export function changeReportMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/reports/{id}"] as const,
    mutationFn: async (change: ChangeReport): Promise<ReportDetail> => {
      const init = { params: { path: { id: change.id } } };
      const { data, error } = await (() => {
        switch (change.operation) {
          case "assign_to_self":
            return api.fetch.POST("/admin/reports/{id}/assign_to_self", init);
          case "unassign":
            return api.fetch.POST("/admin/reports/{id}/unassign", init);
          case "reopen":
            return api.fetch.POST("/admin/reports/{id}/reopen", init);
          case "resolve":
            return api.fetch.POST("/admin/reports/{id}/resolve", { ...init, body: { outcome: change.outcome } });
        }
      })();
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Writes a note on a report (POST /admin/reports/{id}/notes), as the options of useMutation. Returns the report after it. */
export function addReportNoteMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/reports/{id}/notes"] as const,
    mutationFn: async ({ id, body }: { id: string; body: string }): Promise<ReportDetail> => {
      const { data, error } = await api.fetch.POST("/admin/reports/{id}/notes", { params: { path: { id } }, body: { body } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/**
 * Takes a moderation action on an account (POST /admin/accounts/{id}/actions), as the options of useMutation.
 * With a `report_id`, the action resolves that report as `actioned`.
 */
export function createAccountActionMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/accounts/{id}/actions"] as const,
    mutationFn: async ({ accountId, body }: { accountId: string; body: NewAccountAction }): Promise<AccountAction> => {
      const { data, error } = await api.fetch.POST("/admin/accounts/{id}/actions", { params: { path: { id: accountId } }, body });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}
