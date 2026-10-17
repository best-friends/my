// SPDX-License-Identifier: AGPL-3.0-only
import { type ReactNode, useId } from "react";
import { tv } from "tailwind-variants";
import { FeedArticle, type FeedArticleProps } from "./Feed";

const postCard = tv({
  slots: {
    // The card is the size container its layout answers to (web-client.md D2).
    base: "@container border-b border-border bg-surface px-4 py-3",
    context: "mb-2 flex min-w-0 items-center gap-2 text-sm wrap-anywhere text-fg-muted [&>svg]:size-4 [&>svg]:shrink-0",
    grid: "grid grid-cols-[auto_minmax(0,1fr)] items-start gap-x-3 gap-y-2",
    // Narrow: the avatar sits beside the header and the body spans the card. Wide: the avatar has its own column.
    avatar: "row-span-1 @md:row-span-5",
    // One row: the author takes the room the time leaves. Below `@sm` a PostCardAuthor puts its handle on a
    // second line and the author fills the row, so the time stays at the end of the name's line. A long time
    // (a full date) wraps inside at most half of the row instead of pushing the card wider.
    header: "flex min-w-0 items-baseline gap-x-2 self-center",
    author: "min-w-0 flex-1 wrap-anywhere @sm:flex-initial",
    time: "max-w-1/2 shrink-0 text-sm text-fg-muted",
    body: "col-span-2 min-w-0 @md:col-span-1 @md:col-start-2",
    actions: "col-span-2 flex min-w-0 flex-wrap items-center gap-1 @md:col-span-1 @md:col-start-2",
  },
});

export type PostCardProps = Omit<FeedArticleProps, "children" | "content" | "aria-labelledby" | "aria-describedby"> & {
  /**
   * Why the post is here, in one line above it ("Bob boosted"): a decorative icon (`aria-hidden`) and a text.
   * It starts the article's accessible name, before the author and time.
   */
  context?: ReactNode;
  /** The author's avatar, decorative because the name is next to it (`<Avatar decorative />`). */
  avatar?: ReactNode;
  /**
   * Who wrote it: usually a link around a PostCardAuthor (display name and handle). With `time`, it is the
   * article's accessible name. It shares one row with the time and takes the room the time leaves.
   */
  author: ReactNode;
  /** When it was posted: a `<time>` element, usually a link to the post. */
  time: ReactNode;
  /** The post body (PostContent), or a ContentWarning around it. It is the article's description. */
  content: ReactNode;
  /** The attachments (MediaGrid). */
  media?: ReactNode;
  /** The actions on the post: reply, boost, favourite, the overflow menu. */
  actions?: ReactNode;
};

/**
 * The layout of one post: an optional context line, avatar, author and time, body, media and actions in
 * slots. The card knows nothing about posts; the caller fills the slots. It is an `<article>` labelled by
 * the context, author and time and described by the content, and inside a Feed it is one of the feed's
 * articles. The header is one row of the author and the time (with a PostCardAuthor, the handle goes to a
 * second line below the `@sm` container width). On a narrow card the body spans the full width under the
 * header; from the `@md` container width the avatar gets its own column.
 */
export function PostCard({ context, avatar, author, time, content, media, actions, className, ...props }: PostCardProps) {
  const slots = postCard();
  const id = useId();
  const contextId = `${id}-context`;
  const authorId = `${id}-author`;
  const timeId = `${id}-time`;
  const contentId = `${id}-content`;
  const labelledBy = context === undefined ? `${authorId} ${timeId}` : `${contextId} ${authorId} ${timeId}`;
  return (
    <FeedArticle {...props} aria-labelledby={labelledBy} aria-describedby={contentId} className={slots.base({ className })}>
      {context !== undefined && (
        <p id={contextId} className={slots.context()}>
          {context}
        </p>
      )}
      <div className={slots.grid()}>
        <div className={slots.avatar()}>{avatar}</div>
        <header className={slots.header()}>
          <div id={authorId} className={slots.author()}>
            {author}
          </div>
          <div id={timeId} className={slots.time()}>
            {time}
          </div>
        </header>
        <div id={contentId} className={slots.body()}>
          {content}
        </div>
        {media !== undefined && <div className={slots.body()}>{media}</div>}
        {actions !== undefined && <div className={slots.actions()}>{actions}</div>}
      </div>
    </FeedArticle>
  );
}

const postCardAuthor = tv({
  slots: {
    // Below `@sm` (of the card) the name and the handle are a line each; from `@sm` they share the header's row.
    // The lines start at the card's start, as wide as their text: a right-to-left name in a left-to-right
    // card lines up with the card, not with its own direction (G9; Chromium has no `text-align: match-parent`).
    base: "flex min-w-0 flex-col items-start @sm:flex-row @sm:items-baseline @sm:gap-1",
    name: "max-w-full min-w-0 truncate font-semibold",
    // Left to right in any page, so the ellipsis at its end cuts the domain and the user part stays. The
    // handle is one text: elements inside would be blocks in the flex row and split it in the accessible name.
    handle: "max-w-full min-w-0 truncate text-sm text-fg-muted",
  },
});

export type PostCardAuthorProps = {
  /** The display name: the user's text in any script (`dir="auto"`). */
  name: ReactNode;
  /** The handle: `@user` for a local account, `@user@domain` for one of another server. */
  handle?: string;
};

/**
 * A display name and a handle for the author slot of PostCard, each cut to one line with an ellipsis
 * (web-client.md G7). A handle is cut from its end, the domain's side. The text stays whole in the
 * DOM, so a link around it is named by the full name and handle. Below the card's `@sm` width the handle
 * is a line under the name; from it, both share the header's row with the time.
 */
export function PostCardAuthor({ name, handle }: PostCardAuthorProps) {
  const slots = postCardAuthor();
  return (
    <span className={slots.base()}>
      <span dir="auto" className={slots.name()}>
        {name}
      </span>
      {handle !== undefined && (
        <>
          {/* The space keeps the name and the handle apart in the accessible name. */}{" "}
          <span dir="ltr" className={slots.handle()}>
            {handle}
          </span>
        </>
      )}
    </span>
  );
}
