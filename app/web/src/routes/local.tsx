// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { useId } from "react";
import { TimelineFeed, TimelineSkeleton, timelineQueryOptions } from "../features/timeline";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** The local timeline (FR-202): the public posts of this server's accounts, and their local posts for a signed-in user. */
export const Route = createFileRoute("/local")({
  // A feed to add to: the shell floats the new-post button over this page (web-client.md G4).
  staticData: { title: "client:local.title", composeButton: true },
  loader: async ({ context: { queryClient, api } }) => {
    // The cached pages are shown as they are; the page refetches them in the background when stale.
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "local" }), staleTime: "static" });
  },
  component: LocalPage,
  pendingComponent: LocalPending,
});

function LocalPage() {
  const t = useT();
  const headingId = useId();
  return (
    <>
      <PageHeading id={headingId}>{t("client:local.title")}</PageHeading>
      <TimelineFeed timeline={{ kind: "local" }} aria-labelledby={headingId} />
    </>
  );
}

function LocalPending() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:local.title")}</PageHeading>
      <TimelineSkeleton />
    </>
  );
}
