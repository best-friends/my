// SPDX-License-Identifier: AGPL-3.0-only
// Whether animated images (GIF, animated WebP) play by themselves (ADR-0054 §3). Like the theme (lib/theme), it is
// how this browser shows the app, so it is kept in localStorage and not in the user's preferences on the server: it
// applies before anyone signs in and outlives the session. On by default; even on, an animation stays still while
// the system asks for reduced motion (@my/ui Picture).
import { useSyncExternalStore } from "react";

/** The localStorage key of the setting: "off" when animations do not play by themselves; no key is on. */
const autoplayKey = "my:display:autoplay";

/** Whether animations play by themselves, as kept in storage; on when nothing is kept or storage is blocked. */
function storedAutoplay(): boolean {
  try {
    return localStorage.getItem(autoplayKey) !== "off";
  } catch {
    return true;
  }
}

const listeners = new Set<() => void>();

/** Turns the autoplay of animations on or off at once and keeps the choice for the next start on this browser. */
export function setAutoplay(autoplay: boolean): void {
  try {
    if (autoplay) localStorage.removeItem(autoplayKey);
    else localStorage.setItem(autoplayKey, "off");
  } catch {
    // Storage is full or blocked: the next read falls back to on.
  }
  for (const listener of listeners) listener();
}

/** Follows a choice made in another tab: the storage event fires in the other documents of the origin only. */
function onStorage(event: StorageEvent): void {
  if (event.key !== autoplayKey && event.key !== null) return;
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

/** Whether animations play by themselves, rendering again when the setting changes. */
export function useAutoplay(): boolean {
  return useSyncExternalStore(subscribe, storedAutoplay);
}
