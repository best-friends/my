// SPDX-License-Identifier: AGPL-3.0-only
// The emoji picker of posts, profiles and reactions (ADR-0091 §3, §5, §6; ADR-0051 §4; ADR-0052 §5). It holds no data
// (ADR-0031): the app passes the sections to show (the emoji used often, the custom emoji by category, the Unicode
// groups), each with its heading, its icon and its emoji, and what to do with the one picked.
//
// From the top: a search field and the skin tone menu button, a toolbar of one button per section, and one grid of all
// the sections, scrolled as a whole. The grid is React Aria's GridList in its grid layout (role="grid", one row per
// emoji; WAI-ARIA APG, grid pattern): Tab goes from the search field to the skin tone button, the toolbar and the grid,
// the arrow keys move between emoji in two dimensions (left and right follow the writing direction), Home / End go to
// the first and last, and Enter, Space or a press picks one. Picking with Shift held (Shift+Enter, Shift+click) tells
// the app to keep the picker open, and focus stays in the grid; a touch never does.
//
// The section buttons are not APG tabs, since there are no panels to switch: a button scrolls the grid to its
// section's heading and moves focus to the section's first emoji, and the button of the section at the top of the
// grid has aria-current="true", which follows the scrolling (an IntersectionObserver on the headings). The grid
// scrolls smoothly only under motion-safe:.
//
// A search ranks the emoji of every section by `searchEmojis` (exported for the completion of ADR-0092) into one
// section of results, disables the section buttons, and announces the count in a role="status" region once typing
// pauses. Every emoji is rendered, without virtualization (ADR-0091 §3: measure first).
import { type JSX, useEffect, useRef, useState } from "react";
import {
  GridList,
  GridListHeader,
  GridListItem,
  GridListSection,
  type Key,
  type PressEvent,
  type Selection,
  Toolbar,
} from "react-aria-components";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { CustomEmoji, type CustomEmojiImage } from "./CustomEmoji";
import {
  ActivityEmojiIcon,
  CheckIcon,
  CustomEmojiIcon,
  EmojiIcon,
  FlagEmojiIcon,
  FoodEmojiIcon,
  FrequentEmojiIcon,
  NatureEmojiIcon,
  ObjectEmojiIcon,
  PeopleEmojiIcon,
  SymbolEmojiIcon,
  TravelEmojiIcon,
} from "./Icons";
import { Menu, MenuItem, MenuTrigger } from "./Menu";
import { countAnnounceDelay } from "./TextArea";
import { TextField } from "./TextField";
import { Tooltip, TooltipTrigger } from "./Tooltip";

/** A custom emoji the picker offers: its image and `:shortcode:`, and other names that the search finds it by. */
export type PickerCustomEmoji = CustomEmojiImage & {
  /** Other names that the search finds the emoji by, ranked as keywords. */
  aliases?: readonly string[] | undefined;
};

/** One skin tone variant of a Unicode emoji. */
export type PickerSkinVariant = {
  /** The fully-qualified emoji with its skin tone modifiers ("👋🏽"). */
  unicode: string;
  /** Its name in the locale shown, which names its row (CLDR annotationsDerived: "waving hand: medium skin tone"). */
  name: string;
};

/** A Unicode emoji the picker offers. */
export type PickerUnicodeEmoji = {
  /** The fully-qualified emoji without a skin tone ("👋"). */
  unicode: string;
  /** Its short name in the locale shown (CLDR), which names its row and ranks first in the search. */
  name: string;
  /** Other words that the search finds it by (the CLDR keywords, the `en` name), ranked as keywords. */
  keywords?: readonly string[] | undefined;
  /**
   * Its skin tone variants, in any order; the picker shows and picks the one whose every modifier is the chosen tone
   * (`skinToneVariant`), so an emoji of several people gets the same tone for all of them.
   */
  skins?: readonly PickerSkinVariant[] | undefined;
};

/** An emoji the picker offers: a custom emoji (it has a `shortcode`) or a Unicode emoji (it has `unicode`). */
export type PickerEmoji = PickerCustomEmoji | PickerUnicodeEmoji;

