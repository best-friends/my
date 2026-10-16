// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Button } from "./Button";
import { NewItemsButton } from "./NewItemsButton";

describe("NewItemsButton", () => {
  it("keeps its status region in the page and draws the button only when items wait", () => {
    const { rerender } = render(<NewItemsButton count={0} label="0 new posts" onPress={() => undefined} />);
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
    rerender(<NewItemsButton count={3} label="3 new posts" onPress={() => undefined} />);
    expect(screen.getByRole("status")).toHaveTextContent("3 new posts");
    expect(screen.getByRole("button", { name: "3 new posts" })).toBeVisible();
  });

  it("does not take focus when the count changes, and calls onPress when pressed", async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    const { rerender } = render(
      <>
        <Button>Reply</Button>
        <NewItemsButton count={1} label="1 new post" onPress={onPress} />
      </>,
    );
    await user.tab();
    const reply = screen.getByRole("button", { name: "Reply" });
    expect(reply).toHaveFocus();
    rerender(
      <>
        <Button>Reply</Button>
        <NewItemsButton count={2} label="2 new posts" onPress={onPress} />
      </>,
    );
    expect(reply).toHaveFocus();
    expect(onPress).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "2 new posts" }));
    expect(onPress).toHaveBeenCalledOnce();
  });
});
