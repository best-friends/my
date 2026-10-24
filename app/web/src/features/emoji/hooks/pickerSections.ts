// SPDX-License-Identifier: AGPL-3.0-only
// The sections and words that both pickers of custom emoji (the post form's and the reactions') pass to the
// EmojiPicker of @my/ui: the emoji picked recently, then the custom emoji by category, those without one first as
// the server lists them (ADR-0051 §4).
import type { EmojiPickerLabels, EmojiPickerSection, PickerCustomEmoji } from "@my/ui";
import type { Translate } from "../../../lib/i18n";

/** A custom emoji of the server, with the category it is listed under. */
type ServerEmoji = PickerCustomEmoji & { category?: string | undefined };

/** The section of the emoji used recently, and then one section per category of `emojis`. */
export function customEmojiSections(emojis: readonly ServerEmoji[], recent: readonly ServerEmoji[], t: Translate): EmojiPickerSection[] {
  const sections: EmojiPickerSection[] = [];
  if (recent.length > 0) sections.push({ id: "recent", heading: t("client:emoji.picker.recent"), icon: "frequent", emojis: recent });
  const byCategory = new Map<string, ServerEmoji[]>();
  for (const emoji of emojis) {
    const category = emoji.category ?? "";
    const list = byCategory.get(category) ?? [];
    list.push(emoji);
    byCategory.set(category, list);
  }
  const uncategorized = byCategory.get("");
  if (uncategorized !== undefined) {
    sections.push({ id: "custom:", heading: t("client:emoji.picker.uncategorized"), icon: "custom", emojis: uncategorized });
  }
  for (const [category, list] of byCategory) {
    if (category !== "") sections.push({ id: `custom:${category}`, heading: category, icon: "custom", emojis: list });
  }
  return sections;
}

/** The words of the picker, with the accessible name of its grid. */
export function pickerLabels(grid: string, t: Translate): EmojiPickerLabels {
  return {
    search: t("client:emoji.picker.search"),
    grid,
    categories: t("client:emoji.picker.categories"),
    results: t("client:emoji.picker.results"),
    resultCount: (count) => t("client:emoji.picker.result_count", { count }),
    empty: t("client:emoji.picker.empty"),
  };
}
