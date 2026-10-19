// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the moderators' account screens (FR-714, FR-703, FR-713): searching the local and
// remote accounts, one account as the moderators see it, showing the email address and the IP addresses of a
// local account (recorded in the audit log, so only on the moderator's explicit action: a mutation, never a
// query that runs by itself or again), and lifting a moderation action. The search follows the API's keyset
// pagination as an infinite query whose key ends with "pages" (web-client.md §10).
import { infiniteQueryOptions, type QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { pageQuery, refreshQueries } from "./admin";
import { type AccountAction, accountActionsQueryOptions } from "./reports";

type Schemas = components["schemas"];

export type AdminAccount = Schemas["AdminAccount"];
export type AdminAccountDetail = Schemas["AdminAccountDetail"];
export type AdminAccountState = Schemas["AdminAccountState"];
export type AdminAccountRestriction = Schemas["AdminAccountRestriction"];
export type AdminAccountRegistrationState = Schemas["AdminAccountRegistrationState"];
export type AdminAccountPrivateData = Schemas["AdminAccountPrivateData"];

/** The states of an account, in the order of the search's select. */
export const adminAccountStates = ["active", "pending", "rejected", "limited", "disabled", "suspended", "deleted"] as const satisfies readonly AdminAccountState[];

/** Where the accounts of a search are: on this server or on others. */
export const adminAccountOrigins = ["local", "remote"] as const;

export type AdminAccountOrigin = (typeof adminAccountOrigins)[number];

/** The search fields that look for a local account by what only the moderators may see; at most one is given. */
export const adminAccountContactFields = ["email", "email_domain", "ip"] as const;

export type AdminAccountContactField = (typeof adminAccountContactFields)[number];

/** The most characters of a search field (GET /admin/accounts). */
export const adminAccountSearchLimit = 320;

/** A search of the accounts, with the API's own names; an omitted field keeps every account. */
export type AdminAccountSearch = {
  username?: string | undefined;
  domain?: string | undefined;
  origin?: AdminAccountOrigin | undefined;
  state?: AdminAccountState | undefined;
  email?: string | undefined;
  email_domain?: string | undefined;
  ip?: string | undefined;
};

/** A search without its omitted fields. */
type SearchQuery = { [K in keyof AdminAccountSearch]?: Exclude<AdminAccountSearch[K], undefined> };

/** The search without its omitted fields, as the key and the query of the request. */
function searchQuery(search: AdminAccountSearch): SearchQuery {
  return Object.fromEntries(Object.entries(search).filter(([, value]) => value !== undefined));
}

/** The accounts of `search`, newest first (GET /admin/accounts). A page may be short, or empty, and still continue. */
export function adminAccountsQueryOptions(api: Api, search: AdminAccountSearch) {
  const query = searchQuery(search);
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/accounts", query, "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/accounts", { params: { query: { ...query, ...pageQuery(pageParam) } }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** One account as the moderators see it, without its email and IP addresses (GET /admin/accounts/{id}). */
export function adminAccountQueryOptions(api: Api, id: string) {
  return api.query.queryOptions("get", "/admin/accounts/{id}", { params: { path: { id } } });
}

/** Whether a query key is a search of the accounts, of any fields. */
function isAccountSearchQuery(queryKey: readonly unknown[]): boolean {
  return queryKey[1] === "/admin/accounts";
}

/**
 * Fetches again (refreshQueries) what a change of the state, the restrictions or the roles of account `id`
 * changes: the searches that are cached (each shows those of its accounts) and the account itself, and with
 * `actions` its moderation actions too.
 */
export function refreshAdminAccount(queryClient: QueryClient, api: Api, id: string, { actions }: { actions: boolean }): Promise<void> {
  return refreshQueries(
    queryClient,
    { predicate: (query) => isAccountSearchQuery(query.queryKey) },
    { queryKey: adminAccountQueryOptions(api, id).queryKey },
    ...(actions ? [{ queryKey: accountActionsQueryOptions(api, id).queryKey }] : []),
  );
}

/**
 * Shows the email address and the IP addresses of a local account (POST /admin/accounts/{id}/reveal), as the
 * options of useMutation. The server records every call in the audit log before it answers.
 */
export function revealAccountMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/accounts/{id}/reveal"] as const,
    mutationFn: async (id: string): Promise<AdminAccountPrivateData> => {
      const { data, error } = await api.fetch.POST("/admin/accounts/{id}/reveal", { params: { path: { id } } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Lifts a moderation action on an account (POST /admin/account_actions/{id}/lift), as the options of useMutation. */
export function liftAccountActionMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/account_actions/{id}/lift"] as const,
    mutationFn: async ({ id, reason }: { id: string; reason: string }): Promise<AccountAction> => {
      const { data, error } = await api.fetch.POST("/admin/account_actions/{id}/lift", { params: { path: { id } }, body: reason === "" ? {} : { reason } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}
