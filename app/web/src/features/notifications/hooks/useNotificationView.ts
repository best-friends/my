// SPDX-License-Identifier: AGPL-3.0-only
// What a notification's row shows, as plain values for the slots of @my/ui's PostCard (web-client.md A4):
// the hook decides what to show and formats it; the component only places it. It returns no JSX.
import { useIntl } from "react-intl";
import type { ClientMessageKey } from "../../../lib/i18n";
import { accountStatusPath, moderationAppealPath, moderationReportPath, followListPath, profilePath } from "../../../lib/paths";
import { type TimeView, useTimeView } from "../../../lib/time";
import { useViewer } from "../../session";
import { statusPath } from "../../status";
import type { ModerationAction, Notification, NotificationAccount, NotificationStatus, NotificationType } from "../api/notifications";
import type { NotificationGroup } from "./groupNotifications";

/** An account in a row: its name in the UI (the username when it has no display name), profile page, and avatar URL. */
export type NotificationActor = { name: string; href: string; avatar: string | undefined };

/** A link from a row to the page where the user follows up what it tells. */
type NotificationLink = { href: string; label: ClientMessageKey };

/** What a `moderation_notice` tells of the action on the user's account, formatted for the row. */
export type ModerationNoticeView = {
  /** What the action does, and why, in the user's language as the server sends them. */
  kind: string;
  category: string;
  /** The moderators' explanation, their own text; null when they gave none. */
  reason: string | null;
  /** When the action ends by itself, as a date and time in the user's locale; null when it lasts until lifted. */
  until: { dateTime: string; text: string } | null;
  /**
   * For an `appeal_decided`: what the moderators decided on the user's appeal, and their explanation (their own
   * text, null when they gave none). Null for a `moderation_notice`, and while the appeal is pending.
   */
  decision: { sentence: ClientMessageKey; reason: string | null } | null;
};

