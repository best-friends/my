// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, ContentWarning, Link, MediaGrid, PostCard, PostContent } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import type { Status } from "../api/status";
import { useStatusView } from "../hooks/useStatusView";
import { BoostIcon } from "./icons";
import { StatusActions } from "./StatusActions";

export type StatusCardProps = {
  status: Status;
  /**
   * `focused` for the post a thread page is about: its full date and time instead of the relative one, and
   * no link to the page it is on. `compact` (the default) for timelines and the rest of a thread.
   */
  variant?: "compact" | "focused";
  /** Called after the viewer deleted the post. */
  onDeleted?: () => void;
};

/**
 * One status: the hook's view of it placed into the slots of PostCard, with the action bar. Its time links
 * to the post's page (FR-113).
 */
export function StatusCard({ status, variant = "compact", onDeleted }: StatusCardProps) {
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
  const body = <PostContent html={view.html} lang={view.lang} {...(variant === "focused" && { className: "text-lg" })} />;
  const time =
    variant === "focused" ? (
      <time dateTime={view.time.dateTime}>{view.time.absolute}</time>
    ) : (
      <Link href={view.path} className="text-fg-muted">
        <time dateTime={view.time.dateTime} title={view.time.absolute}>
          {view.time.relative}
        </time>
      </Link>
    );

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
      avatar={<Avatar name={view.author.name} decorative />}
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
      time={time}
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
      actions={<StatusActions status={status} {...(onDeleted !== undefined && { onDeleted })} />}
    />
  );
}
