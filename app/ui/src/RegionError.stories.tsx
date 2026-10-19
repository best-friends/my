// SPDX-License-Identifier: AGPL-3.0-only
// RegionError stories. The plays check that an error the page opened with is not a live region, and that a retry
// that fails again is announced as an alert while focus stays on the retry button.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn } from "storybook/test";
import { RegionError } from "./RegionError";

const meta = {
  title: "Feedback/RegionError",
  component: RegionError,
  args: {
    heading: "Couldn't load the posts",
    reason: "The server took too long to answer.",
    retryLabel: "Try again",
    onRetry: fn(),
  },
} satisfies Meta<typeof RegionError>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The failure of a region the page opened with: no live region, a paragraph, and the retry button. */
export const Default: Story = {
  play: async ({ args, canvas, userEvent }) => {
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
    await expect(canvas.getByText("Couldn't load the posts")).toBeVisible();
    await userEvent.click(canvas.getByRole("button", { name: "Try again" }));
    await expect(args.onRetry).toHaveBeenCalledOnce();
  },
};

/** In place of a region headed by an h2, the message is an h3. */
export const InARegion: Story = {
  render: () => (
    <section aria-labelledby="region-error-replies" className="flex flex-col gap-4">
      <h2 id="region-error-replies" className="text-lg font-semibold">
        Replies
      </h2>
      <RegionError
        heading="Couldn't load the replies"
        headingLevel={3}
        reason="The server took too long to answer."
        retryLabel="Try again"
        onRetry={fn()}
      />
    </section>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("heading", { level: 3, name: "Couldn't load the replies" })).toBeVisible();
  },
};

/** While the retry runs, the button waits and keeps its focus. */
export const Retrying: Story = {
  args: { isRetrying: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("button", { name: "Try again" })).toHaveAttribute("data-pending", "true");
  },
};

/** Without `onRetry` there is no button, for a failure that retrying cannot fix. */
export const WithoutRetry: Story = {
  render: () => <RegionError heading="This list is not available" reason="The account hides who it follows." />,
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("button")).not.toBeInTheDocument();
  },
};

export const LongReason: Story = {
  args: {
    heading: "Couldn't load the followers of @someone-with-a-long-name@very-long-subdomain.example.social",
    reason:
      "https://very-long-subdomain.example.social/users/someone-with-a-long-name/followers answered 502 Bad Gateway. The server may be restarting; try again in a few minutes.",
  },
};

function RetryDemo() {
  const [attempts, setAttempts] = useState(0);
  return (
    <RegionError
      heading="Couldn't load the posts"
      reason="The server took too long to answer."
      retryLabel="Try again"
      onRetry={() => {
        setAttempts((count) => count + 1);
      }}
      // The first error came with the page; a retry the user asked for that fails again is announced.
      alert={attempts > 0}
    />
  );
}

/** A retry that fails again becomes an alert; focus stays on the retry button. */
export const FailedAgain: Story = {
  render: () => <RetryDemo />,
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.queryByRole("alert")).not.toBeInTheDocument();
    const retry = canvas.getByRole("button", { name: "Try again" });
    await userEvent.click(retry);
    await expect(canvas.getByRole("alert")).toHaveTextContent("Couldn't load the posts");
    await expect(retry).toHaveFocus();
  },
};
