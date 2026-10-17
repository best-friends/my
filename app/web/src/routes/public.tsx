// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps } from "@tanstack/react-router";
import { Link } from "@my/ui";
import { useId } from "react";
import { TimelineFeed, TimelineSkeleton, timelineQueryOptions } from "../features/timeline";
import { ApiError } from "../lib/errors";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";
import { RouteError } from "./-layout/RouteStates";

/**
 * The federated timeline (FR-202): the public posts of this server's accounts and of the accounts of other
 * servers whose posts this server holds. The navigation offers it only while the instance serves it.
 */
export const Route = createFileRoute("/public")({
  loader: async ({ context: { queryClient, api } }) => {
    // The cached pages are shown as they are; the page refetches them in the background when stale.
    await queryClient.infiniteQuery({ ...timelineQueryOptions(api, { kind: "public" }), staleTime: "static" });
  },
  component: PublicPage,
  pendingComponent: PublicPending,
  errorComponent: PublicError,
});

function PublicPage() {
  const t = useT();
  const headingId = useId();
  return (
    <>
      <PageHeading id={headingId}>{t("client:public.title")}</PageHeading>
      <TimelineFeed timeline={{ kind: "public" }} aria-labelledby={headingId} />
    </>
  );
}

function PublicPending() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:public.title")}</PageHeading>
      <TimelineSkeleton />
    </>
  );
}

/**
 * A 404 is the instance setting that turns the federated timeline off (api/openapi.yaml,
 * getPublicTimeline), reached from an old link or bookmark: it says so instead of offering to try again.
 */
function PublicError(props: ErrorComponentProps) {
  const t = useT();
  if (!(props.error instanceof ApiError && props.error.status === 404)) return <RouteError {...props} />;
  return (
    <>
      <PageHeading>{t("client:public.disabled.title")}</PageHeading>
      <p>{t("client:public.disabled.body")}</p>
      <p>
        <Link href="/home" variant="standalone">
          {t("client:shell.not_found.home")}
        </Link>
      </p>
    </>
  );
}
