// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery } from "@tanstack/react-query";
import { useApi, useSessionStatus } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { type Account, viewerQueryOptions } from "../api/viewer";

/** Who is using the app. */
export type Viewer =
  /** The session is resuming, or the account is loading. */
  | { status: "loading" }
  | { status: "signed-out" }
  | { status: "signed-in"; account: Account }
  /** Signed in, but the account could not be loaded; `message` is localized. `isRetrying` while `retry` loads it again. */
  | { status: "error"; message: string; retry: () => void; isRetrying: boolean };

/** Returns the viewer: the session's status and, once signed in, the account from the API. */
export function useViewer(): Viewer {
  const api = useApi();
  const status = useSessionStatus();
  const account = useQuery({ ...viewerQueryOptions(api), enabled: status === "signed-in" });
  if (status === "signed-out") return { status: "signed-out" };
  if (status === "unknown") return { status: "loading" };
  if (account.isError) {
    return {
      status: "error",
      message: errorMessage(account.error),
      retry: () => {
        void account.refetch();
      },
      isRetrying: account.isFetching,
    };
  }
  if (account.data === undefined) return { status: "loading" };
  return { status: "signed-in", account: account.data };
}
