// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Avatar } from "./Avatar";
import { PostCard } from "./PostCard";
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
});
