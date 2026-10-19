// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { z } from "zod";
import { type SignupBlockFilter, signupBlockFilters, SignupBlocks, signupBlocksQueryOptions } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** `filter`: the tab of the list; omitted, or anything unknown, shows the blocks in effect (web-client.md S3). */
const searchSchema = z.object({
  filter: z.enum(signupBlockFilters).exclude(["active"]).optional().catch(undefined),
});

/** The blocks on sign-ups by email domain, email address or IP range, and adding and lifting them (FR-719), for manage_accounts. */
export const Route = createFileRoute("/admin/signup-blocks")({
  validateSearch: searchSchema,
  loaderDeps: ({ search: { filter } }): { filter: SignupBlockFilter } => ({ filter: filter ?? "active" }),
  staticData: { title: "client:admin.signup_blocks.title" },
  beforeLoad: requireSignIn,
  loader: ({ context, deps }) =>
    loadWithPermission(context, "manage_accounts", () => context.queryClient.infiniteQuery({ ...signupBlocksQueryOptions(context.api, deps.filter), staleTime: "static" })),
  component: SignupBlocksPage,
  errorComponent: SignupBlocksError,
});

function SignupBlocksPage() {
  const t = useT();
  const { filter } = Route.useLoaderDeps();
  const navigate = Route.useNavigate();
  const changeFilter = (next: SignupBlockFilter) => {
    void navigate({ search: (previous) => ({ ...previous, filter: next === "active" ? undefined : next }) });
  };
  return (
    <>
      <SettingsHeading>{t("client:admin.signup_blocks.title")}</SettingsHeading>
      <SignupBlocks filter={filter} onFilterChange={changeFilter} />
    </>
  );
}

function SignupBlocksError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_accounts" heading={t("client:admin.signup_blocks.title")} />;
}
