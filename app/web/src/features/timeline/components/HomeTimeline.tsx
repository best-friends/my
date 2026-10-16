// SPDX-License-Identifier: AGPL-3.0-only
import { EmptyState, Link } from "@my/ui";
import { useSessionStatus } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { TimelineFeed } from "./TimelineFeed";

export type HomeTimelineProps = {
  /** The ID of the page heading that names the timeline. */
  "aria-labelledby": string;
};

/** The home timeline of the signed-in user, or the way to sign in. The route loads it only for a signed-in user. */
export function HomeTimeline({ "aria-labelledby": labelledBy }: HomeTimelineProps) {
  const t = useT();
  const status = useSessionStatus();
  if (status !== "signed-in") {
    return (
      <EmptyState
        heading={t("client:timeline.signed_out.heading")}
        description={t("client:timeline.signed_out.description")}
        action={
          <Link href={`/login?${new URLSearchParams({ return_to: "/home" }).toString()}`} variant="standalone">
            {t("client:timeline.signed_out.action")}
          </Link>
        }
      />
    );
  }
  return <TimelineFeed timeline={{ kind: "home" }} aria-labelledby={labelledBy} />;
}
