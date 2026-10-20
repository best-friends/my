// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, waitFor, within } from "storybook/test";
import { MediaGrid, type MediaItem } from "./MediaGrid";

/** A placeholder picture as an SVG data URL, so stories load no network images. */
function picture(fill: string, shape: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><rect width="160" height="90" fill="${fill}"/>${shape}</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

/** The first three pictures have a blurhash; the fourth has none, and its plain cell stands in for the blur. */
const pictures: readonly MediaItem[] = [
  {
    id: "1",
    src: picture("#4f6d8f", '<circle cx="80" cy="45" r="24" fill="#f2c14e"/>'),
    alt: "A yellow sun over a blue sky",
    lang: "en",
    blurhash: "LEHV6nWB2yk8pyo0adR*.7kCMdnj",
  },
  {
    id: "2",
    src: picture("#3d7a52", '<path d="M0 90 60 30l40 40 20-20 40 40z" fill="#e8efe9"/>'),
    alt: "Snowy hills in a green valley",
    lang: "en",
    blurhash: "LGF5]+Yk^6#M@-5c,1J5@[or[Q6.",
  },
  {
    id: "3",
    src: picture("#7a3d6b", '<rect x="50" y="20" width="60" height="50" fill="#f4d6ea"/>'),
    alt: "A pink square on purple",
    lang: "en",
    blurhash: "L6PZfSi_.AyE_3t7t7R**0o#DgR4",
  },
  { id: "4", src: picture("#8f5a2e", '<circle cx="50" cy="45" r="15" fill="#fbe3c8"/><circle cx="110" cy="45" r="15" fill="#fbe3c8"/>'), alt: null },
];

const meta = {
  title: "Timeline/MediaGrid",
  component: MediaGrid,
  args: { items: pictures.slice(0, 1), noDescriptionLabel: "No description", revealLabel: "Show sensitive media", onRevealedChange: fn() },
} satisfies Meta<typeof MediaGrid>;

export default meta;
type Story = StoryObj<typeof meta>;

export const One: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("img", { name: "A yellow sun over a blue sky" })).toBeVisible();
  },
};

export const Two: Story = { args: { items: pictures.slice(0, 2) } };

export const Three: Story = { args: { items: pictures.slice(0, 3) } };

/** The fourth picture has no description: it is named "No description" and carries the badge. */
export const Four: Story = {
  args: { items: pictures },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img")).toHaveLength(4);
    await expect(canvas.getByRole("img", { name: "No description" })).toBeVisible();
  },
};

export const NoDescription: Story = { args: { items: pictures.slice(3) } };

/**
 * Sensitive media are not loaded until the reader presses the toggle, which keeps focus. Until then each cell shows
 * the blur of its picture (its blurhash).
 */
export const Sensitive: Story = {
  args: { items: pictures.slice(0, 2), sensitive: true },
  play: async ({ canvas, canvasElement, userEvent }) => {
    await expect(canvas.queryByRole("img")).not.toBeInTheDocument();
    await expect(canvasElement.querySelectorAll("canvas")).toHaveLength(2);
    const toggle = canvas.getByRole("button", { name: "Show sensitive media" });
    await expect(toggle).toHaveAttribute("aria-pressed", "false");
    await userEvent.tab();
    await userEvent.keyboard("{Enter}");
    await expect(toggle).toHaveAttribute("aria-pressed", "true");
    await expect(toggle).toHaveFocus();
    await expect(canvas.getAllByRole("img")).toHaveLength(2);
    // Back to concealed, so the screenshots show the concealed state.
    await userEvent.keyboard("{Enter}");
    await expect(canvas.queryByRole("img")).not.toBeInTheDocument();
  },
};

/** A picture without a blurhash (the fourth) keeps the plain surface of its cell while concealed. */
export const SensitiveWithoutBlurhash: Story = {
  args: { items: pictures.slice(2, 4), sensitive: true },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelectorAll("canvas")).toHaveLength(1);
  },
};

export const SensitiveRevealed: Story = {
  args: { items: pictures.slice(0, 3), sensitive: true, defaultRevealed: true },
};

/**
 * Pressing a picture opens it at its original size in the viewer (a modal dialog), whose buttons and arrow keys step
 * through the post's pictures, going round at either end. Esc closes it and returns focus to the picture; the story
 * opens it again on the second picture for the screenshots.
 */
export const Viewer: Story = {
  args: {
    items: pictures.slice(0, 2).map((item) => ({ ...item, original: { sources: { still: [{ url: item.src }] }, width: 1600, height: 900 } })),
    viewerLabels: {
      title: (position, count) => `Image ${String(position)} of ${String(count)}`,
      close: "Close",
      previous: "Previous image",
      next: "Next image",
    },
  },
  // The viewer covers the story frame, so give it room.
  decorators: [
    (Story) => (
      <div className="min-h-136">
        <Story />
      </div>
    ),
  ],
  play: async ({ canvas, userEvent }) => {
    const first = canvas.getByRole("button", { name: "A yellow sun over a blue sky" });
    await userEvent.click(first);
    const dialog = await canvas.findByRole("dialog", { name: "Image 1 of 2" });
    // Visible once the panel has faded in.
    await waitFor(() => expect(within(dialog).getByRole("img", { name: "A yellow sun over a blue sky" })).toBeVisible());
    // The arrow keys follow the screen: the next picture is to the right in left-to-right text, to the left otherwise.
    const forward = document.documentElement.dir === "rtl" ? "{ArrowLeft}" : "{ArrowRight}";
    await userEvent.keyboard(forward);
    await expect(canvas.getByRole("dialog", { name: "Image 2 of 2" })).toBeInTheDocument();
    await userEvent.keyboard(forward);
    await expect(canvas.getByRole("dialog", { name: "Image 1 of 2" })).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Previous image" }));
    await expect(canvas.getByRole("dialog", { name: "Image 2 of 2" })).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(canvas.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(first).toHaveFocus());

    await userEvent.click(canvas.getByRole("button", { name: "Snowy hills in a green valley" }));
    await expect(await canvas.findByRole("dialog", { name: "Image 2 of 2" })).toBeInTheDocument();
  },
};
