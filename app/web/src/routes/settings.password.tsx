// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { PasswordSettings } from "../features/settings";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** Changing the password (FR-002). */
export const Route = createFileRoute("/settings/password")({
  staticData: { title: "client:settings.password.title" },
  beforeLoad: requireSignIn,
  component: PasswordPage,
});

function PasswordPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:settings.password.title")}</SettingsHeading>
      <PasswordSettings />
    </>
  );
}
