// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { AccountStatus, ownActionsQueryOptions } from "../features/moderation";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { SettingsHeading } from "./-layout/SettingsHeading";

/**
 * The account status (FR-715): the moderation actions in effect on the user's account and the past ones, and
 * appealing them (FR-707). A `moderation_notice` notification leads here.
 */
export const Route = createFileRoute("/settings/account-status")({
  staticData: { title: "client:account_status.title" },
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.infiniteQuery({ ...ownActionsQueryOptions(context.api, "active"), staleTime: "static" }),
      context.queryClient.infiniteQuery({ ...ownActionsQueryOptions(context.api, "all"), staleTime: "static" }),
    ]),
  component: AccountStatusPage,
});

function AccountStatusPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:account_status.title")}</SettingsHeading>
      <AccountStatus />
    </>
  );
}
