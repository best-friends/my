// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { z } from "zod";
import { useApi } from "../../../lib/auth/SessionProvider";
import { instanceQueryOptions } from "../api/timelines";

/** Which public timelines to offer: an instance setting turns each off, and it is then a 404 (FR-202). */
export type TimelineAvailability = {
  /** Whether to offer the local timeline (`/local`). */
  local: boolean;
  /** Whether to offer the federated timeline (`/public`). */
  federated: boolean;
};

/** The localStorage key of the last `timelines` that GET /instance answered on this origin. */
const storageKey = "my:instance:timelines";

const storedSchema = z.object({ local: z.boolean(), federated: z.boolean() });

/** The last answer kept on this origin, or undefined when there is none or storage is unavailable. */
function readStored(): TimelineAvailability | undefined {
  try {
    const stored = localStorage.getItem(storageKey);
    if (stored === null) return undefined;
    const parsed = storedSchema.safeParse(JSON.parse(stored));
    return parsed.success ? parsed.data : undefined;
  } catch {
    return undefined;
  }
}

function writeStored(availability: TimelineAvailability): void {
  try {
    localStorage.setItem(storageKey, JSON.stringify(availability));
  } catch {
    // Storage is full or blocked; the navigation then waits for GET /instance on the next start.
  }
}

/**
 * Which public timelines the instance serves, from GET /instance. The local timeline is part of every
 * instance's navigation, so it is offered until the instance says it is off. The federated timeline is
 * offered only once the instance says it is on, so that an instance that turned it off never shows it,
 * not even while GET /instance loads or after it failed.
 *
 * The last answer is kept in localStorage (an instance setting, the same for every user of the origin), so
 * that a page opened or reloaded offline, where GET /instance fails, offers the same timelines as before
 * (web-client.md G17). It is read once when the navigation mounts and gives way to the server's answer.
 */
export function useTimelineAvailability(): TimelineAvailability {
  const api = useApi();
  const instance = useQuery(instanceQueryOptions(api));
  const [stored] = useState(readStored);
  const local = instance.data?.timelines.local;
  const federated = instance.data?.timelines.federated;
  useEffect(() => {
    if (local !== undefined && federated !== undefined) writeStored({ local, federated });
  }, [local, federated]);
  return { local: local ?? stored?.local ?? true, federated: federated ?? stored?.federated ?? false };
}
