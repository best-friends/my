// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ariaSnapshot } from "../test/aria-snapshot";
import { PostContent, serverSanitizedHtml } from "./PostContent";

describe("PostContent", () => {
  it("renders the sanitized HTML with the post's own direction and language", () => {
    const { container } = render(<PostContent html={serverSanitizedHtml('<p>مرحبا <a href="https://example.com/">#tag</a></p>')} lang="ar" />);
    expect(ariaSnapshot(container)).toBe([`- generic [lang=ar] [dir=auto]:`, `  - text: "مرحبا"`, `  - link "#tag"`].join("\n"));
  });

  it("accepts only HTML marked as server-sanitized", () => {
    // @ts-expect-error -- a plain string is not SanitizedHtml; this line is the type test.
    render(<PostContent html="<p>unchecked</p>" />);
    expect(screen.getByText("unchecked")).toBeInTheDocument();
  });
});
