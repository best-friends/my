// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Button } from "./Button";

describe("Button", () => {
  it("is a button named by its children", () => {
    render(<Button>Post</Button>);
    expect(screen.getByRole("button", { name: "Post" })).toBeInTheDocument();
  });

  it("is pressed with a pointer, Enter and Space", async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(<Button onPress={onPress}>Post</Button>);
    const button = screen.getByRole("button", { name: "Post" });

    await user.click(button);
    await user.tab();
    await user.tab({ shift: true });
    expect(button).toHaveFocus();
    await user.keyboard("{Enter}");
    await user.keyboard(" ");
    expect(onPress).toHaveBeenCalledTimes(3);
  });

  it("ignores presses while disabled and is skipped by Tab", async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(
      <Button isDisabled onPress={onPress}>
        Post
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Post" });
    await user.click(button);
    await user.tab();
    expect(onPress).not.toHaveBeenCalled();
    expect(button).toBeDisabled();
    expect(button).not.toHaveFocus();
  });

  it("ignores presses while pending but stays focusable", async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(
      <Button isPending onPress={onPress}>
        Post
      </Button>,
    );
    await user.tab();
    const button = screen.getByRole("button", { name: "Post" });
    expect(button).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onPress).not.toHaveBeenCalled();
  });

  it("merges className into the recipe, the later class winning", () => {
    render(
      <Button variant="primary" className="px-8">
        Post
      </Button>,
    );
    const button = screen.getByRole("button", { name: "Post" });
    expect(button).toHaveClass("px-8", "bg-accent-fill");
    expect(button).not.toHaveClass("px-4");
  });
});
