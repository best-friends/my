// SPDX-License-Identifier: AGPL-3.0-only
import { useSuspenseQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { pinnedStatusesQueryOptions, type Status } from "../../status";

/**
 * The posts that account `accountId` pinned to its profile, newest pin first (FR-403); the route loader has
 * fetched them with the profile, and a failure goes to its error boundary. Read from the query cache on every
 * render, never copied (S2).
 */
export function usePinnedStatuses(accountId: string): readonly Status[] {
  const api = useApi();
  return useSuspenseQuery(pinnedStatusesQueryOptions(api, accountId)).data.items;
}
