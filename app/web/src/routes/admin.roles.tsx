// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { RoleSettings } from "../features/admin";
import { accountRolesQueryOptions, rolesQueryOptions } from "../features/moderation";
import { viewerQueryOptions } from "../features/session";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** The roles (FR-720): creating them and changing their positions and permissions, for manage_roles. */
export const Route = createFileRoute("/admin/roles")({
  staticData: { title: "client:admin.roles.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    loadWithPermission(context, "manage_roles", async () => {
      const viewer = await context.queryClient.query(viewerQueryOptions(context.api));
      await Promise.all([
        context.queryClient.query({ ...rolesQueryOptions(context.api), staleTime: Infinity }),
        context.queryClient.query({ ...accountRolesQueryOptions(context.api, viewer.id), staleTime: Infinity }),
      ]);
    }),
  component: RolesPage,
  errorComponent: RolesError,
});

function RolesPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:admin.roles.title")}</SettingsHeading>
      <RoleSettings />
    </>
  );
}

function RolesError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_roles" heading={t("client:admin.roles.title")} />;
}
