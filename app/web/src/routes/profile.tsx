// SPDX-License-Identifier: AGPL-3.0-only
import { EmptyState, Link } from "@my/ui";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { viewerQueryOptions } from "../features/session";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/**
 * The navigation's "Profile": the signed-in user's own profile page (`/@username`), to which it
 * redirects; without a session it offers to sign in.
 */
export const Route = createFileRoute("/profile")({
  staticData: { title: "client:profile.title" },
  loader: async ({ context: { queryClient, api, session } }) => {
    await session.restore();
    if (session.status !== "signed-in") return;
    const viewer = await queryClient.query({ ...viewerQueryOptions(api), staleTime: Infinity });
    // The address bar shows the profile's own address; going back skips this one.
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router's redirects are thrown objects.
    throw redirect({ to: "/@{$acct}", params: { acct: viewer.acct }, replace: true });
  },
  component: ProfilePage,
});

function ProfilePage() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:profile.title")}</PageHeading>
      <EmptyState
        heading={t("client:timeline.signed_out.heading")}
        description={t("client:profile.signed_out.description")}
        action={
          <Link href={`/login?${new URLSearchParams({ return_to: "/profile" }).toString()}`} variant="standalone">
            {t("client:timeline.signed_out.action")}
          </Link>
        }
      />
    </>
  );
}
