// SPDX-License-Identifier: AGPL-3.0-only
import { screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { App } from "./App";
import { messages } from "./lib/i18n";
import { renderWithProviders } from "./test/render";

test("the placeholder page shows its heading as the only level-one heading", () => {
  renderWithProviders(<App />);

  expect(screen.getByRole("main")).toBeInTheDocument();
  expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(messages["client:app.placeholder"] ?? "");
});
