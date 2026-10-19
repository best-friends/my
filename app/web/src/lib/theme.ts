// SPDX-License-Identifier: AGPL-3.0-only
// The color theme the app is displayed in: the OS setting (system), light or dark (app/ui/README.md, "Light and
// dark"). It is how this browser shows the app, like the remembered display locale, so it is kept in localStorage
// and not in the user's preferences on the server: it outlives the session and applies before anyone signs in.
//
// <html data-theme> is the state. public/theme.js, a render-blocking classic script in the shell's <head>, sets it
// from storage before the first paint, so that the page never flashes in the OS theme first. This module reads
// the attribute as an external store (react.dev: useSyncExternalStore), changes it with the storage, and follows
// a choice made in another tab of the same origin (the storage event).
import { useSyncExternalStore } from "react";

export type Theme = "system" | "light" | "dark";

/** The themes in the order the controls offer them. */
export const themes: readonly Theme[] = ["system", "light", "dark"];

/** The localStorage key of the chosen theme; public/theme.js reads the same key. No key is the OS setting. */
export const themeKey = "my:display:theme";

function isStoredTheme(value: string | null | undefined): value is "light" | "dark" {
  return value === "light" || value === "dark";
}

/** The theme `doc` is displayed in now. */
export function documentTheme(doc: Document): Theme {
  const theme = doc.documentElement.dataset["theme"];
  return isStoredTheme(theme) ? theme : "system";
}

/** Sets <html data-theme> to `theme`, or removes it for the OS setting. */
function applyTheme(doc: Document, theme: Theme): void {
  if (theme === "system") delete doc.documentElement.dataset["theme"];
  else doc.documentElement.dataset["theme"] = theme;
}

/** The theme kept in storage; the OS setting when there is none or storage is blocked. */
function storedTheme(): Theme {
  try {
    const stored = localStorage.getItem(themeKey);
    return isStoredTheme(stored) ? stored : "system";
  } catch {
    return "system";
  }
}

const listeners = new Set<() => void>();

function notify(): void {
  for (const listener of listeners) listener();
}

/**
 * Applies the theme kept in storage to `doc`. public/theme.js has done so before the first paint wherever the
 * shell loads it; this covers a shell without it (a Vite dev server).
 */
export function applyStoredTheme(doc: Document): void {
  applyTheme(doc, storedTheme());
}

/** Displays the app in `theme` at once and keeps it for the next start on this browser. */
export function setTheme(theme: Theme, doc: Document = document): void {
  try {
    if (theme === "system") localStorage.removeItem(themeKey);
    else localStorage.setItem(themeKey, theme);
  } catch {
    // Storage is full or blocked: the theme still applies to this page, and the next start follows the OS.
  }
  applyTheme(doc, theme);
  notify();
}

/** Follows a theme chosen in another tab: the storage event fires in the other documents of the origin only. */
function onStorage(event: StorageEvent): void {
  if (event.key !== themeKey && event.key !== null) return;
  applyStoredTheme(document);
  notify();
}

function subscribe(listener: () => void): () => void {
  if (listeners.size === 0) window.addEventListener("storage", onStorage);
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) window.removeEventListener("storage", onStorage);
  };
}

function getSnapshot(): Theme {
  return documentTheme(document);
}

/** The theme the app is displayed in, rendering again when it changes. */
export function useTheme(): Theme {
  return useSyncExternalStore(subscribe, getSnapshot);
}
