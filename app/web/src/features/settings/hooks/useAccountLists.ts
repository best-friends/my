// SPDX-License-Identifier: AGPL-3.0-only
// The lists of blocked and muted accounts and of follow requests, and their actions (FR-302, FR-303). An
// account that an action takes off a list leaves the cached pages at once, and the caller moves focus to the
// next row, so that it is not lost with the removed row.
import { useAnnouncer } from "@my/ui";
import { type InfiniteData, useMutation, useQueryClient, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useT } from "../../../lib/i18n";
import { credentialsQueryOptions } from "../api/me";
import { type Account, accountActionMutation, blocksQueryOptions, followRequestsQueryOptions, type MutedAccount, mutesQueryOptions } from "../api/relationships";

/** What a list screen needs from its list: the items so far, and loading more. */
export type AccountList<T> = {
  items: T[];
  hasMore: boolean;
  isLoadingMore: boolean;
  loadMore: () => void;
};

type InfiniteList<P> = {
  data: InfiniteData<P>;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  isFetching: boolean;
  fetchNextPage: () => Promise<unknown>;
};

function toList<P, T>(query: InfiniteList<P>, itemsOf: (page: P) => T[]): AccountList<T> {
  return {
    items: query.data.pages.flatMap(itemsOf),
    hasMore: query.hasNextPage,
    isLoadingMore: query.isFetchingNextPage,
    loadMore: () => {
      // A refetch in progress would overwrite the page fetched now (TanStack Query, "Infinite Queries").
      if (!query.isFetching) void query.fetchNextPage();
    },
  };
}

/** An action on one account of a list, which takes it off the list when it succeeds. */
export type AccountAction = {
  /** Runs the action for `account`; `onDone` is called once the account is off the list. */
  run: (account: Account, onDone: () => void) => void;
  /** The account the action is running for. */
  pendingId: string | undefined;
};

type MutationCallbacks = { onSuccess: () => void; onError: (error: unknown) => void };

/** An AccountAction from a mutation of one account, the update that takes the account off the cached list, and its announcement. */
function useListAction(options: {
  mutate: (id: string, callbacks: MutationCallbacks) => void;
  pendingId: string | undefined;
  remove: (id: string) => void;
  done: (name: string) => string;
}): AccountAction {
  const queryClient = useQueryClient();
  const announce = useAnnouncer();
  return {
    pendingId: options.pendingId,
    run: (account, onDone) => {
      options.mutate(account.id, {
        onSuccess: () => {
          options.remove(account.id);
          onDone();
          announce(options.done(account.display_name || account.username));
          // Other screens show the relationship with the account (getRelationships, under any parameters).
          void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[1] === "/accounts/relationships" });
        },
        // A failure keeps the row and its focus, and says why (web-client.md E2).
        onError: (error) => {
          announce(errorMessage(error), "assertive");
        },
      });
    },
  };
}

/** The blocked accounts and unblocking them. */
export function useBlocks(): { list: AccountList<Account>; unblock: AccountAction } {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const options = blocksQueryOptions(api);
  const query = useSuspenseInfiniteQuery(options);
  const unblock = useMutation(accountActionMutation(api, "/accounts/{id}/unblock"));
  return {
    list: toList(query, (page) => page.accounts),
    unblock: useListAction({
      mutate: (id, callbacks) => {
        unblock.mutate(id, callbacks);
      },
      pendingId: unblock.isPending ? unblock.variables : undefined,
      remove: (id) => {
        queryClient.setQueryData(options.queryKey, (data) =>
          data && { ...data, pages: data.pages.map((page) => ({ ...page, accounts: page.accounts.filter((account) => account.id !== id) })) },
        );
      },
      done: (name) => t("client:settings.blocks.unblocked", { name }),
    }),
  };
}

/** The muted accounts and unmuting them. */
export function useMutes(): { list: AccountList<MutedAccount>; unmute: AccountAction } {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const options = mutesQueryOptions(api);
  const query = useSuspenseInfiniteQuery(options);
  const unmute = useMutation(accountActionMutation(api, "/accounts/{id}/unmute"));
  return {
    list: toList(query, (page) => page.mutes),
    unmute: useListAction({
      mutate: (id, callbacks) => {
        unmute.mutate(id, callbacks);
      },
      pendingId: unmute.isPending ? unmute.variables : undefined,
      remove: (id) => {
        queryClient.setQueryData(options.queryKey, (data) =>
          data && { ...data, pages: data.pages.map((page) => ({ ...page, mutes: page.mutes.filter((mute) => mute.account.id !== id) })) },
        );
      },
      done: (name) => t("client:settings.mutes.unmuted", { name }),
    }),
  };
}

/** The follow requests to the viewer, and accepting or rejecting them. */
export function useFollowRequests(): { list: AccountList<Account>; accept: AccountAction; reject: AccountAction } {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const options = followRequestsQueryOptions(api);
  const query = useSuspenseInfiniteQuery(options);
  const accept = useMutation({
    ...accountActionMutation(api, "/follow_requests/{id}/authorize"),
    // The viewer has one more follower.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: credentialsQueryOptions(api).queryKey }),
  });
  const reject = useMutation(accountActionMutation(api, "/follow_requests/{id}/reject"));
  const remove = (id: string) => {
    queryClient.setQueryData(options.queryKey, (data) =>
      data && { ...data, pages: data.pages.map((page) => ({ ...page, accounts: page.accounts.filter((account) => account.id !== id) })) },
    );
  };
  return {
    list: toList(query, (page) => page.accounts),
    accept: useListAction({
      mutate: (id, callbacks) => {
        accept.mutate(id, callbacks);
      },
      pendingId: accept.isPending ? accept.variables : undefined,
      remove,
      done: (name) => t("client:settings.follow_requests.accepted", { name }),
    }),
    reject: useListAction({
      mutate: (id, callbacks) => {
        reject.mutate(id, callbacks);
      },
      pendingId: reject.isPending ? reject.variables : undefined,
      remove,
      done: (name) => t("client:settings.follow_requests.rejected", { name }),
    }),
  };
}
