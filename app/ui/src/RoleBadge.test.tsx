// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RoleBadge, RoleBadgeList } from "./RoleBadge";

describe("RoleBadge", () => {
  it("draws the label as user content in no recorded language", () => {
    render(<RoleBadge label="داعم" color="green" />);
    const badge = screen.getByText("داعم");
    expect(badge).toHaveAttribute("dir", "auto");
    expect(badge).toHaveAttribute("lang", "");
    expect(badge).toHaveClass("bg-badge-green", "text-badge-green-fg");
  });
});

describe("RoleBadgeList", () => {
  it("is a named list of the badges in their order", () => {
    render(
      <RoleBadgeList
        label="Badges"
        badges={[
          { id: "r1", label: "Staff", color: "purple" },
          { id: "r2", label: "Supporter", color: "blue" },
        ]}
      />,
    );
    const list = screen.getByRole("list", { name: "Badges" });
    const items = within(list).getAllByRole("listitem");
    expect(items.map((i) => i.textContent)).toEqual(["Staff", "Supporter"]);
  });

  it("renders nothing without badges", () => {
    const { container } = render(<RoleBadgeList label="Badges" badges={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
