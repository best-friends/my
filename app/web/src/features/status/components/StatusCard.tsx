// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, BoostIcon, ContentWarning, ExternalLinkIcon, Link, MediaGrid, PostCard, PostCardAuthor, PostContent } from "@my/ui";
import type { Ref } from "react";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import type { Status } from "../api/status";
import { useStatusView } from "../hooks/useStatusView";
import { StatusActions } from "./StatusActions";

export type StatusCardProps = {
  status: Status;
  /**
   * `focused` for the post a thread page is about: highlighted and named apart from the rest, its body
   * larger, its full date and its counts in words under the body, and no link to the page it is on.
   * `compact` (the default) for timelines and the rest of a thread.
   */
  variant?: "compact" | "focused";
  /** Whether the post replies to the post above it in a thread, and whether the post below replies to it. */
  thread?: { lineAbove: boolean; lineBelow: boolean };
  /** Called after the viewer deleted the post. */
  onDeleted?: () => void;
  /**
   * Whether the card has the viewer's actions on the post (reply, boost, favourite, the menu); true by default.
   * A moderator's page about the post (a report) shows it without them, so that nothing there acts on it by mistake.
   */
  withActions?: boolean;
  /** The card's article, for the page that scrolls it into view. */
  ref?: Ref<HTMLElement>;
  /** Whether the card is one of the pinned posts at the top of a profile, which a line above it says (FR-403). */
  pinned?: boolean;
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
export function StatusCard({ status, variant = "compact", thread, onDeleted, withActions = true, ref, pinned = false }: StatusCardProps) {
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
  const body = <PostContent html={view.html} lang={view.lang} onLinkClick={view.onLinkClick} {...(variant === "focused" && { className: "text-lg" })} />;
  const time =
    variant === "focused" ? (
      // The relative time as everywhere else (G8); the full date is in the details.
      <time dateTime={view.time.dateTime}>{view.time.relative}</time>
    ) : (
      <Link href={view.path} className={timeLink}>
        <time dateTime={view.time.dateTime} title={view.time.absolute}>
          {view.time.relative}
        </time>
      </Link>
    );

  return (
    <PostCard
      ref={ref}
      {...(thread !== undefined && { lineAbove: thread.lineAbove, lineBelow: thread.lineBelow })}
      {...(variant === "focused" && {
        label: t("client:status.focused.label"),
        highlighted: true,
        details: (
          <>
            <time dateTime={view.time.dateTime}>{view.time.absolute}</time>
            {view.counts.boosts > 0 && <span className="tabular-nums">{t("client:status.focused.boosts", { count: view.counts.boosts })}</span>}
            {view.counts.favourites > 0 && (
              <span className="tabular-nums">{t("client:status.focused.favourites", { count: view.counts.favourites })}</span>
            )}
          </>
        ),
      })}
      {...(pinned && {
        // Part of the article's name, so that a reader of the feed hears it before the author.
        context: <span>{t("client:status.pin.label")}</span>,
      })}
      {...(!pinned && view.boostedBy !== null && {
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
      {...((withActions || view.original !== null) && {
        actions: (
          <>
            {withActions && <StatusActions status={status} {...(onDeleted !== undefined && { onDeleted })} />}
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
        ),
      })}
    />
  );
}
