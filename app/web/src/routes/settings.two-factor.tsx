// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { TwoFactorSettings, twoFactorQueryOptions } from "../features/settings";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** Two-factor authentication with an authenticator app (FR-005): enrolling it, the recovery codes, and turning it off. */
export const Route = createFileRoute("/settings/two-factor")({
  staticData: { title: "client:settings.two_factor.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) => context.queryClient.query({ ...twoFactorQueryOptions(context.api), staleTime: Infinity }),
  component: TwoFactorPage,
});

function TwoFactorPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:settings.two_factor.title")}</SettingsHeading>
      <TwoFactorSettings />
    </>
  );
}
