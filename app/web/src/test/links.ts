// SPDX-License-Identifier: AGPL-3.0-only
// The names of the links that open in a new tab (web-client/README.md Y12) for tests in English.

/**
 * The name of a link followed by its hidden "(opens in a new tab)". jsdom lays nothing out, so the name it computes
 * has no space between the two; a browser's has one, as the hidden text is a block (`sr-only` is absolute).
 */
export function newTabName(name: string): RegExp {
  return new RegExp(`^${name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")} ?\\(opens in a new tab\\)$`);
}

/** The visible text of a link that opens in a new tab: its text content without the hidden "(opens in a new tab)". */
export function visibleText(link: HTMLElement): string {
  return link.textContent.replace(/ \(opens in a new tab\)$/, "");
}
