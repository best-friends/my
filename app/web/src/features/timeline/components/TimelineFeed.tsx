// SPDX-License-Identifier: AGPL-3.0-only
import { Button, EmptyState, Feed, Link, VisuallyHidden } from "@my/ui";
import { useT } from "../../../lib/i18n";
import type { Timeline } from "../api/timelines";
import { useTimeline } from "../hooks/useTimeline";
import { StatusCard } from "../../status";

export type TimelineFeedProps = {
  timeline: Timeline;
  /** The ID of the page heading that names the timeline. */
  "aria-labelledby": string;
};

/**
 * A timeline as a feed (web-client.md Y6) with its older pages loaded as the reader nears the end, or an
 * empty state. The first page comes from the route loader; render it inside the route that loads it.
 */
export function TimelineFeed({ timeline, "aria-labelledby": labelledBy }: TimelineFeedProps) {
  const t = useT();
  const { statuses, hasMore, isLoadingMore, loadMore, loadMoreError } = useTimeline(timeline);

  if (statuses.length === 0 && !hasMore) return <TimelineEmpty timeline={timeline} />;

  return (
    <div className="flex flex-col">
      {/* The feed role has no keyboard convention, so its keys are told before it (Feed's documentation). */}
      <VisuallyHidden>
        <p>{t("client:timeline.keyboard_hint")}</p>
      </VisuallyHidden>
      <Feed
        aria-labelledby={labelledBy}
        items={statuses}
        getKey={(status) => status.id}
        renderItem={(status) => <StatusCard status={status} />}
        isLoading={isLoadingMore}
        // After a failure the feed stops loading by itself; the retry button below asks again.
        hasMore={hasMore && loadMoreError === null}
        {...(hasMore && { setSize: -1 })}
        onLoadMore={loadMore}
        loadMoreLabel={t("client:timeline.load_more")}
        loadingLabel={t("client:timeline.loading")}
      />
      {loadMoreError !== null && (
        <div className="flex flex-col items-center gap-3 p-4 text-center">
          {/* Announced by useTimeline when it fails; shown here for everyone. */}
          <p>{t("client:timeline.load_more.failed", { reason: loadMoreError })}</p>
          <Button onPress={loadMore}>{t("client:timeline.retry")}</Button>
        </div>
      )}
    </div>
  );
}

function TimelineEmpty({ timeline }: { timeline: Timeline }) {
  const t = useT();
  switch (timeline.kind) {
    case "home":
      return (
        <EmptyState
          heading={t("client:timeline.empty.home.heading")}
          description={t("client:timeline.empty.home.description")}
          action={
            <Link href="/local" variant="standalone">
              {t("client:timeline.empty.home.action")}
            </Link>
          }
        />
      );
    case "local":
      return <EmptyState heading={t("client:timeline.empty.local.heading")} description={t("client:timeline.empty.local.description")} />;
    case "tag":
      return (
        <EmptyState
          heading={t("client:timeline.empty.tag.heading", { tag: timeline.tag })}
          description={t("client:timeline.empty.tag.description", { tag: timeline.tag })}
        />
      );
  }
}
