// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { PostContent, serverSanitizedHtml } from "./PostContent";

const meta = {
  title: "Timeline/PostContent",
  component: PostContent,
  args: {
    html: serverSanitizedHtml(
      '<p>Trying out the new instance. Say hi to <a href="https://example.com/@bob">@bob</a> and follow <a href="https://example.com/tags/fediverse">#fediverse</a>.</p><p>Second paragraph.</p>',
    ),
    lang: "en",
  },
} satisfies Meta<typeof PostContent>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("link", { name: "@bob" })).toBeVisible();
    await expect(canvas.getByText(/Trying out/).parentElement).toHaveAttribute("dir", "auto");
  },
};

/** A post in a right-to-left script reads right to left in any page direction (`dir="auto"`). */
export const RightToLeftText: Story = {
  args: { html: serverSanitizedHtml("<p>مرحبا بالجميع! هذا أول منشور لي هنا.</p>"), lang: "ar" },
};

/** Lists, quotes and code from the sanitizer's allowlist. */
export const RichText: Story = {
  args: {
    html: serverSanitizedHtml(
      "<p>Release notes:</p><ul><li>Feeds</li><li>Content warnings</li></ul><blockquote><p>Quoted text</p></blockquote><p><code>make check</code> passes.</p>",
    ),
  },
};

/** A long URL wraps instead of making the page scroll sideways at 320 px. */
export const LongLink: Story = {
  args: {
    html: serverSanitizedHtml(
      '<p>See <a href="https://example.com/a/very/long/path/that/does/not/fit/on/a/phone/screen/at/all">https://example.com/a/very/long/path/that/does/not/fit/on/a/phone/screen/at/all</a></p>',
    ),
  },
};

/** `size="sm" lineClamp={3}`: an excerpt of a post in a form, cut after three lines. */
export const Excerpt: Story = {
  args: { size: "sm", lineClamp: 3, html: serverSanitizedHtml("<p>One</p><p>Two</p><p>Three</p><p>Four</p><p>Five</p>") },
};

/** A quote and a code block in a right-to-left script: the boxes and their padding mirror with the text. */
export const RightToLeftQuote: Story = {
  args: {
    html: serverSanitizedHtml("<p>مرحبا</p><blockquote><p>اقتباس من منشور آخر</p></blockquote><pre><code>make check</code></pre>"),
    lang: "ar",
  },
};
