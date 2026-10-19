// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { useId } from "react";
import { z } from "zod";
import { type NotificationFilter, notificationFilters, notificationMarkerQueryOptions, notificationsQueryOptions, NotificationsSkeleton, NotificationsView } from "../features/notifications";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** `filter`: the type filter of the list; omitted, or anything unknown, shows every type (web-client.md S3). */
const searchSchema = z.object({
  filter: z.enum(notificationFilters).exclude(["all"]).optional().catch(undefined),
});

/** The signed-in user's notifications (FR-205). */
export const Route = createFileRoute("/notifications")({
  staticData: { title: "client:notifications.title" },
  validateSearch: searchSchema,
  loaderDeps: ({ search: { filter } }): { filter: NotificationFilter } => ({ filter: filter ?? "all" }),
  // The first page and the read marker load with the page; without a session there is nothing to load,
  // and the page offers to sign in. A failure (offline, say) is the list's own, shown below the page's
  // heading with what to do (useNotifications), so the loader lets the page open.
  loader: async ({ context: { queryClient, api, session }, deps: { filter } }) => {
    if (session.status === "unknown") await session.restore();
    if (session.status !== "signed-in") return;
    await Promise.allSettled([
      queryClient.infiniteQuery({ ...notificationsQueryOptions(api, filter), staleTime: Infinity }),
      queryClient.query({ ...notificationMarkerQueryOptions(api), staleTime: Infinity }),
    ]);
  },
  component: NotificationsPage,
  pendingComponent: NotificationsPending,
});

function NotificationsPage() {
  const t = useT();
  const headingId = useId();
  const { filter } = Route.useLoaderDeps();
  const navigate = Route.useNavigate();
  const changeFilter = (next: NotificationFilter) => {
    void navigate({ search: next === "all" ? {} : { filter: next } });
  };
  return (
    <>
      <PageHeading id={headingId}>{t("client:notifications.title")}</PageHeading>
      <NotificationsView filter={filter} onFilterChange={changeFilter} headingId={headingId} />
    </>
  );
}

function NotificationsPending() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:notifications.title")}</PageHeading>
      <NotificationsSkeleton />
    </>
  );
}
