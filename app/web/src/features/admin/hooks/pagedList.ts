// SPDX-License-Identifier: AGPL-3.0-only
import type { InfiniteData } from "@tanstack/react-query";

/** What a list screen needs from an infinite query: the items so far, and loading more. */
export type PagedList<T> = {
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

/** The PagedList of an infinite query whose pages have `items`. */
export function toPagedList<T>(query: InfiniteList<{ items: T[] }>): PagedList<T> {
  return {
    items: query.data.pages.flatMap((page) => page.items),
    hasMore: query.hasNextPage,
    isLoadingMore: query.isFetchingNextPage,
    loadMore: () => {
      // A fetch in progress would be overwritten by the page fetched now (TanStack Query, "Infinite Queries").
      if (!query.isFetching) void query.fetchNextPage();
    },
  };
}
