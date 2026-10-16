// SPDX-License-Identifier: AGPL-3.0-only
// The render helper for component tests: renders inside the app's providers and sets up user-event
// before rendering, as the user-event docs recommend (web-client.md T2).
import { render, type RenderOptions } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import type { ReactNode } from "react";
import { IntlProvider } from "react-intl";
import { locale, messages } from "../lib/i18n";

export function renderWithProviders(ui: ReactNode, options?: Omit<RenderOptions, "wrapper">) {
  const user = userEvent.setup();
  const result = render(ui, {
    ...options,
    wrapper: ({ children }) => (
      <IntlProvider locale={locale} defaultLocale="en" messages={messages}>
        {children}
      </IntlProvider>
    ),
  });
  return { user, ...result };
}
