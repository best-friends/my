// SPDX-License-Identifier: AGPL-3.0-only
// The rules of a new post that the client applies before the server does: the visibilities and their
// order (ADR-0017), the default of a reply, the account's visibility ceiling, the mentions a reply starts
// with and the remote ones a post names, and the character count.
import type { PostingLimits, Status, Visibility } from "../api/compose";

/** The visibilities from the widest to the narrowest (ADR-0017). */
export const visibilities: readonly Visibility[] = ["public", "local", "unlisted", "private", "direct"];

/** The narrower of two visibilities. */
export function narrower(a: Visibility, b: Visibility): Visibility {
  return visibilities.indexOf(a) >= visibilities.indexOf(b) ? a : b;
}

/**
 * The visibility a post gets when the user chooses none: the preference, and for a reply the narrower of
 * the preference and the replied-to post's (a reply to a `local` post is `local` by default, ADR-0017).
 */
export function defaultVisibility(preference: Visibility, replyTo: Status | undefined): Visibility {
  return replyTo === undefined ? preference : narrower(preference, replyTo.visibility);
}

/**
 * The visibility a post gets: the chosen one when the account's posting limits keep it, and their
 * ceiling otherwise. The server decides the limits and applies the same rule when it posts (PostingLimits
 * in api/openapi.yaml); without limits (not loaded yet) the choice stands.
 */
export function postedVisibility(chosen: Visibility, limits: PostingLimits | undefined): Visibility {
  return limits === undefined || limits.visibilities.includes(chosen) ? chosen : limits.visibility_ceiling;
}

/**
 * The mentions a reply starts with: the author of the replied-to post and the accounts it mentions,
 * without the viewer, each once.
 */
export function replyMentions(replyTo: Status, viewerId: string): string {
  const accts = [
    ...(replyTo.account.id === viewerId ? [] : [replyTo.account.acct]),
    ...replyTo.mentions.filter((m) => m.id !== viewerId).map((m) => m.acct),
  ];
  const unique = [...new Set(accts)];
  return unique.length === 0 ? "" : `${unique.map((acct) => `@${acct}`).join(" ")} `;
}

/**
 * `@username@domain`, as the server parses a mention with a domain (ParseText in
 * internal/app/content/text.go): the `@` follows no letter, digit, mark, `_`, `/` or `=`, and the domain
 * ends in a letter or digit.
 */
const domainMentionPattern = /(?<![\p{L}\p{N}\p{M}_/=])@([A-Za-z0-9_]+)@([A-Za-z0-9.-]*[A-Za-z0-9])/gu;

/**
 * The handles (`username@domain`, lowercase, each once) of the mentions in `text` that name a domain:
 * those that can be accounts of other servers. A mention without a domain is a local account.
 */
export function domainMentions(text: string): string[] {
  const handles = Array.from(text.matchAll(domainMentionPattern), (m) => `${m[1] ?? ""}@${m[2] ?? ""}`.toLowerCase());
  return [...new Set(handles)];
}

/** Characters as the server counts them: Unicode code points (data-model.md). */
export function countCharacters(text: string): number {
  return Array.from(text).length;
}

/** The most images a post can have (StatusMediaInput in api/openapi.yaml). */
export const maxAttachments = 4;

/** The longest alternative text (StatusMediaItem.description in api/openapi.yaml). */
export const maxDescriptionLength = 1500;

/** The image types the server accepts (the uploadMedia operation). */
export const acceptedImageTypes: readonly string[] = ["image/jpeg", "image/png", "image/gif", "image/webp"];
