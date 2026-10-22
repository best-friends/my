// SPDX-License-Identifier: AGPL-3.0-only
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Avatar } from "./Avatar";

describe("Avatar", () => {
  it("picks the same tint for the same seed, whatever the name", () => {
    const tint = (name: string, seed: string) => {
      const { unmount } = render(<Avatar name={name} seed={seed} decorative={false} />);
      const found = [...screen.getByRole("img", { name }).classList].find((c) => c.startsWith("bg-avatar-"));
      unmount();
      return found;
    };
    expect(tint("Alice", "109")).toBeDefined();
    expect(tint("Alice", "109")).toBe(tint("Alicia", "109"));
    expect(new Set(Array.from({ length: 20 }, (_, i) => tint("A", String(i)))).size).toBeGreaterThan(2);
  });

  it("is an image named after the account when standalone", () => {
    render(<Avatar src="/a.png" name="Alice" decorative={false} />);
    expect(screen.getByRole("img", { name: "Alice" })).toHaveAttribute("src", "/a.png");
  });

  it("has an empty alt when decorative, so the adjacent name is read once", () => {
    render(<Avatar src="/a.png" name="Alice" decorative />);
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.getByRole("presentation")).toHaveAttribute("alt", "");
  });

  it("falls back to the first letter, keeping the name, when the image fails", () => {
    render(<Avatar src="/broken.png" name="bob" decorative={false} />);
    fireEvent.error(screen.getByRole("img", { name: "bob" }));
    const fallback = screen.getByRole("img", { name: "bob" });
    expect(fallback.tagName).toBe("SPAN");
    expect(fallback).toHaveTextContent("B");
  });

  it("tries a new src again after a failure", () => {
    const { rerender } = render(<Avatar src="/broken.png" name="Bob" decorative={false} />);
    fireEvent.error(screen.getByRole("img", { name: "Bob" }));
    rerender(<Avatar src="/b.png" name="Bob" decorative={false} />);
    expect(screen.getByRole("img", { name: "Bob" })).toHaveAttribute("src", "/b.png");
  });

  it("loads the rendition of its sources that fits its size", () => {
    const still = [
      { url: "/s.webp", width: 96 },
      { url: "/m.webp", width: 192 },
    ];
    render(<Avatar sources={{ still }} name="Alice" decorative={false} size="lg" />);
    const img = screen.getByRole("img", { name: "Alice" });
    expect(img).toHaveAttribute("src", "/s.webp");
    expect(img).toHaveAttribute("srcset", "/s.webp 96w, /m.webp 192w");
    expect(img).toHaveAttribute("sizes", "3rem");
    fireEvent.error(img);
    expect(screen.getByRole("img", { name: "Alice" })).toHaveTextContent("A");
  });

  it("takes one grapheme of any script", () => {
    render(<Avatar name="👩‍🚀 Astro" decorative={false} />);
    expect(screen.getByRole("img", { name: "👩‍🚀 Astro" })).toHaveTextContent(/^👩‍🚀$/u);
  });

  it("hides the decorative fallback from assistive technology", () => {
    render(<Avatar name=" " decorative />);
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("merges className onto the root", () => {
    render(<Avatar name="Alice" decorative={false} size="sm" className="size-9" title="Alice" />);
    const root = screen.getByTitle("Alice");
    expect(root).toHaveClass("size-9");
    expect(root).not.toHaveClass("size-8");
  });
});
