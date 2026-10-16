// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "./Button";

const meta = {
  title: "Forms/Button",
  component: Button,
  args: { children: "Post" },
} satisfies Meta<typeof Button>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = { args: { variant: "primary" } };
export const Secondary: Story = { args: { variant: "secondary" } };
export const Ghost: Story = { args: { variant: "ghost" } };
export const Danger: Story = { args: { variant: "danger", children: "Delete post" } };

const variants = ["primary", "secondary", "ghost", "danger"] as const;
const sizes = ["sm", "md"] as const;

/** Every variant × size, enabled, disabled and pending. */
export const Matrix: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      {sizes.map((size) => (
        <div key={size} className="flex flex-wrap items-center gap-3">
          {variants.map((variant) => (
            <Button key={variant} variant={variant} size={size}>
              {variant} {size}
            </Button>
          ))}
          {variants.map((variant) => (
            <Button key={`${variant}-disabled`} variant={variant} size={size} isDisabled>
              {variant} disabled
            </Button>
          ))}
          <Button variant="primary" size={size} isPending>
            Posting…
          </Button>
        </div>
      ))}
    </div>
  ),
};

/** A long label wraps instead of overflowing a narrow container. */
export const LongLabel: Story = {
  args: { variant: "primary", children: "Publish this post to everyone who follows you, and to the local timeline" },
};

/** An icon-only button is named with aria-label. */
export const IconOnly: Story = {
  args: {
    variant: "ghost",
    "aria-label": "Close",
    children: (
      <svg viewBox="0 0 24 24" aria-hidden className="size-5 fill-none stroke-current stroke-2">
        <path d="M6 6l12 12M18 6 6 18" />
      </svg>
    ),
  },
};
