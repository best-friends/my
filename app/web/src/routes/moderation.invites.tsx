// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { Invites, invitesQueryOptions, moderationInstanceQueryOptions } from "../features/moderation";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** The invites that let people sign up in the invite registration mode (FR-001): creating and revoking them, for manage_invites. */
export const Route = createFileRoute("/moderation/invites")({
  staticData: { title: "client:admin.invites.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    loadWithPermission(context, "manage_invites", () =>
      Promise.all([
        context.queryClient.infiniteQuery({ ...invitesQueryOptions(context.api), staleTime: Infinity }),
        context.queryClient.query({ ...moderationInstanceQueryOptions(context.api), staleTime: Infinity }),
      ]),
    ),
  component: InvitesPage,
  errorComponent: InvitesError,
});

function InvitesPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:admin.invites.title")}</SettingsHeading>
      <Invites />
    </>
  );
}

function InvitesError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_invites" heading={t("client:admin.invites.title")} />;
}
