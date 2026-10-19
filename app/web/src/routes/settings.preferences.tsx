// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { credentialsQueryOptions, PreferencesForm, preferencesQueryOptions } from "../features/settings";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/**
 * The display language, time zone, color theme (kept on this browser), default visibility, timeline languages and, for moderators, the moderator
 * notifications by email (FR-007, FR-208, FR-712). The viewer's permissions decide the last one.
 */
export const Route = createFileRoute("/settings/preferences")({
  staticData: { title: "client:settings.preferences.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.query({ ...preferencesQueryOptions(context.api), staleTime: "static" }),
      context.queryClient.query({ ...credentialsQueryOptions(context.api), staleTime: "static" }),
    ]),
  component: PreferencesPage,
});

function PreferencesPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:settings.preferences.title")}</SettingsHeading>
      <PreferencesForm />
    </>
  );
}
