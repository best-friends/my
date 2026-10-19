// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { AdminMenu } from "../features/admin";
import { DocumentsSection } from "../features/documents";
import { AccountSection } from "../features/session";
import { SettingsMenu } from "../features/settings";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/**
 * The settings: the links to their pages and, for a viewer with admin permissions, to the admin screens; the
 * signed-in account with signing out; and the links to the server's terms of service and privacy policy.
 */
export const Route = createFileRoute("/settings/")({
  staticData: { title: "client:settings.title" },
  component: SettingsPage,
});

function SettingsPage() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:settings.title")}</PageHeading>
      <SettingsMenu />
      <AdminMenu />
      <AccountSection />
      <DocumentsSection />
    </>
  );
}