/**
 * The icon of a section's button: `frequent` for the emoji used often, `custom` for the custom emoji, and the id of
 * the group of emoji-test.txt for the Unicode emoji of that group.
 */
export type EmojiSectionIcon =
  | "frequent"
  | "custom"
  | "smileys-emotion"
  | "people-body"
  | "animals-nature"
  | "food-drink"
  | "travel-places"
  | "activities"
  | "objects"
  | "symbols"
  | "flags";

/** One section of the grid, in the order the app lists them. */
export type EmojiPickerSection = {
  /** Unique among the sections, and the same from render to render. */
  id: string;
  /** The heading of the section in the grid, which is also the name and tooltip of its button. */
  heading: string;
  icon: EmojiSectionIcon;
  /** The emoji in the order to show them. An emoji may also be in another section (the frequent ones). */
  emojis: readonly PickerEmoji[];
};

/** The skin tones of the picker: the default yellow and the five Fitzpatrick modifiers (U+1F3FB..U+1F3FF). */
export type SkinTone = "none" | "light" | "medium-light" | "medium" | "medium-dark" | "dark";

/** The skin tones in the order the menu lists them. */
export const skinTones: readonly SkinTone[] = ["none", "light", "medium-light", "medium", "medium-dark", "dark"];

/** The modifier of each skin tone; the default has none. */
const skinToneModifiers: Readonly<Record<SkinTone, string>> = {
  none: "",
  light: "\u{1F3FB}",
  "medium-light": "\u{1F3FC}",
  medium: "\u{1F3FD}",
  "medium-dark": "\u{1F3FE}",
  dark: "\u{1F3FF}",
};

const modifierPattern = /[\u{1F3FB}-\u{1F3FF}]/gu;

/** The raised hand that the skin tone button and menu show each tone with. */
const toneHand = "✋";

/**
 * The variant of `emoji` in `tone`: the one whose every skin tone modifier is the tone's, or `undefined` for the
 * default tone and for an emoji without such a variant.
 */
export function skinToneVariant(emoji: PickerUnicodeEmoji, tone: SkinTone): PickerSkinVariant | undefined {
  if (tone === "none") return undefined;
  const modifier = skinToneModifiers[tone];
  return emoji.skins?.find((variant) => {
    const modifiers = variant.unicode.match(modifierPattern);
    return modifiers !== null && modifiers.every((found) => found === modifier);
  });
}

/** The emoji picked, and whether the app should keep the picker open (picked with Shift held, not by touch). */
export type EmojiPickerSelection =
  | { kind: "custom"; emoji: PickerCustomEmoji; keepOpen: boolean }
  | {
      kind: "unicode";
      emoji: PickerUnicodeEmoji;
      /** The emoji to insert: its variant in the chosen skin tone when it has one, or else `emoji.unicode`. */
      text: string;
      keepOpen: boolean;
    };

/** The words of the picker, which the app localizes (web-client/README.md A2). */
export type EmojiPickerLabels = {
  /** The label of the search field. */
  search: string;
  /** The accessible name of the grid of emoji. */
  grid: string;
  /** The accessible name of the toolbar of section buttons. */
  categories: string;
  /** The heading of the one section of search results. */
  results: string;
  /** What the status region says once typing pauses: the number of emoji found ("12 emoji"). */
  resultCount: (count: number) => string;
  /** What the grid says when no emoji matches the search, or there are none. */
  empty: string;
  /** The name and tooltip of the skin tone button and the name of its menu; needed for the button to show. */
  skinTone?: string | undefined;
  /** The name of each skin tone in the menu; needed for the button to show. */
  skinTones?: Readonly<Record<SkinTone, string>> | undefined;
};

