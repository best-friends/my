// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the notifications feature (web-client.md S1, FR-205): the list, filtered
// by type and loaded a page at a time, with the posts that notifications are about embedded; and the read
// marker with the unread count. Keys follow openapi-react-query's `[method, path, init]`, so a prefix such as
// `["get", "/notifications"]` matches every filter of the list.
import { infiniteQueryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";

export type Notification = components["schemas"]["Notification"];
export type NotificationType = components["schemas"]["NotificationType"];
export type NotificationMarker = components["schemas"]["NotificationMarker"];
export type NotificationAccount = components["schemas"]["Account"];
export type NotificationStatus = components["schemas"]["Status"];

/** The filters of the notifications page, in the order of its tabs; `all` sends no `types`. */
export const notificationFilters = ["all", "mentions", "follows", "boosts", "favourites"] as const;

export type NotificationFilter = (typeof notificationFilters)[number];

/** The notification types each filter keeps; the types without a filter of their own are under `all` only. */
const filterTypes: Readonly<Record<Exclude<NotificationFilter, "all">, readonly NotificationType[]>> = {
  mentions: ["mention"],
  follows: ["follow", "follow_request", "follow_accepted"],
  boosts: ["boost"],
  favourites: ["favourite"],
};

/** The `init` of GET /notifications for a filter (without the cursor). */
function listInit(filter: NotificationFilter) {
  return filter === "all" ? {} : { params: { query: { types: [...filterTypes[filter]] } } };
}

/**
 * The user's notifications under `filter`, newest first, a page at a time (GET /notifications). The
 * page's `next_cursor` is the cursor of the next one; a page without one is the last.
 */
export function notificationsQueryOptions(api: Api, filter: NotificationFilter) {
  const init = listInit(filter);
  return infiniteQueryOptions({
    queryKey: ["get", "/notifications", init] as const,
    queryFn: async ({ pageParam, signal }) => {
      const query = { ...init.params?.query, ...(pageParam === undefined ? {} : { cursor: pageParam }) };
      const { data, error } = await api.fetch.GET("/notifications", { params: { query }, signal });
      // The middleware throws for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** The read marker and the unread count (GET /notifications/marker). */
export function notificationMarkerQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/notifications/marker");
}

/** Moves the read marker forward to `lastReadId` (PUT /notifications/marker); returns the marker after it. */
export async function updateNotificationMarker(api: Api, lastReadId: string): Promise<NotificationMarker> {
  const { data, error } = await api.fetch.PUT("/notifications/marker", { body: { last_read_id: lastReadId } });
  if (data === undefined) throw new ApiError(error);
  return data;
}
