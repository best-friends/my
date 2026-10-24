// SPDX-License-Identifier: AGPL-3.0-only
// The emoji picker of a text field (ADR-0091; ADR-0051 §4): the emoji used often, the custom emoji of the server and
// every Unicode emoji, in the skin tone chosen on this browser, and putting a picked one into the field at its caret.
import type { EmojiPickerLabels, EmojiPickerSection, EmojiPickerSelection, SkinTone } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useLocale, useT } from "../../../lib/i18n";
import { skinToneKey, useStorage } from "../../../lib/storage";
import { customEmojisQueryOptions } from "../api/customEmojis";
import { customEmojiKey, defaultFrequentEmojis, rememberEmoji, useFrequentEmojis } from "./frequentEmojis";
import { insertShortcode, insertText } from "./insertShortcode";
import { customEmojiSections, emojisOfKeys, frequentSection, pickerLabels, skinToneLabels, unicodeEmojiSections } from "./pickerSections";
import { preloadUnicodeEmoji, useUnicodeEmoji } from "./unicodeEmoji";

export type EmojiTarget = {
  /** The id of the input or text area that a picked emoji goes into. */
  fieldId: string;
  /** The field's value. */
  value: string;
  /** Sets the field's value. */
  onChange: (value: string) => void;
};

export type EmojiPickerControl = {
  /**
   * `loading` until the Unicode emoji have arrived, `error` when they could not be loaded (the custom emoji are still
   * offered), `ready` otherwise.
   */
  status: "loading" | "ready" | "error";
  /** Starts loading the Unicode emoji (from the button's hover, focus and opening), unless they are loaded. */
  preload: () => void;
  /** The emoji used often, the custom emoji by category, then the Unicode emoji by group. */
  sections: readonly EmojiPickerSection[];
  labels: EmojiPickerLabels;
  skinTone: SkinTone;
  setSkinTone: (tone: SkinTone) => void;
  /**
   * Puts the emoji of `selection` into the field at its caret (a Unicode emoji as itself, a custom emoji as
   * `:shortcode:` with the spaces it needs) and counts it as used. Unless the selection asks to keep the picker open,
   * closes it with `close` and moves focus back to the field, after the emoji.
   */
  select: (selection: EmojiPickerSelection, close: () => void) => void;
};

/** The field of `fieldId` when it is a text input or a text area. */
function textFieldOf(fieldId: string): HTMLInputElement | HTMLTextAreaElement | null {
  const element = document.getElementById(fieldId);
  return element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement ? element : null;
}

/** The emoji picker for the field `target`. */
export function useEmojiPicker(target: EmojiTarget): EmojiPickerControl {
  const api = useApi();
  const t = useT();
  const locale = useLocale();
  const { data } = useQuery(customEmojisQueryOptions(api));
  const unicode = useUnicodeEmoji(locale);
  const frequentKeys = useFrequentEmojis();
  const skinTone = useStorage(skinToneKey);
  const custom = data?.items ?? [];
  const unicodeSet = unicode.status === "ready" ? unicode.emoji : undefined;
  const frequent = emojisOfKeys(frequentKeys.length > 0 ? frequentKeys : defaultFrequentEmojis, custom, unicodeSet);
  return {
    status: unicode.status === "ready" || unicode.status === "error" ? unicode.status : "loading",
    preload: () => {
      preloadUnicodeEmoji(locale);
    },
    sections: [frequentSection(frequent, t), ...customEmojiSections(custom, t), ...(unicodeSet === undefined ? [] : unicodeEmojiSections(unicodeSet, t))],
    labels: { ...pickerLabels(t("client:emoji.picker.grid"), t), ...skinToneLabels(t) },
    skinTone,
    setSkinTone: (tone) => {
      skinToneKey.write(tone);
    },
    select: (selection, close) => {
      const field = textFieldOf(target.fieldId);
      const end = field?.selectionEnd ?? target.value.length;
      const start = field?.selectionStart ?? end;
      const next =
        selection.kind === "custom"
          ? insertShortcode(target.value, start, end, selection.emoji.shortcode)
          : insertText(target.value, start, end, selection.text);
      target.onChange(next.value);
      // A Unicode emoji counts without its skin tone, so that it is one emoji whichever tone it was picked in.
      rememberEmoji(selection.kind === "custom" ? customEmojiKey(selection.emoji.shortcode) : selection.emoji.unicode);
      if (selection.keepOpen) {
        // Focus stays on the emoji in the picker; the next one goes after this one. Setting the value moved the caret
        // of the field to its end once React rendered, so it is put back after the emoji.
        setTimeout(() => {
          textFieldOf(target.fieldId)?.setSelectionRange(next.caret, next.caret);
        }, 0);
        return;
      }
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