export type EmojiPickerProps = {
  /** The sections of the grid, in order. A section without emoji is left out, and so is its button. */
  sections: readonly EmojiPickerSection[];
  /**
   * Called with the emoji picked. Unless `keepOpen` is true, the app closes the picker (and returns focus to its
   * field); with `keepOpen`, focus stays on the emoji picked so that the user can go on picking.
   */
  onSelect: (selection: EmojiPickerSelection) => void;
  /** The skin tone the Unicode emoji with variants are shown and picked in; `none` (the default yellow) by default. */
  skinTone?: SkinTone | undefined;
  /**
   * Called with the skin tone chosen in the menu; the app keeps it and passes it back as `skinTone`. Without it, or
   * without `labels.skinTone` and `labels.skinTones`, there is no skin tone button.
   */
  onSkinToneChange?: ((tone: SkinTone) => void) | undefined;
  labels: EmojiPickerLabels;
  /** Whether animated emoji may move when the user allows motion; the default is the app's `AutoplayContext`. */
  allowMotion?: boolean | undefined;
  /** Focus the search field when the picker appears (in a popover the user just opened). */
  autoFocus?: boolean | undefined;
};

const picker = tv({
  slots: {
    root: "flex min-w-0 flex-col gap-2",
    top: "flex min-w-0 items-end gap-2",
    tone: "text-xl",
    toneItem: "flex min-w-0 grow items-center gap-2",
    toneHand: "text-xl",
    toneCheck: "ms-auto size-4 shrink-0",
    toolbar: "flex min-w-0 flex-wrap gap-0.5",
    // The section at the top of the grid shows by a ground behind its icon, not by the color of the icon alone (WCAG 1.4.1).
    tab: "aria-current:bg-soft aria-current:text-accent",
    grid: [
      "flex max-h-64 min-w-0 flex-col gap-2 overflow-y-auto overscroll-contain outline-none",
      // The section buttons scroll the grid; it moves smoothly only when the user allows motion (web-client/README.md D6).
      "motion-safe:scroll-smooth",
    ],
    section: "flex flex-wrap gap-1",
    header: "basis-full text-xs font-extrabold text-fg-muted",
    // As large as a tap target needs (D4); the emoji is as high as the text of the cell.
    item: [
      "flex size-10 cursor-default items-center justify-center rounded-md text-2xl outline-none pointer-coarse:size-11",
      "data-focus-visible:focus-ring data-hovered:bg-surface-sunken data-pressed:bg-surface-sunken",
    ],
    empty: "p-2 text-sm text-fg-muted",
    status: "sr-only",
  },
});

/**
 * The search text as the emoji are compared with it: NFKC, lower case, katakana as hiragana, without spaces or
 * colons around it. The names and keywords of the emoji are compared in the same form.
 */
export function normalizeEmojiSearch(text: string): string {
  return text
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[ァ-ヶ]/gu, (katakana) => String.fromCodePoint((katakana.codePointAt(0) ?? 0) - 0x60))
    .trim()
    .replace(/^:+|:+$/gu, "");
}

type SearchForms = { emoji: string | undefined; name: string; keywords: readonly string[] };

/** The normalized names of each emoji object, computed once: the search compares every emoji on every keystroke. */
const searchForms = new WeakMap<PickerEmoji, SearchForms>();

function searchFormsOf(emoji: PickerEmoji): SearchForms {
  let forms = searchForms.get(emoji);
  if (forms === undefined) {
    forms =
      "unicode" in emoji
        ? { emoji: normalizeEmojiSearch(emoji.unicode), name: normalizeEmojiSearch(emoji.name), keywords: (emoji.keywords ?? []).map(normalizeEmojiSearch) }
        : { emoji: undefined, name: normalizeEmojiSearch(emoji.shortcode), keywords: (emoji.aliases ?? []).map(normalizeEmojiSearch) };
    searchForms.set(emoji, forms);
  }
  return forms;
}

/**
 * How well `emoji` matches the normalized search text `query` (`normalizeEmojiSearch`), best first: 0 when its name
 * (the shortcode of a custom emoji) is the text or it is the emoji itself, 1 when its name starts with it, 2 when a
 * keyword (an alias of a custom emoji) starts with it, 3 when its name or a keyword contains it; `undefined` when it
 * does not match or the text is empty.
 */
export function emojiMatchRank(emoji: PickerEmoji, query: string): number | undefined {
  if (query === "") return undefined;
  const forms = searchFormsOf(emoji);
  if (forms.name === query || forms.emoji === query) return 0;
  if (forms.name.startsWith(query)) return 1;
  if (forms.keywords.some((keyword) => keyword.startsWith(query))) return 2;
  if (forms.name.includes(query) || forms.keywords.some((keyword) => keyword.includes(query))) return 3;
  return undefined;
}

