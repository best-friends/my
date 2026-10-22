// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Heading } from "./Heading";
import { Hint, InlineError, Text } from "./Text";

describe("Text", () => {
  it("is a span unless `as` says otherwise", () => {
    render(
      <>
        <Text numeric>12</Text>
        <Text as="p" tone="muted">
          Muted
        </Text>
      </>,
    );
    expect(screen.getByText("12").tagName).toBe("SPAN");
    expect(screen.getByText("12")).toHaveClass("tabular-nums");
    expect(screen.getByText("Muted").tagName).toBe("P");
  });

  it("gives a hint the secondary role, or only its tone", () => {
    render(
      <>
        <Hint>Small</Hint>
        <Hint size="inherit">Inherited</Hint>
      </>,
    );
    expect(screen.getByText("Small")).toHaveClass("text-sm", "text-fg-muted");
    expect(screen.getByText("Inherited")).not.toHaveClass("text-sm");
  });
});

describe("InlineError", () => {
  it("is not announced unless it is an alert", () => {
    const { rerender } = render(<InlineError>Failed</InlineError>);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    rerender(<InlineError alert>Failed</InlineError>);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed");
  });
});

describe("Heading", () => {
  it("takes its level and its look apart", () => {
    render(
      <Heading level={2} size="subsection">
        Name
      </Heading>,
    );
    const heading = screen.getByRole("heading", { level: 2, name: "Name" });
    expect(heading).toHaveClass("font-semibold");
    expect(heading).not.toHaveClass("text-lg");
    expect(heading).not.toHaveAttribute("tabindex");
  });

  it("receives focus from script only when focusable", () => {
    render(
      <Heading level={3} focusable>
        example.com
      </Heading>,
    );
    const heading = screen.getByRole("heading", { level: 3 });
    expect(heading).toHaveAttribute("tabindex", "-1");
    heading.focus();
    expect(heading).toHaveFocus();
  });
});
