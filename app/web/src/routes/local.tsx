// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { useId } from "react";
import { TimelineFeed, TimelineSkeleton, timelineQueryOptions } from "../features/timeline";
import { ApiError } from "../lib/errors";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";
import { RouteError } from "./-layout/RouteStates";

/**
 * The local timeline (FR-202): the public posts of this server's accounts, and their local posts for a signed-in
 * user. The navigation offers it, at every width, as an item of its own while the instance serves it.
 */
export const Route = createFileRoute("/local")({
  // A feed to add to: the shell floats the new-post button over this page (web-client.md G4).
  staticData: { title: "client:local.title", hideHeading: true, composeButton: true },
  loader: async ({ context: { queryClient, api } }) => {
    // Cached pages that no write has marked stale are shown at once (and refetched in the background once
    // old); marked or missing ones are fetched and waited for (web-client.md S7).
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "local" }), staleTime: Infinity });
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

/**
 * A 404 is the instance setting that turns the local timeline off (api/openapi.yaml, getLocalTimeline), reached
 * from an old link or bookmark: it says so instead of offering to try again.
 */
function LocalError(props: ErrorComponentProps) {
  const t = useT();
  if (!(props.error instanceof ApiError && props.error.status === 404)) return <RouteError {...props} />;
  return (
    <>
      <PageHeading visuallyHidden={false}>{t("client:local.disabled.title")}</PageHeading>
      <p>{t("client:local.disabled.body")}</p>
      <p>
        <Link href="/home" variant="standalone">
          {t("client:shell.not_found.home")}
        </Link>
      </p>
    </>
  );
}
