// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, BoostIcon, ContentWarning, ExternalLinkIcon, Link, MediaGrid, PostCard, PostCardAuthor, PostContent } from "@my/ui";
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
 * A link to the post on its time: muted like the other details of the header (G6) and underlined only on
 * hover and focus, so that a feed is no column of underlines. It is no running text, so no underline is
 * needed to tell it from the words around it. It is at least 24 px high (D4).
 */
const timeLink =
  "inline-flex min-h-6 items-center text-fg-muted no-underline data-hovered:text-fg-muted data-hovered:underline data-focus-visible:underline";

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
      <Link href={view.path} className={timeLink}>
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
        // The same profile as the name, for the pointer: out of the tab order and hidden from screen
        // readers, which reach it by the name (one tab stop per profile). React Aria's Link does not take
        // `aria-hidden`, so an element around it hides it.
        <div aria-hidden="true">
          <Link href={view.author.href} excludeFromTabOrder className="block rounded-full">
            <Avatar src={view.author.avatar} name={view.author.name} decorative />
          </Link>
        </div>
      }
      author={
        // A block, so that the name and the handle inside are cut to the room of the header's row.
        <Link href={view.author.href} className={`${nameLink} block min-w-0`}>
          <PostCardAuthor name={view.author.name} handle={view.author.handle} />
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
