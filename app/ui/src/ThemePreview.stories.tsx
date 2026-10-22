// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import type { CSSProperties } from "react";
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
export const Overridden: Story = {
  args: {
    tokens: {
      "--color-accent": "light-dark(#0f766e, #5eead4)",
      "--color-accent-fg": "light-dark(#ffffff, #042f2e)",
      "--color-surface": "light-dark(#fdf6e3, #002b36)",
    } as CSSProperties,
  },
};
