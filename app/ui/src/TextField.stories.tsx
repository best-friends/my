// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";
import { EmojiIcon } from "./Icons";
import { TextField } from "./TextField";
import { Tooltip, TooltipTrigger } from "./Tooltip";

const meta = {
  title: "Forms/TextField",
  component: TextField,
  args: { label: "Display name" },
} satisfies Meta<typeof TextField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithDescription: Story = { args: { description: "Shown on your profile and next to your posts." } };

export const WithPlaceholder: Story = { args: { label: "Search", placeholder: "Posts, people, tags" } };

export const Filled: Story = { args: { defaultValue: "Alice Liddell" } };

export const Invalid: Story = {
  args: {
    label: "Username",
    defaultValue: "alice",
    description: "Letters, digits and underscores.",
    isInvalid: true,
    errorMessage: "This username is already taken.",
  },
};

export const Disabled: Story = { args: { defaultValue: "Alice Liddell", isDisabled: true } };

export const ReadOnly: Story = { args: { label: "Handle", defaultValue: "@alice@example.com", isReadOnly: true } };

export const Required: Story = { args: { label: "Email", type: "email", isRequired: true } };

/** An icon button at the end of the input (the emoji picker's, ADR-0091 §4); a long value stays clear of it. */
export const WithEndAction: Story = {
  args: {
    defaultValue: "Alice Liddell, who went down the rabbit hole and came back with a story",
    endAction: (
      <TooltipTrigger>
        <Button variant="ghost" size="icon" aria-label="Add emoji">
          <EmojiIcon />
        </Button>
        <Tooltip>Add emoji</Tooltip>
      </TooltipTrigger>
    ),
  },
};
