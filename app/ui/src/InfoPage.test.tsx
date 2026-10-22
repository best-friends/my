// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen, within } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Heading } from "./Heading";
import { InfoPage } from "./InfoPage";

describe("InfoPage", () => {
  beforeEach(() => {
    class FakeObserver {
      observe() {}
      disconnect() {}
      unobserve() {}
    }
    vi.stubGlobal("IntersectionObserver", FakeObserver);
    Element.prototype.scrollIntoView = vi.fn();
    window.matchMedia = vi.fn().mockReturnValue({ matches: false });
  });

  it("builds a labelled table of contents from the h2 headings and moves focus to the chosen one", async () => {
    const user = userEvent.setup();
    render(
      <InfoPage heading={<h1>Title</h1>} contentsLabel="On this page">
        <Heading level={2} id="a">
          Alpha
        </Heading>
        <Heading level={2} id="b">
          Beta
        </Heading>
      </InfoPage>,
    );
    const nav = (await screen.findAllByRole("navigation", { name: "On this page" }))[0];
    if (nav === undefined) throw new Error("no navigation");
    const link = within(nav).getByRole("link", { name: "Beta" });
    expect(link).toHaveAttribute("href", "#b");
    await user.click(link);
    expect(screen.getByRole("heading", { name: "Beta" })).toHaveFocus();
    expect(link).toHaveAttribute("aria-current", "location");
  });
});
