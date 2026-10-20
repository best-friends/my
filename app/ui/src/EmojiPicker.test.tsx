// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { EmojiPicker, type PickerEmoji } from "./EmojiPicker";

const still = "https://media.example/still.webp";
const moving = "https://media.example/moving.webp";

const wave: PickerEmoji = { shortcode: "wave", url: still, static_url: still, aliases: ["hello"] };
const blobcat: PickerEmoji = { shortcode: "blobcat", url: moving, static_url: still, category: "Blobs" };
const blobfox: PickerEmoji = { shortcode: "blobfox", url: still, static_url: still, category: "Blobs", aliases: ["fox"] };
const emojis = [wave, blobcat, blobfox];

const labels = { search: "Search emoji", grid: "Custom emoji", recent: "Recently used", uncategorized: "Other", empty: "No emoji found" };

describe("EmojiPicker", () => {
  it("groups the emoji by category, the recent ones first, each named by its shortcode", () => {
    render(<EmojiPicker emojis={emojis} recent={[blobcat]} onSelect={() => {}} labels={labels} />);
    expect(screen.getByRole("grid", { name: "Custom emoji" })).toBeInTheDocument();
    const rows = screen.getAllByRole("row");
    const names = rows.map((row) => row.getAttribute("aria-label") ?? row.textContent);
    expect(names).toEqual(["Recently used", ":blobcat:", "Other", ":wave:", "Blobs", ":blobcat:", ":blobfox:"]);
    expect(screen.getAllByRole("img", { name: ":blobcat:" })).toHaveLength(2);
  });

  it("finds emoji by shortcode or alias, with or without colons, and hides the recent ones while searching", async () => {
    const user = userEvent.setup();
    render(<EmojiPicker emojis={emojis} recent={[blobcat]} onSelect={() => {}} labels={labels} />);
    await user.type(screen.getByRole("searchbox", { name: "Search emoji" }), ":FOX");
    expect(screen.getByRole("row", { name: ":blobfox:" })).toBeInTheDocument();
    expect(screen.queryByRole("row", { name: ":blobcat:" })).not.toBeInTheDocument();
    expect(screen.queryByText("Recently used")).not.toBeInTheDocument();
    await user.clear(screen.getByRole("searchbox", { name: "Search emoji" }));
    await user.type(screen.getByRole("searchbox", { name: "Search emoji" }), "nothing");
    expect(screen.getByText("No emoji found")).toBeInTheDocument();
  });

  it("moves through the grid with the arrow keys and picks with Enter", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<EmojiPicker emojis={emojis} onSelect={onSelect} labels={labels} autoFocus />);
    expect(screen.getByRole("searchbox", { name: "Search emoji" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("row", { name: ":wave:" })).toHaveFocus();
    await user.keyboard("{End}");
    expect(screen.getByRole("row", { name: ":blobfox:" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenCalledWith(blobfox);
  });

  it("picks an emoji with a press", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(<EmojiPicker emojis={emojis} onSelect={onSelect} labels={labels} />);
    await user.click(screen.getByRole("row", { name: ":wave:" }));
    expect(onSelect).toHaveBeenCalledWith(wave);
  });
});
