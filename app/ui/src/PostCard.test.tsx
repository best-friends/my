// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Avatar } from "./Avatar";
import { PostCard, PostCardAuthor } from "./PostCard";
import { PostContent, serverSanitizedHtml } from "./PostContent";

describe("PostCard", () => {
  it("is an article labelled by its author and time and described by its content", () => {
    render(
      <PostCard
        avatar={<Avatar name="Alice" decorative />}
        author="Alice @alice"
        time={<time dateTime="2026-10-16T09:00:00Z">2h</time>}
        content={<PostContent html={serverSanitizedHtml("<p>Hello</p>")} lang="en" />}
      />,
    );
    const article = screen.getByRole("article", { name: "Alice @alice 2h" });
    expect(article).toHaveAccessibleDescription("Hello");
    // Outside a feed it is a plain article: not focusable, no position.
    expect(article).not.toHaveAttribute("tabindex");
    expect(article).not.toHaveAttribute("aria-posinset");
  });

  it("starts its name with the context line when there is one", () => {
    render(
      <PostCard
        context="Bob boosted"
        author="Alice @alice"
        time={<time dateTime="2026-10-16T09:00:00Z">2h</time>}
        content={<PostContent html={serverSanitizedHtml("<p>Hello</p>")} lang="en" />}
      />,
    );
    expect(screen.getByRole("article", { name: "Bob boosted Alice @alice 2h" })).toHaveAccessibleDescription("Hello");
  });

  it("starts its name with its label, and shows its details outside the description", () => {
    render(
      <PostCard
        label="Selected post"
        highlighted
        lineAbove
        author="Alice @alice"
        time={<time dateTime="2026-10-16T09:00:00Z">2h</time>}
        content={<PostContent html={serverSanitizedHtml("<p>Hello</p>")} lang="en" />}
        details="3 boosts"
      />,
    );
    const article = screen.getByRole("article", { name: "Selected post Alice @alice 2h" });
    expect(article).toHaveAccessibleDescription("Hello");
    expect(screen.getByText("3 boosts")).toBeInTheDocument();
  });
});

describe("PostCardAuthor", () => {
  it("names a link around it by the full name and handle, the handle left to right", () => {
    const handle = "@carol@a-rather-long-community-server-name.social.example.com";
    render(
      <a href="#profile">
        <PostCardAuthor name="Carol" handle={handle} />
      </a>,
    );
    expect(screen.getByRole("link", { name: `Carol ${handle}` })).toBeInTheDocument();
    // The ellipsis of a cut handle is at its end, the domain's side, in a right-to-left page too.
    expect(screen.getByText(handle)).toHaveAttribute("dir", "ltr");
    expect(screen.getByText("Carol")).toHaveAttribute("dir", "auto");
  });

  it("is the name alone without a handle", () => {
    render(
      <a href="#profile">
        <PostCardAuthor name="Bob" />
      </a>,
    );
    expect(screen.getByRole("link", { name: "Bob" })).toBeInTheDocument();
  });
});
