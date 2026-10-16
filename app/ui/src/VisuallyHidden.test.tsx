// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { VisuallyHidden } from "./VisuallyHidden";

describe("VisuallyHidden", () => {
  it("names an element without being drawn", () => {
    render(
      <a href="#s">
        <VisuallyHidden>Settings</VisuallyHidden>
      </a>,
    );
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
    expect(screen.getByText("Settings")).toHaveClass("sr-only");
  });
});
