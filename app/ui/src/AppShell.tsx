// SPDX-License-Identifier: AGPL-3.0-only
// The layout around every page of the app (ADR-0026, web-client.md §9). Mobile first (D1): one column with the
// navigation as a bottom tab bar; from md a side rail and the page; from xl a third column. Heights use svh (D5) and
// every edge that touches the screen adds its safe-area inset. It answers to the viewport, not to a container: the
// shell is the one place where viewport breakpoints belong (D2), since what it lays out is the screen itself (the tab
// bar is fixed to the screen's bottom edge, the rail and the third column are as tall as the screen). Its stories set
// the story's viewport instead of the frame's width (storyViewports.ts).
import { type ReactNode, type Ref, useId } from "react";
import { tv } from "tailwind-variants";
import { Card } from "./Card";
import { Link } from "./Link";

const appShell = tv({
  slots: {
    base: [
      "min-h-svh px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pt-[env(safe-area-inset-top)]",
      "md:grid md:grid-cols-[auto_minmax(0,1fr)]",
    ],
    skipLink: "sr-only z-20 rounded-md bg-surface-raised px-3 py-2 shadow-md focus:not-sr-only focus:fixed focus:inset-s-2 focus:top-2",
    // The end padding keeps the last item of a list clear of what floats over it (web-client.md G4): below md the tab
    // bar (min-h-16 and its border; min-h-12 on short screens) and, while the floating button shows (data-compose-fab),
    // the button (3rem) above it; from md the button in the corner; from xl nothing. Each gap is 1rem (0.5rem on short
    // screens). The app's stylesheet clears focused elements of the same (scroll-padding-block-end).
    main: [
      "mx-auto flex w-full max-w-2xl min-w-0 flex-col gap-4 px-4 pt-6 md:px-8 md:pb-6",
      "pb-[calc(5rem+env(safe-area-inset-bottom))] short:pb-[calc(4rem+env(safe-area-inset-bottom))]",
      "data-compose-fab:pb-[calc(9rem+env(safe-area-inset-bottom))] data-compose-fab:short:pb-[calc(7rem+env(safe-area-inset-bottom))]",
      "data-compose-fab:md:pb-[calc(5.5rem+env(safe-area-inset-bottom))] data-compose-fab:xl:pb-6",
    ],
    aside: "sticky top-0 flex h-svh flex-col gap-6 overflow-y-auto border-s border-border p-6",
    floating: [
      "fixed inset-e-[max(1rem,env(safe-area-inset-right))] bottom-[calc(5rem+env(safe-area-inset-bottom))] z-10",
      "short:bottom-[calc(3.5rem+env(safe-area-inset-bottom))] md:bottom-[max(1.5rem,env(safe-area-inset-bottom))]",
    ],
  },
  variants: {
    withAside: { true: { base: "xl:grid-cols-[auto_minmax(0,1fr)_24rem]" } },
  },
});

export type AppShellProps = {
  /** The main navigation (`AppNav`): the bottom tab bar below md, the rail and the sidebar from md. */
  navigation: ReactNode;
  /** The page, in the `main` landmark. */
  children: ReactNode;
  /** The skip link's text ("Skip to main content"), the first focusable element of the page. */
  skipLinkLabel: string;
  /** The id of `main`, the skip link's target. */
  mainId?: string;
  /** The `main` element, for the code that moves focus to the page's heading after a navigation. */
  mainRef?: Ref<HTMLElement>;
  /**
   * The third column, sticky and as tall as the screen; pass it only on screens that show it (xl). The app gives it
   * the new-post form and the server's information (`AsidePanel`).
   */
  aside?: ReactNode;
  /**
   * The slot that floats over the page's end corner: above the tab bar below md, in the corner from md. The app puts
   * the new-post button there on screens without the third column. The slot may hold nothing visible.
   */
  floating?: ReactNode;
  /** Whether the floating slot shows a button now, so that `main` keeps the end of the page clear of it (G4). */
  floatingVisible?: boolean;
};

/**
 * The app's frame: the skip link, the navigation, `main` and the third column or the floating slot. Render the
 * connection bar (`OfflineBanner`) before it, since it pads for the top safe-area inset itself.
 */
export function AppShell({ navigation, children, skipLinkLabel, mainId = "main", mainRef, aside, floating, floatingVisible = false }: AppShellProps) {
  const slots = appShell({ withAside: aside !== undefined });
  return (
    <div className={slots.base()}>
      {/* The first focusable element of the page; visible only while focused. */}
      <Link href={`#${mainId}`} variant="standalone" className={slots.skipLink()}>
        {skipLinkLabel}
      </Link>
      {navigation}
      <main ref={mainRef} id={mainId} tabIndex={-1} data-compose-fab={floatingVisible ? "" : undefined} className={slots.main()}>
        {children}
      </main>
      {aside !== undefined && <div className={slots.aside()}>{aside}</div>}
      {floating !== undefined && <div className={slots.floating()}>{floating}</div>}
    </div>
  );
}

const asidePanel = tv({
  slots: {
    base: "flex flex-col gap-2",
    heading: "text-sm font-bold text-fg-muted",
  },
});

export type AsidePanelProps = {
  /** The panel's heading, an `h2` that names the `aside` landmark ("About this server"). */
  heading: string;
  children: ReactNode;
};

/** A section of the third column: an `aside` landmark named by its small, muted heading, its content one above another. */
export function AsidePanel({ heading, children }: AsidePanelProps) {
  const slots = asidePanel();
  const headingId = useId();
  return (
    <Card as="aside" padding="4" aria-labelledby={headingId}>
      <div className={slots.base()}>
        <h2 id={headingId} className={slots.heading()}>
          {heading}
        </h2>
        {children}
      </div>
    </Card>
  );
}
