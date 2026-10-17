// SPDX-License-Identifier: AGPL-3.0-only
import { useBlocker } from "@tanstack/react-router";

/** Whether a navigation waits for the user, and the two answers. */
export type UnsavedChangesGuard = {
  /** A navigation to another page is waiting: ask whether to leave. */
  isAsking: boolean;
  /** Goes on to the other page; the changes are lost. */
  leave: () => void;
  /** Stays on this page with the changes. */
  stay: () => void;
};

/**
 * Holds a navigation to another page while `hasChanges` until the user answers (TanStack Router's useBlocker
 * with a resolver). A change of only the search or the hash stays on the page and is not held. Reloading or
 * closing the tab gets the browser's own confirmation (beforeunload), whose words a page cannot set.
 */
export function useUnsavedChangesGuard(hasChanges: boolean): UnsavedChangesGuard {
  const blocker = useBlocker({
    shouldBlockFn: ({ current, next }) => hasChanges && current.pathname !== next.pathname,
    enableBeforeUnload: hasChanges,
    withResolver: true,
  });
  return {
    isAsking: blocker.status === "blocked",
    leave: () => blocker.proceed?.(),
    stay: () => blocker.reset?.(),
  };
}
