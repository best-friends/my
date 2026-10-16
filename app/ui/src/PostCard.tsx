// SPDX-License-Identifier: AGPL-3.0-only
import { type ReactNode, useId } from "react";
import { tv } from "tailwind-variants";
import { FeedArticle, type FeedArticleProps } from "./Feed";

const postCard = tv({
  slots: {
    // The card is the size container its layout answers to (web-client.md D2).
    base: "@container border-b border-border bg-surface px-4 py-3",
    grid: "grid grid-cols-[auto_minmax(0,1fr)] items-start gap-x-3 gap-y-2",
    // Narrow: the avatar sits beside the header and the body spans the card. Wide: the avatar has its own column.
    avatar: "row-span-1 @md:row-span-5",
    header: "flex min-w-0 flex-wrap items-baseline gap-x-2 self-center",
    author: "min-w-0 wrap-anywhere",
    time: "text-sm text-fg-muted",
    body: "col-span-2 min-w-0 @md:col-span-1 @md:col-start-2",
    actions: "col-span-2 flex min-w-0 flex-wrap items-center gap-1 @md:col-span-1 @md:col-start-2",
  },
});

export type PostCardProps = Omit<FeedArticleProps, "children" | "content" | "aria-labelledby" | "aria-describedby"> & {
  /** The author's avatar, decorative because the name is next to it (`<Avatar decorative />`). */
  avatar?: ReactNode;
  /** The author's display name and handle. With `time`, it is the article's accessible name. */
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
 * The layout of one post: avatar, author and time, body, media and actions in slots. The card knows
 * nothing about posts; the caller fills the slots. It is an `<article>` labelled by the author and time
 * and described by the content, and inside a Feed it is one of the feed's articles. On a narrow card the
 * body spans the full width under the header; from the `@md` container width the avatar gets its own column.
 */
export function PostCard({ avatar, author, time, content, media, actions, className, ...props }: PostCardProps) {
  const slots = postCard();
  const id = useId();
  const authorId = `${id}-author`;
  const timeId = `${id}-time`;
  const contentId = `${id}-content`;
  return (
    <FeedArticle {...props} aria-labelledby={`${authorId} ${timeId}`} aria-describedby={contentId} className={slots.base({ className })}>
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
