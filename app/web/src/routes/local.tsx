// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { useId } from "react";
import { TimelineFeed, TimelineSkeleton, TimelineTabs, timelineQueryOptions } from "../features/timeline";
import { ApiError } from "../lib/errors";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";
import { RouteError } from "./-layout/RouteStates";

/**
 * The local timeline (FR-202): the public posts of this server's accounts, and their local posts for a signed-in
 * user. It is the Timelines screen of the phones' tab bar, with the federated timeline as its other tab (ADR-0039).
 */
export const Route = createFileRoute("/local")({
  // A feed to add to: the shell floats the new-post button over this page (web-client.md G4).
  staticData: { composeButton: true },
  loader: async ({ context: { queryClient, api } }) => {
    // The cached pages are shown as they are; the page refetches them in the background when stale.
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "local" }), staleTime: "static" });
  },
  component: LocalPage,
  pendingComponent: LocalPending,
  errorComponent: LocalError,
});

function LocalPage() {
  const t = useT();
  const headingId = useId();
  return (
    <>
      <PageHeading id={headingId}>{t("client:local.title")}</PageHeading>
      <TimelineTabs current="local">
        <TimelineFeed timeline={{ kind: "local" }} aria-labelledby={headingId} />
      </TimelineTabs>
    </>
  );
}

function LocalPending() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:local.title")}</PageHeading>
      <TimelineTabs current="local">
        <TimelineSkeleton />
      </TimelineTabs>
    </>
  );
}

/**
 * A 404 is the instance setting that turns the local timeline off (api/openapi.yaml, getLocalTimeline), reached
 * from an old link, a bookmark, or the tab bar's Timelines on a server that serves neither public timeline: it
 * says so instead of offering to try again.
 */
function LocalError(props: ErrorComponentProps) {
  const t = useT();
  if (!(props.error instanceof ApiError && props.error.status === 404)) return <RouteError {...props} />;
  return (
    <>
      <PageHeading>{t("client:local.disabled.title")}</PageHeading>
      <p>{t("client:local.disabled.body")}</p>
      <p>
        <Link href="/home" variant="standalone">
          {t("client:shell.not_found.home")}
        </Link>
      </p>
    </>
  );
}
