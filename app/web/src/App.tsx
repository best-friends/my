// SPDX-License-Identifier: AGPL-3.0-only
// The app's providers, outermost first: the runtime config, i18n (react-intl and React Aria's locale),
// the TanStack Query cache, the session and the API client, the live regions and toasts of @my/ui, React Aria's router binding, and the
// router with every page.
import { AnnouncerProvider, Center, I18nProvider as AriaI18nProvider, RouterProvider as AriaRouterProvider, ToastProvider } from "@my/ui";
import { QueryClientProvider } from "@tanstack/react-query";
import { type AnyRouter, RouterProvider } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { SessionProvider } from "./lib/auth/SessionProvider";
import { ConfigProvider, type RuntimeConfig } from "./lib/config";
import { type I18n, MessagesProvider, useT } from "./lib/i18n";
import type { AppServices } from "./lib/services";

export type AppProps = AppServices & {
  config: RuntimeConfig;
  i18n: I18n;
  /** The app router (createAppRouter); tests may pass a router over routes of their own. */
  router: AnyRouter;
};

export function App({ config, i18n, queryClient, api, session, leaveApp, router }: AppProps) {
  /**
   * Every @my/ui Link (and Menu item, Tab) with a same-origin href navigates through the router instead
   * of loading a new document. An in-page target (the skip link's "#main") gets focus instead, as the
   * browser would do for a fragment, without touching the router's location.
   */
  const navigate = (href: string) => {
    if (href.startsWith("#")) {
      document.getElementById(decodeURIComponent(href.slice(1)))?.focus();
      return;
    }
    void router.navigate({ href });
  };
  return (
    <ConfigProvider config={config}>
      <AppI18nProvider i18n={i18n}>
        <QueryClientProvider client={queryClient}>
          <SessionProvider session={session} api={api} leaveApp={leaveApp}>
            <AnnouncerProvider>
              <Toasts>
                <AriaRouterProvider navigate={navigate}>
                  <RouterProvider router={router} />
                </AriaRouterProvider>
              </Toasts>
            </AnnouncerProvider>
          </SessionProvider>
        </QueryClientProvider>
      </AppI18nProvider>
    </ConfigProvider>
  );
}

export type AppI18nProviderProps = { i18n: I18n; children: ReactNode };

/** react-intl's messages and React Aria's locale (for the @my/ui components), from the same locale. */
export function AppI18nProvider({ i18n, children }: AppI18nProviderProps) {
  return (
    <MessagesProvider i18n={i18n}>
      <AriaI18nProvider locale={i18n.locale}>{children}</AriaI18nProvider>
    </MessagesProvider>
  );
}

function Toasts({ children }: { children: ReactNode }) {
  const t = useT();
  return (
    <ToastProvider regionLabel={t("client:shell.toast.region")} dismissLabel={t("client:shell.toast.dismiss")}>
      {children}
    </ToastProvider>
  );
}

export type StartupErrorProps = { i18n: I18n };

/** Shown instead of the app when the runtime config is missing or invalid; nothing else can work then. */
export function StartupError({ i18n }: StartupErrorProps) {
  return (
    <AppI18nProvider i18n={i18n}>
      <StartupErrorMessage />
    </AppI18nProvider>
  );
}

function StartupErrorMessage() {
  const t = useT();
  return (
    <main className="px-4">
      <Center>
        <div className="flex max-w-prose flex-col gap-4">
          <h1 className="text-2xl font-semibold">{t("client:shell.startup_error.title")}</h1>
          <p>{t("client:shell.startup_error.body")}</p>
        </div>
      </Center>
    </main>
  );
}
