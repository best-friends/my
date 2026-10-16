// SPDX-License-Identifier: AGPL-3.0-only
// One notification as an article of the notifications feed (web-client.md Y6): a sentence that says what
// happened ("Alice followed you"), when, and for the types about a post, the post itself. PostCard lays it
// out, so the article is named by the sentence and the time and described by the post.
import { Avatar, ContentWarning, Link, PostCard, PostContent, serverSanitizedHtml, VisuallyHidden } from "@my/ui";
import { type Translate, useT } from "../../../lib/i18n";
import type { Notification, NotificationAccount, NotificationStatus } from "../api/notifications";

/**
 * The account's name inside a sentence of the UI's language. It is the user's text in any script, so it
 * is wrapped in Unicode directional isolates (FSI … PDI): a right-to-left name does not reorder the
 * sentence around it, as `dir="auto"` does for a whole element (web-client.md Y10).
 */
function isolatedName(account: NotificationAccount | undefined): string {
  const name = account === undefined ? "" : account.display_name || account.username;
  return `\u2068${name}\u2069`;
}

/** The sentence that says what happened, in the UI's language. */
function sentenceOf(t: Translate, notification: Notification): string {
  const name = isolatedName(notification.account);
  switch (notification.type) {
    case "follow":
      return t("client:notifications.type.follow", { name });
    case "follow_request":
      return t("client:notifications.type.follow_request", { name });
    case "follow_accepted":
      return t("client:notifications.type.follow_accepted", { name });
    case "mention":
      return t("client:notifications.type.mention", { name });
    case "boost":
      return t("client:notifications.type.boost", { name });
    case "favourite":
      return t("client:notifications.type.favourite", { name });
    case "poll_closed":
      return t("client:notifications.type.poll_closed");
    case "moderation_notice":
      return t("client:notifications.type.moderation_notice");
  }
}

export type NotificationItemProps = {
  notification: Notification;
  /** Whether it was unread when the page opened. */
  isUnread: boolean;
};

export function NotificationItem({ notification, isUnread }: NotificationItemProps) {
  const t = useT();
  const { account } = notification;
  return (
    <PostCard
      avatar={account === undefined ? undefined : <Avatar name={account.display_name || account.username} decorative />}
      author={
        <p className="flex items-baseline gap-2">
          {isUnread && (
            <>
              <span aria-hidden="true" className="inline-block size-2 shrink-0 self-center rounded-full bg-accent" />
              {/* The space keeps the words apart in the article's name. */}
              <VisuallyHidden>{t("client:notifications.unread")}</VisuallyHidden>{" "}
            </>
          )}
          <span className={isUnread ? "font-semibold" : "font-normal"}>{sentenceOf(t, notification)}</span>
        </p>
      }
      time={
        <time dateTime={notification.created_at}>{t("client:notifications.time", { time: new Date(notification.created_at) })}</time>
      }
      content={notification.status === undefined ? null : <NotificationPostView status={notification.status} />}
    />
  );
}

/**
 * The post a notification is about, quieter than in a timeline, with its content warning kept. The server
 * embeds it in the notification, and leaves it out when the user may not see it.
 */
function NotificationPostView({ status }: { status: NotificationStatus }) {
  const t = useT();
  const lang = status.language === "und" ? undefined : status.language;
  const body = <PostContent html={serverSanitizedHtml(status.content)} lang={lang} className="text-fg-muted" />;
  return (
    <div className="flex flex-col gap-2">
      {status.spoiler_text === "" ? (
        body
      ) : (
        <ContentWarning summary={status.spoiler_text} summaryLang={lang} toggleLabel={t("client:notifications.post.show")}>
          {body}
        </ContentWarning>
      )}
      {status.url !== undefined && (
        <Link href={status.url} variant="standalone" className="self-start text-sm">
          {t("client:notifications.post.open")}
        </Link>
      )}
    </div>
  );
}
