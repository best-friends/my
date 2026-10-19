// SPDX-License-Identifier: AGPL-3.0-only
// The layout around every page (ADR-0026). Mobile first (web-client.md D1): one column with the
// navigation as a bottom tab bar; from md a side rail and the page; from xl a third column about the
// server. Heights use svh (D5) and every edge that touches the screen adds its safe-area inset.
// Viewport breakpoints are allowed here because this is the app shell (D2).
import { Link } from "@my/ui";
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
        change is announced. It pads for the top safe-area inset itself, so it comes before the element that pads for it. */}
    <ConnectionBanner />
    <div className="min-h-svh px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pt-[env(safe-area-inset-top)] md:grid md:grid-cols-[auto_minmax(0,1fr)] xl:grid-cols-[auto_minmax(0,1fr)_24rem]">
      {/* The first focusable element of the page; visible only while focused. */}
      <Link
        href={`#${mainId}`}
        variant="standalone"
        className="sr-only z-20 rounded-md bg-surface-raised px-3 py-2 shadow-md focus:not-sr-only focus:fixed focus:inset-s-2 focus:top-2"
      >
        {t("client:shell.skip_to_main")}
      </Link>
      <Navigation />
      <main
        ref={mainRef}
        id={mainId}
        tabIndex={-1}
        // The end padding keeps the last item of a list clear of what floats over it (web-client.md G4): below md
        // the tab bar (min-h-16 and its border; min-h-12 on short screens) and, while the new-post button shows
        // (data-compose-fab), the button (3rem) above it; from md the button in the corner; from xl nothing.
        // Each gap is 1rem (0.5rem on short screens). index.css clears focused elements of the same.
        data-compose-fab={showButton ? "" : undefined}
        className="mx-auto flex w-full max-w-2xl min-w-0 flex-col gap-4 px-4 pt-6 pb-[calc(5rem+env(safe-area-inset-bottom))] short:pb-[calc(4rem+env(safe-area-inset-bottom))] md:px-8 md:pb-6 data-compose-fab:pb-[calc(9rem+env(safe-area-inset-bottom))] data-compose-fab:short:pb-[calc(7rem+env(safe-area-inset-bottom))] data-compose-fab:md:pb-[calc(5.5rem+env(safe-area-inset-bottom))] data-compose-fab:xl:pb-6"
      >
        <Outlet />
      </main>
      {wide ? (
        <div className="sticky top-0 flex h-svh flex-col gap-6 overflow-y-auto border-s border-border p-6">
          {/* Independent sections: if one breaks, the page goes on without it (web-client.md E1). */}
          <ErrorBoundary fallback={null}>
            <ComposeEntry />
          </ErrorBoundary>
          <ErrorBoundary fallback={null}>
            <ServerInfo />
          </ErrorBoundary>
        </div>
      ) : (
        // Above the tab bar on phones (only its icon, named by its label), in the corner from md; it is empty while
        // signed out. Pages without a feed keep the sheet, for replies, but not the button.
        <div className="fixed inset-e-[max(1rem,env(safe-area-inset-right))] bottom-[calc(5rem+env(safe-area-inset-bottom))] z-10 short:bottom-[calc(3.5rem+env(safe-area-inset-bottom))] md:bottom-[max(1.5rem,env(safe-area-inset-bottom))]">
          <ComposeEntry trigger={showButton ? (medium ? "labelled" : "icon") : "none"} />
        </div>
      )}
    </div>
    </ComposeProvider>
  );
}
