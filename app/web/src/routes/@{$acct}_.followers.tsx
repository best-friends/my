// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { followListQueryOptions, FollowListView } from "../features/profile";
import { PageHeading } from "./-layout/PageHeading";
import { loadAccount, profileParamsSchema } from "./-profile/loadAccount";
import { ProfileNotFound } from "./-profile/ProfileNotFound";

/** The accounts that follow an account (FR-301). */
export const Route = createFileRoute("/@{$acct}_/followers")({
  params: { parse: (params) => profileParamsSchema.parse(params) },
  loader: async ({ context, params: { acct } }) => {
    const account = await loadAccount(context, acct);
    await context.queryClient.infiniteQuery({ ...followListQueryOptions(context.api, account.id, "followers"), staleTime: "static" });
  },
  component: FollowersPage,
  notFoundComponent: ProfileNotFound,
});

function FollowersPage() {
  const { acct } = Route.useParams();
  return <FollowListView key={acct} acct={acct} list="followers" renderHeading={(text) => <PageHeading>{text}</PageHeading>} />;
}
