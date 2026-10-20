// SPDX-License-Identifier: AGPL-3.0-only
// The picker of the emoji to react to a post with (ADR-0052 §2, §5): a few common Unicode emoji, and the custom emoji
// of this server that the post form's picker offers, with those picked recently on this browser first.
import type { EmojiPickerLabels, PickerEmoji } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { customEmojisQueryOptions } from "../api/customEmojis";
import { rememberEmoji, useRecentShortcodes } from "./recentEmojis";

/**
 * The Unicode emoji that the picker of reactions offers, those that Misskey offers by default and a few more. Each is
 * fully qualified, the form the server counts reactions in.
 */
const unicodeReactions: readonly string[] = ["👍", "❤️", "😆", "🤔", "😮", "🎉", "😢", "😇", "🙏", "👀", "🔥", "💯"];

/** The emoji that a reaction is made with: a Unicode emoji, or a custom emoji of this server. */
export type ReactionChoice = { unicode: string } | { custom: PickerEmoji };

export type ReactionPickerControl = {
  emojis: readonly PickerEmoji[];
  recent: readonly PickerEmoji[];
  unicode: readonly string[];
  labels: EmojiPickerLabels;
  /** Remembers a picked custom emoji as recent. */
  remember: (emoji: PickerEmoji) => void;
};

/** The emoji of the picker of reactions. */
export function useReactionPicker(): ReactionPickerControl {
  const api = useApi();
  const t = useT();
  const { data } = useQuery(customEmojisQueryOptions(api));
  const recentCodes = useRecentShortcodes();
  const emojis = data?.items ?? [];
  const byShortcode = new Map(emojis.map((emoji) => [emoji.shortcode, emoji]));
  return {
    emojis,
    recent: recentCodes.flatMap((code) => byShortcode.get(code) ?? []),
    unicode: unicodeReactions,
    labels: {
      search: t("client:emoji.picker.search"),
      grid: t("client:reaction.picker.grid"),
      recent: t("client:emoji.picker.recent"),
      uncategorized: t("client:emoji.picker.uncategorized"),
      empty: t("client:emoji.picker.empty"),
      unicode: t("client:reaction.picker.unicode"),
    },
    remember: (emoji) => {
      rememberEmoji(emoji.shortcode);
    },
  };
}
