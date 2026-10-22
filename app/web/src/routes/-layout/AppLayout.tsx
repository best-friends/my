// SPDX-License-Identifier: AGPL-3.0-only
// The layout around every page (ADR-0026). Mobile first (web-client.md D1): one column with the
// navigation as a bottom tab bar; from md a side rail and the page; from xl a third column about the
// server. Its look is @my/ui's AppShell; this file decides what goes in it.
import { AppShell } from "@my/ui";
import { Outlet, useMatches } from "@tanstack/react-router";
import { useRef } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { ComposeEntry, ComposeProvider } from "../../features/compose";
import { useViewer } from "../../features/session";
import { useUserDisplaySettings } from "../../features/settings";
import { useT } from "../../lib/i18n";
import { ConnectionBanner } from "./ConnectionBanner";
import { Navigation } from "./Navigation";
import { ServerInfo } from "./ServerInfo";
import { useLiveUpdates } from "./useLiveUpdates";
import { useMediaQuery } from "./useMediaQuery";
import { useViewChange } from "./useViewChange";

/** The id of the main landmark, the target of the skip link. */
const mainId = "main";

/** The `xl` breakpoint of Tailwind (80rem), from which the new-post form is a side panel instead of a sheet. */
const wideQuery = "(min-width: 80rem)";

/** The `md` breakpoint of Tailwind (48rem), from which the floating new-post button shows its label. */
const mediumQuery = "(min-width: 48rem)";

export function AppLayout() {
  const t = useT();
  const mainRef = useRef<HTMLElement>(null);
  useViewChange(mainRef);
  useLiveUpdates();
  // Every page is displayed in the signed-in user's locale and time zone.
  useUserDisplaySettings();
  const wide = useMediaQuery(wideQuery);
  const medium = useMediaQuery(mediumQuery);
  // Only pages with a feed float the new-post button (timelines and profiles: their routes' static data).
  const pageWantsButton = useMatches({ select: (matches) => matches.some((match) => match.staticData.composeButton === true) });
  const signedIn = useViewer().status === "signed-in";
  const showButton = !wide && pageWantsButton && signedIn;
  return (
    <ComposeProvider layout={wide ? "panel" : "sheet"}>
      {/* The one bar for being offline or rate-limited (web-client.md G17), in the page from the start so that the
          change is announced. It pads for the top safe-area inset itself, so it comes before the shell, which pads for it. */}
      <ConnectionBanner />
      <AppShell
        navigation={<Navigation />}
        skipLinkLabel={t("client:shell.skip_to_main")}
        mainId={mainId}
        mainRef={mainRef}
        floatingVisible={showButton}
        {...(wide
          ? {
              aside: (
                // Independent sections: if one breaks, the page goes on without it (web-client.md E1).
                <>
                  <ErrorBoundary fallback={null}>
                    <ComposeEntry />
                  </ErrorBoundary>
                  <ErrorBoundary fallback={null}>
                    <ServerInfo />
                  </ErrorBoundary>
                </>
              ),
            }
          : {
              // Above the tab bar on phones (only its icon, named by its label), in the corner from md; it is empty while
              // signed out. Pages without a feed keep the sheet, for replies, but not the button.
              floating: <ComposeEntry trigger={showButton ? (medium ? "labelled" : "icon") : "none"} />,
            })}
      >
        <Outlet />
      </AppShell>
    </ComposeProvider>
  );
}
