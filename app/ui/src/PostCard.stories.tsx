// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Avatar } from "./Avatar";
import { Button } from "./Button";
import { ContentWarning } from "./ContentWarning";
import { MediaGrid } from "./MediaGrid";
import { PostCard } from "./PostCard";
import { PostContent, serverSanitizedHtml } from "./PostContent";

function picture(fill: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><rect width="160" height="90" fill="${fill}"/><circle cx="80" cy="45" r="20" fill="#f2c14e"/></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function Author({ name, handle }: { name: string; handle: string }) {
  return (
    <>
      <a href="#profile" dir="auto" className="font-semibold text-fg hover:underline">
        {name}
      </a>{" "}
      <span dir="ltr" className="text-sm text-fg-muted">
        {handle}
      </span>
    </>
  );
}

function Actions() {
  return (
    <>
      <Button variant="ghost" size="sm">
        Reply
      </Button>
      <Button variant="ghost" size="sm">
        Boost
      </Button>
      <Button variant="ghost" size="sm">
        Favourite
      </Button>
    </>
  );
}

const meta = {
  title: "Timeline/PostCard",
  component: PostCard,
  args: {
    avatar: <Avatar name="Alice" decorative />,
    author: <Author name="Alice" handle="@alice" />,
    time: (
      <a href="#post" className="hover:underline">
        <time dateTime="2026-10-16T09:00:00Z">2h</time>
      </a>
    ),
    content: <PostContent html={serverSanitizedHtml("<p>Hello from the local timeline! Containers decide this layout, not the viewport.</p>")} lang="en" />,
    actions: <Actions />,
  },
} satisfies Meta<typeof PostCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvas }) => {
    const article = canvas.getByRole("article", { name: /Alice\s+@alice\s+2h/ });
    await expect(article).toHaveAccessibleDescription(/Hello from the local timeline/);
  },
};

export const WithMedia: Story = {
  args: {
    media: (
      <MediaGrid
        items={[
          { id: "1", src: picture("#4f6d8f"), alt: "A sun over a blue sky" },
          { id: "2", src: picture("#3d7a52"), alt: null },
        ]}
        noDescriptionLabel="No description"
        revealLabel="Show sensitive media"
      />
    ),
  },
};

export const WithContentWarning: Story = {
  args: {
    content: (
      <ContentWarning summary="Food (pictures of a very large cake)" toggleLabel="Show content">
        <PostContent html={serverSanitizedHtml("<p>It took three days to bake.</p>")} lang="en" />
        <MediaGrid
          items={[{ id: "1", src: picture("#7a3d6b"), alt: "A cake" }]}
          sensitive
          noDescriptionLabel="No description"
          revealLabel="Show sensitive media"
        />
      </ContentWarning>
    ),
  },
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.queryByRole("img", { name: "A cake" })).not.toBeInTheDocument();
    await userEvent.click(canvas.getByRole("button", { name: "Show content" }));
    await expect(canvas.getByText("It took three days to bake.")).toBeVisible();
    // The media inside stays concealed until they are shown on their own.
    await expect(canvas.getByRole("button", { name: "Show sensitive media" })).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(canvas.getByRole("button", { name: "Show content" }));
  },
};

/** A long display name and handle wrap inside the card at 320 px. */
export const LongNames: Story = {
  args: {
    avatar: <Avatar name="Bartholomew" decorative />,
    author: <Author name="Bartholomew Montgomery-Fitzwilliam the Third" handle="@bartholomew_montgomery_fitzwilliam@social.example.com" />,
  },
};

/** A post written in a right-to-left script. */
export const RightToLeftPost: Story = {
  args: {
    avatar: <Avatar name="سارة" decorative />,
    author: <Author name="سارة" handle="@sara" />,
    content: <PostContent html={serverSanitizedHtml("<p>صباح الخير من الخط الزمني المحلي!</p>")} lang="ar" />,
  },
};

export const WithoutAvatarOrActions: Story = {
  args: { avatar: undefined, actions: undefined },
};
