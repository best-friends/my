// SPDX-License-Identifier: AGPL-3.0-only
// What a notification's row shows, as plain values for the slots of @my/ui's PostCard (web-client.md A4):
// the hook decides what to show and formats it; the component only places it. It returns no JSX.
import type { ClientMessageKey } from "../../../lib/i18n";
import { profilePath } from "../../../lib/paths";
import { type TimeView, useTimeView } from "../../../lib/time";
import { statusPath } from "../../status";
import type { NotificationAccount, NotificationStatus, NotificationType } from "../api/notifications";
import type { NotificationGroup } from "./groupNotifications";

/** An account in a row: its name in the UI (the username when it has no display name), profile page, and avatar URL. */
export type NotificationActor = { name: string; href: string; avatar: string | undefined };

export type NotificationView = {
  /**
   * The sentence that says what happened; `{name}` in it is the actor's name, for the types that have an
   * actor, and `{count}` the number of the other actors of a group.
   */
  sentence: ClientMessageKey;
  /** The account that did it, the newest of a group; null for the types without an actor. */
  actor: NotificationActor | null;
  /**
   * The other accounts of a group, newest first, each once: the people who also favourited or boosted the
   * post. Empty for a row of one notification.
   */
  others: readonly NotificationActor[];
  /** When it happened, relative to now as in the timelines (web-client.md G8). */
  time: TimeView;
  /**
   * The post it is about, as a quote that links to the post's page: its text without markup (the quote is
   * a link and cannot hold the post's own links), or only the author's content warning, which the post's
   * page asks to open; null for the types that are not about a post, and when the user may not see it.
   */
  post: { href: string; text: string; lang: string | undefined; contentWarning: string | null } | null;
  /** The account to offer a follow back to: the follower of a follow notification, else null. */
  followBack: NotificationAccount | null;
};

const sentences: Readonly<Record<NotificationType, ClientMessageKey>> = {
  follow: "client:notifications.type.follow",
  follow_request: "client:notifications.type.follow_request",
  follow_accepted: "client:notifications.type.follow_accepted",
  mention: "client:notifications.type.mention",
  boost: "client:notifications.type.boost",
  favourite: "client:notifications.type.favourite",
  poll_closed: "client:notifications.type.poll_closed",
  moderation_notice: "client:notifications.type.moderation_notice",
  report_resolved: "client:notifications.type.report_resolved",
  admin_report: "client:notifications.type.admin_report",
  admin_sign_up: "client:notifications.type.admin_sign_up",
};

/** The sentences of a group of the types that rows group (groupNotifications). */
const groupSentences: Readonly<Partial<Record<NotificationType, ClientMessageKey>>> = {
  boost: "client:notifications.group.boost",
  favourite: "client:notifications.group.favourite",
};

function actorOf(account: NotificationAccount): NotificationActor {
  return { name: account.display_name || account.username, href: profilePath(account.acct), avatar: account.avatar_url };
}

/**
 * The text of sanitized post HTML (api/openapi.yaml, Status.content), with its paragraphs and line breaks
 * kept as line breaks. The parsed document is inert: nothing in it loads or runs.
 */
function textOf(html: string): string {
  const doc = new DOMParser().parseFromString(html, "text/html");
  for (const br of doc.body.querySelectorAll("br")) br.replaceWith("\n");
  const blocks = [...doc.body.children].map((element) => element.textContent.trim());
  return (blocks.length > 0 ? blocks.join("\n") : doc.body.textContent).trim();
}

function postOf(status: NotificationStatus): NonNullable<NotificationView["post"]> {
  const contentWarning = status.spoiler_text === "" ? null : status.spoiler_text;
  return {
    href: statusPath(status),
    // Behind a content warning, the text stays on the post's page.
    text: contentWarning === null ? textOf(status.content) : "",
    lang: status.language === "und" ? undefined : status.language,
    contentWarning,
  };
}

/** Returns what the row of `group` shows: one notification, or the favourites or the boosts of one post. */
export function useNotificationView(group: NotificationGroup): NotificationView {
  const [notification] = group.notifications;
  const time = useTimeView(notification.created_at);
  const { account, status } = notification;
  // The same account can favourite a post again after taking it back: it counts once.
  const seen = new Set(account === undefined ? [] : [account.id]);
  const others: NotificationActor[] = [];
  for (const { account: other } of group.notifications) {
    if (other === undefined || seen.has(other.id)) continue;
    seen.add(other.id);
    others.push(actorOf(other));
  }
  const groupSentence = groupSentences[notification.type];
  return {
    sentence: others.length > 0 && groupSentence !== undefined ? groupSentence : sentences[notification.type],
    actor: account === undefined ? null : actorOf(account),
    others,
    time,
    post: status === undefined ? null : postOf(status),
    followBack: notification.type === "follow" && account !== undefined ? account : null,
  };
}
