// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { z } from "zod";
import { DocumentSettings, instanceTextsQueryOptions } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/**
 * `locale`: the language whose texts the screen edits, a BCP 47 tag (web-client.md S3); anything else is dropped,
 * and the screen starts in the display language.
 */
const searchSchema = z.object({
  locale: z
    .string()
    .regex(/^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{1,8})*$/)
    .max(64)
    .optional()
    .catch(undefined),
});

/** The terms of service and the privacy policy that the server publishes (#489), for manage_settings. */
export const Route = createFileRoute("/admin/documents")({
  staticData: { title: "client:admin.documents.title" },
  validateSearch: searchSchema,
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    loadWithPermission(context, "manage_settings", () => context.queryClient.query({ ...instanceTextsQueryOptions(context.api), staleTime: Infinity })),
  component: DocumentsPage,
  errorComponent: DocumentsError,
});

function DocumentsPage() {
  const t = useT();
  const { locale } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <>
      <SettingsHeading>{t("client:admin.documents.title")}</SettingsHeading>
      <DocumentSettings
        locale={locale}
        onLocaleChange={(next) => {
          void navigate({ search: { locale: next } });
        }}
      />
    </>
  );
}

function DocumentsError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_settings" heading={t("client:admin.documents.title")} />;
}
