// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ComponentType } from "react";
import { expect } from "storybook/test";
import { Button } from "./Button";
import {
  BackIcon,
  BookmarkIcon,
  BoostIcon,
  CloseIcon,
  ExternalLinkIcon,
  FederatedIcon,
  HomeIcon,
  LocalIcon,
  MenuIcon,
  MoreIcon,
  NotificationsIcon,
  PlusIcon,
  ProfileIcon,
  ReplyIcon,
  SearchIcon,
  SettingsIcon,
  StarIcon,
  TimelinesIcon,
  type ToggleIconProps,
} from "./Icons";
import { Link } from "./Link";

export default {
  title: "Foundations/Icons",
} satisfies Meta;

type Story = StoryObj;

const icons: readonly (readonly [string, ComponentType<ToggleIconProps>])[] = [
  ["CloseIcon", CloseIcon],
  ["PlusIcon", PlusIcon],
  ["MoreIcon", MoreIcon],
  ["BackIcon", BackIcon],
  ["ExternalLinkIcon", ExternalLinkIcon],
  ["ReplyIcon", ReplyIcon],
  ["BoostIcon", BoostIcon],
  ["StarIcon", StarIcon],
  ["BookmarkIcon", BookmarkIcon],
  ["HomeIcon", HomeIcon],
  ["TimelinesIcon", TimelinesIcon],
  ["LocalIcon", LocalIcon],
  ["FederatedIcon", FederatedIcon],
  ["SearchIcon", SearchIcon],
  ["NotificationsIcon", NotificationsIcon],
  ["ProfileIcon", ProfileIcon],
  ["SettingsIcon", SettingsIcon],
  ["MenuIcon", MenuIcon],
];

/** The icons of toggles, which show their on state by shape (filled, or a heavier stroke) and not by color alone. */
const toggleIcons: readonly (readonly [string, ComponentType<ToggleIconProps>])[] = [
  ["BoostIcon", BoostIcon],
  ["StarIcon", StarIcon],
  ["BookmarkIcon", BookmarkIcon],
];

/**
 * Every icon at the navigation size (`size-6`), and the toggle icons on. ReplyIcon, BackIcon and
 * ExternalLinkIcon mirror in right-to-left text; the others do not.
 */
export const Gallery: Story = {
  render: () => (
    <div className="flex flex-col gap-6">
      <ul className="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-2">
        {icons.map(([name, Icon]) => (
          <li key={name} className="flex items-center gap-3 rounded-md border border-border p-3 text-fg [&>svg]:size-6 [&>svg]:shrink-0">
            <Icon />
            <code className="min-w-0 font-mono text-sm wrap-anywhere">{name}</code>
          </li>
        ))}
      </ul>
      <ul className="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-2">
        {toggleIcons.map(([name, Icon]) => (
          <li key={name} className="flex items-center gap-3 rounded-md border border-border p-3 text-accent [&>svg]:size-6 [&>svg]:shrink-0">
            <Icon filled />
            <code className="min-w-0 font-mono text-sm wrap-anywhere">{name} filled</code>
          </li>
        ))}
      </ul>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const svgs = [...canvasElement.querySelectorAll("svg")];
    await expect(svgs).toHaveLength(icons.length + toggleIcons.length);
    for (const svg of svgs) {
      await expect(svg).toHaveAttribute("aria-hidden", "true");
      await expect(svg).toHaveAttribute("focusable", "false");
      await expect(svg.getBoundingClientRect().width).toBe(24);
    }
  },
};

/** The controls size their icons: `size-5` in a button, `size-4` in a small button and next to a link's text. */
export const InControls: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="primary">
        <PlusIcon />
        New post
      </Button>
      <Button variant="ghost" aria-label="More">
        <MoreIcon />
      </Button>
      <Button size="sm">
        <ReplyIcon />
        Reply
      </Button>
      <Link href="https://example.com/" variant="standalone" className="gap-1">
        example.com
        <ExternalLinkIcon />
      </Link>
    </div>
  ),
  play: async ({ canvas }) => {
    const sizeOf = (element: HTMLElement) => element.querySelector("svg")?.getBoundingClientRect().width;
    await expect(sizeOf(canvas.getByRole("button", { name: "New post" }))).toBe(20);
    await expect(sizeOf(canvas.getByRole("button", { name: "More" }))).toBe(20);
    await expect(sizeOf(canvas.getByRole("button", { name: "Reply" }))).toBe(16);
    await expect(sizeOf(canvas.getByRole("link", { name: "example.com" }))).toBe(16);
  },
};
