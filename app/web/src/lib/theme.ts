// SPDX-License-Identifier: AGPL-3.0-only
// The color theme the app is displayed in: the OS setting (system), light or dark (app/ui/README.md, "Light and
// dark"). It is how this browser shows the app, like the remembered display locale, so it is kept in localStorage
// and not in the user's preferences on the server: it outlives the session and applies before anyone signs in.
//
// <html data-theme> is the state. public/theme.js, a render-blocking classic script in the shell's <head>, sets it
// from storage before the first paint, so that the page never flashes in the OS theme first. This module reads
// the attribute as an external store (react.dev: useSyncExternalStore), changes it with the stored choice
// (lib/storage's themeKey), and follows a choice made in another tab of the same origin.
import { useSyncExternalStore } from "react";
import { themeKey } from "./storage";

export type Theme = "system" | "light" | "dark";

/** The themes in the order the controls offer them. */
export const themes: readonly Theme[] = ["system", "light", "dark"];

/** The theme `doc` is displayed in now. */
export function documentTheme(doc: Document): Theme {
  const theme = doc.documentElement.dataset["theme"];
  return theme === "light" || theme === "dark" ? theme : "system";
}

/** Sets <html data-theme> to `theme`, or removes it for the OS setting. */
function applyTheme(doc: Document, theme: Theme): void {
  if (theme === "system") delete doc.documentElement.dataset["theme"];
  else doc.documentElement.dataset["theme"] = theme;
}

/**
 * Applies the theme kept in storage to `doc`. public/theme.js has done so before the first paint wherever the
 * shell loads it; this covers a shell without it (a Vite dev server).
 */
export function applyStoredTheme(doc: Document): void {
  applyTheme(doc, themeKey.read());
}

/** Displays the app in `theme` at once and keeps it for the next start on this browser. */
export function setTheme(theme: Theme, doc: Document = document): void {
  applyTheme(doc, theme);
  themeKey.write(theme);
}

/** Follows the stored choice, made here or in another tab, and calls `listener` once <html data-theme> has it. */
function subscribe(listener: () => void): () => void {
  return themeKey.subscribe(() => {
    applyStoredTheme(document);
    listener();
  });
}

function getSnapshot(): Theme {
  return documentTheme(document);
}

/** The theme the app is displayed in, rendering again when it changes. */
export function useTheme(): Theme {
  return useSyncExternalStore(subscribe, getSnapshot);
}
