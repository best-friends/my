// SPDX-License-Identifier: AGPL-3.0-only
// A picker of custom emoji (ADR-0051 §4): a search field over a grid of emoji grouped by category, with the
// emoji used recently first and, for the reactions (ADR-0052 §5), a group of Unicode emoji before the custom ones. The grid is React Aria's GridList in its grid layout (role="grid", one row per
// emoji): Tab moves from the search field into the grid and back, the arrow keys move between emoji in two
// dimensions (left and right follow the writing direction), Home / End go to the first and last, and Enter,
// Space or a press picks one (WAI-ARIA APG, grid pattern). Each custom emoji is named by its `:shortcode:`, the
// text that picking it inserts, and each Unicode emoji by itself. The picker holds no data: the app passes the emoji and what to do with the one
// picked, so that posts, profiles and reactions use the same picker.
import { useState } from "react";
import { GridList, GridListHeader, GridListItem, GridListSection } from "react-aria-components";
import { tv } from "tailwind-variants";
import { CustomEmoji, type CustomEmojiImage } from "./CustomEmoji";
import { TextField } from "./TextField";

/** An emoji the picker offers: the image of a custom emoji, the category it is grouped in, and other names it is found by. */
export type PickerEmoji = CustomEmojiImage & {
  /** The category of the emoji; those without one are grouped first, under `labels.uncategorized`. */
  category?: string | undefined;
  /** Other names that the search finds the emoji by. */
  aliases?: readonly string[] | undefined;
};

/** The words of the picker, which the app localizes (web-client.md A2). */
export type EmojiPickerLabels = {
  /** The label of the search field. */
  search: string;
  /** The accessible name of the grid of emoji. */
  grid: string;
  /** The heading of the emoji used recently. */
  recent: string;
  /** The heading of the emoji without a category. */
  uncategorized: string;
  /** What the grid says when no emoji matches the search, or there are none. */
  empty: string;
  /** The heading of the Unicode emoji, when the picker offers them. */
  unicode?: string | undefined;
};

export type EmojiPickerProps = {
  /** The emoji to offer, in the order to show them within each category. */
  emojis: readonly PickerEmoji[];
  /** The emoji used recently, most recent first; they come first while nothing is searched. */
  recent?: readonly PickerEmoji[] | undefined;
  /** Called with the custom emoji the user picks. */
  onSelect: (emoji: PickerEmoji) => void;
  /**
   * Unicode emoji to offer before the custom ones, under `labels.unicode`, while nothing is searched or the search
   * is one of them; none by default.
   */
  unicode?: readonly string[] | undefined;
  /** Called with the Unicode emoji the user picks. */
  onSelectUnicode?: ((emoji: string) => void) | undefined;
  labels: EmojiPickerLabels;
  /** Whether animated emoji may move when the user allows motion; the default is the app's `AutoplayContext`. */
  allowMotion?: boolean | undefined;
  /** Focus the search field when the picker appears (in a popover the user just opened). */
  autoFocus?: boolean | undefined;
};

const picker = tv({
  slots: {
    root: "flex min-w-0 flex-col gap-2",
    grid: "flex max-h-64 min-w-0 flex-col gap-2 overflow-y-auto overscroll-contain outline-none",
    section: "flex flex-wrap gap-1",
    header: "basis-full text-sm text-fg-muted",
    // As large as a tap target needs (D4); the emoji is as high as the text of the cell.
    item: [
      "flex size-10 cursor-default items-center justify-center rounded-md text-2xl outline-none pointer-coarse:size-11",
      "data-focus-visible:focus-ring data-hovered:bg-surface-sunken data-pressed:bg-surface-sunken",
    ],
    empty: "p-2 text-sm text-fg-muted",
  },
});

/** The search text as the emoji are compared with it: without colons around it, in lower case. */
function normalize(text: string): string {
  return text.trim().replace(/^:+|:+$/gu, "").toLowerCase();
}

/** Whether an emoji matches the normalized search text by its shortcode or one of its aliases. */
function matches(emoji: PickerEmoji, query: string): boolean {
  if (query === "") return true;
  return [emoji.shortcode, ...(emoji.aliases ?? [])].some((name) => name.toLowerCase().includes(query));
}

