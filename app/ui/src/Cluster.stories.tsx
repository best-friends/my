// SPDX-License-Identifier: AGPL-3.0-only
// Cluster stories: a wrapping row of buttons, facts with a column gap only, the alignments, and a single line that does
// not wrap.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { Cluster } from "./Cluster";
import { Stack } from "./Stack";
import { Tag } from "./Tag";
import { Text } from "./Text";

const meta = {
  title: "Layout/Cluster",
  component: Cluster,
  args: {
    gap: "2",
    children: (
      <>
        <Button size="sm">Edit</Button>
        <Button size="sm">Change the badge</Button>
        <Button size="sm" variant="danger">
          Delete
        </Button>
      </>
    ),
  },
} satisfies Meta<typeof Cluster>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Controls in a row: `gap="2"`, wrapping when the row is full. */
export const Default: Story = {};

/** `justify="end"` puts the buttons at the inline end (the left in right-to-left text). */
export const JustifyEnd: Story = { args: { justify: "end" } };

/** `justify="between"` pushes the first and the last child apart. */
export const JustifyBetween: Story = {
  args: {
    justify: "between",
    align: "center",
    children: (
      <>
        <Text weight="semibold">Invites</Text>
        <Button size="sm">Create an invite</Button>
      </>
    ),
  },
};

/** Facts of a line separated by a column gap only (`gap="2" rowGap="0"`), with long words that wrap anywhere. */
export const Facts: Story = {
  render: () => (
    <Stack as="ul" gap="1">
      {[
        ["Amazon Web Services", "Compute", "$12.40"],
        ["a-very-long-provider-name-without-any-spaces-that-wraps.example", "Storage", "€3.10"],
        ["خدمة الاستضافة", "Domain", "¥1,200"],
      ].map(([provider, category, amount]) => (
        <Cluster as="li" key={provider} gap="2" rowGap="0">
          <Text dir="auto" wrap="anywhere">
            {provider}
          </Text>
          <Text tone="muted">{category}</Text>
          <Text numeric>{amount}</Text>
        </Cluster>
      ))}
    </Stack>
  ),
};

/** Centered items: an amount and the tag that qualifies it. */
export const AlignCenter: Story = {
  render: () => (
    <Cluster as="span" align="center" gap="2">
      <Text numeric>$1,234.56 · €78.90 · ¥12,000</Text>
      <Tag>Estimated</Tag>
    </Cluster>
  ),
};

/** `wrap={false}` keeps one line; a `shrink` child gives way. */
export const NoWrap: Story = {
  render: () => (
    <Cluster wrap={false} align="center" gap="3" shrink>
      <Tag>Avatar</Tag>
      <Stack shrink>
        <Text weight="semibold" wrap="truncate">
          A display name that is much longer than the room it has on a phone
        </Text>
        <Text size="sm" tone="muted" wrap="truncate" dir="ltr">
          @someone@a-very-long-domain-name.example.social
        </Text>
      </Stack>
    </Cluster>
  ),
};
