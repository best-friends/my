// SPDX-License-Identifier: AGPL-3.0-only
import { act, render, screen, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { I18nProvider } from "react-aria-components";
import { describe, expect, it, vi } from "vitest";
import {
  EmojiPicker,
  type EmojiPickerLabels,
  type EmojiPickerSection,
  normalizeEmojiSearch,
  type PickerCustomEmoji,
  type PickerUnicodeEmoji,
  searchEmojis,
  type SkinTone,
  skinToneVariant,
} from "./EmojiPicker";
import { countAnnounceDelay } from "./TextArea";

const still = "https://media.example/still.webp";
const moving = "https://media.example/moving.webp";

const wave: PickerCustomEmoji = { shortcode: "wave", url: still, static_url: still, aliases: ["hello"] };
const blobcat: PickerCustomEmoji = { shortcode: "blobcat", url: moving, static_url: still };
const blobfox: PickerCustomEmoji = { shortcode: "blobfox", url: still, static_url: still, aliases: ["fox"] };

const wavingHand: PickerUnicodeEmoji = {
  unicode: "👋",
  name: "waving hand",
  keywords: ["hello", "wave"],
  skins: [
    { unicode: "👋🏻", name: "waving hand: light skin tone" },
    { unicode: "👋🏽", name: "waving hand: medium skin tone" },
  ],
};
const handshake: PickerUnicodeEmoji = {
  unicode: "🤝",
  name: "handshake",
  keywords: ["agreement", "hand"],
  skins: [
    { unicode: "🫱🏽‍🫲🏻", name: "handshake: medium skin tone, light skin tone" },
    { unicode: "🤝🏽", name: "handshake: medium skin tone" },
  ],
};
const thumbsUp: PickerUnicodeEmoji = { unicode: "👍", name: "thumbs up", keywords: ["hand", "+1"] };
const fox: PickerUnicodeEmoji = { unicode: "🦊", name: "きつね", keywords: ["どうぶつ", "フォックス", "fox"] };

const sections: EmojiPickerSection[] = [
  { id: "recent", heading: "Recently used", icon: "frequent", emojis: [blobcat] },
  { id: "custom", heading: "Blobs", icon: "custom", emojis: [wave, blobcat, blobfox] },
  { id: "people", heading: "People", icon: "people-body", emojis: [wavingHand, handshake, thumbsUp] },
  { id: "animals", heading: "Animals", icon: "animals-nature", emojis: [fox] },
];

const tones: Record<SkinTone, string> = {
  none: "Default",
  light: "Light",
  "medium-light": "Medium-light",
  medium: "Medium",
  "medium-dark": "Medium-dark",
  dark: "Dark",
};

const labels: EmojiPickerLabels = {
  search: "Search emoji",
  grid: "Emoji",
  categories: "Categories",
  results: "Search results",
  resultCount: (count) => `${String(count)} emoji found`,
  empty: "No emoji found",
  skinTone: "Skin tone",
  skinTones: tones,
};

/** The name of each row: an emoji, or a heading. */
function rowNames(): string[] {
  return screen.getAllByRole("row").map((row) => row.getAttribute("aria-label") ?? row.textContent);
}

describe("EmojiPicker", () => {
  it("shows the sections in order, each emoji named by its shortcode or its name", () => {
    render(<EmojiPicker sections={sections} onSelect={() => {}} labels={labels} />);
    expect(screen.getByRole("grid", { name: "Emoji" })).toBeInTheDocument();
    expect(rowNames()).toEqual([
      "Recently used",
      ":blobcat:",
      "Blobs",
      ":wave:",
      ":blobcat:",
      ":blobfox:",
      "People",
      "waving hand",
      "handshake",
      "thumbs up",
      "Animals",
      "きつね",
    ]);
    expect(screen.getAllByRole("img", { name: ":blobcat:" })).toHaveLength(2);
    const toolbar = screen.getByRole("toolbar", { name: "Categories" });
    expect(within(toolbar).getAllByRole("button").map((button) => button.getAttribute("aria-label"))).toEqual(["Recently used", "Blobs", "People", "Animals"]);
    expect(within(toolbar).getByRole("button", { name: "Recently used" })).toHaveAttribute("aria-current", "true");
  });

  it("goes from the search field to the skin tone, the section buttons and the grid with Tab, and through the grid with the keys", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<EmojiPicker sections={sections} onSelect={onSelect} labels={labels} skinTone="none" onSkinToneChange={() => {}} autoFocus />);
    expect(screen.getByRole("searchbox", { name: "Search emoji" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Skin tone" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Recently used" })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("button", { name: "Blobs" })).toHaveFocus();
    await user.tab();
    const [recent] = screen.getAllByRole("row", { name: ":blobcat:" });
    expect(recent).toHaveFocus();
    await user.keyboard("{End}");
    expect(screen.getByRole("row", { name: "きつね" })).toHaveFocus();
    await user.keyboard("{Home}");
    expect(recent).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "custom", emoji: blobcat, keepOpen: false });
    // jsdom lays nothing out, so only the keys along the line move here; the stories check Up and Down in Chromium.
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("row", { name: ":wave:" })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getAllByRole("row", { name: ":blobcat:" })[1]).toHaveFocus();
    await user.keyboard(" ");
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "custom", emoji: blobcat, keepOpen: false });
  });

  it("moves the arrow keys along the writing direction", async () => {
    const user = userEvent.setup();
    render(
      <I18nProvider locale="ar-XB">
        <EmojiPicker sections={sections} onSelect={() => {}} labels={labels} />
      </I18nProvider>,
    );
    await user.click(screen.getByRole("searchbox", { name: "Search emoji" }));
    await user.tab();
    await user.tab();
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("row", { name: ":wave:" })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getAllByRole("row", { name: ":blobcat:" })[0]).toHaveFocus();
  });

  it("moves focus to the first emoji of a section with its button", async () => {
    const user = userEvent.setup();
    render(<EmojiPicker sections={sections} onSelect={() => {}} labels={labels} />);
    const toolbar = screen.getByRole("toolbar", { name: "Categories" });
    await user.click(within(toolbar).getByRole("button", { name: "People" }));
    expect(screen.getByRole("row", { name: "waving hand" })).toHaveFocus();
    expect(within(toolbar).getByRole("button", { name: "People" })).toHaveAttribute("aria-current", "true");
    expect(within(toolbar).getByRole("button", { name: "Recently used" })).not.toHaveAttribute("aria-current");
  });

  it("ranks the search results in one section, disables the section buttons and announces the count once typing pauses", async () => {
    // Testing Library waits on real timeouts inside act(), so the fake clock also follows real time (as in TextArea's tests).
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"], shouldAdvanceTime: true });
    try {
      const user = userEvent.setup({ delay: null });
      render(<EmojiPicker sections={sections} onSelect={() => {}} labels={labels} />);
      const status = screen.getByRole("status");
      await user.type(screen.getByRole("searchbox", { name: "Search emoji" }), "Hand");
      // A name prefix, then a keyword prefix, then a substring of a name.
      expect(rowNames()).toEqual(["Search results", "handshake", "thumbs up", "waving hand"]);
      for (const button of within(screen.getByRole("toolbar", { name: "Categories" })).getAllByRole("button")) {
        expect(button).toBeDisabled();
        expect(button).not.toHaveAttribute("aria-current");
      }
      expect(status).toHaveTextContent("");
      act(() => {
        vi.advanceTimersByTime(countAnnounceDelay);
      });
      expect(status).toHaveTextContent("3 emoji found");
      await user.clear(screen.getByRole("searchbox", { name: "Search emoji" }));
      expect(status).toHaveTextContent("");
      expect(screen.getByRole("button", { name: "People" })).toBeEnabled();
      await user.type(screen.getByRole("searchbox", { name: "Search emoji" }), "nothing");
      expect(screen.getByText("No emoji found")).toBeInTheDocument();
      act(() => {
        vi.advanceTimersByTime(countAnnounceDelay);
      });
      expect(status).toHaveTextContent("0 emoji found");
    } finally {
      vi.useRealTimers();
    }
  });

  it("puts a frequent emoji once among the results, and finds a custom emoji by its shortcode with colons or an alias", async () => {
    const user = userEvent.setup();
    render(<EmojiPicker sections={sections} onSelect={() => {}} labels={labels} />);
    const search = screen.getByRole("searchbox", { name: "Search emoji" });
    await user.type(search, ":blob");
    expect(rowNames()).toEqual(["Search results", ":blobcat:", ":blobfox:"]);
    await user.clear(search);
    await user.type(search, "fox");
    // The alias of :blobfox: and the keyword of 🦊 start with it; :blobfox: is given first.
    expect(rowNames()).toEqual(["Search results", ":blobfox:", "きつね"]);
  });

  it("shows and picks the emoji with variants in the skin tone chosen, the same for every person", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const onSkinToneChange = vi.fn();
    const { rerender } = render(<EmojiPicker sections={sections} onSelect={onSelect} labels={labels} skinTone="none" onSkinToneChange={onSkinToneChange} />);
    expect(screen.getByRole("row", { name: "waving hand" })).toHaveTextContent("👋");
    await user.click(screen.getByRole("button", { name: "Skin tone" }));
    const menu = screen.getByRole("menu", { name: "Skin tone" });
    expect(within(menu).getByRole("menuitemradio", { name: "Default" })).toHaveAttribute("aria-checked", "true");
    await user.click(within(menu).getByRole("menuitemradio", { name: "Medium" }));
    expect(onSkinToneChange).toHaveBeenCalledWith("medium");
    rerender(<EmojiPicker sections={sections} onSelect={onSelect} labels={labels} skinTone="medium" onSkinToneChange={onSkinToneChange} />);
    expect(screen.getByRole("row", { name: "waving hand: medium skin tone" })).toHaveTextContent("👋🏽");
    expect(screen.getByRole("row", { name: "handshake: medium skin tone" })).toHaveTextContent("🤝🏽");
    // Without a variant, an emoji stays as it is.
    expect(screen.getByRole("row", { name: "thumbs up" })).toHaveTextContent("👍");
    await user.click(screen.getByRole("row", { name: "handshake: medium skin tone" }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "unicode", emoji: handshake, text: "🤝🏽", keepOpen: false });
  });

  it("has no skin tone button without its handler", () => {
    render(<EmojiPicker sections={sections} onSelect={() => {}} labels={labels} />);
    expect(screen.queryByRole("button", { name: "Skin tone" })).not.toBeInTheDocument();
  });

  it("asks to stay open when an emoji is picked with Shift held, and focus stays in the grid", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<EmojiPicker sections={sections} onSelect={onSelect} labels={labels} />);
    await user.keyboard("{Shift>}");
    await user.click(screen.getByRole("row", { name: "handshake" }));
    await user.keyboard("{/Shift}");
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "unicode", emoji: handshake, text: "🤝", keepOpen: true });
    expect(screen.getByRole("row", { name: "handshake" })).toHaveFocus();
    await user.keyboard("{ArrowRight}{Shift>}{Enter}{/Shift}");
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "unicode", emoji: thumbsUp, text: "👍", keepOpen: true });
    expect(screen.getByRole("row", { name: "thumbs up" })).toHaveFocus();
    await user.keyboard("{ArrowLeft}{Enter}");
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "unicode", emoji: handshake, text: "🤝", keepOpen: false });
  });

  it("does not stay open for a touch, Shift or not", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<EmojiPicker sections={sections} onSelect={onSelect} labels={labels} />);
    await user.keyboard("{Shift>}");
    await user.pointer({ keys: "[TouchA]", target: screen.getByRole("row", { name: "thumbs up" }) });
    await user.keyboard("{/Shift}");
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "unicode", emoji: thumbsUp, text: "👍", keepOpen: false });
  });
});

