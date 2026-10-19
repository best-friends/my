// SPDX-License-Identifier: AGPL-3.0-only
// One notification as an article of the notifications feed (web-client.md Y6): a sentence that says what
// happened ("Alice followed you") with the actor's name as a link to their profile, when, relative to now
// as in the timelines (G8), and for the types about a post, a quote of the post that links to its page.
// A follow offers to follow back. PostCard lays it out, so the article is named by the sentence and the
// time and described by the post. The follows that a suspension by the moderators ended say which server or
// account was suspended and how many followers and follows the user lost.
//
// A group of the favourites or the boosts of one post is one row: "Alice and 2 others favourited your
// post", with the avatars of the others in a row above the sentence, so the number of people is in the
// article's name.
//
// A row has one tab stop per place it leads to: the actor's name (the avatar is the same link for the
// pointer, out of the tab order), and the quote of the post or the follow back button. The avatars of the
// others are links to their profiles for the pointer only; the post's page lists who favourited and
// boosted it.
import { Avatar, Link, PostCard, VisuallyHidden } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { RelationshipActions, useRelationship } from "../../profile";
import type { NotificationAccount } from "../api/notifications";
import type { NotificationGroup } from "../hooks/groupNotifications";
import { type NotificationActor, type NotificationView, useNotificationView } from "../hooks/useNotificationView";

/** A link to a profile on a name: in the color of the text, underlined on hover, since it is no running text. */
const nameLink = "text-fg no-underline data-hovered:text-fg data-hovered:underline";

/** The most avatars of the others that a group shows; the sentence counts them all. */
const maxOtherAvatars = 6;

export type NotificationItemProps = {
  /** One notification, or the favourites or the boosts of one post. */
  group: NotificationGroup;
  /** Whether it was unread when the page opened. */
  isUnread: boolean;
};

export function NotificationItem({ group, isUnread }: NotificationItemProps) {
  const t = useT();
  const intl = useIntl();
  const view = useNotificationView(group);
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
        <>
          {view.others.length > 0 && <OtherAvatars others={view.others.slice(0, maxOtherAvatars)} />}
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
                  count: view.others.length,
                  // The server's name is no running text, isolated from the sentence around it (Y10).
                  domain: view.severance === null || view.severance.domain === null ? null : <bdi key="domain">{view.severance.domain}</bdi>,
                  followers: view.severance?.followers ?? 0,
                  following: view.severance?.following ?? 0,
                },
              )}
            </span>
          </p>
        </>
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
 * The avatars of the other people of a group, as links to their profiles for the pointer: out of the tab
 * order and hidden from screen readers, which hear their number in the sentence.
 */
function OtherAvatars({ others }: { others: readonly NotificationActor[] }) {
  return (
    <div aria-hidden="true" className="mb-1 flex flex-wrap gap-1">
      {others.map((other) => (
        <Link key={other.href} href={other.href} excludeFromTabOrder className="block rounded-full">
          <Avatar src={other.avatar} name={other.name} size="xs" decorative />
        </Link>
      ))}
    </div>
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
