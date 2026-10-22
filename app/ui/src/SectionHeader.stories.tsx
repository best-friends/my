// SPDX-License-Identifier: AGPL-3.0-only
// SectionHeader stories: a heading with actions under it in a narrow container and beside it from @md, and a long
// heading that wraps.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Button } from "./Button";
import { Heading } from "./Heading";
import { SectionHeader } from "./SectionHeader";
import { Stack } from "./Stack";
import { Hint } from "./Text";

const meta = {
  title: "Layout/SectionHeader",
  component: SectionHeader,
  args: {
    children: <Heading level={3}>Moderator</Heading>,
    actions: (
      <>
        <Button size="sm">Edit</Button>
        <Button size="sm" variant="danger">
          Delete
        </Button>
      </>
    ),
  },
  decorators: [
    (Story) => (
      <Stack as="ul" divided>
        <Stack as="li" container paddingY="3" gap="3">
          <Story />
          <Hint>Can suspend accounts and domains.</Hint>
        </Stack>
      </Stack>
    ),
  ],
} satisfies Meta<typeof SectionHeader>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The actions sit under the heading in a narrow container and at the inline end from @md. */
export const Default: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("heading", { level: 3, name: "Moderator" })).toBeVisible();
    await expect(canvas.getByRole("button", { name: "Edit" })).toBeVisible();
  },
};

/** A long name or code wraps anywhere and pushes the actions down, never off the row. */
export const LongHeading: Story = {
  args: {
    children: (
      <Heading level={3} mono wrap="anywhere" focusable dir="ltr">
        INVITE-CODE-THAT-IS-MUCH-LONGER-THAN-ANY-REAL-CODE-0123456789
      </Heading>
    ),
    actions: <Button size="sm">Copy the link</Button>,
  },
};

/** Without actions (`false` from a condition), the heading stands alone. */
export const NoActions: Story = { args: { actions: false } };
