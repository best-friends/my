// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, BoostIcon, ContentWarning, ExternalLinkIcon, Link, MediaGrid, PostCard, PostContent } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import type { Status } from "../api/status";
import { useStatusView } from "../hooks/useStatusView";
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

/** A link to a profile on a name: in the color of the text, underlined on hover, since it is no running text. */
const nameLink = "text-fg no-underline data-hovered:text-fg data-hovered:underline";

/**
 * One status: the hook's view of it placed into the slots of PostCard, with the action bar. Its names link
 * to the profiles and its time to the post's page (FR-113).
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
              {intl.formatMessage(
                { id: "client:timeline.boosted_by" },
                {
                  name: (
                    <Link key="name" href={view.boostedBy.href} className={nameLink}>
                      <bdi>{view.boostedBy.name}</bdi>
                    </Link>
                  ),
                },
              )}
            </span>
          </>
        ),
      })}
      avatar={
        // React Aria's Link cannot leave the tab order, so the picture is a link of its own to the same
        // profile as the name, and named by it.
        <Link href={view.author.href} aria-label={view.author.name} className="block rounded-full">
          <Avatar src={view.author.avatar} name={view.author.name} decorative />
        </Link>
      }
      author={
        <Link href={view.author.href} className={nameLink}>
          <>
            <span dir="auto" className="font-semibold">
              {view.author.name}
            </span>{" "}
            <span dir="ltr" className="text-sm text-fg-muted">
              {view.author.handle}
            </span>
          </>
        </Link>
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
      actions={
        <>
          <StatusActions status={status} {...(onDeleted !== undefined && { onDeleted })} />
          {view.original !== null && (
            // The post's page on its own server, shown also while nobody is signed in. The host is what it
            // shows; the name says where it leads, and contains the host (WCAG 2.5.3, label in name).
            <Link
              href={view.original.href}
              rel="noreferrer"
              variant="standalone"
              aria-label={t("client:status.original", { host: view.original.host })}
              className="ms-auto min-w-0 gap-1 text-sm font-normal text-fg-muted data-hovered:text-fg"
            >
              <span dir="ltr" className="min-w-0 truncate">
                {view.original.host}
              </span>
              <ExternalLinkIcon />
            </Link>
          )}
        </>
      }
    />
  );
}
