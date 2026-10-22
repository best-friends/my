// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { AdminMenu } from "../features/admin";
import { DocumentsSection } from "../features/documents";
import { ModerationMenu } from "../features/moderation";
import { AccountSection } from "../features/session";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/**
 * The settings: the links to their pages and, for a viewer with staff permissions, to the moderation screens and
 * to the server administration screens (ADR-0058); the
 * signed-in account with signing out; and the links to the server's terms of service and privacy policy.
 */
export const Route = createFileRoute("/settings/")({
  staticData: { title: "client:settings.title", hideHeading: true },
  // Every setting is the user's own, so the page needs a session (web-client.md §10).
  beforeLoad: requireSignIn,
  component: SettingsPage,
});

function SettingsPage() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:settings.title")}</PageHeading>
      <ModerationMenu />
      <AdminMenu />
      <AccountSection />
      <DocumentsSection />
    </>
  );
}
