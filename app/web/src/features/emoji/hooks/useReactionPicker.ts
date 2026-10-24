// SPDX-License-Identifier: AGPL-3.0-only
// The picker of the emoji to react to a post with (ADR-0052 §2, §5): a few common Unicode emoji, and the custom emoji
// of this server that the post form's picker offers, with those picked recently on this browser first.
import type { EmojiPickerLabels, EmojiPickerSection, PickerCustomEmoji } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { customEmojisQueryOptions } from "../api/customEmojis";
import { customEmojiSections, pickerLabels } from "./pickerSections";
import { rememberEmoji, useRecentShortcodes } from "./recentEmojis";

/**
 * The Unicode emoji that the picker of reactions offers, those that Misskey offers by default and a few more. Each is
 * fully qualified, the form the server counts reactions in.
 */
const unicodeReactions: readonly string[] = ["👍", "❤️", "😆", "🤔", "😮", "🎉", "😢", "😇", "🙏", "👀", "🔥", "💯"];

/** The emoji that a reaction is made with: a Unicode emoji, or a custom emoji of this server. */
export type ReactionChoice = { unicode: string } | { custom: PickerCustomEmoji };

export type ReactionPickerControl = {
  /** The custom emoji picked recently, the common Unicode emoji, then the custom emoji by category. */
  sections: readonly EmojiPickerSection[];
  labels: EmojiPickerLabels;
  /** Remembers a picked custom emoji as recent. */
  remember: (emoji: PickerCustomEmoji) => void;
};

/** The emoji of the picker of reactions. */
export function useReactionPicker(): ReactionPickerControl {
  const api = useApi();
  const t = useT();
  const { data } = useQuery(customEmojisQueryOptions(api));
  const recentCodes = useRecentShortcodes();
  const emojis = data?.items ?? [];
  const byShortcode = new Map(emojis.map((emoji) => [emoji.shortcode, emoji]));
  const recent = recentCodes.flatMap((code) => byShortcode.get(code) ?? []);
  const [first, ...rest] = customEmojiSections(emojis, recent, t);
  // Each common emoji is named by itself until the picker of reactions offers every Unicode emoji (ADR-0091 §7).
  const unicode: EmojiPickerSection = {
    id: "unicode",
    heading: t("client:reaction.picker.unicode"),
    icon: "smileys-emotion",
    emojis: unicodeReactions.map((emoji) => ({ unicode: emoji, name: emoji })),
  };
  // The recent emoji come first, then the Unicode ones, then the custom emoji by category.
  const custom = first === undefined ? [] : [first, ...rest];
  const sections = first?.id === "recent" ? [first, unicode, ...rest] : [unicode, ...custom];
  return {
    sections,
    labels: pickerLabels(t("client:reaction.picker.grid"), t),
    remember: (emoji) => {
      rememberEmoji(emoji.shortcode);
    },
  };
}
