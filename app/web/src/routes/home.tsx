// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { useId } from "react";
import { HomeTimeline, TimelineSkeleton, timelineQueryOptions } from "../features/timeline";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** The home timeline (FR-201): the signed-in user's posts and those of the accounts they follow. */
export const Route = createFileRoute("/home")({
  staticData: { title: "client:home.title" },
  loader: async ({ context: { queryClient, api, session } }) => {
    // The home timeline needs a session; without one the page offers to sign in instead.
    await session.restore();
    if (session.status !== "signed-in") return;
    // The cached pages are shown as they are; the page refetches them in the background when stale.
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "home" }), staleTime: "static" });
  },
  component: HomePage,
  pendingComponent: HomePending,
});

function HomePage() {
  const t = useT();
  const headingId = useId();
  return (
    <>
      <PageHeading id={headingId}>{t("client:home.title")}</PageHeading>
      <HomeTimeline aria-labelledby={headingId} />
    </>
  );
}

function HomePending() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:home.title")}</PageHeading>
      <TimelineSkeleton />
    </>
  );
}
