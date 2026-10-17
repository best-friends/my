// SPDX-License-Identifier: AGPL-3.0-only
// Shared by the lists whose actions take a row off them: the settings lists and the admin dead jobs.
import { type RefObject, useEffect, useRef } from "react";

/**
 * Keeps focus when a row of a list goes away with the button that had focus: once the list has rendered
 * without the row, focus moves to the first button of the row that took its place, else of the row before
 * it, else to the page's heading (an empty list has no rows left).
 *
 * Returns `removed(id)`, to call when the row of `id` is taken off the list. Rows carry `data-row-id`.
 */
export function useFocusAfterRemoval(list: RefObject<HTMLElement | null>, ids: readonly string[]): (id: string) => void {
  /** The row being removed and where it was. */
  const pendingRef = useRef<{ id: string; index: number } | null>(null);

  // Runs after every render: focus is moved in the DOM, outside React, once the row is gone.
  useEffect(() => {
    const removal = pendingRef.current;
    if (removal === null || ids.includes(removal.id)) return;
    pendingRef.current = null;
    const rows = list.current?.querySelectorAll<HTMLElement>("[data-row-id]") ?? [];
    const row = rows[Math.min(removal.index, rows.length - 1)];
    const target = row?.querySelector<HTMLElement>("button") ?? document.querySelector<HTMLElement>("main h1");
    target?.focus();
  });

  return (id) => {
    pendingRef.current = { id, index: ids.indexOf(id) };
  };
}
