// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { BadgeSettings, ProfileForm, badgeChoicesQueryOptions, credentialsQueryOptions } from "../features/settings";
import { SupporterListingSettings, supporterListingQueryOptions } from "../features/supporters";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/**
 * Editing the signed-in user's profile (FR-006), choosing the badges it shows (ADR-0071), and, for a member of a
 * supporter plan, whether to be on the supporter list (ADR-0071 §4).
 */
export const Route = createFileRoute("/settings/profile")({
  staticData: { title: "client:settings.profile.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.query({ ...credentialsQueryOptions(context.api), staleTime: Infinity }),
      context.queryClient.query({ ...badgeChoicesQueryOptions(context.api), staleTime: Infinity }),
      // A user who is not a member of a plan gets a 404, which only leaves the section out.
      context.queryClient.query({ ...supporterListingQueryOptions(context.api), staleTime: Infinity }).catch(() => undefined),
    ]),
  component: ProfilePage,
});

function ProfilePage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:settings.profile.title")}</SettingsHeading>
      <div className="flex flex-col gap-8">
        <ProfileForm />
        <BadgeSettings />
        <SupporterListingSettings />
      </div>
    </>
  );
}
