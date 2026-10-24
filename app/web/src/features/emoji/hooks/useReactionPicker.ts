// SPDX-License-Identifier: AGPL-3.0-only
// The picker of the emoji to react to a post with (ADR-0052 §2, §5): a few common Unicode emoji, and the custom emoji
// of this server that the post form's picker offers, with the custom emoji used often on this browser first.
import type { EmojiPickerLabels, EmojiPickerSection, PickerCustomEmoji } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { customEmojisQueryOptions } from "../api/customEmojis";
import { customEmojiKey, defaultFrequentEmojis, rememberEmoji, shortcodeOf, useFrequentEmojis } from "./frequentEmojis";
import { customEmojiSections, emojisOfKeys, frequentSection, pickerLabels } from "./pickerSections";

/** The emoji that a reaction is made with: a Unicode emoji, or a custom emoji of this server. */
export type ReactionChoice = { unicode: string } | { custom: PickerCustomEmoji };

export type ReactionPickerControl = {
  /** The custom emoji used often, the common Unicode emoji, then the custom emoji by category. */
  sections: readonly EmojiPickerSection[];
  labels: EmojiPickerLabels;
  /** Counts a picked custom emoji as used. */
  remember: (emoji: PickerCustomEmoji) => void;
};

/** The emoji of the picker of reactions. */
export function useReactionPicker(): ReactionPickerControl {
  const api = useApi();
  const t = useT();
  const { data } = useQuery(customEmojisQueryOptions(api));
  const frequentKeys = useFrequentEmojis();
  const emojis = data?.items ?? [];
  // Only the custom emoji used often come first until the picker of reactions offers every Unicode emoji (ADR-0091 §7).
  const frequent = emojisOfKeys(
    frequentKeys.filter((key) => shortcodeOf(key) !== undefined),
    emojis,
    undefined,
  );
  // Each common emoji is named by itself until then too.
  const unicode: EmojiPickerSection = {
    id: "unicode",
    heading: t("client:reaction.picker.unicode"),
    icon: "smileys-emotion",
    emojis: defaultFrequentEmojis.map((emoji) => ({ unicode: emoji, name: emoji })),
  };
  return {
    sections: [frequentSection(frequent, t), unicode, ...customEmojiSections(emojis, t)],
    labels: pickerLabels(t("client:reaction.picker.grid"), t),
    remember: (emoji) => {
      rememberEmoji(customEmojiKey(emoji.shortcode));
    },
  };
}
