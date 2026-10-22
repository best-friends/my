// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { HomeIcon } from "./Icons";
import { IconLabel } from "./IconLabel";

const meta = {
  title: "Display/IconLabel",
  component: IconLabel,
  args: { icon: <HomeIcon />, children: "Home" },
} satisfies Meta<typeof IconLabel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const LongWord: Story = {
  args: {
    children: "Pneumonoultramicroscopicsilicovolcanoconiosispneumonoultramicroscopicsilicovolcanoconiosis",
  },
  decorators: [
    (Story) => (
      <div style={{ width: 200 }}>
        <Story />
      </div>
    ),
  ],
};
