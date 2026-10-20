// SPDX-License-Identifier: AGPL-3.0-only
import { EmptyState, Link, NewItemsButton } from "@my/ui";
import { useQueryClient, useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useIntl } from "react-intl";
import { useApi, useSessionStatus, useSessionUnreachable } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { useIsOffline } from "../../../lib/network";
import { keepHomeTimelineSaved } from "../api/savedHome";
import { timelineQueryOptions } from "../api/timelines";
import { useNewHomeStatuses } from "../hooks/useNewHomeStatuses";
import { TimelineFeed } from "./TimelineFeed";

export type HomeTimelineProps = {
  /** The ID of the page heading that names the timeline. */
  "aria-labelledby": string;
  /**
   * Whether the route loader put the home timeline kept on this browser into the query cache, because the
   * session could not be resumed for the network (a page opened offline, FR-604).
   */
  saved?: boolean;
};

/**
 * The home timeline of the signed-in user, or the way to sign in. The route loads it only for a signed-in user,
 * or, while the server cannot be reached to resume the session, the posts last loaded on this browser.
 */
export function HomeTimeline({ "aria-labelledby": labelledBy, saved = false }: HomeTimelineProps) {
  const t = useT();
  const status = useSessionStatus();
  const unreachable = useSessionUnreachable();
  if (status === "signed-in") return <SignedInHomeTimeline aria-labelledby={labelledBy} />;
  if (saved && unreachable) {
    return (
      <div className="flex flex-col">
        <OfflineNotice />
        <TimelineFeed timeline={{ kind: "home" }} aria-labelledby={labelledBy} />
      </div>
    );
  }
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

/**
 * The feed with the "N new posts" button above it (web-client.md Y7): the stream's new entries wait until
 * the reader presses it. Its region always keeps the button's height above the feed, so neither its
 * appearance nor its count moves the posts being read, and at the top of the page it covers no post.
 * The region sticks to the top of the viewport while the reader scrolls; only the button takes pointer
 * input, so the empty region never blocks the post under it. Pressing it moves focus to the page heading,
 * above the inserted posts, since the button goes away (WCAG 2.4.3).
 * While it is shown, the newest posts are kept on this browser for reading offline (FR-604).
 */
function SignedInHomeTimeline({ "aria-labelledby": labelledBy }: HomeTimelineProps) {
  const t = useT();
  const queryClient = useQueryClient();
  const api = useApi();
  const offline = useIsOffline();
  const { count, show } = useNewHomeStatuses();
  // Synchronizes the browser's storage with the query cache, two stores outside React (web-client.md R8).
  useEffect(() => keepHomeTimelineSaved(queryClient, api), [queryClient, api]);
  return (
    <div className="flex flex-col">
      {offline && <OfflineNotice />}
      <NewItemsButton
        count={count}
        label={t("client:timeline.new_posts", { count })}
        onPress={() => {
          show();
          document.getElementById(labelledBy)?.focus();
        }}
        className="pointer-events-none sticky top-[calc(env(safe-area-inset-top)+0.75rem)] z-10 h-8 items-start pointer-coarse:h-11 *:pointer-events-auto"
      />
      <TimelineFeed timeline={{ kind: "home" }} aria-labelledby={labelledBy} />
    </div>
  );
}

/**
 * Says, above the posts, that they are the ones loaded last and when. The shell's offline bar announces the
 * change itself (web-client.md §9), so this is plain text in the reading order, not another live region.
 */
function OfflineNotice() {
  const t = useT();
  const intl = useIntl();
  const api = useApi();
  const { dataUpdatedAt } = useSuspenseInfiniteQuery(timelineQueryOptions(api, { kind: "home" }));
  const time = intl.formatDate(dataUpdatedAt, { dateStyle: "medium", timeStyle: "short" });
  return <p className="border-b border-border px-4 py-3 text-sm text-fg-muted">{t("client:timeline.offline.home", { time })}</p>;
}
