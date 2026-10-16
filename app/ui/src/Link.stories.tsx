// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Link } from "./Link";

const meta = {
  title: "Forms/Link",
  component: Link,
  args: { href: "#", children: "Read the rules" },
} satisfies Meta<typeof Link>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Inside running text the link is underlined, so color is not its only cue. */
export const Inline: Story = {
  render: (args) => (
    <p className="max-w-prose">
      By signing up you agree to the server rules. <Link {...args} variant="inline" /> before you post.
    </p>
  ),
};

export const Standalone: Story = { args: { variant: "standalone", children: "Show more replies" } };

export const Disabled: Story = { args: { variant: "standalone", isDisabled: true, children: "Show more replies" } };
