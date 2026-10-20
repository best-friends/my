// SPDX-License-Identifier: AGPL-3.0-only
// The custom emoji picked recently on this browser (ADR-0051 §4: the recent emoji are kept per device). They are
// how this browser is used, like the theme, so they are in localStorage, outlive the session and are shared by the
// accounts that sign in here. Only shortcodes are kept; the picker shows those that are still offered.
import { useSyncExternalStore } from "react";

/** The localStorage key of the recent shortcodes, a JSON array, most recent first. */
const recentKey = "my:emoji:recent";

/** How many emoji the picker shows as recent. */
const maxRecentEmojis = 16;

/** The stored value as it is, the snapshot of the store: a string compares equal between reads. */
function storedValue(): string {
  try {
    return localStorage.getItem(recentKey) ?? "[]";
  } catch {
    return "[]";
  }
}

/** The shortcodes of a stored value; an unreadable value is none. */
function parse(value: string): string[] {
  try {
    const parsed: unknown = JSON.parse(value);
    return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string").slice(0, maxRecentEmojis) : [];
  } catch {
    return [];
  }
}

const listeners = new Set<() => void>();

/** Puts `shortcode` first among the recent emoji. */
export function rememberEmoji(shortcode: string): void {
  const next = [shortcode, ...parse(storedValue()).filter((code) => code !== shortcode)].slice(0, maxRecentEmojis);
  try {
    localStorage.setItem(recentKey, JSON.stringify(next));
  } catch {
    // Storage is full or blocked: the picker shows no recent emoji.
  }
  for (const listener of listeners) listener();
}

/** Follows a pick in another tab: the storage event fires in the other documents of the origin only. */
function onStorage(event: StorageEvent): void {
  if (event.key !== recentKey && event.key !== null) return;
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  if (listeners.size === 0) window.addEventListener("storage", onStorage);
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) window.removeEventListener("storage", onStorage);
  };
}

/** The shortcodes of the recent emoji, most recent first, rendering again when they change. */
export function useRecentShortcodes(): string[] {
  return parse(useSyncExternalStore(subscribe, storedValue));
}
