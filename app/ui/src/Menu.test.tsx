// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen, waitFor } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { I18nProvider } from "react-aria-components";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";
import { Menu, MenuItem, MenuTrigger, SubmenuTrigger } from "./Menu";

function PostMenu({ locale = "en" }: { locale?: string }) {
  return (
    <I18nProvider locale={locale}>
      <MenuTrigger>
        <Button>Post options</Button>
        <Menu aria-label="Post options">
          <MenuItem id="copy">Copy link</MenuItem>
          <SubmenuTrigger>
            <MenuItem id="share">Share</MenuItem>
            <Menu aria-label="Share">
              <MenuItem id="email">Email</MenuItem>
            </Menu>
          </SubmenuTrigger>
        </Menu>
      </MenuTrigger>
    </I18nProvider>
  );
}

describe("Menu", () => {
  it("opens on the first item, closes on Esc and returns focus to the trigger", async () => {
    const user = userEvent.setup();
    render(<PostMenu />);
    const trigger = screen.getByRole("button", { name: "Post options" });
    await user.tab();
    await user.keyboard("{Enter}");
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: "Copy link" })).toHaveFocus();
    });
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
  });

  it.each([
    ["en", "{ArrowRight}", "{ArrowLeft}"],
    ["ar", "{ArrowLeft}", "{ArrowRight}"],
  ])("opens and closes a submenu with the keys of the reading direction (%s)", async (locale, open, close) => {
    const user = userEvent.setup();
    render(<PostMenu locale={locale} />);
    await user.tab();
    await user.keyboard("{Enter}");
    const share = await screen.findByRole("menuitem", { name: "Share" });
    await user.keyboard("{ArrowDown}");
    expect(share).toHaveFocus();
    await user.keyboard(open);
    await waitFor(() => {
      expect(screen.getByRole("menuitem", { name: "Email" })).toHaveFocus();
    });
    await user.keyboard(close);
    await waitFor(() => {
      expect(screen.queryByRole("menu", { name: "Share" })).not.toBeInTheDocument();
    });
    expect(share).toHaveFocus();
  });
});
