// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { TextField } from "./TextField";

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
