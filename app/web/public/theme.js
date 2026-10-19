// SPDX-License-Identifier: AGPL-3.0-only
// Applies the color theme chosen on this browser before the first paint (app/ui/README.md, "Light and dark").
// The shell loads it as a render-blocking classic script in <head>, before its stylesheets: the CSP allows no
// inline script, and the shell is shared by every viewer, so it cannot carry the choice itself (ADR-0028).
// src/lib/theme.ts owns the choice and the same localStorage key; no key, or another value, is the OS setting.
try {
  const theme = localStorage.getItem("my:display:theme");
  if (theme === "light" || theme === "dark") document.documentElement.dataset.theme = theme;
} catch {
  // Storage is blocked: the page follows the OS setting.
}
