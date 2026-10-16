// SPDX-License-Identifier: AGPL-3.0-only
// Keeps React Aria inside the shell's Content-Security-Policy (ADR-0019 §9), whose `style-src 'self'` has
// no nonce (ADR-0028) and so blocks every inline <style> element. Zod is the other library that would break
// the policy; src/lib/zod.ts turns off its eval.
/**
 * The id of the <style> element that React Aria's usePress adds to <head> on first use, and skips when an
 * element with this id is already in the document (react-aria `interactions/usePress`). The rule it holds
 * is in index.css instead.
 */
export const reactAriaPressableStyleID = "react-aria-pressable-style";

/** Puts an inert <template> with the id of React Aria's pressable <style> in <head>, before anything renders. */
export function conformToContentSecurityPolicy(document: Document): void {
  if (document.getElementById(reactAriaPressableStyleID) !== null) return;
  const marker = document.createElement("template");
  marker.id = reactAriaPressableStyleID;
  document.head.append(marker);
}
