// SPDX-License-Identifier: AGPL-3.0-only
// EmojiPicker stories. The play functions test the search, the arrow keys of the grid in the writing
// direction, and picking an emoji.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, waitFor } from "storybook/test";
import { Button } from "./Button";
import { EmojiPicker, type PickerEmoji } from "./EmojiPicker";
import { EmojiIcon } from "./Icons";
import { Popover, PopoverTrigger } from "./Popover";

// Plain squares stand in for emoji images, as in the CustomEmoji stories.
const orange = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGUlEQVR4nGN4ViH3nxLMMGrAqAGjBgwXAwAvJnsfj06SzAAAAABJRU5ErkJggg==";
const blue = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAAGUlEQVR4nGPQ6LnznxLMMGrAqAGjBgwXAwBfb48fVw/D1AAAAABJRU5ErkJggg==";

const blobcat: PickerEmoji = { shortcode: "blobcat", url: blue, static_url: orange, category: "Blobs" };
const thanks: PickerEmoji = { shortcode: "thanks", url: blue, static_url: blue };

const emojis: PickerEmoji[] = [
  { shortcode: "wave", url: orange, static_url: orange, aliases: ["hello"] },
  thanks,
  blobcat,
  { shortcode: "blobfox", url: orange, static_url: orange, category: "Blobs", aliases: ["fox"] },
  { shortcode: "blobheart", url: blue, static_url: blue, category: "Blobs" },
  { shortcode: "party", url: orange, static_url: orange, category: "Celebration" },
];

const meta = {
  title: "Forms/EmojiPicker",
  component: EmojiPicker,
  args: {
    emojis,
    recent: [blobcat],
    onSelect: fn(),
    labels: { search: "Search emoji", grid: "Custom emoji", recent: "Recently used", uncategorized: "Other", empty: "No emoji found" },
  },
  decorators: [
    (Story) => (
      <div className="max-w-80">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EmojiPicker>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The emoji by category, those used recently first; the arrow keys move in the writing direction, and Enter picks. */
export const Grid: Story = {
  play: async ({ canvas, userEvent, args }) => {
    // The first of the rows named so: the recent ones come first.
    const rowOf = (name: string) => canvas.getAllByRole("row", { name })[0];
    await userEvent.click(canvas.getByRole("searchbox", { name: "Search emoji" }));
    await userEvent.tab();
    await expect(rowOf(":blobcat:")).toHaveFocus();
    // The first emoji of the next group is below the recent one.
    await userEvent.keyboard("{ArrowDown}");
    await expect(rowOf(":wave:")).toHaveFocus();
    const forward = document.documentElement.dir === "rtl" ? "{ArrowLeft}" : "{ArrowRight}";
    await userEvent.keyboard(forward);
    await expect(rowOf(":thanks:")).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    await expect(args.onSelect).toHaveBeenCalledWith(thanks);
  },
};

/** A search keeps the emoji whose shortcode or alias contains it. */
export const Searched: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("searchbox", { name: "Search emoji" }), "fox");
    await expect(canvas.getByRole("row", { name: ":blobfox:" })).toBeVisible();
    await expect(canvas.queryByRole("row", { name: ":wave:" })).not.toBeInTheDocument();
  },
};

/** Nothing matches the search. */
export const NoMatch: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("searchbox", { name: "Search emoji" }), "nothing");
    await expect(canvas.getByText("No emoji found")).toBeVisible();
  },
};

/** Animated emoji stay still when the user turns animations off. */
export const StillOnly: Story = { args: { allowMotion: false } };

/** In a popover behind an icon button, as the post form has it: the search field takes focus when it opens. */
export const InPopover: Story = {
  decorators: [
    (Story) => (
      <div className="min-h-96">
        <Story />
      </div>
    ),
  ],
  render: (args) => (
    <PopoverTrigger>
      <Button variant="ghost" size="toolbar" aria-label="Add emoji">
        <EmojiIcon />
      </Button>
      <Popover aria-label="Custom emoji">
        {({ close }) => (
          <EmojiPicker
            {...args}
            autoFocus
            onSelect={(emoji) => {
              args.onSelect(emoji);
              close();
            }}
          />
        )}
      </Popover>
    </PopoverTrigger>
  ),
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: "Add emoji" }));
    await waitFor(() => expect(canvas.getByRole("searchbox", { name: "Search emoji" })).toHaveFocus());
  },
};