/** The key an emoji is the same by in every section: the `:shortcode:` of a custom emoji, a Unicode emoji itself. */
function emojiKey(emoji: PickerEmoji): string {
  return "unicode" in emoji ? emoji.unicode : `:${emoji.shortcode}:`;
}

/**
 * The emoji of `emojis` that match the search text `search` (normalized here), ranked by `emojiMatchRank`, and in the
 * order given within a rank; each emoji once, at its first place. The picker's search and the completion after ":"
 * (ADR-0092) use it.
 */
export function searchEmojis(emojis: Iterable<PickerEmoji>, search: string): PickerEmoji[] {
  const query = normalizeEmojiSearch(search);
  const ranks: PickerEmoji[][] = [[], [], [], []];
  const seen = new Set<string>();
  for (const emoji of emojis) {
    const key = emojiKey(emoji);
    if (seen.has(key)) continue;
    const rank = emojiMatchRank(emoji, query);
    if (rank === undefined) continue;
    seen.add(key);
    ranks[rank]?.push(emoji);
  }
  return ranks.flat();
}

/** One emoji of the grid, under an id unique in the grid. */
type Item = { id: string; emoji: PickerEmoji };

type Group = { id: string; heading: string; items: Item[] };

function* emojisOf(sections: readonly EmojiPickerSection[]): Generator<PickerEmoji> {
  for (const section of sections) yield* section.emojis;
}

const sectionIcons: Readonly<Record<EmojiSectionIcon, () => JSX.Element>> = {
  frequent: FrequentEmojiIcon,
  custom: CustomEmojiIcon,
  "smileys-emotion": EmojiIcon,
  "people-body": PeopleEmojiIcon,
  "animals-nature": NatureEmojiIcon,
  "food-drink": FoodEmojiIcon,
  "travel-places": TravelEmojiIcon,
  activities: ActivityEmojiIcon,
  objects: ObjectEmojiIcon,
  symbols: SymbolEmojiIcon,
  flags: FlagEmojiIcon,
};

function SectionIcon({ icon }: { icon: EmojiSectionIcon }) {
  const Icon = sectionIcons[icon];
  return <Icon />;
}

/** The share of the grid's height at its top where a heading counts as the top of the grid. */
const topBand = 0.1;

/** The skin tone `keys` holds, from the menu's selection. */
function toneOf(keys: Selection): SkinTone | undefined {
  if (keys === "all") return undefined;
  return skinTones.find((tone) => keys.has(tone));
}

type SkinToneButtonProps = { value: SkinTone; onChange: (tone: SkinTone) => void; label: string; names: Readonly<Record<SkinTone, string>> };

/**
 * The menu button of the skin tone (WAI-ARIA APG, menu button pattern): the raised hand in the tone chosen, and a menu
 * of the six tones with single selection (role="menuitemradio"), the chosen one also marked by a check.
 */
function SkinToneButton({ value, onChange, label, names }: SkinToneButtonProps) {
  const styles = picker();
  return (
    <TooltipTrigger>
      <MenuTrigger>
        <Button variant="ghost" size="compact" aria-label={label}>
          <span aria-hidden="true" className={styles.tone()}>
            {toneHand + skinToneModifiers[value]}
          </span>
        </Button>
        <Menu
          aria-label={label}
          placement="bottom end"
          selectionMode="single"
          disallowEmptySelection
          selectedKeys={[value]}
          onSelectionChange={(keys) => {
            const tone = toneOf(keys);
            if (tone !== undefined) onChange(tone);
          }}
        >
          {skinTones.map((tone) => (
            <MenuItem key={tone} id={tone} textValue={names[tone]}>
              <span className={styles.toneItem()}>
                <span aria-hidden="true" className={styles.toneHand()}>
                  {toneHand + skinToneModifiers[tone]}
                </span>
                {names[tone]}
                {tone === value && (
                  <span className={styles.toneCheck()}>
                    <CheckIcon />
                  </span>
                )}
              </span>
            </MenuItem>
          ))}
        </Menu>
      </MenuTrigger>
      <Tooltip>{label}</Tooltip>
    </TooltipTrigger>
  );
}

