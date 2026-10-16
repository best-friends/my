// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { type NotificationStatus, statusQueryOptions } from "../api/notifications";

/** The post a notification is about. */
export type NotificationPost = { status: "loading" } | { status: "error" } | { status: "loaded"; post: NotificationStatus };

/** Loads the post with `id` (a notification's `status_id`); the cache is shared with every other view of it. */
export function useNotificationPost(id: string): NotificationPost {
  const api = useApi();
  const query = useQuery(statusQueryOptions(api, id));
  if (query.isError) return { status: "error" };
  if (query.data === undefined) return { status: "loading" };
  return { status: "loaded", post: query.data };
}
