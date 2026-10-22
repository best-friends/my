// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { OutboxItem } from "./OutboxItem";
import { Stack } from "./Stack";

const meta = {
  title: "Compose/OutboxItem",
  component: OutboxItem,
  args: { text: "Good morning, everyone.\nThe garden is in bloom today." },
  render: (args) => (
    <Stack as="ul" aria-label="Waiting to be sent" gap="2" width="prose">
      <OutboxItem {...args} />
    </Stack>
  ),
} satisfies Meta<typeof OutboxItem>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The post is on its way: a spinner and the item is `aria-busy`. */
export const Sending: Story = { args: { sendingLabel: "Sending" } };

/** The send failed: the reason and the actions. */
export const Failed: Story = {
  args: {
    error: "Could not send: the server did not answer.",
    children: (
      <>
        <Button size="sm">Retry</Button>
        <Button size="sm" variant="ghost">
          Edit
        </Button>
        <Button size="sm" variant="ghost">
          Discard
        </Button>
      </>
    ),
  },
};

/** A long text is cut after three lines, a right-to-left text takes its own direction, and a long word wraps. */
export const LongText: Story = {
  args: {
    text: "مرحبا بالجميع، هذا نص طويل جدا يتجاوز ثلاثة أسطر ليظهر كيف يقص النص عند الحد المسموح به في قائمة الانتظار.\n\nSecond paragraph with https://a-very-long-subdomain.example.social/users/someone/statuses/112233445566778899\nFourth line",
    error: "Not sent yet",
    children: <Button size="sm">Retry</Button>,
  },
};
