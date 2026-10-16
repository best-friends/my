// SPDX-License-Identifier: AGPL-3.0-only
import { screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { renderWithProviders } from "../../test/render";
import { RoutePending } from "./RouteStates";

test("the pending page is a progress bar named in the page's locale", async () => {
  await renderWithProviders(<RoutePending />, { locale: "ja" });

  expect(screen.getByRole("progressbar", { name: "読み込み中" })).toBeInTheDocument();
});
