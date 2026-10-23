// SPDX-License-Identifier: AGPL-3.0-only
// The one subscription of the app to the streaming API (web-client/README.md S10). The stream is an external
// system that the app stays in sync with while a user is signed in, which is what an Effect is for
// (react.dev, "Synchronizing with Effects"; "Lifecycle of Reactive Effects"): it connects when the session
// is signed in, and its cleanup aborts the request when the session ends or the app unmounts. The
// callbacks are Effect Events (react.dev, "Separating Events from Effects"), so new callbacks on a
// re-render do not reconnect.
import { useEffect, useEffectEvent } from "react";
import { useApi, useSessionStatus } from "../auth/SessionProvider";
import { runEventStream, type StreamEvent } from "./stream";

export type EventStreamHandlers = {
  /** An event arrived: write what it means into the Query cache. */
  onEvent: (event: StreamEvent) => void;
  /** A stream opened again after one ended: read again what the gap may have missed. */
  onReconnect: () => void;
};

/** Keeps the signed-in user's event stream open, reconnecting with backoff, and hands its events to `handlers`. */
export function useEventStream({ onEvent, onReconnect }: EventStreamHandlers): void {
  const api = useApi();
  const signedIn = useSessionStatus() === "signed-in";
  const handleEvent = useEffectEvent((event: StreamEvent) => {
    onEvent(event);
  });
  const handleOpen = useEffectEvent((reconnected: boolean) => {
    if (reconnected) onReconnect();
  });
  useEffect(() => {
    if (!signedIn) return;
    const controller = new AbortController();
    void runEventStream(api, controller.signal, {
      onOpen: (reconnected) => {
        handleOpen(reconnected);
      },
      onEvent: (event) => {
        handleEvent(event);
      },
    });
    return () => {
      controller.abort();
    };
  }, [api, signedIn]);
}
