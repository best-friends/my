// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { VisuallyHidden } from "./VisuallyHidden";

const meta = {
  title: "Display/VisuallyHidden",
  component: VisuallyHidden,
  args: { children: "Settings" },
} satisfies Meta<typeof VisuallyHidden>;

export default meta;
type Story = StoryObj<typeof meta>;

/** An icon-only link gets its accessible name from hidden text, which takes no space on screen. */
export const IconLink: Story = {
  render: (args) => (
    <a href="#settings" className="inline-flex size-10 items-center justify-center rounded-md text-fg">
      <svg viewBox="0 0 24 24" aria-hidden="true" className="size-6" fill="none" stroke="currentColor" strokeWidth="2">
        <circle cx="12" cy="12" r="3" />
        <path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1 7 17M17 7l2.1-2.1" />
      </svg>
      <VisuallyHidden {...args} />
    </a>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: "Settings" })).toBeVisible();
    const hidden = canvas.getByText("Settings");
    await expect(hidden.getBoundingClientRect().width).toBeLessThanOrEqual(1);
  },
};
