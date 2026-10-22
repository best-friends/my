// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Card } from "./Card";
import { Heading } from "./Heading";
import { Stack } from "./Stack";
import { Hint, Text } from "./Text";

const meta = {
  title: "Layout/Card",
  component: Card,
  args: {
    padding: "6",
    children: (
      <Stack gap="2">
        <Text as="p" eyebrow>
          Welcome to my
        </Text>
        <Heading level={3} spark="✳">
          A small place for what is on your mind
        </Heading>
        <Hint>Share small thoughts, safely.</Hint>
      </Stack>
    ),
  },
} satisfies Meta<typeof Card>;

export default meta;
type Story = StoryObj<typeof meta>;

/** `padding="6"`: a panel of a page. */
export const Default: Story = {};

/** `padding="4"`: a side panel. */
export const Compact: Story = { args: { padding: "4" } };

/** `padding="8"`: a settings pane. */
export const Spacious: Story = { args: { padding: "8" } };

/** `padding="0"`: rows that bring their own padding, divided by `border`. */
export const Rows: Story = {
  args: { padding: "0", as: "section", "aria-label": "Settings" },
  render: (args) => (
    <Card {...args}>
      <Stack as="ul" divided>
        <Stack as="li" gap="1">
          <Text weight="semibold">Private account</Text>
          <Hint>Only people you approve see your posts</Hint>
        </Stack>
        <Stack as="li" gap="1">
          <Text weight="semibold">Email notifications</Text>
          <Hint>Only important notices</Hint>
        </Stack>
      </Stack>
    </Card>
  ),
};
