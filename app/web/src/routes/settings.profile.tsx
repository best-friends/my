// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { ProfileForm, credentialsQueryOptions } from "../features/settings";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** Editing the signed-in user's profile (FR-006). */
export const Route = createFileRoute("/settings/profile")({
  beforeLoad: requireSignIn,
  loader: ({ context }) => context.queryClient.query({ ...credentialsQueryOptions(context.api), staleTime: "static" }),
  component: ProfilePage,
});

function ProfilePage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:settings.profile.title")}</SettingsHeading>
      <ProfileForm />
    </>
  );
}
