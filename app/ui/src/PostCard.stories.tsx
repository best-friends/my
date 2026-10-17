// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Avatar } from "./Avatar";
import { Button } from "./Button";
import { ContentWarning } from "./ContentWarning";
import { BoostIcon } from "./Icons";
import { MediaGrid } from "./MediaGrid";
import { PostCard, PostCardAuthor } from "./PostCard";
import { PostContent, serverSanitizedHtml } from "./PostContent";

function picture(fill: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 90"><rect width="160" height="90" fill="${fill}"/><circle cx="80" cy="45" r="20" fill="#f2c14e"/></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function Author({ name, handle }: { name: string; handle: string }) {
  return (
    <a href="#profile" className="block min-w-0 text-fg hover:underline">
      <PostCardAuthor name={name} handle={handle} />
    </a>
  );
}

/**
 * Checks that the header of the card takes at most two lines on a 320 px card: the name and the time,
 * and the handle under them. The frame is the story's container, set back after the check.
 */
async function expectHeaderInTwoLines(canvasElement: HTMLElement) {
  const frame = canvasElement.closest<HTMLElement>("[data-story-frame]") ?? canvasElement;
  const previous = frame.style.inlineSize;
  frame.style.inlineSize = "320px";
  try {
    const header = canvasElement.querySelector("header");
    const name = header?.querySelector<HTMLElement>("[dir=auto]");
    if (header == null || name == null) throw new Error("the card has no header with a name");
    const lineHeight = Number.parseFloat(getComputedStyle(name).lineHeight);
    await expect(header.getBoundingClientRect().height).toBeLessThanOrEqual(2 * lineHeight);
  } finally {
    frame.style.inlineSize = previous;
  }
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

/** A boost: the line above the post says who boosted it, and starts the article's name. */
export const Boosted: Story = {
  args: {
    context: (
      <>
        <BoostIcon />
        <span dir="auto">Bob boosted</span>
      </>
    ),
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("article", { name: /^Bob boosted\s+Alice\s+@alice\s+2h$/ })).toBeInTheDocument();
  },
};

/** A long display name and handle are cut with an ellipsis; the link is still named by both in full. */
export const LongNames: Story = {
  args: {
    avatar: <Avatar name="Bartholomew" decorative />,
    author: <Author name="Bartholomew Montgomery-Fitzwilliam the Third" handle="@bartholomew_montgomery_fitzwilliam@social.example.com" />,
  },
  play: async ({ canvas, canvasElement }) => {
    await expect(
      canvas.getByRole("link", { name: "Bartholomew Montgomery-Fitzwilliam the Third @bartholomew_montgomery_fitzwilliam@social.example.com" }),
    ).toBeInTheDocument();
    await expectHeaderInTwoLines(canvasElement);
  },
};

/** A handle of a server with a long domain: the domain is cut, the user part stays. */
export const LongDomain: Story = {
  args: {
    avatar: <Avatar name="Carol" decorative />,
    author: <Author name="Carol" handle="@carol@a-rather-long-community-server-name.social.example.com" />,
  },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getByRole("link", { name: "Carol @carol@a-rather-long-community-server-name.social.example.com" })).toBeInTheDocument();
    await expectHeaderInTwoLines(canvasElement);
  },
};

/** A display name in a right-to-left script lines up with the card's direction, not its own. */
export const RightToLeftName: Story = {
  args: {
    avatar: <Avatar name="سارة" decorative />,
    author: <Author name="سارة الحسيني من مجتمع الرسامين المستقلين" handle="@sara@fann.example" />,
  },
  play: async ({ canvasElement }) => {
    await expectHeaderInTwoLines(canvasElement);
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

/**
 * The post a page is about: its own surface and a bar on its start edge, a name that tells it apart, and
 * its full date and counts in words between the body and the actions.
 */
export const Highlighted: Story = {
  args: {
    label: "Selected post",
    highlighted: true,
    time: <time dateTime="2026-10-16T09:00:00Z">2h</time>,
    content: <PostContent html={serverSanitizedHtml("<p>The post this page is about, in larger text.</p>")} lang="en" className="text-lg" />,
    details: (
      <>
        <time dateTime="2026-10-16T09:00:00Z">Friday, October 16, 2026 at 9:00 AM</time>
        <span className="tabular-nums">3 boosts</span>
        <span className="tabular-nums">12 favourites</span>
      </>
    ),
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("article", { name: /^Selected post\s+Alice\s+@alice\s+2h$/ })).toHaveAccessibleDescription(/The post this page is about/);
    await expect(canvas.getByText("3 boosts")).toBeVisible();
  },
};

/**
 * Three posts of a thread, each a reply to the one above: a line joins their avatars. The middle one is
 * the highlighted post of the page; the cards with a line below keep their avatar's column free.
 */
export const Thread: Story = {
  render: (args) => (
    <div>
      <PostCard {...args} lineBelow />
      <PostCard
        {...args}
        avatar={<Avatar name="Bob" decorative />}
        author={<Author name="Bob" handle="@bob" />}
        content={<PostContent html={serverSanitizedHtml("<p>A reply, and the post this page is about.</p>")} lang="en" className="text-lg" />}
        label="Selected post"
        highlighted
        lineAbove
        lineBelow
      />
      <PostCard {...args} content={<PostContent html={serverSanitizedHtml("<p>A reply to the reply.</p>")} lang="en" />} lineAbove />
    </div>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("article")).toHaveLength(3);
    await expect(canvas.getByRole("article", { name: /^Selected post\s+Bob/ })).toBeInTheDocument();
  },
};

export const WithoutAvatarOrActions: Story = {
  args: { avatar: undefined, actions: undefined },
};
