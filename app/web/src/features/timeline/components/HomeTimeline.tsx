// SPDX-License-Identifier: AGPL-3.0-only
import { EmptyState, Link, NewItemsButton } from "@my/ui";
import { useSessionStatus } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { useNewHomeStatuses } from "../hooks/useNewHomeStatuses";
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
  return <SignedInHomeTimeline aria-labelledby={labelledBy} />;
}

/**
 * The feed with the "N new posts" button above it (web-client.md Y7): the stream's new entries wait until
 * the reader presses it. The button floats over the top of the feed in a region of no height, so neither
 * its appearance nor its count moves the posts being read. Pressing it moves focus to the page heading,
 * above the inserted posts, since the button goes away (WCAG 2.4.3).
 */
function SignedInHomeTimeline({ "aria-labelledby": labelledBy }: HomeTimelineProps) {
  const t = useT();
  const { count, show } = useNewHomeStatuses();
  return (
    <div className="flex flex-col">
      <NewItemsButton
        count={count}
        label={t("client:timeline.new_posts", { count })}
        onPress={() => {
          show();
          document.getElementById(labelledBy)?.focus();
        }}
        className="sticky top-[calc(env(safe-area-inset-top)+0.75rem)] z-10 h-0 items-start"
      />
      <TimelineFeed timeline={{ kind: "home" }} aria-labelledby={labelledBy} />
    </div>
  );
}
