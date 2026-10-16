// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { Checkbox } from "./Checkbox";

describe("Checkbox", () => {
  it("is a checkbox named by its children and described by its description", () => {
    render(<Checkbox description="Others see who you follow.">Show follows</Checkbox>);
    const checkbox = screen.getByRole("checkbox", { name: "Show follows" });
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toHaveAccessibleDescription("Others see who you follow.");
  });

  it("toggles with a click on the label and with Space", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Checkbox onChange={onChange}>Sensitive</Checkbox>);
    const checkbox = screen.getByRole("checkbox", { name: "Sensitive" });
    await user.click(screen.getByText("Sensitive"));
    expect(checkbox).toBeChecked();
    await user.keyboard(" ");
    expect(checkbox).not.toBeChecked();
    expect(checkbox).toHaveFocus();
    expect(onChange.mock.calls).toEqual([[true], [false]]);
  });

  it("is controlled with isSelected and onChange", async () => {
    const user = userEvent.setup();
    function Controlled() {
      const [selected, setSelected] = useState(true);
      return (
        <>
          <Checkbox isSelected={selected} onChange={setSelected}>
            Notify
          </Checkbox>
          <output>{selected ? "on" : "off"}</output>
        </>
      );
    }
    render(<Controlled />);
    await user.click(screen.getByRole("checkbox", { name: "Notify" }));
    expect(screen.getByRole("status")).toHaveTextContent("off");
  });

  it("shows the mixed state", () => {
    render(<Checkbox isIndeterminate>All</Checkbox>);
    expect(screen.getByRole("checkbox", { name: "All" })).toBePartiallyChecked();
  });

  it("links the error message while invalid", () => {
    render(
      <Checkbox isInvalid errorMessage="Accept the rules to continue.">
        I accept the rules
      </Checkbox>,
    );
    const checkbox = screen.getByRole("checkbox", { name: "I accept the rules" });
    expect(checkbox).toBeInvalid();
    expect(checkbox).toHaveAccessibleDescription("Accept the rules to continue.");
  });

  it("is skipped while disabled", async () => {
    const user = userEvent.setup();
    render(<Checkbox isDisabled>Off</Checkbox>);
    await user.tab();
    expect(screen.getByRole("checkbox", { name: "Off" })).toBeDisabled();
    expect(document.body).toHaveFocus();
  });
});
