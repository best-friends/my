// SPDX-License-Identifier: AGPL-3.0-only
// What the app forgets when the user changes (a sign-in, or the end of a session): every cached query and,
// when the session ends, every Cache Storage cache of the origin, the Service Worker's precache included
// (ADR-0028, lib/pwa's clearCaches), and the tab's sessionStorage entries under `userStoragePrefix` (what
// a user typed, such as the draft of a post), so that the next person at a shared computer does not find
// them. A sign-in keeps those entries, as each is stored under its account's ID.
import type { QueryClient } from "@tanstack/react-query";
import { clearCaches } from "../pwa";
import type { Session } from "./session";

/** The prefix of the sessionStorage keys that hold a user's own input; they are dropped when the session ends. */
export const userStoragePrefix = "my:user:";

/** Drops the user's cached data whenever the user of `session` changes; returns the unsubscribe function. */
export function forgetUserDataOnUserChange(session: Session, queryClient: QueryClient): () => void {
  return session.onUserChange((status) => {
    queryClient.removeQueries();
    if (status === "signed-out") {
      clearUserStorage();
      clearCaches().catch((error: unknown) => {
        console.error("the caches could not be cleared", error);
      });
    }
  });
}

function clearUserStorage(): void {
  try {
    const keys = Array.from({ length: sessionStorage.length }, (_, i) => sessionStorage.key(i));
    for (const key of keys) if (key?.startsWith(userStoragePrefix) === true) sessionStorage.removeItem(key);
  } catch {
    // No storage (a sandboxed frame): nothing was saved.
  }
}
