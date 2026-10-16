// SPDX-License-Identifier: AGPL-3.0-only
// Render helpers for tests (web-client.md T2, T4). They render inside the app's real providers and set
// up user-event before rendering, as the user-event docs recommend.
import type { QueryClient } from "@tanstack/react-query";
import { type AnyRouter, createMemoryHistory, type RouterHistory } from "@tanstack/react-router";
import { render, type RenderOptions } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import type { ReactNode } from "react";
import { App, AppI18nProvider } from "../App";
import { ConfigProvider, type RuntimeConfig } from "../lib/config";
import { type I18n, loadI18n } from "../lib/i18n";
import { createQueryClient } from "../lib/query";
import { createAppRouter } from "../router";

/** The runtime config tests run with, as the server would write it into the shell. */
const testConfig: RuntimeConfig = {
  apiOrigin: "http://localhost:3000",
  instanceName: "my example",
  accountDomain: "my.example",
  defaultLocale: "en",
};

/** Renders a component inside the config and i18n providers, in `locale` (the source locale by default). */
export async function renderWithProviders(ui: ReactNode, { locale = "en", ...options }: Omit<RenderOptions, "wrapper"> & { locale?: string } = {}) {
  const i18n = await loadI18n(locale);
  const user = userEvent.setup();
  const result = render(ui, {
    ...options,
    wrapper: ({ children }) => (
      <ConfigProvider config={testConfig}>
        <AppI18nProvider i18n={i18n}>{children}</AppI18nProvider>
      </ConfigProvider>
    ),
  });
  return { user, i18n, ...result };
}

export type RenderAppOptions = {
  /** The address the app opens at. */
  path?: string;
  locale?: string;
  /** Creates the router; createAppRouter (every route of the app) by default. */
  createRouter?: (options: { queryClient: QueryClient; history: RouterHistory }) => AnyRouter;
};

/**
 * Renders the whole app (an integration test, T4): the real router on a memory history, a fresh
 * QueryClient, and MSW for the network. Wait for the first page with a `findBy` query.
 */
export async function renderApp({ path = "/", locale = "en", createRouter = createAppRouter }: RenderAppOptions = {}) {
  const i18n: I18n = await loadI18n(locale);
  const queryClient = createQueryClient();
  const router = createRouter({ queryClient, history: createMemoryHistory({ initialEntries: [path] }) });
  const user = userEvent.setup();
  const result = render(<App config={testConfig} i18n={i18n} queryClient={queryClient} router={router} />);
  return { user, i18n, router, queryClient, ...result };
}
