// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, ContentWarning, EmojiText, Link, MediaGrid, PostCardAuthor, PostContent, QuoteCard } from "@my/ui";
import { useT } from "../../../lib/i18n";
import type { Status, StatusQuote } from "../api/status";
import { useMediaViewerLabels } from "../hooks/useMediaViewerLabels";
import { useQuoteView } from "../hooks/useQuoteView";
import { useStatusView } from "../hooks/useStatusView";

/** A link to a profile on a name: in the color of the text, underlined on hover, as on the post's own card. */
const nameLink = "text-fg no-underline data-hovered:text-fg data-hovered:underline";

/** The link to the quoted post on its time: muted like the details of a header, underlined only on hover and focus. */
const timeLink = "text-fg-muted no-underline data-hovered:text-fg-muted data-hovered:underline data-focus-visible:underline";

/**
 * The post that a post quotes (FR-114, ADR-0069 §5), inside the quoting post: the quoted author, the time as the
 * link to the quoted post, and its body and media, or only the notice of why it is not shown. Quotes are one level
 * deep: the quoted post shows no quote of its own.
 */
export function QuotedStatus({ quote }: { quote: StatusQuote }) {
  const t = useT();
  const view = useQuoteView(quote);
  if (view.kind === "notice") return <QuoteCard label={t("client:status.quote.label")} notice={view.notice} />;
  return <QuotedPost status={view.status} notice={view.notice} />;
}

function QuotedPost({ status, notice }: { status: Status; notice: string | null }) {
  const t = useT();
  const view = useStatusView(status);
  const viewerLabels = useMediaViewerLabels();
  const body = (
    <>
      <PostContent html={view.html} lang={view.lang} emojis={view.emojis} onLinkClick={view.onLinkClick} />
      {view.media.length > 0 && (
        <MediaGrid
          items={view.media}
          sensitive={view.sensitive}
          noDescriptionLabel={t("client:timeline.media.no_description")}
          revealLabel={t("client:timeline.media.reveal")}
          viewerLabels={viewerLabels}
        />
      )}
    </>
  );
  return (
    <QuoteCard
      label={t("client:status.quote.label")}
      avatar={<Avatar sources={view.author.avatar} name={view.author.name} size="xs" decorative />}
      author={
        <Link href={view.author.href} className={`${nameLink} block min-w-0`}>
          <PostCardAuthor name={<EmojiText text={view.author.name} emojis={view.author.emojis} />} handle={view.author.handle} />
        </Link>
      }
      time={
        <Link href={view.path} className={timeLink}>
          <time dateTime={view.time.dateTime} title={view.time.absolute}>
            {view.time.relative}
          </time>
        </Link>
      }
      {...(notice !== null && { notice, revealLabel: t("client:status.quote.reveal") })}
    >
      {view.contentWarning === null ? (
        body
      ) : (
        <ContentWarning summary={<EmojiText text={view.contentWarning} emojis={view.emojis} />} summaryLang={view.lang} toggleLabel={t("client:timeline.content_warning.toggle")}>
          {body}
        </ContentWarning>
      )}
    </QuoteCard>
  );
}