/** What the status region announced last: the count of the search `query`. */
type Announced = { query: string; count: number };

/**
 * A search field, a skin tone menu, a toolbar of section buttons and a grid of the emoji of `sections`; picking one
 * calls `onSelect`. See `EmojiPickerProps` for the props, and the comment at the top of this file for the keyboard.
 */
export function EmojiPicker({ sections, onSelect, skinTone = "none", onSkinToneChange, labels, allowMotion, autoFocus = false }: EmojiPickerProps) {
  const styles = picker();
  const [search, setSearch] = useState("");
  // The section the user moved to with its button, until the scrolling says which one is at the top.
  const [current, setCurrent] = useState<string | undefined>(undefined);
  const [announced, setAnnounced] = useState<Announced | null>(null);
  const gridRef = useRef<HTMLDivElement>(null);
  // The headings in the grid, and the observer of them while the grid is mounted.
  const headingsRef = useRef(new Set<HTMLElement>());
  const observerRef = useRef<IntersectionObserver | null>(null);

  const shown = sections.filter((section) => section.emojis.length > 0);
  const query = normalizeEmojiSearch(search);
  const searching = query !== "";
  const results = searching ? searchEmojis(emojisOf(shown), search) : [];
  const groups: Group[] = searching
    ? results.length > 0
      ? [{ id: "search", heading: labels.results, items: results.map((emoji) => ({ id: `search\n${emojiKey(emoji)}`, emoji })) }]
      : []
    : shown.map((section) => ({
        id: section.id,
        heading: section.heading,
        items: section.emojis.map((emoji) => ({ id: `${section.id}\n${emojiKey(emoji)}`, emoji })),
      }));
  const byId = new Map(groups.flatMap((group) => group.items.map((item) => [item.id, item] as const)));
  const currentSection = searching ? undefined : shown.some((section) => section.id === current) ? current : shown[0]?.id;
  const count = results.length;

  // Announce the number of results once typing pauses (web-client/README.md Y7, like TextArea's count).
  useEffect(() => {
    if (query === "") return;
    const timer = setTimeout(() => {
      setAnnounced({ query, count });
    }, countAnnounceDelay);
    return () => {
      clearTimeout(timer);
    };
  }, [query, count]);

  // Follow the heading at the top of the grid as it scrolls: an observer of the headings entering or leaving the band
  // at the top of the grid, which then takes the last heading at or above that band. React Aria renders the grid and
  // its rows after a first pass over the collection, so the grid starts the observer from its ref (observeGrid), and
  // each heading joins it from its own (observeHeading).
  function observeGrid(grid: HTMLDivElement) {
    gridRef.current = grid;
    if (typeof IntersectionObserver === "undefined") {
      return () => {
        gridRef.current = null;
      };
    }
    const observer = new IntersectionObserver(
      () => {
        const box = grid.getBoundingClientRect();
        const headings = [...headingsRef.current]
          .map((heading) => ({ id: heading.dataset["sectionId"], top: heading.getBoundingClientRect().top - box.top }))
          .toSorted((first, second) => first.top - second.top);
        let top = headings[0]?.id;
        for (const heading of headings) if (heading.top <= box.height * topBand) top = heading.id;
        setCurrent(top);
      },
      { root: grid, rootMargin: `0px 0px -${String(100 - topBand * 100)}% 0px` },
    );
    observerRef.current = observer;
    for (const heading of headingsRef.current) observer.observe(heading);
    return () => {
      observer.disconnect();
      observerRef.current = null;
      gridRef.current = null;
    };
  }

  function observeHeading(heading: HTMLDivElement) {
    headingsRef.current.add(heading);
    observerRef.current?.observe(heading);
    return () => {
      headingsRef.current.delete(heading);
      observerRef.current?.unobserve(heading);
    };
  }

  function pick(key: Key, event: PressEvent) {
    const item = byId.get(String(key));
    if (item === undefined) return;
    // Shift keeps the picker open for the keyboard and the mouse; a touch has no Shift to hold (ADR-0091 §5).
    const keepOpen = event.shiftKey && (event.pointerType === "keyboard" || event.pointerType === "mouse");
    const { emoji } = item;
    if ("unicode" in emoji) onSelect({ kind: "unicode", emoji, text: skinToneVariant(emoji, skinTone)?.unicode ?? emoji.unicode, keepOpen });
    else onSelect({ kind: "custom", emoji, keepOpen });
  }

  function moveTo(section: EmojiPickerSection) {
    const grid = gridRef.current;
    if (grid === null) return;
    setCurrent(section.id);
    const first = section.emojis[0];
    const firstId = first === undefined ? undefined : `${section.id}\n${emojiKey(first)}`;
    const row = [...grid.querySelectorAll<HTMLElement>("[role=row][data-key]")].find((element) => element.dataset["key"] === firstId);
    const heading = [...grid.querySelectorAll<HTMLElement>("[data-section-id]")].find((element) => element.dataset["sectionId"] === section.id);
    row?.focus({ preventScroll: true });
    // React Aria scrolls the focused row into view in the next animation frame after a keyboard press; the heading
    // goes to the top of the grid after that, smoothly under motion-safe: (the grid's scroll-behavior).
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (heading === undefined) return;
        grid.scrollTop += heading.getBoundingClientRect().top - grid.getBoundingClientRect().top;
      });
    });
  }

  const status = announced !== null && announced.query === query ? labels.resultCount(announced.count) : "";

  return (
    <div className={styles.root()}>
      <div className={styles.top()}>
        <TextField type="search" label={labels.search} value={search} onChange={setSearch} autoFocus={autoFocus} grow shrink />
        {onSkinToneChange !== undefined && labels.skinTone !== undefined && labels.skinTones !== undefined && (
          <SkinToneButton value={skinTone} onChange={onSkinToneChange} label={labels.skinTone} names={labels.skinTones} />
        )}
      </div>
      {shown.length > 1 && (
        <Toolbar aria-label={labels.categories} className={styles.toolbar()}>
          {shown.map((section) => (
            <TooltipTrigger key={section.id}>
              <Button
                variant="ghost"
                size="icon"
                aria-label={section.heading}
                {...(section.id === currentSection ? { "aria-current": "true" } : {})}
                isDisabled={searching}
                className={styles.tab()}
                onPress={() => {
                  moveTo(section);
                }}
              >
                <SectionIcon icon={section.icon} />
              </Button>
              <Tooltip>{section.heading}</Tooltip>
            </TooltipTrigger>
          ))}
        </Toolbar>
      )}
      <GridList
        ref={observeGrid}
        aria-label={labels.grid}
        layout="grid"
        className={styles.grid()}
        // React Aria passes a row's presses to its onPress only when the grid has an action. The picking is in the rows'
        // onPress, whose event says whether Shift was held and whether it was a touch, and which also hears Space.
        onAction={() => {}}
        renderEmptyState={() => <p className={styles.empty()}>{labels.empty}</p>}
      >
        {groups.map((group) => (
          <GridListSection key={group.id} id={group.id} className={styles.section()}>
            <GridListHeader ref={observeHeading} data-section-id={group.id} className={styles.header()}>
              {group.heading}
            </GridListHeader>
            {group.items.map(({ id, emoji }) => {
              const onPress = (event: PressEvent) => {
                pick(id, event);
              };
              if ("unicode" in emoji) {
                const variant = skinToneVariant(emoji, skinTone);
                return (
                  <GridListItem key={id} id={id} textValue={variant?.name ?? emoji.name} onPress={onPress} className={styles.item()}>
                    {variant?.unicode ?? emoji.unicode}
                  </GridListItem>
                );
              }
              return (
                <GridListItem key={id} id={id} textValue={`:${emoji.shortcode}:`} onPress={onPress} className={styles.item()}>
                  <CustomEmoji emoji={emoji} allowMotion={allowMotion} />
                </GridListItem>
              );
            })}
          </GridListSection>
        ))}
      </GridList>
      <span role="status" className={styles.status()}>
        {status}
      </span>
    </div>
  );
}
