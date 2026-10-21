// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { QuoteCard } from "./QuoteCard";

describe("QuoteCard", () => {
  it("shows the quoted post in a group named by its label", () => {
    render(
      <QuoteCard label="Quoted post" author="Carol" time="2h">
        <p>The sun rose.</p>
      </QuoteCard>,
    );
    const group = screen.getByRole("group", { name: "Quoted post" });
    expect(group).toHaveTextContent("Carol");
    expect(group).toHaveTextContent("2h");
    expect(group).toHaveTextContent("The sun rose.");
  });

  it("shows only the notice without the post", () => {
    render(<QuoteCard label="Quoted post" notice="The quoted post was deleted." />);
    expect(screen.getByRole("group", { name: "Quoted post" })).toHaveTextContent("The quoted post was deleted.");
  });

  it("keeps the post behind the notice until the reader shows it", async () => {
    const user = userEvent.setup();
    render(
      <QuoteCard label="Quoted post" author="Carol" notice="You muted the author." revealLabel="Show the quoted post">
        <p>The sun rose.</p>
      </QuoteCard>,
    );
    expect(screen.getByText("The sun rose.")).not.toBeVisible();
    const toggle = screen.getByRole("button", { name: "Show the quoted post" });
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("The sun rose.")).toBeVisible();
  });
});
