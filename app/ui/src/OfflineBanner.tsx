// SPDX-License-Identifier: AGPL-3.0-only
// The app's one offline bar (web-client/16-look-states-and-wording.md G17). It is a polite live region (role="status") that is in the page from
// the first render, empty while online, so that going offline is announced: a live region announces only what changes
// after it is in the page (web-client/README.md Y7). It sticks to the top edge and pads itself for the top safe-area inset,
// so the app renders it before, and outside of, any element that already pads for that inset. On a short screen (the
// short: variant, such as 320 × 225 at 400% zoom) it does not stick but scrolls away with the page: the tab bar and the
// floating new-post button already take almost half of the screen's height, and what stays in place must not take
// more (WCAG 1.4.10; the layout of web-client/09-app-shell.md §9).
import type { ComponentPropsWithRef, ReactNode } from "react";
import { tv } from "tailwind-variants";
import { WarningIcon } from "./Icons";

const offlineBanner = tv({
  slots: {
    region: "sticky top-0 z-30 short:static",
    bar: [
      "flex items-center justify-center gap-2 bg-warning-bg px-[max(1rem,env(safe-area-inset-left),env(safe-area-inset-right))] pt-[calc(0.5rem+env(safe-area-inset-top))] pb-2",
      "text-center text-sm font-bold text-fg",
    ],
    icon: "shrink-0 text-warning [&>svg]:size-4.5",
  },
});

export type OfflineBannerProps = Omit<ComponentPropsWithRef<"div">, "children" | "role"> & {
  /** Whether the app is offline; the bar shows only then. */
  isOffline: boolean;
  /** The message ("You're offline. Posts you send are kept and sent when you're back online."). */
  children: ReactNode;
};

/** Render it once, at the top of the app, for the whole session; switch `isOffline` as the connection changes. */
export function OfflineBanner({ isOffline, children, className, ...props }: OfflineBannerProps) {
  const slots = offlineBanner();
  return (
    <div {...props} role="status" className={slots.region({ className })}>
      {isOffline && (
        <p className={slots.bar()}>
          <span className={slots.icon()}>
            <WarningIcon filled />
          </span>
          <span>{children}</span>
        </p>
      )}
    </div>
  );
}
