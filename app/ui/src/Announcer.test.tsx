// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { AnnouncerProvider, useAnnouncer } from "./Announcer";
import { Button } from "./Button";
import { ToastProvider, useToast } from "./Toast";

function Announce() {
  const announce = useAnnouncer();
  return (
    <>
      <Button
        onPress={() => {
          announce("3 new posts");
        }}
      >
        Load
      </Button>
      <Button
        onPress={() => {
          announce("Could not send", "assertive");
        }}
      >
        Send
      </Button>
    </>
  );
}

function ShowToast() {
  const toast = useToast();
  return (
    <Button
      onPress={() => {
        toast.show({ message: "Draft saved", timeout: null });
      }}
    >
      Save
    </Button>
  );
}

describe("Announcer", () => {
  it("announces politely and assertively without moving focus", async () => {
    const user = userEvent.setup();
    render(
      <AnnouncerProvider>
        <Announce />
      </AnnouncerProvider>,
    );
    const load = screen.getByRole("button", { name: "Load" });
    await user.click(load);
    expect(screen.getByRole("status")).toHaveTextContent("3 new posts");
    expect(load).toHaveFocus();

    const send = screen.getByRole("button", { name: "Send" });
    await user.click(send);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not send");
    expect(send).toHaveFocus();
  });

  it("is what a toast announces through, and the toast does not take focus", async () => {
    const user = userEvent.setup();
    render(
      <AnnouncerProvider>
        <ToastProvider regionLabel="Notifications" dismissLabel="Dismiss">
          <ShowToast />
        </ToastProvider>
      </AnnouncerProvider>,
    );
    const save = screen.getByRole("button", { name: "Save" });
    await user.click(save);
    expect(screen.getByRole("region", { name: "Notifications" })).toHaveTextContent("Draft saved");
    expect(screen.getByRole("status")).toHaveTextContent("Draft saved");
    expect(save).toHaveFocus();
  });
});
