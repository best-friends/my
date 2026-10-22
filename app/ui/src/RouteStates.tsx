// SPDX-License-Identifier: AGPL-3.0-only
// The frames of what a route shows instead of its page (web-client.md E1, E3): while it loads, and the error of the
// app's layout itself, which is shown without the layout.
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Center } from "./Center";
import { Spinner } from "./Spinner";

const routeStates = tv({
  slots: {
    loading: "grid place-items-center py-10",
    page: "px-4",
    pageContent: "flex max-w-prose flex-col gap-4",
  },
});

export type RouteLoadingProps = {
  /** The spinner's accessible name ("Loading"). */
  label: string;
};

/** A page that is still loading: a large spinner in the middle of the page's column. */
export function RouteLoading({ label }: RouteLoadingProps) {
  return (
    <div className={routeStates().loading()}>
      <Spinner size="lg" label={label} />
    </div>
  );
}

export type StandalonePageProps = {
  /** The page's heading and content, one above another. */
  children: ReactNode;
};

/**
 * A page without the app's layout, such as the error of the layout itself: a `main` landmark with its content in a
 * column of prose width, centered in the screen.
 */
export function StandalonePage({ children }: StandalonePageProps) {
  const slots = routeStates();
  return (
    <main className={slots.page()}>
      <Center>
        <div className={slots.pageContent()}>{children}</div>
      </Center>
    </main>
  );
}
