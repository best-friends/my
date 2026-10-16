// SPDX-License-Identifier: AGPL-3.0-only
import { useState } from "react";
import { useLeaveApp } from "../../../lib/auth/SessionProvider";
import { beginSignIn } from "../../../lib/auth/signIn";
import { useConfig } from "../../../lib/config";

/**
 * Returns `signIn`, which sends the browser to the server's authorization screen with a new PKCE
 * challenge, and whether it is on its way there.
 */
export function useSignIn(returnTo: string): { signIn: () => void; isPending: boolean } {
  const { apiOrigin } = useConfig();
  const leaveApp = useLeaveApp();
  const [isPending, setIsPending] = useState(false);
  const signIn = () => {
    setIsPending(true);
    void beginSignIn({ apiOrigin, appOrigin: window.location.origin, returnTo }).then(leaveApp, (error: unknown) => {
      // Only the Web Crypto API can fail here, which every supported browser has in a secure context.
      console.error(error);
      setIsPending(false);
    });
  };
  return { signIn, isPending };
}
