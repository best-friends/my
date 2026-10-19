// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ariaSnapshot } from "../test/aria-snapshot";
import { PostContent, serverSanitizedHtml } from "./PostContent";

describe("PostContent", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders the sanitized HTML with the post's own direction and language", () => {
    const { container } = render(<PostContent html={serverSanitizedHtml('<p>مرحبا <a href="https://example.com/">#tag</a></p>')} lang="ar" />);
    expect(ariaSnapshot(container)).toBe([`- generic [lang=ar] [dir=auto]:`, `  - text: "مرحبا"`, `  - link "#tag"`].join("\n"));
  });

  it("accepts only HTML marked as server-sanitized", () => {
    // @ts-expect-error -- a plain string is not SanitizedHtml; this line is the type test.
    render(<PostContent html="<p>unchecked</p>" />);
    expect(screen.getByText("unchecked")).toBeInTheDocument();
  });

  it("hands the links followed in the same tab to onLinkClick", async () => {
    const user = userEvent.setup();
    const followed: string[] = [];
    const onLinkClick = vi.fn((event: MouseEvent, link: HTMLAnchorElement) => {
      followed.push(link.href);
      // jsdom does not navigate; the handler keeps the browser from it as the app does.
      event.preventDefault();
    });
    const html = serverSanitizedHtml(
      '<p><a href="https://example.com/tags/cats"><span>#</span>cats</a> <a href="https://example.com/x" target="_blank">x</a> <a href="https://example.com/y" download>y</a></p>',
    );
    render(<PostContent html={html} onLinkClick={onLinkClick} />);

    await user.click(screen.getByText("cats"));
    await user.click(screen.getByText("#"));
    await user.keyboard("{Enter}");
    await user.click(screen.getByRole("link", { name: "x" }));
    await user.click(screen.getByRole("link", { name: "y" }));
    await user.keyboard("{Control>}");
    await user.click(screen.getByRole("link", { name: "#cats" }));
    await user.keyboard("{/Control}");

    expect(followed).toEqual(["https://example.com/tags/cats", "https://example.com/tags/cats", "https://example.com/tags/cats"]);
    expect(onLinkClick.mock.calls.every(([event]) => event.defaultPrevented)).toBe(true);
  });

  it("passes the HTML through a Trusted Types policy where the browser has Trusted Types", () => {
    // A stand-in for the browser's factory: its TrustedHTML is an object that innerHTML turns into its string.
    const createHTML = vi.fn((html: string) => ({ toString: () => html }));
    const createPolicy = vi.fn((_name: string, rules: { createHTML: (html: string) => string }) => ({
      createHTML: (html: string) => createHTML(rules.createHTML(html)),
    }));
    vi.stubGlobal("trustedTypes", { createPolicy });

    render(<PostContent html={serverSanitizedHtml("<p>trusted</p>")} />);
    render(<PostContent html={serverSanitizedHtml("<p>again</p>")} />);

    expect(screen.getByText("trusted")).toBeInTheDocument();
    expect(screen.getByText("again")).toBeInTheDocument();
    expect(createPolicy).toHaveBeenCalledOnce();
    expect(createHTML).toHaveBeenCalledWith("<p>trusted</p>");
  });
});
