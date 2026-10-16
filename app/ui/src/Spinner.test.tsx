// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Spinner } from "./Spinner";

describe("Spinner", () => {
  it("is an indeterminate progress bar named by its label", () => {
    render(<Spinner label="Loading posts" />);
    expect(screen.getByRole("progressbar", { name: "Loading posts" })).not.toHaveAttribute("aria-valuenow");
  });
});
