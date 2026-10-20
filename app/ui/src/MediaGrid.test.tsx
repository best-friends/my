// SPDX-License-Identifier: AGPL-3.0-only
import { fireEvent, render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ariaSnapshot } from "../test/aria-snapshot";
import { MediaGrid, type MediaItem } from "./MediaGrid";

const items: readonly MediaItem[] = [
  { id: "a", src: "/a.png", alt: "A cat on a sofa", lang: "en" },
  { id: "b", src: "/b.png", alt: null },
];

describe("MediaGrid", () => {
  it("names each picture by its description, or says it has none", () => {
    const { container } = render(<MediaGrid items={items} noDescriptionLabel="No description" revealLabel="Show sensitive media" />);
    expect(ariaSnapshot(container)).toBe([`- img "A cat on a sofa" [lang=en] [dir=auto]`, `- img "No description"`].join("\n"));
    // The badge is drawn for sighted users; the image's name already says it to screen readers.
    expect(screen.getByText("No description")).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("shows at most four pictures", () => {
    const many = ["1", "2", "3", "4", "5"].map((id) => ({ id, src: `/${id}.png`, alt: `Picture ${id}` }));
    render(<MediaGrid items={many} noDescriptionLabel="No description" revealLabel="Show sensitive media" />);
    expect(screen.getAllByRole("img")).toHaveLength(4);
  });

  it("conceals sensitive media until the toggle is pressed, keeping focus on it", async () => {
    const user = userEvent.setup();
    const onRevealedChange = vi.fn();
    render(<MediaGrid items={items} sensitive noDescriptionLabel="No description" revealLabel="Show sensitive media" onRevealedChange={onRevealedChange} />);
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    const toggle = screen.getByRole("button", { name: "Show sensitive media" });
    expect(toggle).toHaveAttribute("aria-pressed", "false");

    await user.tab();
    await user.keyboard(" ");
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(toggle).toHaveFocus();
    expect(screen.getAllByRole("img")).toHaveLength(2);

    await user.keyboard(" ");
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(onRevealedChange.mock.calls).toEqual([[true], [false]]);
  });

  it("covers sensitive media with their blur until the picture has loaded", async () => {
    const user = userEvent.setup();
    const blurred: readonly MediaItem[] = [
      { id: "a", src: "/a.png", alt: "A cat on a sofa", blurhash: "LEHV6nWB2yk8pyo0adR*.7kCMdnj" },
      { id: "b", src: "/b.png", alt: "A dog", blurhash: "not a blurhash" },
      { id: "c", src: "/c.png", alt: "A bird" },
    ];
    const { container } = render(<MediaGrid items={blurred} sensitive noDescriptionLabel="No description" revealLabel="Show sensitive media" />);
    // Only a valid blurhash draws a blur; the other cells keep their plain surface. The blur is decorative.
    // eslint-disable-next-line testing-library/no-container, testing-library/no-node-access -- a decorative canvas has no role.
    const blurs = () => container.querySelectorAll("canvas");
    expect(blurs()).toHaveLength(1);
    expect(blurs()[0]).toHaveAttribute("aria-hidden", "true");

    await user.click(screen.getByRole("button", { name: "Show sensitive media" }));
    expect(blurs()).toHaveLength(1);
    fireEvent.load(screen.getByRole("img", { name: "A cat on a sofa" }));
    expect(blurs()).toHaveLength(0);
  });
});
