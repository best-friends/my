// SPDX-License-Identifier: AGPL-3.0-only
// What the app forgets when the user changes (a sign-in, or the end of a session): every cached query, and
// the Cache Storage that the Service Worker keeps responses in. Only the precache of the app's own build
// stays, as it holds no user data.
import type { QueryClient } from "@tanstack/react-query";
import type { Session } from "./session";

/** The prefix of Workbox's precache, which holds only the files of the build. */
const precachePrefix = "workbox-precache";

/** Drops the user's cached data whenever the user of `session` changes; returns the unsubscribe function. */
export function forgetUserDataOnUserChange(session: Session, queryClient: QueryClient): () => void {
  return session.onUserChange(() => {
    queryClient.removeQueries();
    void clearCacheStorage();
  });
}

async function clearCacheStorage(): Promise<void> {
  // Cache Storage exists only in secure contexts.
  if (typeof caches === "undefined") return;
  const names = await caches.keys();
  await Promise.all(names.filter((name) => !name.startsWith(precachePrefix)).map((name) => caches.delete(name)));
}
