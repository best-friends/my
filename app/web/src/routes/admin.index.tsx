// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { AdminPages } from "../features/admin";
import { viewerQueryOptions } from "../features/session";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/**
 * The server administration page that the navigation's Administration entry opens (ADR-0058): the links to the
 * server administration screens that the viewer's roles allow. The loader reads the viewer's account, whose
 * permissions decide the links.
 */
export const Route = createFileRoute("/admin/")({
  staticData: { title: "client:admin.menu.title" },
  beforeLoad: requireSignIn,
  loader: async ({ context: { queryClient, api } }) => {
    await queryClient.query({ ...viewerQueryOptions(api), staleTime: Infinity });
  },
  component: AdminPage,
});

function AdminPage() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:admin.menu.title")}</PageHeading>
      <AdminPages />
    </>
  );
}