/** One emoji of the grid: a custom emoji, or a Unicode emoji. */
type Item = { id: string; emoji: PickerEmoji } | { id: string; unicode: string };

type Group = { id: string; label: string; items: Item[] };

/**
 * The groups of the grid: the recent emoji while nothing is searched, the Unicode emoji while nothing is searched or
 * the search is one of them, then the emoji by category in their order.
 */
function groupsOf(
  emojis: readonly PickerEmoji[],
  recent: readonly PickerEmoji[],
  unicode: readonly string[],
  search: string,
  query: string,
  labels: EmojiPickerLabels,
): Group[] {
  const groups: Group[] = [];
  if (query === "" && recent.length > 0) {
    groups.push({ id: "recent", label: labels.recent, items: recent.map((emoji) => ({ id: `recent:${emoji.shortcode}`, emoji })) });
  }
  const unicodeItems = unicode.filter((emoji) => query === "" || emoji === search.trim()).map((emoji) => ({ id: `unicode:${emoji}`, unicode: emoji }));
  if (unicodeItems.length > 0) groups.push({ id: "unicode", label: labels.unicode ?? "", items: unicodeItems });
  const byCategory = new Map<string, Group>();
  for (const emoji of emojis) {
    if (!matches(emoji, query)) continue;
    const category = emoji.category ?? "";
    let group = byCategory.get(category);
    if (group === undefined) {
      group = { id: `category:${category}`, label: category === "" ? labels.uncategorized : category, items: [] };
      byCategory.set(category, group);
    }
    group.items.push({ id: `emoji:${emoji.shortcode}`, emoji });
  }
  // The emoji without a category come first, as the server lists them.
  const uncategorized = byCategory.get("");
  if (uncategorized !== undefined) groups.push(uncategorized);
  for (const [category, group] of byCategory) if (category !== "") groups.push(group);
  return groups;
}

/** No recent emoji: the default, one array so that it is the same on every render. */
const noRecentEmojis: readonly PickerEmoji[] = [];

/** No Unicode emoji: the default. */
const noUnicodeEmojis: readonly string[] = [];

/**
 * A search field and a grid of custom emoji, and of Unicode emoji when given; picking one calls `onSelect`, or
 * `onSelectUnicode` for a Unicode emoji.
 */
export function EmojiPicker({
  emojis,
  recent = noRecentEmojis,
  unicode = noUnicodeEmojis,
  onSelect,
  onSelectUnicode,
  labels,
  allowMotion,
  autoFocus = false,
}: EmojiPickerProps) {
  const styles = picker();
  const [search, setSearch] = useState("");
  const query = normalize(search);
  const groups = groupsOf(emojis, recent, unicode, search, query, labels);
  const byId = new Map(groups.flatMap((group) => group.items.map((item) => [item.id, item] as const)));
  return (
    <div className={styles.root()}>
      <TextField type="search" label={labels.search} value={search} onChange={setSearch} autoFocus={autoFocus} />
      <GridList
        aria-label={labels.grid}
        layout="grid"
        className={styles.grid()}
        onAction={(key) => {
          const item = byId.get(String(key));
          if (item === undefined) return;
          if ("unicode" in item) onSelectUnicode?.(item.unicode);
          else onSelect(item.emoji);
        }}
        renderEmptyState={() => <p className={styles.empty()}>{labels.empty}</p>}
      >
        {groups.map((group) => (
          <GridListSection key={group.id} id={group.id} className={styles.section()}>
            <GridListHeader className={styles.header()}>{group.label}</GridListHeader>
            {group.items.map((item) =>
              "unicode" in item ? (
                <GridListItem key={item.id} id={item.id} textValue={item.unicode} className={styles.item()}>
                  {item.unicode}
                </GridListItem>
              ) : (
                <GridListItem key={item.id} id={item.id} textValue={`:${item.emoji.shortcode}:`} className={styles.item()}>
                  <CustomEmoji emoji={item.emoji} allowMotion={allowMotion} />
                </GridListItem>
              ),
            )}
          </GridListSection>
        ))}
      </GridList>
    </div>
  );
}
