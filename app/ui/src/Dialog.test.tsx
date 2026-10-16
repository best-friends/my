// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen, waitFor } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { Button } from "./Button";
import { Dialog, DialogTrigger, Sheet } from "./Dialog";

/** The focus trap holds focus somewhere inside the dialog; which element it is does not matter here. */
function expectFocusInside(dialog: HTMLElement) {
  // eslint-disable-next-line testing-library/no-node-access -- the focused element is what the focus trap is about
  expect(dialog).toContainElement(document.activeElement as HTMLElement);
}

function EditProfile({ sheet = false }: { sheet?: boolean }) {
  const Modal = sheet ? Sheet : Dialog;
  return (
    <DialogTrigger>
      <Button>Edit profile</Button>
      <Modal title="Edit profile" closeLabel="Close">
        {({ close }) => (
          <>
            <Button onPress={close}>Cancel</Button>
            <Button onPress={close}>Save</Button>
          </>
        )}
      </Modal>
    </DialogTrigger>
  );
}

describe.each([
  ["Dialog", false],
  ["Sheet", true],
])("%s", (_, sheet) => {
  it("moves focus in, keeps Tab inside, closes on Esc and returns focus to the trigger", async () => {
    const user = userEvent.setup();
    render(<EditProfile sheet={sheet} />);
    const trigger = screen.getByRole("button", { name: "Edit profile" });
    await user.click(trigger);
    const dialog = await screen.findByRole("dialog", { name: "Edit profile" });
    await waitFor(() => {
      expectFocusInside(dialog);
    });
    for (let i = 0; i < 4; i++) {
      await user.tab();
      expectFocusInside(dialog);
    }
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
  });

  it("returns focus to the trigger after the × button closes it", async () => {
    const user = userEvent.setup();
    render(<EditProfile sheet={sheet} />);
    const trigger = screen.getByRole("button", { name: "Edit profile" });
    await user.click(trigger);
    await user.click(await screen.findByRole("button", { name: "Close" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
  });
});
