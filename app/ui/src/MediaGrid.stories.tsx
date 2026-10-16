// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn } from "storybook/test";
import { MediaGrid, type MediaItem } from "./MediaGrid";

/** A placeholder picture as an SVG data URL, so stories load no network images. */
function picture(fill: string, shape: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><rect width="160" height="90" fill="${fill}"/>${shape}</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

const pictures: readonly MediaItem[] = [
  { id: "1", src: picture("#4f6d8f", '<circle cx="80" cy="45" r="24" fill="#f2c14e"/>'), alt: "A yellow sun over a blue sky", lang: "en" },
  { id: "2", src: picture("#3d7a52", '<path d="M0 90 60 30l40 40 20-20 40 40z" fill="#e8efe9"/>'), alt: "Snowy hills in a green valley", lang: "en" },
  { id: "3", src: picture("#7a3d6b", '<rect x="50" y="20" width="60" height="50" fill="#f4d6ea"/>'), alt: "A pink square on purple", lang: "en" },
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

/** Sensitive media are not loaded until the reader presses the toggle, which keeps focus. */
export const Sensitive: Story = {
  args: { items: pictures.slice(0, 2), sensitive: true },
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.queryByRole("img")).not.toBeInTheDocument();
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

export const SensitiveRevealed: Story = {
  args: { items: pictures.slice(0, 3), sensitive: true, defaultRevealed: true },
};
