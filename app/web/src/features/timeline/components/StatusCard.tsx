// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, ContentWarning, Link, MediaGrid, PostCard, PostContent } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import type { Status } from "../api/timelines";
import { useStatusView } from "../hooks/useStatusView";

export type StatusCardProps = { status: Status };

/** A link to a profile on a name: in the color of the text, underlined on hover, since it is no running text. */
const nameLink = "text-fg no-underline data-hovered:text-fg data-hovered:underline";

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
          <Avatar name={view.author.name} decorative />
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
