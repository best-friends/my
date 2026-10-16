// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Skeleton } from "./Skeleton";

describe("Skeleton", () => {
  it("stays out of the accessibility tree inside a busy region", () => {
    render(
      <section aria-label="Posts" aria-busy="true">
        <Skeleton title="placeholder" />
      </section>,
    );
    const region = screen.getByRole("region", { name: "Posts" });
    expect(region).toHaveAttribute("aria-busy", "true");
    const shape = screen.getByTitle("placeholder");
    expect(shape).toHaveAttribute("aria-hidden", "true");
    expect(shape).toHaveClass("motion-safe:animate-pulse");
  });
});
