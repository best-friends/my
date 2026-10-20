// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { z } from "zod";
import { adminCustomEmojisQueryOptions, type CustomEmojiFilter, CustomEmojiSettings } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/**
 * `origin`: the tab of the list, the emoji of other servers; omitted, or anything unknown, shows this server's.
 * `shortcode` and `domain`: the search (web-client.md S3); the domain counts for other servers only.
 */
const searchSchema = z.object({
  origin: z.literal("remote").optional().catch(undefined),
  shortcode: z.string().trim().min(1).max(128).optional().catch(undefined),
  domain: z.string().trim().min(1).max(253).optional().catch(undefined),
});

/** The custom emoji (ADR-0051): registering this server's and managing those of other servers, for manage_custom_emojis. */
export const Route = createFileRoute("/admin/custom-emojis")({
  validateSearch: searchSchema,
  loaderDeps: ({ search }): { filter: CustomEmojiFilter } => ({
    filter: {
      origin: search.origin ?? "local",
      shortcode: search.shortcode,
      domain: search.origin === "remote" ? search.domain : undefined,
    },
  }),
  staticData: { title: "client:admin.custom_emojis.title" },
  beforeLoad: requireSignIn,
  loader: ({ context, deps }) =>
    loadWithPermission(context, "manage_custom_emojis", () =>
      context.queryClient.infiniteQuery({ ...adminCustomEmojisQueryOptions(context.api, deps.filter), staleTime: Infinity }),
    ),
  component: CustomEmojisPage,
  errorComponent: CustomEmojisError,
});

function CustomEmojisPage() {
  const t = useT();
  const { filter } = Route.useLoaderDeps();
  const navigate = Route.useNavigate();
  const changeFilter = (next: CustomEmojiFilter) => {
    void navigate({ search: { origin: next.origin === "remote" ? "remote" : undefined, shortcode: next.shortcode, domain: next.domain } });
  };
  return (
    <>
      <SettingsHeading>{t("client:admin.custom_emojis.title")}</SettingsHeading>
      <CustomEmojiSettings filter={filter} onFilterChange={changeFilter} />
    </>
  );
}

function CustomEmojisError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_custom_emojis" heading={t("client:admin.custom_emojis.title")} />;
}
