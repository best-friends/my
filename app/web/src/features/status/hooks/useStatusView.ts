// SPDX-License-Identifier: AGPL-3.0-only
// What a status card shows, as plain values for the slots of @my/ui's PostCard (web-client.md A4): the
// hook decides what to show and formats it; the component only places it. It returns no JSX.
import { type CustomEmojiImage, type ImageSources, type MediaItem, type SanitizedHtml, serverSanitizedHtml } from "@my/ui";
import { useContentLinkClick } from "../../../lib/contentLinks";
import { handleOf, isRemoteAcct } from "../../../lib/handle";
import { hostOf } from "../../../lib/host";
import { profilePath } from "../../../lib/paths";
import { type TimeView, useTimeView } from "../../../lib/time";
import { type MediaAttachment, type Status, statusHistoryPath, statusPath, statusShareUrl } from "../api/status";
import { imageSources } from "../../../lib/images";

export type StatusView = {
  /** The display name of the account that boosted the post and its profile page, or null for a post of its own. */
  boostedBy: { name: string; emojis: readonly CustomEmojiImage[]; href: string } | null;
  /**
   * The author's display name (the username when there is none), handle (`@alice` for a local account, `@bob@example.com` for a remote one),
   * profile page, and avatar URL (undefined when the account has none, and the avatar shows its fallback),
   * with the custom emoji of the name.
   */
  author: { name: string; emojis: readonly CustomEmojiImage[]; handle: string; href: string; avatar: ImageSources | undefined };
  /** When the post was written: the ISO date for `<time>`, the relative time to show, and the full date and time. */
  time: TimeView;
  /** The web client's address of the post (the boosted post for a boost), which its time links to. */
  path: string;
  /** For a post that was edited, the address of its edit history (FR-109); null for one that never was. */
  historyPath: string | null;
  /**
   * For a post of another server, its page there (its ActivityPub ID when it has none) and the host of that
   * page; null for a post of this instance.
   */
  original: { href: string; host: string } | null;
  /** The body, as the server sanitized it. */
  html: SanitizedHtml;
  /** Opens the links of the body to this instance's pages through the router (the body's `onLinkClick`). */
  onLinkClick: (event: MouseEvent, link: HTMLAnchorElement) => void;
  /** The custom emoji of the body and the content warning. */
  emojis: readonly CustomEmojiImage[];
  /** The language of the post (BCP 47), or undefined when the server does not know it. */
  lang: string | undefined;
  /** The author's content warning, or null when there is none. */
  contentWarning: string | null;
  /** The pictures that are ready to show. */
  media: MediaItem[];
  /** Whether the author marked the media sensitive. */
  sensitive: boolean;
  /** How many times the post was boosted and favourited. */
  counts: { boosts: number; favourites: number };
};

/**
 * The pictures of a status that are ready to show: `preview_url` is the small still rendition of an upload, and
 * the media proxy's small one of a remote file; an upload also has its renditions to choose from by size, still and
 * moving (api/openapi.yaml, MediaAttachment).
 */
export function mediaItems(attachments: readonly MediaAttachment[], lang: string | undefined): MediaItem[] {
  return attachments.flatMap((attachment) =>
    attachment.state === "ready" && attachment.type === "image" && attachment.preview_url !== undefined
      ? [{ id: attachment.id, src: attachment.preview_url, sources: imageSources(attachment.variants), alt: attachment.description ?? null, lang }]
      : [],
  );
}

/** The page of a remote post on its server and that page's host; null for a local post. */
function originalOf(post: Status): StatusView["original"] {
  if (!isRemoteAcct(post.account.acct)) return null;
  const href = statusShareUrl(post);
  // The server gives absolute URIs (format: uri); the account's domain stands in should one not parse.
  return { href, host: hostOf(href, post.account.domain) };
}

/** Returns what the card of `status` shows. A boost shows the boosted post, with the booster above it. */
export function useStatusView(status: Status): StatusView {
  const post = status.boost ?? status;
  const lang = post.language === "und" ? undefined : post.language;
  const time = useTimeView(post.created_at);
  const onLinkClick = useContentLinkClick();
  return {
    boostedBy:
      status.boost === undefined
        ? null
        : { name: status.account.display_name || status.account.username, emojis: status.account.emojis, href: profilePath(status.account.acct) },
    author: {
      name: post.account.display_name || post.account.username,
      emojis: post.account.emojis,
      handle: handleOf(post.account.acct),
      href: profilePath(post.account.acct),
      avatar: imageSources(post.account.avatar),
    },
    time,
    path: statusPath(post),
    historyPath: post.edited_at === undefined ? null : statusHistoryPath(post),
    original: originalOf(post),
    // `content` is sanitized HTML (api/openapi.yaml, Status.content).
    html: serverSanitizedHtml(post.content),
    onLinkClick,
    emojis: post.emojis,
    lang,
    contentWarning: post.spoiler_text === "" ? null : post.spoiler_text,
    media: mediaItems(post.media_attachments, lang),
    sensitive: post.sensitive,
    counts: { boosts: post.boosts_count, favourites: post.favourites_count },
  };
}
