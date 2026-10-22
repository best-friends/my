// SPDX-License-Identifier: AGPL-3.0-only
// The layout around every page (ADR-0026). Mobile first (web-client.md D1): one column with the
// navigation as a bottom tab bar; from md a side rail and the page; from xl a third column about the
// server. Its look is @my/ui's AppShell; this file decides what goes in it.
import { AppShell } from "@my/ui";
import { Outlet } from "@tanstack/react-router";
import { useRef } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { ComposeEntry, ComposeProvider } from "../../features/compose";
import { useUserDisplaySettings } from "../../features/settings";
import { useT } from "../../lib/i18n";
import { ConnectionBanner } from "./ConnectionBanner";
import { Navigation, useSignedOut } from "./Navigation";
import { ServerInfo } from "./ServerInfo";
import { useLiveUpdates } from "./useLiveUpdates";
import { useMediaQuery } from "./useMediaQuery";
import { useViewChange } from "./useViewChange";

/** The id of the main landmark, the target of the skip link. */
const mainId = "main";

/** The `xl` breakpoint of Tailwind (80rem), from which the third column shows. */
const wideQuery = "(min-width: 80rem)";

export function AppLayout() {
  const t = useT();
  const mainRef = useRef<HTMLElement>(null);
  useViewChange(mainRef);
  useLiveUpdates();
  // Every page is displayed in the signed-in user's locale and time zone.
  useUserDisplaySettings();
  const wide = useMediaQuery(wideQuery);
  const signedOut = useSignedOut();
  return (
    <ComposeProvider>
      {/* The one bar for being offline or rate-limited (web-client.md G17), in the page from the start so that the
          change is announced. It pads for the top safe-area inset itself, so it comes before the shell, which pads for it. */}
      <ConnectionBanner />
      <AppShell
        navigation={<Navigation />}
        navigationPlacement={signedOut ? "top" : "side"}
        skipLinkLabel={t("client:shell.skip_to_main")}
        mainId={mainId}
        mainRef={mainRef}
        {...(wide && {
          aside: (
            // Independent sections: if one breaks, the page goes on without it (web-client.md E1).
            <ErrorBoundary fallback={null}>
              <ServerInfo />
            </ErrorBoundary>
          ),
        })}
      >
        <Outlet />
      </AppShell>
      {/* The new-post sheet, opened by the navigation's button, a reply or a quote; nothing while signed out. */}
      <ComposeEntry />
    </ComposeProvider>
  );
}
