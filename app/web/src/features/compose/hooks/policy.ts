// SPDX-License-Identifier: AGPL-3.0-only
// The rules of a new post that the client applies before the server does: the visibilities and their
// order (ADR-0017), the default of a reply, the mentions a reply starts with, and the character count.
import type { Status, Visibility } from "../api/compose";

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
