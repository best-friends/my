// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Switch } from "./Switch";

describe("Switch", () => {
  it("is a switch named by its children and described by its description", () => {
    render(
      <Switch defaultSelected description="Plays GIFs without a click.">
        Autoplay
      </Switch>,
    );
    const toggle = screen.getByRole("switch", { name: "Autoplay" });
    expect(toggle).toBeChecked();
    expect(toggle).toHaveAccessibleDescription("Plays GIFs without a click.");
  });

  it("toggles with a click and with Space after Tab", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Switch onChange={onChange}>Dark mode</Switch>);
    const toggle = screen.getByRole("switch", { name: "Dark mode" });
    await user.click(toggle);
    expect(toggle).toBeChecked();
    await user.tab({ shift: true });
    await user.tab();
    expect(toggle).toHaveFocus();
    await user.keyboard(" ");
    expect(toggle).not.toBeChecked();
    expect(onChange.mock.calls).toEqual([[true], [false]]);
  });

  it("does not toggle while disabled", async () => {
    const user = userEvent.setup();
    render(<Switch isDisabled>Off</Switch>);
    const toggle = screen.getByRole("switch", { name: "Off" });
    await user.click(toggle);
    expect(toggle).not.toBeChecked();
    expect(toggle).toBeDisabled();
  });
});
