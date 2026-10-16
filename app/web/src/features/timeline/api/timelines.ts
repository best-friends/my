// SPDX-License-Identifier: AGPL-3.0-only
// The queries of the timeline feature (web-client.md S1). A timeline is an infinite query of pages that
// follow each other by `next_cursor` (TanStack Query, "Infinite Queries"). openapi-react-query has a hook
// for infinite queries but no options factory, so the options are built here with infiniteQueryOptions
// over the openapi-fetch client; the key follows openapi-react-query's `[method, path, init]` and ends with
// "pages", so that it never collides with a plain query of the same operation (an infinite query caches
// another shape of data).
import { infiniteQueryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";

/** A status (post or boost), as the client API returns it. */
export type Status = components["schemas"]["Status"];

/** One attachment of a status. */
export type MediaAttachment = components["schemas"]["MediaAttachment"];

/** A page of a timeline. */
export type StatusPage = components["schemas"]["StatusPage"];

/** Which timeline to show; the route puts it in the URL (web-client.md S3). */
export type Timeline = { kind: "home" } | { kind: "local" } | { kind: "tag"; tag: string };

/** The query key of a timeline. */
function timelineQueryKey(timeline: Timeline) {
  switch (timeline.kind) {
    case "home":
      return ["get", "/timelines/home", "pages"] as const;
    case "local":
      return ["get", "/timelines/local", "pages"] as const;
    case "tag":
      return ["get", "/timelines/tag/{hashtag}", { params: { path: { hashtag: timeline.tag } } }, "pages"] as const;
  }
}

/** Fetches one page of `timeline`; `cursor` is the previous page's `next_cursor`, or undefined for the first. */
async function fetchTimelinePage(api: Api, timeline: Timeline, cursor: string | undefined, signal: AbortSignal): Promise<StatusPage> {
  const query = cursor === undefined ? {} : { cursor };
  const { data, error } =
    timeline.kind === "home"
      ? await api.fetch.GET("/timelines/home", { params: { query }, signal })
      : timeline.kind === "local"
        ? await api.fetch.GET("/timelines/local", { params: { query }, signal })
        : await api.fetch.GET("/timelines/tag/{hashtag}", { params: { path: { hashtag: timeline.tag }, query }, signal });
  // The client's middleware throws an ApiError for every error status (lib/api/client.ts).
  if (data === undefined) throw new ApiError(error);
  return data;
}

/** The pages of a timeline, newest first; the last page has no `next_cursor`. */
export function timelineQueryOptions(api: Api, timeline: Timeline) {
  // eslint-disable-next-line @tanstack/query/exhaustive-deps -- the app has one API client (lib/services.ts); the openapi-react-query keys leave it out too.
  return infiniteQueryOptions({
    queryKey: timelineQueryKey(timeline),
    queryFn: ({ pageParam, signal }) => fetchTimelinePage(api, timeline, pageParam, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor,
    // A failed page is shown at once with a button to try again (the route's error for the first page,
    // the end of the feed for an older one), instead of after seconds of silent retries.
    retry: false,
  });
}
