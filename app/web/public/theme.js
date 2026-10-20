// SPDX-License-Identifier: AGPL-3.0-only
// Applies the color theme chosen on this browser before the first paint (app/ui/README.md, "Light and dark").
// The shell loads it as a render-blocking classic script in <head>, before its stylesheets: the CSP allows no
// inline script, and the shell is shared by every viewer, so it cannot carry the choice itself (ADR-0028).
// It cannot import src/lib/storage, which declares the key, so it reads the same key and format itself,
// {"v":1,"value":"light"} (src/lib/theme.test.ts checks that they agree). No key, another version or another
// value is the OS setting.
try {
  const stored = JSON.parse(localStorage.getItem("my:display:theme") ?? "null");
  const theme = typeof stored === "object" && stored !== null && stored.v === 1 ? stored.value : undefined;
  if (theme === "light" || theme === "dark") document.documentElement.dataset.theme = theme;
} catch {
  // Storage is blocked or holds no JSON: the page follows the OS setting.
}
