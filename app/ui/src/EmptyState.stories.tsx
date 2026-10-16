// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { EmptyState } from "./EmptyState";

const meta = {
  title: "Display/EmptyState",
  component: EmptyState,
  args: { heading: "No posts yet" },
} satisfies Meta<typeof EmptyState>;

export default meta;
type Story = StoryObj<typeof meta>;

function Illustration() {
  return (
    <svg viewBox="0 0 64 64" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round">
      <rect x="10" y="12" width="44" height="40" rx="6" />
      <path d="M20 26h24M20 36h14" />
    </svg>
  );
}

export const HeadingOnly: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("heading", { level: 2, name: "No posts yet" })).toBeVisible();
  },
};

/** The full set: a decorative illustration, the heading, a description and an action. */
export const Complete: Story = {
  args: {
    description: "Posts from the people you follow appear here. Find someone to follow to get started.",
    illustration: <Illustration />,
    action: (
      <a href="#explore" className="font-semibold text-accent underline">
        Explore the local timeline
      </a>
    ),
  },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getByRole("heading", { level: 2, name: "No posts yet" })).toBeVisible();
    await expect(canvas.getByRole("link", { name: "Explore the local timeline" })).toBeVisible();
    await expect(canvasElement.querySelector("svg")?.closest("[aria-hidden='true']")).not.toBeNull();
  },
};

/** Inside a section headed by an h2, the empty state's heading is one level lower. */
export const NestedHeading: Story = {
  args: { heading: "No matching posts", headingLevel: 3, description: "Try a different search term." },
  render: (args) => (
    <section aria-labelledby="empty-search" className="flex flex-col gap-2">
      <h2 id="empty-search" className="text-xl font-semibold">
        Search results
      </h2>
      <EmptyState {...args} />
    </section>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("heading", { level: 3, name: "No matching posts" })).toBeVisible();
  },
};
