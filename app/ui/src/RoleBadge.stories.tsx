// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { RoleBadge, RoleBadgeList } from "./RoleBadge";
import { badgeColors } from "./tokens";

const meta = {
  title: "Display/RoleBadge",
  component: RoleBadge,
  args: { label: "Supporter", color: "blue" },
} satisfies Meta<typeof RoleBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByText("Supporter")).toHaveAttribute("dir", "auto");
  },
};

/** Every badge color, in the order the staff choose from; each keeps 4.5:1 in both themes. */
export const Colors: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-2">
      {badgeColors.map((color) => (
        <RoleBadge key={color} label={color} color={color} />
      ))}
    </div>
  ),
};

/** Labels are the staff's own text in any script; a long one wraps inside the badge. */
export const Labels: Story = {
  render: () => (
    <div className="flex max-w-60 flex-col items-start gap-2">
      <RoleBadge label="サポーター" color="pink" />
      <RoleBadge label="داعم شهري" color="teal" />
      <RoleBadge label="Supporter of the medium plan 2026" color="orange" />
      <RoleBadge label="Averyveryverylongbadgenamewithoutspace" color="yellow" />
    </div>
  ),
};

/** The badges of a profile: a named list, highest role first. */
export const List: StoryObj<typeof RoleBadgeList> = {
  render: () => (
    <RoleBadgeList
      label="Badges"
      badges={[
        { id: "staff", label: "Staff", color: "purple" },
        { id: "supporter", label: "Supporter", color: "green" },
      ]}
    />
  ),
  play: async ({ canvas }) => {
    const list = canvas.getByRole("list", { name: "Badges" });
    await expect(list).toBeVisible();
    await expect(canvas.getAllByRole("listitem")).toHaveLength(2);
  },
};
