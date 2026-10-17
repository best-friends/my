// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { instanceQueryOptions } from "../api/timelines";

/** Which public timelines to offer: an instance setting turns each off, and it is then a 404 (FR-202). */
export type TimelineAvailability = {
  /** Whether to offer the local timeline (`/local`). */
  local: boolean;
  /** Whether to offer the federated timeline (`/public`). */
  federated: boolean;
};

/**
 * Which public timelines the instance serves, from GET /instance. The local timeline is part of every
 * instance's navigation, so it is offered until the instance says it is off. The federated timeline is
 * offered only once the instance says it is on, so that an instance that turned it off never shows it,
 * not even while GET /instance loads or after it failed.
 */
export function useTimelineAvailability(): TimelineAvailability {
  const api = useApi();
  const instance = useQuery(instanceQueryOptions(api));
  const timelines = instance.data?.timelines;
  return { local: timelines?.local ?? true, federated: timelines?.federated ?? false };
}
