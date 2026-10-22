// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Cluster } from "./Cluster";
import { Tag } from "./Tag";
import { Text } from "./Text";

const meta = {
  title: "Display/Tag",
  component: Tag,
  args: { children: "Estimated" },
} satisfies Meta<typeof Tag>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Beside the value it qualifies, wrapping under it when the row is full. */
export const BesideAValue: Story = {
  render: (args) => (
    <Cluster as="span" align="center" gap="2">
      <Text numeric wrap="anywhere">
        $1,234.56 · €78.90 · ¥12,000 · £4.20 · CHF 9.99
      </Text>
      <Tag {...args} />
    </Cluster>
  ),
};
