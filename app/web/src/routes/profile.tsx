// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, redirect } from "@tanstack/react-router";
import { viewerQueryOptions } from "../features/session";
import { requireSignIn } from "../lib/auth/requireSignIn";

/**
 * The navigation's "Profile": the signed-in user's own profile page (`/@username`), to which it redirects. It needs
 * a session (web-client/10-api-client-and-auth.md §10).
 */
export const Route = createFileRoute("/profile")({
  staticData: { title: "client:profile.title" },
  beforeLoad: requireSignIn,
  loader: async ({ context: { queryClient, api } }) => {
    const viewer = await queryClient.query({ ...viewerQueryOptions(api), staleTime: Infinity });
    // The address bar shows the profile's own address; going back skips this one.
    // eslint-disable-next-line typescript/only-throw-error -- TanStack Router's redirects are thrown objects.
    throw redirect({ to: "/@{$acct}", params: { acct: viewer.acct }, replace: true });
  },
});
