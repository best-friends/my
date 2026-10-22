// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { CodeBlock } from "./CodeBlock";

const meta = {
  title: "Display/CodeBlock",
  component: CodeBlock,
  args: { dir: "ltr", children: '{"id":"01J0","kind":"deliver","target":"https://example.com/users/alice/inbox"}' },
} satisfies Meta<typeof CodeBlock>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** A long word wraps instead of scrolling the page sideways. */
export const LongWord: Story = { args: { children: "a".repeat(200) } };
