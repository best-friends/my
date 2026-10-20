// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { ModerationPages } from "../features/moderation";
import { viewerQueryOptions } from "../features/session";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/**
 * The moderation page that the navigation's Moderation entry opens (ADR-0058): the links to the moderation
 * screens that the viewer's roles allow, with what awaits the moderators on them (FR-712). The loader reads the
 * viewer's account, whose permissions decide the links.
 */
export const Route = createFileRoute("/moderation/")({
  staticData: { title: "client:admin.moderation.title" },
  beforeLoad: requireSignIn,
  loader: async ({ context: { queryClient, api } }) => {
    await queryClient.query({ ...viewerQueryOptions(api), staleTime: Infinity });
  },
  component: ModerationPage,
});

function ModerationPage() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:admin.moderation.title")}</PageHeading>
      <ModerationPages />
    </>
  );
}
