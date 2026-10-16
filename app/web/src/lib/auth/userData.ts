// SPDX-License-Identifier: AGPL-3.0-only
// What the app forgets when the user changes (a sign-in, or the end of a session): every cached query and,
// when the session ends, every Cache Storage cache of the origin, the Service Worker's precache included
// (ADR-0028, lib/pwa's clearCaches).
import type { QueryClient } from "@tanstack/react-query";
import { clearCaches } from "../pwa";
import type { Session } from "./session";

/** Drops the user's cached data whenever the user of `session` changes; returns the unsubscribe function. */
export function forgetUserDataOnUserChange(session: Session, queryClient: QueryClient): () => void {
  return session.onUserChange((status) => {
    queryClient.removeQueries();
    if (status === "signed-out") {
      clearCaches().catch((error: unknown) => {
        console.error("the caches could not be cleared", error);
      });
    }
  });
}
