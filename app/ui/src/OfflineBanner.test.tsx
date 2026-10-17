// SPDX-License-Identifier: AGPL-3.0-only
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OfflineBanner } from "./OfflineBanner";

describe("OfflineBanner", () => {
  it("keeps one status region and fills it only while offline", () => {
    const { rerender } = render(<OfflineBanner isOffline={false}>You're offline.</OfflineBanner>);
    const status = screen.getByRole("status");
    expect(status).toBeEmptyDOMElement();
    rerender(<OfflineBanner isOffline>You're offline.</OfflineBanner>);
    expect(screen.getByRole("status")).toBe(status);
    expect(status).toHaveTextContent("You're offline.");
    rerender(<OfflineBanner isOffline={false}>You're offline.</OfflineBanner>);
    expect(status).toBeEmptyDOMElement();
  });
});
