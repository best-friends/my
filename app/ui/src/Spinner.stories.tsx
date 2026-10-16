// SPDX-License-Identifier: AGPL-3.0-only
// Every story is also captured with prefers-reduced-motion: reduce, where the setup fails on any
// running animation (.storybook/vitest.setup.ts); the plays here check the spin without it.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Spinner } from "./Spinner";

const meta = {
  title: "Display/Spinner",
  component: Spinner,
  args: { label: "Loading posts" },
} satisfies Meta<typeof Spinner>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The ring follows the user's motion preference: it spins with no preference and stands still with reduce. */
async function expectSpinFollowsMotionPreference(root: HTMLElement) {
  const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  for (const ring of root.querySelectorAll("[role='progressbar'] > [aria-hidden='true']")) {
    await expect(getComputedStyle(ring).animationName).toBe(reduce ? "none" : "spin");
  }
}

/** An indeterminate progress bar named by its label: it has no value. */
export const Default: Story = {
  play: async ({ canvas, canvasElement }) => {
    const spinner = canvas.getByRole("progressbar", { name: "Loading posts" });
    await expect(spinner).not.toHaveAttribute("aria-valuenow");
    await expectSpinFollowsMotionPreference(canvasElement);
  },
};

export const Sizes: Story = {
  render: (args) => (
    <div className="flex items-center gap-4">
      <Spinner {...args} size="sm" />
      <Spinner {...args} size="md" />
      <Spinner {...args} size="lg" />
    </div>
  ),
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getAllByRole("progressbar", { name: "Loading posts" })).toHaveLength(3);
    await expectSpinFollowsMotionPreference(canvasElement);
  },
};

/** The arc takes the text color, so a caller can match the surrounding text. */
export const InheritsColor: Story = {
  render: (args) => (
    <p className="flex items-center gap-2 text-fg-muted">
      <Spinner {...args} size="sm" className="text-fg-muted" />
      <span>Fetching older posts</span>
    </p>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("progressbar", { name: "Loading posts" })).toBeVisible();
  },
};
