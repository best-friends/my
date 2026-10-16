// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, ContentWarning, MediaGrid, PostCard, PostContent } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import type { Status } from "../api/timelines";
import { useStatusView } from "../hooks/useStatusView";

export type StatusCardProps = { status: Status };

/** One status of a timeline: the hook's view of it placed into the slots of PostCard. */
export function StatusCard({ status }: StatusCardProps) {
  const t = useT();
  const intl = useIntl();
  const view = useStatusView(status);

  const media =
    view.media.length > 0 ? (
      <MediaGrid
        items={view.media}
        sensitive={view.sensitive}
        noDescriptionLabel={t("client:timeline.media.no_description")}
        revealLabel={t("client:timeline.media.reveal")}
      />
    ) : undefined;
  const body = <PostContent html={view.html} lang={view.lang} />;

  return (
    <PostCard
      {...(view.boostedBy !== null && {
        context: (
          <>
            <BoostIcon />
            <span>
              {/* The booster's name is the user's text, isolated from the sentence around it (Y10). */}
              {intl.formatMessage({ id: "client:timeline.boosted_by" }, { name: <bdi key="name">{view.boostedBy}</bdi> })}
            </span>
          </>
        ),
      })}
      avatar={<Avatar src={view.author.avatar} name={view.author.name} decorative />}
      author={
        <>
          <span dir="auto" className="font-semibold">
            {view.author.name}
          </span>{" "}
          <span dir="ltr" className="text-sm text-fg-muted">
            {view.author.handle}
          </span>
        </>
      }
      time={
        <time dateTime={view.time.dateTime} title={view.time.absolute}>
          {view.time.relative}
        </time>
      }
      content={
        view.contentWarning === null ? (
          body
        ) : (
          <ContentWarning summary={view.contentWarning} summaryLang={view.lang} toggleLabel={t("client:timeline.content_warning.toggle")}>
            {body}
            {media}
          </ContentWarning>
        )
      }
      {...(view.contentWarning === null && media !== undefined && { media })}
    />
  );
}

function BoostIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M17 2l4 4-4 4M3 11V9a3 3 0 0 1 3-3h15M7 22l-4-4 4-4M21 13v2a3 3 0 0 1-3 3H3" />
    </svg>
  );
}
