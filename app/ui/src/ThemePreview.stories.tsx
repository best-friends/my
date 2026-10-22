// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { ThemePreview } from "./ThemePreview";

const meta = {
  title: "Display/ThemePreview",
  component: ThemePreview,
  args: {
    tokens: {},
    heading: "Preview",
    text: "Body text on the surface.",
    muted: "Secondary text.",
    link: "A link",
    button: "A button",
    danger: "Something went wrong.",
  },
} satisfies Meta<typeof ThemePreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The overrides apply to the preview alone. */
export const Overridden: Story = { args: { tokens: { "--color-accent": "#0d9488", "--color-surface": "#fdf6e3" } } };
