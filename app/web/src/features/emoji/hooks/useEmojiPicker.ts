// SPDX-License-Identifier: AGPL-3.0-only
// The picker of custom emoji next to a text field (ADR-0051 §4): the emoji the server offers, those picked recently
// on this browser, and putting a picked one into the field at its caret.
import type { EmojiPickerLabels, PickerEmoji } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { customEmojisQueryOptions } from "../api/customEmojis";
import { insertShortcode } from "./insertShortcode";
import { rememberEmoji, useRecentShortcodes } from "./recentEmojis";

export type EmojiTarget = {
  /** The id of the input or text area that a picked emoji goes into. */
  fieldId: string;
  /** The field's value. */
  value: string;
  /** Sets the field's value. */
  onChange: (value: string) => void;
};

export type EmojiPickerControl = {
  /** Whether the server offers emoji; without any, the picker is not shown. */
  available: boolean;
  emojis: readonly PickerEmoji[];
  recent: readonly PickerEmoji[];
  labels: EmojiPickerLabels;
  /** Puts `emoji` into the field at its caret, remembers it as recent and closes the picker with `close`. */
  select: (emoji: PickerEmoji, close: () => void) => void;
};

/** The field of `fieldId` when it is a text input or a text area. */
function textFieldOf(fieldId: string): HTMLInputElement | HTMLTextAreaElement | null {
  const element = document.getElementById(fieldId);
  return element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement ? element : null;
}

/** The picker of custom emoji for the field `target`. */
export function useEmojiPicker(target: EmojiTarget): EmojiPickerControl {
  const api = useApi();
  const t = useT();
  const { data } = useQuery(customEmojisQueryOptions(api));
  const recentCodes = useRecentShortcodes();
  const emojis = data?.items ?? [];
  const byShortcode = new Map(emojis.map((emoji) => [emoji.shortcode, emoji]));
  const recent = recentCodes.flatMap((code) => byShortcode.get(code) ?? []);
  return {
    available: emojis.length > 0,
    emojis,
    recent,
    labels: {
      search: t("client:emoji.picker.search"),
      grid: t("client:emoji.picker.grid"),
      recent: t("client:emoji.picker.recent"),
      uncategorized: t("client:emoji.picker.uncategorized"),
      empty: t("client:emoji.picker.empty"),
    },
    select: (emoji, close) => {
      const field = textFieldOf(target.fieldId);
      const end = field?.selectionEnd ?? target.value.length;
      const start = field?.selectionStart ?? end;
      const next = insertShortcode(target.value, start, end, emoji.shortcode);
      target.onChange(next.value);
      rememberEmoji(emoji.shortcode);
      close();
      // Typing goes on in the field after the emoji. The popover returns focus to its button once it has closed,
      // in an animation frame (React Aria FocusScope), so focus moves to the field after that.
      setTimeout(() => {
        requestAnimationFrame(() => {
          const current = textFieldOf(target.fieldId);
          current?.focus();
          current?.setSelectionRange(next.caret, next.caret);
        });
      }, 0);
    },
  };
}
