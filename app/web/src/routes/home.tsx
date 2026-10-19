// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { useId } from "react";
import { HomeTimeline, joinWaitingHomeStatuses, TimelineSkeleton, timelineQueryOptions } from "../features/timeline";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** The home timeline (FR-201): the signed-in user's posts and those of the accounts they follow. */
export const Route = createFileRoute("/home")({
  // A feed to add to: the shell floats the new-post button over this page (web-client.md G4).
  staticData: { title: "client:home.title", composeButton: true },
  loader: async ({ context: { queryClient, api, session } }) => {
    // The home timeline needs a session; without one the page offers to sign in instead.
    await session.restore();
    if (session.status !== "signed-in") return;
    // Cached pages that no write has marked stale are shown at once (and refetched in the background once
    // old); marked or missing ones are fetched and waited for (web-client.md S7).
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "home" }), staleTime: Infinity });
    // Opening the timeline from another page: the news that waited for the reader are on it from the first render.
    joinWaitingHomeStatuses(queryClient, api);
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
