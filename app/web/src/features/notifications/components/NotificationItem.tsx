// SPDX-License-Identifier: AGPL-3.0-only
// One notification as an article of the notifications feed (web-client.md Y6): a sentence that says what
// happened ("Alice followed you") with the actor's name as a link to their profile, when, relative to now
// as in the timelines (G8), and for the types about a post, a quote of the post that links to its page.
// A follow offers to follow back. PostCard lays it out, so the article is named by the sentence and the
// time and described by the post.
//
// A row has one tab stop per place it leads to: the actor's name (the avatar is the same link for the
// pointer, out of the tab order), and the quote of the post or the follow back button.
import { Avatar, Link, PostCard, VisuallyHidden } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { RelationshipActions, useRelationship } from "../../profile";
import type { Notification, NotificationAccount } from "../api/notifications";
import { type NotificationView, useNotificationView } from "../hooks/useNotificationView";

/** A link to a profile on a name: in the color of the text, underlined on hover, since it is no running text. */
const nameLink = "text-fg no-underline data-hovered:text-fg data-hovered:underline";

export type NotificationItemProps = {
  notification: Notification;
  /** Whether it was unread when the page opened. */
  isUnread: boolean;
};

export function NotificationItem({ notification, isUnread }: NotificationItemProps) {
  const t = useT();
  const intl = useIntl();
  const view = useNotificationView(notification);
  const { actor } = view;
  return (
    <PostCard
      {...(actor !== null && {
        avatar: (
          // The same profile as the name, for the pointer: out of the tab order and hidden from screen
          // readers, which reach it by the name (one tab stop per profile). React Aria's Link does not take
          // `aria-hidden`, so an element around it hides it.
          <div aria-hidden="true">
            <Link href={actor.href} excludeFromTabOrder className="block rounded-full">
              <Avatar src={actor.avatar} name={actor.name} decorative />
            </Link>
          </div>
        ),
      })}
      author={
        <p className="flex items-baseline gap-2">
          {isUnread && (
            <>
              <span aria-hidden="true" className="inline-block size-2 shrink-0 self-center rounded-full bg-accent" />
              {/* The space keeps the words apart in the article's name. */}
              <VisuallyHidden>{t("client:notifications.unread")}</VisuallyHidden>{" "}
            </>
          )}
          <span className={isUnread ? "font-semibold" : "font-normal"}>
            {/* The actor's name is the user's text, isolated from the sentence around it (Y10). */}
            {intl.formatMessage(
              { id: view.sentence },
              {
                name:
                  actor === null ? null : (
                    <Link key="name" href={actor.href} className={nameLink}>
                      <bdi>{actor.name}</bdi>
                    </Link>
                  ),
              },
            )}
          </span>
        </p>
      }
      time={
        <time dateTime={view.time.dateTime} title={view.time.absolute}>
          {view.time.relative}
        </time>
      }
      content={view.post === null ? null : <PostQuote post={view.post} />}
      {...(view.followBack !== null && { actions: <FollowBack account={view.followBack} /> })}
    />
  );
}

/**
 * The post a notification is about, as a quote that links to its page in the app, with its thread and the
 * actions on it (FR-113). The server embeds it in the notification, and leaves it out when the user may not
 * see it. A post behind a content warning shows the warning only.
 */
function PostQuote({ post }: { post: NonNullable<NotificationView["post"]> }) {
  const t = useT();
  const intl = useIntl();
  const label =
    post.contentWarning !== null ? (
      // The warning is the author's text in the post's language, isolated from the UI's sentence (Y10).
      intl.formatMessage(
        { id: "client:notifications.post.content_warning" },
        {
          summary: (
            <bdi key="summary" lang={post.lang}>
              {post.contentWarning}
            </bdi>
          ),
        },
      )
    ) : post.text === "" ? (
      // A post of pictures only has no text to quote.
      t("client:notifications.post.open")
    ) : (
      <span dir="auto" lang={post.lang} className="line-clamp-3 whitespace-pre-line">
        {post.text}
      </span>
    );
  return (
    <Link href={post.href} className="block border-s-2 border-border ps-3 wrap-anywhere text-fg no-underline data-hovered:underline">
      {label}
    </Link>
  );
}

/** Follow back from a follow notification: the profile's follow button, without its menu. */
function FollowBack({ account }: { account: NotificationAccount }) {
  const relationship = useRelationship(account);
  return <RelationshipActions account={account} name={account.display_name || account.username} state={relationship} showMenu={false} />;
}
