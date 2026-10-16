// SPDX-License-Identifier: AGPL-3.0-only
// The layout keeps the one event stream of the app open (web-client.md S10, FR-207) and routes its events
// to the features that own the cached data: new home entries wait behind the home timeline's "N new
// posts" button, and notifications update the unread badge. After a reconnect, both read again what the
// gap may have missed, since the server replays nothing (client-api.md §5).
import { useQueryClient } from "@tanstack/react-query";
import { catchUpNotifications, receiveNotification } from "../../features/notifications";
import { catchUpHome, receiveHomeStatus } from "../../features/timeline";
import { useApi } from "../../lib/auth/SessionProvider";
import { useEventStream } from "../../lib/sse/useEventStream";

export function useLiveUpdates(): void {
  const api = useApi();
  const queryClient = useQueryClient();
  useEventStream({
    onEvent: (event) => {
      switch (event.type) {
        case "update":
          receiveHomeStatus(queryClient, event.status);
          break;
        case "notification":
          receiveNotification(queryClient, api, event.notification);
          break;
      }
    },
    onReconnect: () => {
      catchUpHome(queryClient, api).catch((error: unknown) => {
        // The next reconnect or a visit to the timeline reads it again.
        console.warn("the home timeline could not be read after reconnecting", error);
      });
      catchUpNotifications(queryClient, api);
    },
  });
}
