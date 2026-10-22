// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { Box } from "./Box";
import { Cluster } from "./Cluster";
import { Grid } from "./Grid";
import { Stack } from "./Stack";

describe("Stack", () => {
  it("renders the element of `as` with its attributes and ref", () => {
    const ref = createRef<HTMLUListElement>();
    render(
      <Stack as="ul" aria-label="Domains" ref={ref} divided>
        <li>example.com</li>
      </Stack>,
    );
    const list = screen.getByRole("list", { name: "Domains" });
    expect(ref.current).toBe(list);
    expect(list).toHaveClass("flex", "flex-col", "divide-y");
  });

  it("passes a form's submit handler through", async () => {
    const onSubmit = vi.fn((event: { preventDefault: () => void }) => {
      event.preventDefault();
    });
    render(
      <Stack as="form" aria-label="Search" gap="4" onSubmit={onSubmit}>
        <button type="submit">Search</button>
      </Stack>,
    );
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(onSubmit).toHaveBeenCalledOnce();
    expect(screen.getByRole("form", { name: "Search" })).toHaveClass("gap-4");
  });
});

describe("Cluster", () => {
  it("uses `gap` for both axes unless `rowGap` says otherwise", () => {
    render(
      <>
        <Cluster as="ul" aria-label="Both" gap="2" />
        <Cluster as="ul" aria-label="Columns" gap="2" rowGap="0" wrap={false} />
      </>,
    );
    expect(screen.getByRole("list", { name: "Both" })).toHaveClass("flex-wrap", "gap-x-2", "gap-y-2");
    const columns = screen.getByRole("list", { name: "Columns" });
    expect(columns).toHaveClass("gap-x-2", "gap-y-0");
    expect(columns).not.toHaveClass("flex-wrap");
  });
});

describe("Grid and Box", () => {
  it("adds the columns from the container width and the box props", () => {
    render(
      <>
        <Grid as="section" aria-label="Fields" at="md" />
        <ul>
          <Box as="li" container paddingY="3" />
        </ul>
      </>,
    );
    expect(screen.getByRole("region", { name: "Fields" })).toHaveClass("grid-cols-1", "@md:grid-cols-2");
    const item = screen.getByRole("listitem");
    expect(item).toHaveClass("@container", "py-3");
    expect(item).not.toHaveClass("flex");
  });
});
