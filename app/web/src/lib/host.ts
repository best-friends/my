// SPDX-License-Identifier: AGPL-3.0-only
// The host of a page on another server, which the links to remote profiles and posts show (FR-006).

/** The host (with its port, if any) of the absolute URL `url`, or `fallback` when it does not parse. */
export function hostOf(url: string, fallback: string): string {
  try {
    return new URL(url).host;
  } catch {
    return fallback;
  }
}
