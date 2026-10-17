// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { DeadJobList, deadJobsQueryOptions } from "../features/admin";
import { requireSignIn } from "../lib/auth/requireSignIn";
import { useT } from "../lib/i18n";
import { AdminError, loadWithPermission } from "./-admin/adminRoute";
import { SettingsHeading } from "./-layout/SettingsHeading";

/** The jobs of the job queue that failed for good, and running them again, for manage_jobs. */
export const Route = createFileRoute("/admin/dead-jobs")({
  beforeLoad: requireSignIn,
  loader: ({ context }) =>
    loadWithPermission(context, "manage_jobs", () => context.queryClient.infiniteQuery({ ...deadJobsQueryOptions(context.api), staleTime: "static" })),
  component: DeadJobsPage,
  errorComponent: DeadJobsError,
});

function DeadJobsPage() {
  const t = useT();
  return (
    <>
      <SettingsHeading>{t("client:admin.dead_jobs.title")}</SettingsHeading>
      <DeadJobList />
    </>
  );
}

function DeadJobsError(props: ErrorComponentProps) {
  const t = useT();
  return <AdminError {...props} permission="manage_jobs" heading={t("client:admin.dead_jobs.title")} />;
}
