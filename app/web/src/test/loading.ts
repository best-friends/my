// SPDX-License-Identifier: AGPL-3.0-only
// The loading state of a region, as web-client.md G15 has it: the region is aria-busy, one visually hidden
// text names what it waits for, and no spinner shares the region with its skeleton.
import { screen, within } from "@testing-library/react";
import { expect } from "vitest";

/** Finds the busy region that `label` names, and asserts that it is named once and shows no spinner. */
export async function findLoadingRegion(label: string): Promise<HTMLElement> {
  const text = await screen.findByText(label);
  expect(screen.getAllByText(label)).toHaveLength(1);
  // The busy region has no role of its own to find it by: it is the nearest busy ancestor of its label.
  const region = text.closest<HTMLElement>("[aria-busy='true']");
  if (region === null) throw new Error(`"${label}" is not inside a busy region`);
  expect(within(region).queryByRole("progressbar")).not.toBeInTheDocument();
  return region;
}
