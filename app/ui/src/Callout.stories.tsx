// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Callout } from "./Callout";
import { Text } from "./Text";

const meta = {
  title: "Display/Callout",
  component: Callout,
} satisfies Meta<typeof Callout>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <Callout>
      <Text as="p" size="sm" tone="fg" wrap="anywhere">
        Your device is set to Asia/Tokyo.
      </Text>
      <Button size="sm">Use the device's time zone</Button>
    </Callout>
  ),
};

export const LongWord: Story = {
  render: () => (
    <div style={{ width: 240 }}>
      <Callout>
        <Text as="p" size="sm" tone="fg" wrap="anywhere">
          Pneumonoultramicroscopicsilicovolcanoconiosispneumonoultramicroscopicsilicovolcanoconiosis
        </Text>
        <Button size="sm">Use</Button>
      </Callout>
    </div>
  ),
};
