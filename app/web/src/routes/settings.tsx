// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { AccountSection } from "../features/session";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** The settings: for now, the signed-in account and signing out. */
export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});

function SettingsPage() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:settings.title")}</PageHeading>
      <AccountSection />
    </>
  );
}
