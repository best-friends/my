// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery } from "@tanstack/react-query";
import { useApi, useSessionStatus } from "../../../lib/auth/SessionProvider";
import { notificationMarkerQueryOptions } from "../api/notifications";

/**
 * The number of unread notifications, from the read marker: 0 while signed out, while the marker loads,
 * and when it cannot be loaded (the badge then stays away rather than showing a guess).
 */
export function useUnreadNotificationCount(): number {
  const api = useApi();
  const status = useSessionStatus();
  const marker = useQuery({ ...notificationMarkerQueryOptions(api), enabled: status === "signed-in" });
  return status === "signed-in" ? (marker.data?.unread_count ?? 0) : 0;
}
