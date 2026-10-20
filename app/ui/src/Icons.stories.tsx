// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ComponentType } from "react";
import { expect } from "storybook/test";
import { spacing } from "./storySpacing";
import { Button } from "./Button";
import * as Icons from "./Icons";
import type { ToggleIconProps } from "./Icons";
import { Link } from "./Link";

export default {
  title: "Foundations/Icons",
} satisfies Meta;

type Story = StoryObj;

/** Each row of the table, with the export of the same name that renders it. */
const entries = (Object.keys(Icons.iconTable) as (keyof typeof Icons.iconTable)[]).map((name) => {
  const entry = Icons.iconTable[name];
  return {
    name,
    Icon: Icons[name] as ComponentType<ToggleIconProps>,
    phosphor: entry.off.phosphor,
    phosphorOn: "on" in entry ? entry.on.phosphor : undefined,
    directional: "directional" in entry && entry.directional,
  };
});

/**
 * Every icon of the kit, from the table in Icons.tsx: its name, the Phosphor icon that draws it at the
 * navigation size (`size-6`), the shape of a toggle that is on, and whether it mirrors in right-to-left text.
 * Look a shape up by its Phosphor name at https://phosphoricons.com/.
 */
export const Gallery: Story = {
  render: () => (
    <table className="w-full border-collapse text-start text-sm">
      <thead>
        <tr className="border-b border-border text-fg-muted">
          <th scope="col" className="p-2 text-start font-medium">
            Name
          </th>
          <th scope="col" className="p-2 text-start font-medium">
            Off
          </th>
          <th scope="col" className="p-2 text-start font-medium">
            On
          </th>
          <th scope="col" className="p-2 text-start font-medium">
            Direction
          </th>
        </tr>
      </thead>
      <tbody>
        {entries.map(({ name, Icon, phosphor, phosphorOn, directional }) => (
          <tr key={name} className="border-b border-border text-fg">
            <th scope="row" className="p-2 text-start font-normal">
              <code className="font-mono wrap-anywhere">{name}</code>
            </th>
            <td className="p-2">
              <span className="flex items-center gap-2 [&>svg]:size-6 [&>svg]:shrink-0">
                <Icon />
                <code className="min-w-0 font-mono text-xs text-fg-muted wrap-anywhere">{phosphor}</code>
              </span>
            </td>
            <td className="p-2">
              {phosphorOn && (
                <span className="flex items-center gap-2 text-accent [&>svg]:size-6 [&>svg]:shrink-0">
                  <Icon filled />
                  <code className="min-w-0 font-mono text-xs text-fg-muted wrap-anywhere">{phosphorOn}</code>
                </span>
              )}
            </td>
            <td className="p-2">{directional ? "Mirrors in RTL" : ""}</td>
          </tr>
        ))}
      </tbody>
    </table>
  ),
  play: async ({ canvasElement }) => {
    const rtl = canvasElement.ownerDocument.documentElement.dir === "rtl";
    const rows = [...canvasElement.querySelectorAll("tbody tr")];
    await expect(rows).toHaveLength(entries.length);
    for (const [index, { directional, phosphorOn }] of entries.entries()) {
      const row = rows[index];
      const svgs = [...(row?.querySelectorAll("svg") ?? [])];
      await expect(svgs).toHaveLength(phosphorOn ? 2 : 1);
      for (const svg of svgs) {
        await expect(svg).toHaveAttribute("aria-hidden", "true");
        await expect(svg).toHaveAttribute("focusable", "false");
        await expect(svg.getBoundingClientRect().width).toBeCloseTo(spacing(6), 1);
        // Only an icon with a direction mirrors, and only in right-to-left text.
        await expect(getComputedStyle(svg).scale.startsWith("-1")).toBe(rtl && directional);
      }
    }
  },
};

/** The controls size their icons: `size-5` in a button, `size-4` in a small button and next to a link's text. */
export const InControls: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      <Button variant="primary">
        <Icons.PlusIcon />
        New post
      </Button>
      <Button variant="ghost" aria-label="More">
        <Icons.MoreIcon />
      </Button>
      <Button size="sm">
        <Icons.ReplyIcon />
        Reply
      </Button>
      <Link href="#example" variant="standalone" className="gap-1">
        example.com
        <Icons.ExternalLinkIcon />
      </Link>
    </div>
  ),
  play: async ({ canvas }) => {
    const sizeOf = (element: HTMLElement) => element.querySelector("svg")?.getBoundingClientRect().width;
    await expect(sizeOf(canvas.getByRole("button", { name: "New post" }))).toBeCloseTo(spacing(5), 1);
    await expect(sizeOf(canvas.getByRole("button", { name: "More" }))).toBeCloseTo(spacing(5), 1);
    await expect(sizeOf(canvas.getByRole("button", { name: "Reply" }))).toBeCloseTo(spacing(4), 1);
    await expect(sizeOf(canvas.getByRole("link", { name: "example.com" }))).toBeCloseTo(spacing(4), 1);
  },
};
