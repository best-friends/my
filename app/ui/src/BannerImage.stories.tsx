// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { BannerImage } from "./BannerImage";

/** A wide stand-in picture, inline so the story does not depend on the network. */
const wide = `data:image/svg+xml,${encodeURIComponent(
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 600"><rect width="1200" height="600" fill="#3b6fd8"/><circle cx="600" cy="300" r="160" fill="#f4d7b5"/></svg>',
)}`;

const meta = {
  title: "Display/BannerImage",
  component: BannerImage,
  args: { src: wide, alt: "A blue banner with a circle", width: 1200, height: 600 },
} satisfies Meta<typeof BannerImage>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Without alt text the image is decorative. */
export const Decorative: Story = { args: { alt: "" } };
