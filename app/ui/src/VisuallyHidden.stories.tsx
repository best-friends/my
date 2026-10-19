// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { SettingsIcon } from "./Icons";
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
    <a href="#settings" className="inline-flex size-10 items-center justify-center rounded-md text-fg [&>svg]:size-6">
      <SettingsIcon />
      <VisuallyHidden {...args} />
    </a>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: "Settings" })).toBeVisible();
    const hidden = canvas.getByText("Settings");
    await expect(hidden.getBoundingClientRect().width).toBeLessThanOrEqual(1);
  },
};
