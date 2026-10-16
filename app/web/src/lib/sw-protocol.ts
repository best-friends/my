// SPDX-License-Identifier: AGPL-3.0-only
// What the page and the Service Worker (app/web/sw/sw.ts) share: the messages between them and the
// names of the worker's caches (ADR-0028). Both programs compile this file, so it uses neither DOM nor
// WebWorker types.

/** The prefix of every Cache Storage cache the worker creates (workbox-core's setCacheNameDetails). */
export const cachePrefix = "my";

/**
 * The precache holds the build and the generic HTML shell, which are the same for every visitor and hold
 * no account data; it is the only cache that survives a sign-out, so that the offline shell keeps working.
 */
export const precachePrefix = `${cachePrefix}-precache-`;

/** The cache of public media (/media/ images), first-hit and bounded. */
export const mediaCacheName = `${cachePrefix}-media`;

/** The path of the generic HTML shell the worker precaches and returns for navigations it cannot fetch. */
export const shellPath = "/";

/** Activates a waiting worker; workbox-window's messageSkipWaiting() sends it. */
export const skipWaitingMessage = { type: "SKIP_WAITING" } as const;

/** Asks the worker to delete every cache but the precache, with their metadata; it replies on the port. */
export const clearCachesMessage = { type: "CLEAR_CACHES" } as const;

/** The worker's reply to clearCachesMessage. */
export const cachesClearedMessage = { type: "CACHES_CLEARED" } as const;

/** Whether `data` is a message of type `type`. */
export function isMessage(data: unknown, type: string): boolean {
  return typeof data === "object" && data !== null && "type" in data && data.type === type;
}
