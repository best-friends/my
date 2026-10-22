// SPDX-License-Identifier: AGPL-3.0-only
// One notification as an article of the notifications feed (web-client.md Y6): a sentence that says what
// happened ("Alice followed you") with the actor's name as a link to their profile, when, relative to now
// as in the timelines (G8), and for the types about a post, a quote of the post that links to its page.
// A follow offers to follow back. PostCard lays it out, so the article is named by the sentence and the
// time and described by the post. The follows that a suspension by the moderators ended say which server or
// account was suspended and how many followers and follows the user lost.
//
// The notifications from the instance lead to where the user follows them up, as links in the row's actions:
// a moderation notice (and the decision on an appeal of it) says what the action does, why, and until when,
// and leads to the account's status; an appeal to the moderators leads to the appeal; a
// resolved report names the reported account; a moderator's new report or registration leads to the report or
// the registrations; a custom emoji request leads the administrators to the requests and its decision the
// requester to their custom emoji, with the reason of a rejection; a supporter notification names the plan and
// says when the grace period ends or why the subscription ended, and leads to the supporter page except in the
// Tauri app, which shows no way to pay (ADR-0072 §14); and lost follows lead to the user's lists of followers and
// follows.
//
// A group of the favourites or the boosts of one post is one row: "Alice and 2 others favourited your
// post", with the avatars of the others in a row above the sentence, so the number of people is in the
// article's name.
//
// A row has one tab stop per place it leads to: the actor's name (the avatar is the same link for the
// pointer, out of the tab order), and the quote of the post or the follow back button. The avatars of the
// others are links to their profiles for the pointer only; the post's page lists who favourited and
// boosted it.
import { Avatar, EmojiText, Link, PostCard, PostCardNameLink, VisuallyHidden } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { RelationshipActions, useRelationship } from "../../profile";
import type { NotificationAccount } from "../api/notifications";
import type { NotificationGroup } from "../hooks/groupNotifications";
import {
  type ModerationNoticeView,
  type NotificationActor,
  type NotificationView,
  type SupporterNoticeView,
  useNotificationView,
} from "../hooks/useNotificationView";

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
  // A row without an actor names the account it is about, without its avatar.
  const named = actor ?? view.subject;
  return (
    <PostCard
      {...(actor !== null && {
        avatar: (
          // The same profile as the name, for the pointer: out of the tab order and hidden from screen
          // readers, which reach it by the name (one tab stop per profile). React Aria's Link does not take
          // `aria-hidden`, so an element around it hides it.
          <div aria-hidden="true">
            <Link href={actor.href} excludeFromTabOrder className="block rounded-full">
              <Avatar sources={actor.avatar} name={actor.name} decorative />
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
                    named === null ? null : (
                      <PostCardNameLink key="name" href={named.href}>
                        <EmojiText text={named.name} emojis={named.emojis} />
                      </PostCardNameLink>
                    ),
                  count: view.others.length,
                  // The server's name is no running text, isolated from the sentence around it (Y10).
                  domain: view.severance === null || view.severance.domain === null ? null : <bdi key="domain">{view.severance.domain}</bdi>,
                  followers: view.severance?.followers ?? 0,
                  following: view.severance?.following ?? 0,
                  // The emoji is the reacting user's choice, isolated from the sentence around it (Y10).
                  // A requested emoji's shortcode is no running text either.
                  emoji:
                    view.reaction !== null ? (
                      <bdi key="emoji">{view.reaction}</bdi>
                    ) : view.emojiRequest === null ? null : (
                      <bdi key="emoji" dir="ltr">
                        {view.emojiRequest.emoji}
                      </bdi>
                    ),
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
      content={
        view.post !== null ? (
          <PostQuote post={view.post} />
        ) : view.notice !== null ? (
          <ModerationNotice notice={view.notice} />
        ) : view.emojiRequest !== null && view.emojiRequest.reason !== null ? (
          <RejectionReason reason={view.emojiRequest.reason} />
        ) : view.supporter !== null ? (
          <SupporterNotice supporter={view.supporter} />
        ) : null
      }
      {...(view.followBack !== null && { actions: <FollowBack account={view.followBack} /> })}
      {...(view.links.length > 0 && {
        actions: (
          <div className="flex flex-wrap gap-x-4">
            {view.links.map((link) => (
              <Link key={link.href} href={link.href} variant="standalone">
                {t(link.label)}
              </Link>
            ))}
          </div>
        ),
      })}
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
          <Avatar sources={other.avatar} name={other.name} size="xs" decorative />
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

/**
 * What a moderation notice tells of the action on the user's account: what it does and why, the moderators'
 * explanation (their own text, in its own direction; Y10), and when it ends by itself.
 */
function ModerationNotice({ notice }: { notice: ModerationNoticeView }) {
  const t = useT();
  const intl = useIntl();
  return (
    <div className="flex flex-col gap-1 border-s-2 border-border ps-3">
      <p className="font-semibold">{t("client:notifications.moderation.action", { kind: notice.kind, category: notice.category })}</p>
      {notice.reason !== null && (
        <p dir="auto" className="whitespace-pre-line wrap-anywhere align-match-parent">
          {notice.reason}
        </p>
      )}
      {notice.decision !== null && <p className="font-semibold">{t(notice.decision.sentence)}</p>}
      {notice.decision !== null && notice.decision.reason !== null && (
        <p dir="auto" className="whitespace-pre-line wrap-anywhere align-match-parent">
          {notice.decision.reason}
        </p>
      )}
      {notice.until !== null && (
        <p className="text-sm text-fg-muted">
          {intl.formatMessage(
            { id: "client:notifications.moderation.until" },
            {
              date: (
                <time key="until" dateTime={notice.until.dateTime}>
                  {notice.until.text}
                </time>
              ),
            },
          )}
        </p>
      )}
    </div>
  );
}

/** The administrator's reason of a rejected custom emoji request: their own text, in its own direction (Y10). */
function RejectionReason({ reason }: { reason: string }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-1 border-s-2 border-border ps-3">
      <p className="text-sm text-fg-muted">{t("client:notifications.custom_emoji_request.reason")}</p>
      <p dir="auto" className="whitespace-pre-line wrap-anywhere">
        {reason}
      </p>
    </div>
  );
}

/**
 * What a supporter notification tells of the user's subscription: the plan's name (the administrators' text, in its
 * own direction and of no known language; Y10), and when the grace period ends, why it ended, or what a retirement
 * does. It names no way to pay, which the Tauri app does not offer (ADR-0072 §14).
 */
function SupporterNotice({ supporter }: { supporter: SupporterNoticeView }) {
  const t = useT();
  const intl = useIntl();
  const { detail } = supporter;
  return (
    <div className="flex flex-col gap-1 border-s-2 border-border ps-3">
      <p dir="auto" lang="" className="font-semibold wrap-anywhere">
        {supporter.plan}
      </p>
      {detail?.kind === "grace" && (
        <p className="text-sm text-fg-muted">
          {intl.formatMessage(
            { id: "client:notifications.supporter.grace" },
            {
              date: (
                <time key="grace" dateTime={detail.date.dateTime}>
                  {detail.date.text}
                </time>
              ),
            },
          )}
        </p>
      )}
      {detail?.kind === "ended" && <p className="text-sm text-fg-muted">{t("client:notifications.supporter.ended", { reason: detail.reason })}</p>}
      {detail?.kind === "retired" && <p className="text-sm text-fg-muted">{t("client:notifications.supporter.retired")}</p>}
    </div>
  );
}

/** Follow back from a follow notification: the profile's follow button, without its menu. */
function FollowBack({ account }: { account: NotificationAccount }) {
  const relationship = useRelationship(account);
  return <RelationshipActions account={account} name={account.display_name || account.username} state={relationship} showMenu={false} />;
}
