// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { adminAnnouncementsQueryOptions, AnnouncementSettings } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** The server's announcements to every local user (FR-726): writing, publishing, editing and withdrawing them, for manage_announcements. */
export const Route = createFileRoute("/admin/announcements")({
  staticData: { title: "client:admin.announcements.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    loadWithPermission(context, "manage_announcements", () =>
      context.queryClient.query({ ...adminAnnouncementsQueryOptions(context.api), staleTime: Infinity }),
    ),
  component: AnnouncementsPage,
  errorComponent: AnnouncementsError,
});

function AnnouncementsPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:admin.announcements.title")}</SettingsHeading>
      <AnnouncementSettings />
    </>
  );
}

function AnnouncementsError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_announcements" heading={t("client:admin.announcements.title")} />;
}
