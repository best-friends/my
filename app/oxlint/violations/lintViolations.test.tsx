// SPDX-License-Identifier: AGPL-3.0-only
// Violations of the 強制 rows of docs/architecture/web-client/README.md that lint checks in tests, copied next to
// LintViolations.tsx by violations.test.js (see there).
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";

vi.mock("../api/timelines"); // expect: restrict(test) T3
vi.spyOn(globalThis, "fetch"); // expect: restrict(test) T3

it("violates", async () => {
  const { getByText } = render(<p />);
  fireEvent.click(getByText("x")); // expect: testing-library(prefer-user-event) testing-library(prefer-screen-queries) T2
  await waitFor(() => screen.getByRole("button")); // expect: testing-library(prefer-find-by) T2
  screen.getByTestId("x"); // expect: testing-library(no-test-id-queries) Y2
  await vi.waitFor(() => undefined); // expect: restrict(test) T7
  expect(<svg />).toBeTruthy(); // A stand-in SVG child is fine in a test (G10).
});
