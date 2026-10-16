// SPDX-License-Identifier: AGPL-3.0-only
// What a status card shows, as plain values for the slots of @my/ui's PostCard (web-client.md A4): the
// hook decides what to show and formats it; the component only places it. It returns no JSX.
import { type MediaItem, type SanitizedHtml, serverSanitizedHtml } from "@my/ui";
import { type IntlShape, useIntl } from "react-intl";
import { useNow } from "../../../lib/clock";
import { useConfig } from "../../../lib/config";
import { profilePath } from "../../../lib/paths";
import type { MediaAttachment, Status } from "../api/timelines";

export type StatusView = {
  /** The display name of the account that boosted the post and its profile page, or null for a post of its own. */
  boostedBy: { name: string; href: string } | null;
  /**
   * The author's display name (the username when there is none), full handle (`@alice@example.com`) and
   * profile page.
   */
  author: { name: string; handle: string; href: string };
  /** When the post was written: the ISO date for `<time>`, the relative time to show, and the full date and time. */
  time: { dateTime: string; relative: string; absolute: string };
  /** The body, as the server sanitized it. */
  html: SanitizedHtml;
  /** The language of the post (BCP 47), or undefined when the server does not know it. */
  lang: string | undefined;
  /** The author's content warning, or null when there is none. */
  contentWarning: string | null;
  /** The pictures that are ready to show. */
  media: MediaItem[];
  /** Whether the author marked the media sensitive. */
  sensitive: boolean;
};

const minute = 60_000;
const hour = 60 * minute;
const day = 24 * hour;

/**
 * The time from `now` back to `then`, in the largest unit that fits ("now", "5 min. ago", "3 hr. ago",
 * "2 days ago"), or the date for a week or more.
 */
function relativeTime(intl: IntlShape, then: number, now: number): string {
  const elapsed = Math.max(0, now - then);
  if (elapsed < minute) return intl.formatRelativeTime(0, "second", { numeric: "auto" });
  if (elapsed < hour) return intl.formatRelativeTime(-Math.floor(elapsed / minute), "minute", { style: "short" });
  if (elapsed < day) return intl.formatRelativeTime(-Math.floor(elapsed / hour), "hour", { style: "short" });
  if (elapsed < 7 * day) return intl.formatRelativeTime(-Math.floor(elapsed / day), "day", { style: "short" });
  return intl.formatDate(then, { dateStyle: "medium" });
}

/** The pictures of a status that the server has finished processing, at their preview size. */
function mediaItems(attachments: readonly MediaAttachment[], lang: string | undefined): MediaItem[] {
  return attachments.flatMap((attachment) =>
    attachment.state === "ready" && attachment.type === "image" && attachment.variants !== undefined
      ? [{ id: attachment.id, src: attachment.variants.thumbnail.url, alt: attachment.description ?? null, lang }]
      : [],
  );
}

/** The handle of an account: `acct` is `username` for local accounts and `username@domain` for remote ones. */
function handleOf(acct: string, localDomain: string): string {
  return acct.includes("@") ? `@${acct}` : `@${acct}@${localDomain}`;
}

/** Returns what the card of `status` shows. A boost shows the boosted post, with the booster above it. */
export function useStatusView(status: Status): StatusView {
  const intl = useIntl();
  const now = useNow();
  const { accountDomain } = useConfig();
  const post = status.boost ?? status;
  const lang = post.language === "und" ? undefined : post.language;
  const writtenAt = Date.parse(post.created_at);
  return {
    boostedBy:
      status.boost === undefined ? null : { name: status.account.display_name || status.account.username, href: profilePath(status.account.acct) },
    author: {
      name: post.account.display_name || post.account.username,
      handle: handleOf(post.account.acct, accountDomain),
      href: profilePath(post.account.acct),
    },
    time: {
      dateTime: post.created_at,
      relative: relativeTime(intl, writtenAt, now),
      absolute: intl.formatDate(writtenAt, { dateStyle: "full", timeStyle: "short" }),
    },
    // `content` is sanitized HTML (api/openapi.yaml, Status.content).
    html: serverSanitizedHtml(post.content),
    lang,
    contentWarning: post.spoiler_text === "" ? null : post.spoiler_text,
    media: mediaItems(post.media_attachments, lang),
    sensitive: post.sensitive,
  };
}
