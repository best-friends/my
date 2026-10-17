// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Link } from "./Link";

describe("Link", () => {
  it("is a link with its href, named by its children", () => {
    render(<Link href="/about">About</Link>);
    expect(screen.getByRole("link", { name: "About" })).toHaveAttribute("href", "/about");
  });

  it("is reached with Tab and followed with Enter", async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(
      <Link href="#top" onPress={onPress}>
        Back to top
      </Link>,
    );
    await user.tab();
    expect(screen.getByRole("link", { name: "Back to top" })).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onPress).toHaveBeenCalledOnce();
  });

  it("can leave the tab order and still be followed with the pointer", async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(
      <>
        <Link href="#a" excludeFromTabOrder onPress={onPress}>
          Avatar
        </Link>
        <Link href="#a">Name</Link>
      </>,
    );
    const avatar = screen.getByRole("link", { name: "Avatar" });
    expect(avatar).toHaveAttribute("tabindex", "-1");
    await user.tab();
    expect(screen.getByRole("link", { name: "Name" })).toHaveFocus();
    await user.click(avatar);
    expect(onPress).toHaveBeenCalledOnce();
  });

  it("keeps the underline inline and merges className", () => {
    render(
      <Link href="/a" className="font-bold">
        A
      </Link>,
    );
    expect(screen.getByRole("link", { name: "A" })).toHaveClass("underline", "font-bold");
  });
});