describe("searchEmojis", () => {
  it("compares in NFKC, lower case and hiragana", () => {
    expect(normalizeEmojiSearch(" ：ＦＯＸ： ")).toBe("fox");
    expect(normalizeEmojiSearch("フォックス")).toBe("ふぉっくす");
    expect(searchEmojis([thumbsUp, fox], "ﾌｫｯｸｽ")).toEqual([fox]);
    expect(searchEmojis([thumbsUp, fox], "キツネ")).toEqual([fox]);
  });

  it("ranks the exact name and the emoji itself first, then name prefixes, keyword prefixes and substrings", () => {
    expect(searchEmojis([thumbsUp, wavingHand, handshake], "hand")).toEqual([handshake, thumbsUp, wavingHand]);
    expect(searchEmojis([wavingHand, thumbsUp], "👍")).toEqual([thumbsUp]);
    expect(searchEmojis([blobfox, wave, blobcat], "b")).toEqual([blobfox, blobcat]);
    expect(searchEmojis([wave, blobcat], "")).toEqual([]);
  });
});

describe("skinToneVariant", () => {
  it("takes the variant whose every modifier is the tone", () => {
    expect(skinToneVariant(handshake, "medium")?.unicode).toBe("🤝🏽");
    expect(skinToneVariant(handshake, "light")).toBeUndefined();
    expect(skinToneVariant(wavingHand, "none")).toBeUndefined();
    expect(skinToneVariant(thumbsUp, "dark")).toBeUndefined();
  });
});
