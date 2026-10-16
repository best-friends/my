// SPDX-License-Identifier: AGPL-3.0-only
// The page's side of the Service Worker (ADR-0028, app/web/sw/sw.ts): registering it, knowing when an
// update waits for the user, clearing the caches at sign-out, and reloading when a deploy removed the
// chunks of the running version.
import { useSyncExternalStore } from "react";
import { cachesClearedMessage, clearCachesMessage, isMessage, skipWaitingMessage } from "./sw-protocol";

/** A downloaded version waiting to take over; `apply` activates it, and the page reloads into it. */
export type PendingUpdate = { apply: () => void };

let pendingUpdate: PendingUpdate | null = null;
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getPendingUpdate(): PendingUpdate | null {
  return pendingUpdate;
}

/** The URL of the worker, which vite-plugin-pwa builds into dist/sw.js and `my serve` serves at the root. */
const scriptURL = "/sw.js";

/** The part of the Trusted Types API (https://developer.mozilla.org/docs/Web/API/Trusted_Types_API) used here. */
type TrustedTypes = {
  createPolicy: (name: string, rules: { createScriptURL: (url: string) => string }) => { createScriptURL: (url: string) => unknown };
};

/**
 * The worker's URL as a TrustedScriptURL where the browser has Trusted Types: the shell's CSP sets
 * `require-trusted-types-for 'script'` (ADR-0019), and register() is a sink for script URLs. The policy
 * passes the one fixed URL and nothing else.
 */
function trustedScriptURL(): string {
  const factory = (globalThis as { trustedTypes?: TrustedTypes }).trustedTypes;
  if (factory === undefined) return scriptURL;
  const policy = factory.createPolicy("my-service-worker", {
    createScriptURL(url) {
      if (url !== scriptURL) throw new TypeError(`not the Service Worker's URL: ${url}`);
      return url;
    },
  });
  // register() accepts the TrustedScriptURL in place of the string its DOM type names.
  return policy.createScriptURL(scriptURL) as string;
}

/**
 * Registers the worker at /sw.js for the whole origin
 * (https://developer.mozilla.org/docs/Web/API/ServiceWorkerContainer/register). The first worker takes over
 * at once (it claims the page). A later version installs in the background and waits: it does not take over
 * by itself, since a reload could lose a draft. usePendingUpdate() returns it until the user applies it,
 * which sends SKIP_WAITING to the waiting worker and reloads once it controls the page (controllerchange,
 * https://web.dev/articles/service-worker-lifecycle). The browser looks for a new version when the page
 * loads; the page also asks whenever it becomes visible again, since an open app may never navigate.
 */
export async function registerServiceWorker(container: ServiceWorkerContainer, reload: () => void): Promise<void> {
  const offer = (worker: ServiceWorker) => {
    // Without a controller this is the first worker, not an update.
    if (container.controller === null) return;
    pendingUpdate = {
      apply() {
        container.addEventListener("controllerchange", reload, { once: true });
        worker.postMessage(skipWaitingMessage);
      },
    };
    for (const listener of listeners) listener();
  };

  const registration = await container.register(trustedScriptURL(), { scope: "/" });
  if (registration.waiting !== null) offer(registration.waiting);
  registration.addEventListener("updatefound", () => {
    const worker = registration.installing;
    if (worker === null) return;
    worker.addEventListener("statechange", () => {
      if (worker.state === "installed") offer(worker);
    });
  });
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") registration.update().catch(() => undefined);
  });
}

/** The waiting update, or null while the running version is the latest one known. */
export function usePendingUpdate(): PendingUpdate | null {
  return useSyncExternalStore(subscribe, getPendingUpdate, () => null);
}

/** How long sign-out waits for the worker to confirm that it cleared its caches. */
const clearTimeoutMs = 3000;

/**
 * Deletes every cache of the origin, the precache included, so that nothing stays on the device after
 * sign-out (ADR-0028). The worker deletes its media cache with its expiration metadata; the page then
 * deletes whatever is left, which also covers a page without a controlling worker. The app keeps working
 * online; it works offline again once a new version of the worker precaches the build.
 */
export async function clearCaches(): Promise<void> {
  const controller = "serviceWorker" in navigator ? navigator.serviceWorker.controller : null;
  if (controller !== null) await askWorkerToClear(controller);
  if (!("caches" in globalThis)) return;
  const names = await caches.keys();
  await Promise.all(names.map((name) => caches.delete(name)));
}

function askWorkerToClear(worker: ServiceWorker): Promise<void> {
  return new Promise((resolve) => {
    const channel = new MessageChannel();
    const timer = setTimeout(done, clearTimeoutMs);
    function done() {
      clearTimeout(timer);
      channel.port1.close();
      resolve();
    }
    channel.port1.onmessage = (event: MessageEvent) => {
      if (isMessage(event.data, cachesClearedMessage.type)) done();
    };
    worker.postMessage(clearCachesMessage, [channel.port2]);
  });
}

/** The session storage key of the time of the last reload after a failed chunk load. */
const reloadedAtKey = "my:preload-error-reloaded-at";
/** A second failure within this time is not reloaded again, so that a broken deploy cannot loop. */
const reloadGuardMs = 10_000;

/**
 * Reloads the page when a chunk of the running version fails to load, which happens after a deploy removed
 * it (https://vite.dev/guide/build#load-error-handling). If the reload does not help, the error reaches the
 * error boundaries as usual.
 */
export function reloadOnPreloadError(target: Window): () => void {
  const onPreloadError = (event: Event) => {
    const now = Date.now();
    let last = Number.NaN;
    try {
      last = Number(target.sessionStorage.getItem(reloadedAtKey));
      target.sessionStorage.setItem(reloadedAtKey, String(now));
    } catch {
      // Without session storage there is no loop guard; reload anyway, as Vite's guide does.
    }
    if (now - last < reloadGuardMs) return;
    event.preventDefault();
    target.location.reload();
  };
  target.addEventListener("vite:preloadError", onPreloadError);
  return () => {
    target.removeEventListener("vite:preloadError", onPreloadError);
  };
}
