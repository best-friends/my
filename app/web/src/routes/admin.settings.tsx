// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { instanceTextsQueryOptions, ownInstanceQueryOptions, ServerSettings } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** The server's settings: its name in each language, which people see in place of its domain, for manage_settings. */
export const Route = createFileRoute("/admin/settings")({
  staticData: { title: "client:admin.settings.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    loadWithPermission(context, "manage_settings", () =>
      Promise.all([
        context.queryClient.query({ ...instanceTextsQueryOptions(context.api), staleTime: Infinity }),
        context.queryClient.query({ ...ownInstanceQueryOptions(context.api), staleTime: Infinity }),
      ]),
    ),
  component: ServerSettingsPage,
  errorComponent: ServerSettingsError,
});

function ServerSettingsPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:admin.settings.title")}</SettingsHeading>
      <ServerSettings />
    </>
  );
}

function ServerSettingsError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_settings" heading={t("client:admin.settings.title")} />;
}
