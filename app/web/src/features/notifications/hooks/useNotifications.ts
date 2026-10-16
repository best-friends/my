// SPDX-License-Identifier: AGPL-3.0-only
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useApi, useSessionStatus } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import {
  type Notification,
  type NotificationFilter,
  type NotificationMarker,
  notificationMarkerQueryOptions,
  notificationsQueryOptions,
  updateNotificationMarker,
} from "../api/notifications";

/** The notifications page's list. */
export type NotificationList =
  | { status: "signed-out" }
  /** The session is resuming, or the first page is loading. */
  | { status: "loading" }
  /** The first page failed; `message` is localized. */
  | { status: "error"; message: string; retry: () => void }
  | {
      status: "loaded";
      items: readonly Notification[];
      hasMore: boolean;
      isLoadingMore: boolean;
      loadMore: () => void;
      /** Whether a notification was unread when the page opened, so it stays marked as new while the page is open. */
      isUnread: (notification: Notification) => boolean;
    };

/** Whether `id` comes after the marker: IDs are UUIDv7 strings, which sort in the order they were made. */
function isAfter(id: string, marker: NotificationMarker): boolean {
  return marker.last_read_id === undefined || id > marker.last_read_id;
}

/**
 * The read marker as it was when the page opened: the page keeps showing which notifications were new
 * after it has marked them read. It is a snapshot taken once (useState's initializer), not a copy kept in
 * sync with the server (web-client.md S2); `null` when the marker had not loaded, and nothing is marked new.
 */
function useMarkerAtOpen(marker: NotificationMarker | undefined): NotificationMarker | null {
  const [atOpen] = useState(() => marker ?? null);
  return atOpen;
}

/**
 * Moves the read marker to `newestId`, the newest notification on screen, whenever a newer one is shown.
 * It is an Effect because the reason is that the notifications were displayed, not something the user
 * did (react.dev, "You Might Not Need an Effect": sending a POST because a component was displayed).
 * The server moves the marker only forward, so a repeated or late request changes nothing; a failure is
 * left alone, and the next visit sends it again.
 */
function useMarkRead(newestId: string | undefined, marker: NotificationMarker | undefined) {
  const api = useApi();
  const queryClient = useQueryClient();
  const { mutate } = useMutation({
    mutationFn: (lastReadId: string) => updateNotificationMarker(api, lastReadId),
    onSuccess: (updated) => {
      queryClient.setQueryData(notificationMarkerQueryOptions(api).queryKey, updated);
    },
  });
  /** The ID last sent, so that a re-run of the Effect (Strict Mode, a refetch) does not send it again. */
  const sentRef = useRef<string | null>(null);
  const shouldSend = newestId !== undefined && marker !== undefined && isAfter(newestId, marker);
  useEffect(() => {
    if (!shouldSend || sentRef.current === newestId) return;
    sentRef.current = newestId;
    mutate(newestId);
  }, [shouldSend, newestId, mutate]);
}

/**
 * The notifications under `filter`, and the read marker. While the page shows every type it marks the
 * newest notification read; a filtered list does not, since the marker is one position over all types and
 * would pass notifications the filter hides.
 */
export function useNotifications(filter: NotificationFilter): NotificationList {
  const api = useApi();
  const session = useSessionStatus();
  const signedIn = session === "signed-in";
  const list = useInfiniteQuery({ ...notificationsQueryOptions(api, filter), enabled: signedIn });
  const marker = useQuery({ ...notificationMarkerQueryOptions(api), enabled: signedIn });
  const markerAtOpen = useMarkerAtOpen(marker.data);
  const items = list.data?.pages.flatMap((page) => page.items) ?? [];
  useMarkRead(signedIn && filter === "all" ? items[0]?.id : undefined, marker.data);

  if (session === "signed-out") return { status: "signed-out" };
  if (list.isError && list.data === undefined) {
    return {
      status: "error",
      message: errorMessage(list.error),
      retry: () => {
        void list.refetch();
      },
    };
  }
  if (list.data === undefined) return { status: "loading" };
  return {
    status: "loaded",
    items,
    hasMore: list.hasNextPage,
    isLoadingMore: list.isFetchingNextPage,
    loadMore: () => {
      if (!list.isFetchingNextPage) void list.fetchNextPage();
    },
    isUnread: (notification) => markerAtOpen !== null && isAfter(notification.id, markerAtOpen),
  };
}