export type NotificationView = {
  /**
   * The sentence that says what happened; `{name}` in it is the actor's name, for the types that have an
   * actor, and `{count}` the number of the other actors of a group.
   */
  sentence: ClientMessageKey;
  /** The account that did it, the newest of a group; null for the types without an actor. */
  actor: NotificationActor | null;
  /**
   * The account that `{name}` names in a row without an actor, shown without an avatar: the account that a
   * `report_resolved`'s report is about. Null otherwise.
   */
  subject: NotificationActor | null;
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
  /**
   * What a `relationships_severed` tells, for `{domain}`, `{followers}`, and `{following}` in its sentence:
   * the suspended server (null for an account, which is the actor), and how many followers and follows the
   * user lost; null for the other types.
   */
  severance: { domain: string | null; followers: number; following: number } | null;
  /** What a `moderation_notice` or an `appeal_decided` tells of the action; null for the other types. */
  notice: ModerationNoticeView | null;
  /**
   * The pages the row leads to besides the actor's profile and the post, where the user follows up what it
   * tells: the account status, the report, the registrations, or the user's follow lists.
   */
  links: readonly NotificationLink[];
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
  relationships_severed: "client:notifications.type.relationships_severed",
  appeal_decided: "client:notifications.type.appeal_decided",
  admin_appeal: "client:notifications.type.admin_appeal",
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

/**
 * The sentence and the values of a `relationships_severed`: the server or the account that the moderators
 * suspended, with the counts; without its severance, the sentence that says only that follows ended.
 */
function severanceOf(notification: Notification): Pick<NotificationView, "sentence" | "severance"> {
  const severance = notification.relationship_severance;
  const fallback = { sentence: sentences.relationships_severed, severance: null };
  if (severance === undefined) return fallback;
  const counts = { followers: severance.followers_lost, following: severance.following_lost };
  if (severance.kind === "domain_suspension") {
    return { sentence: "client:notifications.severed.domain", severance: { domain: severance.domain ?? "", ...counts } };
  }
  if (notification.account === undefined) return fallback;
  return { sentence: "client:notifications.severed.account", severance: { domain: null, ...counts } };
}

/** The sentences of the decisions on an appeal. */
const decisionSentences: Readonly<Record<"upheld" | "overturned", ClientMessageKey>> = {
  upheld: "client:notifications.appeal.upheld",
  overturned: "client:notifications.appeal.overturned",
};

/**
 * What a row shows of the action that a `moderation_notice` or an `appeal_decided` tells of; `formatDate` formats
 * its end.
 */
function noticeOf(action: ModerationAction, formatDate: (value: string) => string): ModerationNoticeView {
  const { appeal } = action;
  return {
    decision:
      appeal === undefined || appeal.state === "pending"
        ? null
        : { sentence: decisionSentences[appeal.state], reason: appeal.decision_reason === undefined || appeal.decision_reason === "" ? null : appeal.decision_reason },
    kind: action.kind_label,
    category: action.reason_category_label,
    reason: action.public_reason === undefined || action.public_reason === "" ? null : action.public_reason,
    until: action.expires_at === undefined ? null : { dateTime: action.expires_at, text: formatDate(action.expires_at) },
  };
}

/** The pages that a row of `notification` links to (NotificationView.links); `ownAcct` is null until the viewer loads. */
function linksOf(notification: Notification, ownAcct: string | null): NotificationLink[] {
  switch (notification.type) {
    case "moderation_notice":
    case "appeal_decided":
      return [{ href: accountStatusPath, label: "client:notifications.link.account_status" }];
    case "admin_appeal":
      return notification.appeal_id === undefined ? [] : [{ href: moderationAppealPath(notification.appeal_id), label: "client:notifications.link.appeal" }];
    case "admin_report":
      return notification.report_id === undefined ? [] : [{ href: moderationReportPath(notification.report_id), label: "client:notifications.link.report" }];
    case "admin_sign_up":
      return [{ href: "/moderation/registrations", label: "client:notifications.link.registrations" }];
    case "relationships_severed":
      return ownAcct === null
        ? []
        : [
            { href: followListPath(ownAcct, "followers"), label: "client:notifications.link.followers" },
            { href: followListPath(ownAcct, "following"), label: "client:notifications.link.following" },
          ];
    default:
      return [];
  }
}

/** Returns what the row of `group` shows: one notification, or the favourites or the boosts of one post. */
export function useNotificationView(group: NotificationGroup): NotificationView {
  const [notification] = group.notifications;
  const intl = useIntl();
  const viewer = useViewer();
  const time = useTimeView(notification.created_at);
  const { account, status } = notification;
  const severed = notification.type === "relationships_severed" ? severanceOf(notification) : null;
  // The same account can favourite a post again after taking it back: it counts once.
  const seen = new Set(account === undefined ? [] : [account.id]);
  const others: NotificationActor[] = [];
  for (const { account: other } of group.notifications) {
    if (other === undefined || seen.has(other.id)) continue;
    seen.add(other.id);
    others.push(actorOf(other));
  }
  const groupSentence = groupSentences[notification.type];
  const target = notification.type === "report_resolved" ? notification.report?.target_account : undefined;
  const action = notification.type === "moderation_notice" || notification.type === "appeal_decided" ? notification.moderation_action : undefined;
  return {
    sentence:
      severed?.sentence ??
      (target === undefined ? null : "client:notifications.report_resolved.account") ??
      (others.length > 0 && groupSentence !== undefined ? groupSentence : sentences[notification.type]),
    actor: account === undefined ? null : actorOf(account),
    subject: target === undefined ? null : actorOf(target),
    others,
    time,
    post: status === undefined ? null : postOf(status),
    followBack: notification.type === "follow" && account !== undefined ? account : null,
    severance: severed?.severance ?? null,
    notice: action === undefined ? null : noticeOf(action, (value) => intl.formatDate(value, { dateStyle: "medium", timeStyle: "short" })),
    links: linksOf(notification, viewer.status === "signed-in" ? viewer.account.acct : null),
  };
}
