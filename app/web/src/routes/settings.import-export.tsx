// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { dataImportsQueryOptions, ImportExport } from "../features/settings";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** The follows, blocks, mutes, and bookmarks as Mastodon's CSV files (FR-008): exporting them and importing them. */
export const Route = createFileRoute("/settings/import-export")({
  staticData: { title: "client:settings.import_export.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    context.queryClient.query({ ...dataImportsQueryOptions(context.api, context.queryClient), staleTime: Infinity }),
  component: ImportExportPage,
});

function ImportExportPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:settings.import_export.title")}</SettingsHeading>
      <ImportExport />
    </>
  );
}
