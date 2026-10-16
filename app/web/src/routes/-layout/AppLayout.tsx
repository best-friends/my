// SPDX-License-Identifier: AGPL-3.0-only
// The layout around every page (ADR-0026). Mobile first (web-client.md D1): one column with the
// navigation as a bottom tab bar; from md a side rail and the page; from xl a third column about the
// server. Heights use svh (D5) and every edge that touches the screen adds its safe-area inset.
// Viewport breakpoints are allowed here because this is the app shell (D2).
import { Link } from "@my/ui";
import { Outlet } from "@tanstack/react-router";
import { useRef } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { useT } from "../../lib/i18n";
import { Navigation } from "./Navigation";
import { ServerInfo } from "./ServerInfo";
import { useLiveUpdates } from "./useLiveUpdates";
import { useViewChange } from "./useViewChange";

/** The id of the main landmark, the target of the skip link. */
const mainId = "main";

export function AppLayout() {
  const t = useT();
  const mainRef = useRef<HTMLElement>(null);
  useViewChange(mainRef);
  useLiveUpdates();
  return (
    <div className="min-h-svh px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pt-[env(safe-area-inset-top)] md:grid md:grid-cols-[auto_minmax(0,1fr)] xl:grid-cols-[auto_minmax(0,1fr)_20rem]">
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
        className="mx-auto flex w-full max-w-2xl min-w-0 flex-col gap-4 px-4 pt-6 pb-[calc(6rem+env(safe-area-inset-bottom))] md:px-8 md:pb-6"
      >
        <Outlet />
      </main>
      {/* An independent section: if it breaks, the page goes on without it (web-client.md E1). */}
      <ErrorBoundary fallback={null}>
        <ServerInfo />
      </ErrorBoundary>
    </div>
  );
}
