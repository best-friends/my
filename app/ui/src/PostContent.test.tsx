// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
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
