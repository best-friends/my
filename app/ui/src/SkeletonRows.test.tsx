// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AccountRowSkeleton, NotificationRowSkeleton, PostSkeleton } from "./SkeletonRows";

describe.each([
  ["PostSkeleton", PostSkeleton],
  ["AccountRowSkeleton", AccountRowSkeleton],
  ["NotificationRowSkeleton", NotificationRowSkeleton],
])("%s", (_, Row) => {
  it("is hidden from assistive technology as a whole inside a busy region", () => {
    render(
      <section aria-label="Loading list" aria-busy="true">
        <Row title="placeholder" />
      </section>,
    );
    expect(screen.getByTitle("placeholder")).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByRole("region", { name: "Loading list" })).toHaveAttribute("aria-busy", "true");
  });
});
