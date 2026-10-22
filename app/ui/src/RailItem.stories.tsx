// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { HomeIcon, NotificationsIcon } from "./Icons";
import { RailItem } from "./RailItem";

const meta = {
  title: "Shell/RailItem",
  component: RailItem,
  args: { href: "#home", icon: <HomeIcon />, label: "Home" },
} satisfies Meta<typeof RailItem>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The item of the place the user is in: `aria-current="page"`. */
export const Current: Story = {
  args: { current: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: "Home" })).toHaveAttribute("aria-current", "page");
  },
};

export const Default: Story = {};

/** With a count: the link is named with it, starting with the visible label. */
export const WithBadge: Story = {
  args: {
    icon: <NotificationsIcon />,
    label: "Notifications",
    badge: { count: 3, label: "3 unread notifications", name: "Notifications, 3 unread" },
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: "Notifications, 3 unread" })).toBeVisible();
  },
};

/** A text link of the signed-out top bar. */
export const Top: Story = { args: { form: "top", current: true } };

/** The top bar's call to action. */
export const TopProminent: Story = { args: { form: "top", prominent: true, label: "Create an account" } };
