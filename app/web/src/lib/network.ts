// SPDX-License-Identifier: AGPL-3.0-only
// Whether the app is offline (web-client.md G17), kept in TanStack Query's onlineManager so that the query cache and
// the app's offline bar read one state. The app is offline when the browser says it has no network (`navigator.onLine`
// and its `offline` / `online` events, which the onlineManager listens to) or when a request to the client API got no
// response (a NetworkError) and no response has arrived since. Going back online refetches the queries that failed
// (lib/query.ts), and a page that failed for the network loads again (useOnReconnect).
import { onlineManager } from "@tanstack/react-query";
import { useEffect, useEffectEvent, useSyncExternalStore } from "react";

/** Whether the browser says it has no network. `navigator.onLine` is false only when it surely has none. */
export function isBrowserOffline(): boolean {
  return typeof navigator !== "undefined" && !navigator.onLine;
}

/**
 * Starts from what the browser says now. The onlineManager assumes a network until an `offline` event, so a page
 * opened offline (from the Service Worker's shell) would otherwise count as online and never see the change back.
 */
export function startConnectivity(): void {
  onlineManager.setOnline(!isBrowserOffline());
}

/** A request got no response: the network is down, or the server is unreachable. */
export function reportNetworkFailure(): void {
  onlineManager.setOnline(false);
}

/** A response arrived, so the server is reachable again, unless the browser has since lost the network. */
export function reportResponse(): void {
  if (!isBrowserOffline()) onlineManager.setOnline(true);
}

const subscribe = (listener: () => void) => onlineManager.subscribe(listener);
const isOnline = () => onlineManager.isOnline();

/** Whether the app is offline now; the component renders again when it changes. */
export function useIsOffline(): boolean {
  return !useSyncExternalStore(subscribe, isOnline, isOnline);
}

/**
 * Calls `onReconnect` each time the app goes back online while `enabled`: a region that failed for the network loads
 * again by itself. It is an Effect because the reason is the connection, not something the user did.
 */
export function useOnReconnect(onReconnect: () => void, enabled: boolean): void {
  const reconnected = useEffectEvent(onReconnect);
  useEffect(() => {
    if (!enabled) return undefined;
    return onlineManager.subscribe((online) => {
      if (online) reconnected();
    });
  }, [enabled]);
}
