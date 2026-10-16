// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery, useSuspenseQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type Status, statusContextQueryOptions, statusQueryOptions } from "../api/status";

/** The thread around the post, which loads after the post and may fail on its own (web-client.md E1). */
export type ThreadState =
  | { status: "loading" }
  | { status: "error"; message: string; retry: () => void }
  | { status: "loaded"; ancestors: readonly Status[]; descendants: readonly Status[] };

export type StatusThread = {
  /** The post the page is about. */
  status: Status;
  thread: ThreadState;
};

/**
 * Reads a post that the route loader has fetched, and the thread around it. The post's failure goes to the
 * route's error boundary; the thread's stays here, so the post stays on screen. Both are read from the
 * query cache on every render, so an action on any post of the thread shows at once (S2).
 */
export function useStatusThread(id: string): StatusThread {
  const api = useApi();
  const { data: status } = useSuspenseQuery(statusQueryOptions(api, id));
  const context = useQuery({
    ...statusContextQueryOptions(api, id),
    // Shown at once with a button to try again, instead of after seconds of silent retries.
    retry: false,
  });
  const thread: ThreadState = context.isError
    ? {
        status: "error",
        message: errorMessage(context.error),
        retry: () => {
          void context.refetch();
        },
      }
    : context.data === undefined
      ? { status: "loading" }
      : { status: "loaded", ancestors: context.data.ancestors, descendants: context.data.descendants };
  return { status, thread };
}
