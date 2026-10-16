// SPDX-License-Identifier: AGPL-3.0-only
// Every story is also captured with prefers-reduced-motion: reduce, where the setup fails on any
// running animation (.storybook/vitest.setup.ts); the plays here check the pulse without it.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Skeleton } from "./Skeleton";
import { VisuallyHidden } from "./VisuallyHidden";

const meta = {
  title: "Display/Skeleton",
  component: Skeleton,
} satisfies Meta<typeof Skeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The pulse follows the user's motion preference: it runs with no preference and stops with reduce. */
async function expectPulseFollowsMotionPreference(root: HTMLElement) {
  const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  for (const shape of root.querySelectorAll("[aria-hidden='true']")) {
    await expect(getComputedStyle(shape).animationName).toBe(reduce ? "none" : "pulse");
  }
}

export const Shapes: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      <Skeleton shape="text" />
      <Skeleton shape="circle" />
      <Skeleton shape="rect" />
    </div>
  ),
  play: async ({ canvasElement }) => {
    await expectPulseFollowsMotionPreference(canvasElement);
  },
};

/**
 * A loading region: it is `aria-busy` and says what is loading, while the skeletons stay out of the
 * accessibility tree. When the posts arrive, the region drops `aria-busy` and renders them instead.
 */
export const LoadingRegion: Story = {
  render: () => (
    <section aria-labelledby="skeleton-posts" aria-busy="true" className="flex flex-col gap-4">
      <h2 id="skeleton-posts" className="text-lg font-semibold">
        Posts
      </h2>
      <VisuallyHidden>Loading posts</VisuallyHidden>
      {["first", "second"].map((key) => (
        <div key={key} className="flex gap-3">
          <Skeleton shape="circle" />
          <div className="flex flex-1 flex-col gap-2 text-base">
            <Skeleton className="w-1/3" />
            <Skeleton />
            <Skeleton className="w-2/3" />
          </div>
        </div>
      ))}
    </section>
  ),
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getByRole("region", { name: "Posts" })).toHaveAttribute("aria-busy", "true");
    await expect(canvas.getByText("Loading posts")).toBeInTheDocument();
    await expectPulseFollowsMotionPreference(canvasElement);
  },
};
