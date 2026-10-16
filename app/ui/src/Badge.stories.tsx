// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Badge } from "./Badge";

const meta = {
  title: "Display/Badge",
  component: Badge,
  args: { count: 3, label: "3 unread notifications" },
} satisfies Meta<typeof Badge>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Formats a number the way the badge does for the story's locale (the preview sets lang on <html>). */
function formatted(value: number) {
  return new Intl.NumberFormat(document.documentElement.lang).format(value);
}

/** The digits are drawn but not read; the label is read instead. */
export const Default: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByText("3 unread notifications")).toBeInTheDocument();
    await expect(canvas.getByText(formatted(3))).toHaveAttribute("aria-hidden", "true");
  },
};

/** Above `max` the badge shows the cap and a plus sign; the label still says the real count. */
export const OverMax: Story = {
  args: { count: 150, label: "150 unread notifications" },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(`${formatted(99)}+`)).toBeVisible();
    await expect(canvas.getByText("150 unread notifications")).toBeInTheDocument();
  },
};

export const CustomMax: Story = {
  args: { count: 12, max: 9, label: "12 new posts" },
  play: async ({ canvas }) => {
    await expect(canvas.getByText(`${formatted(9)}+`)).toBeVisible();
  },
};

export const Tones: Story = {
  render: (args) => (
    <div className="flex flex-wrap items-center gap-3">
      <Badge {...args} tone="accent" />
      <Badge {...args} tone="danger" />
      <Badge {...args} tone="neutral" />
      <Badge count={1234} label="1,234 followers" max={9999} tone="neutral" />
    </div>
  ),
};

/** In a link, the badge's label joins the link's accessible name. */
export const InLink: Story = {
  args: { count: 150, label: "150 unread" },
  render: (args) => (
    <a href="#notifications" className="inline-flex items-center gap-2 text-accent underline">
      Notifications
      <Badge {...args} />
    </a>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: /^Notifications\s*150 unread$/ })).toBeVisible();
  },
};
