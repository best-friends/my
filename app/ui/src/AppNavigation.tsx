// SPDX-License-Identifier: AGPL-3.0-only
// The app's main navigation, one landmark for every width (web-client.md D1, D2, ADR-0039): a tab bar on the bottom
// edge of phones, a side rail of icons with short labels from md, and a sidebar with full labels from lg. Like
// AppShell, it answers to the viewport (the shell is the one place for viewport breakpoints, D2).
//
// The items are one list in one order at every width; each item hides where it does not belong (`shows`, with
// display: none, so it leaves the accessibility tree too). A count shows as a badge on the icon's corner: its digits
// are hidden from assistive technology and the link is named with the count instead ("Notifications, 3 unread"), a
// name that starts with the visible label (WCAG 2.5.3). The tab bar's last item can be a menu (`AppNavMenu`), a
// button that opens a sheet with the items that only the rail shows.
import { type ReactNode, useId } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { Badge } from "./Badge";
import { Sheet, DialogTrigger } from "./Dialog";
import { MenuIcon } from "./Icons";
import { Link } from "./Link";
import { PostCardAuthor } from "./PostCard";

const appNav = tv({
  slots: {
    base: [
      "fixed inset-x-0 bottom-0 z-10 border-t border-border bg-surface-raised",
      "px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pb-[env(safe-area-inset-bottom)]",
      "md:sticky md:top-0 md:bottom-auto md:flex md:h-svh md:flex-col md:overflow-y-auto md:border-t-0 md:border-e md:bg-surface md:px-0 md:pb-0 lg:w-60",
    ],
    header: "mx-2 hidden px-3 pt-4 pb-2 lg:block",
    list: "flex md:flex-col md:gap-1 md:p-2",
  },
});

export type AppNavProps = {
  /** The landmark's name ("Main"). */
  label: string;
  /** Above the list in the sidebar (lg) only: the server's name (`ServerNameLink` at `size="lg"`). */
  header?: ReactNode;
  /** The items: `AppNavItem`s and, last, an `AppNavMenu`. */
  children: ReactNode;
  /** Under the list in the sidebar (lg) only: the signed-in account (`AppNavAccount`). */
  footer?: ReactNode;
};

/**
 * The `nav` landmark and its list. The header and the footer are outside the list, so the list's links stay the same
 * and in the same order at every width.
 */
export function AppNav({ label, header, children, footer }: AppNavProps) {
  const slots = appNav();
  return (
    <nav aria-label={label} className={slots.base()}>
      {header !== undefined && <div className={slots.header()}>{header}</div>}
      <ul className={slots.list()}>{children}</ul>
      {footer}
    </nav>
  );
}

const appNavItem = tv({
  slots: {
    // In the tab bar each item is as wide as its label and the room left is shared out evenly (flex: 1 1 auto), so
    // that a long label ("Notifications") takes the room that short ones ("Menu") leave and fits a 320 px screen.
    base: "min-w-0",
    // The label is never cut: where the labels together are wider than the bar (a long translation), they wrap,
    // inside a word if they must, so that every label is read whole (ADR-0039). The icon is size-6 (G11).
    item: [
      "group flex min-h-16 w-full flex-col items-center justify-center gap-1 rounded-none px-0.5 text-xs font-normal text-fg-muted",
      "data-hovered:bg-surface-sunken pointer-coarse:min-h-16 short:min-h-12 aria-[current=page]:font-semibold aria-[current=page]:text-accent",
      "md:w-20 md:rounded-lg lg:min-h-12 lg:w-auto lg:flex-row lg:justify-start lg:gap-3 lg:px-3 lg:text-base [&_svg]:size-6 [&_svg]:shrink-0",
    ],
    // The pill around the icon, filled for the current item.
    pill: "relative rounded-full px-2 py-0.5 group-aria-[current=page]:bg-accent group-aria-[current=page]:text-accent-fg md:px-4 lg:px-1",
    badge: "absolute -top-1.5 -inset-e-1 ring-2 ring-surface-raised md:ring-surface",
    // The label under the icon (beside it from lg).
    label: "max-w-full text-center leading-tight text-balance wrap-anywhere lg:text-start",
  },
  variants: {
    shows: {
      always: { base: "flex-auto md:flex-none" },
      "tab-bar": { base: "flex-auto md:hidden" },
      rail: { base: "hidden md:block" },
    },
  },
});

/** A count on an item: the number, what it means, and the name of the item while it shows it. */
export type AppNavBadge = {
  count: number;
  /** What the number means, read instead of the digits ("3 unread notifications"). */
  label: string;
  /** The item's accessible name while it shows the count, starting with the visible label ("Notifications, 3 unread"). */
  name: string;
};

export type AppNavItemProps = {
  href: string;
  /** The item's icon (one of the kit's icons); the item sizes it. */
  icon: ReactNode;
  /** The visible label, the link's name without a count. */
  label: string;
  /** Whether the link is the place the user is in: `aria-current="page"`, the filled pill and the accent label. */
  current?: boolean;
  /** The count on the icon's corner, or nothing for an item without a count now. */
  badge?: AppNavBadge;
  /** Where the item shows: everywhere, only in the tab bar (below md), or only in the rail and the sidebar (from md). */
  shows?: "always" | "tab-bar" | "rail";
};

/** One item of the navigation: a link with its icon in a pill, its label and its count. */
export function AppNavItem({ href, icon, label, current = false, badge, shows = "always" }: AppNavItemProps) {
  const slots = appNavItem({ shows });
  return (
    <li className={slots.base()}>
      <Link
        href={href}
        {...(badge !== undefined && { "aria-label": badge.name })}
        aria-current={current ? "page" : undefined}
        variant="navigation"
        className={slots.item()}
      >
        <span className={slots.pill()}>
          {icon}
          {badge !== undefined && <Badge count={badge.count} label={badge.label} className={slots.badge()} />}
        </span>
        <span className={slots.label()}>{label}</span>
      </Link>
    </li>
  );
}

export type AppNavMenuProps = {
  /** The button's visible label ("Menu"). */
  label: string;
  /** The sum of the counts of the sheet's items, with the button's name that says it ("Menu, 3 pending in moderation"). */
  badge?: AppNavBadge;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The sheet's title, its accessible name. */
  title: string;
  /** The sheet's close button's label. */
  closeLabel: string;
  /** The sheet's content: an `AppNavMenuList` of the rail's items, then `AppNavMenuSection`s. */
  children: ReactNode;
};

/**
 * The tab bar's last item (below md only): a button with the items' look that opens a sheet (a modal dialog, WAI-ARIA
 * APG). The button is not a place, so it never has `aria-current`; the sheet marks the current item.
 */
export function AppNavMenu({ label, badge, isOpen, onOpenChange, title, closeLabel, children }: AppNavMenuProps) {
  const slots = appNavItem({ shows: "tab-bar" });
  return (
    <li className={slots.base()}>
      <DialogTrigger isOpen={isOpen} onOpenChange={onOpenChange}>
        <Button variant="ghost" className={slots.item()} {...(badge !== undefined && { "aria-label": badge.name })}>
          <span className={slots.pill()}>
            <MenuIcon />
            {badge !== undefined && <Badge count={badge.count} label={badge.label} className={slots.badge()} />}
          </span>
          <span className={slots.label()}>{label}</span>
        </Button>
        <Sheet title={title} closeLabel={closeLabel}>
          {children}
        </Sheet>
      </DialogTrigger>
    </li>
  );
}

const appNavMenu = tv({
  slots: {
    list: "flex flex-col",
    link: [
      "flex min-h-12 w-full gap-3 rounded-md px-3 text-base font-normal text-fg data-hovered:bg-surface-sunken",
      "aria-[current=page]:font-semibold aria-[current=page]:text-accent",
    ],
  },
});

export type AppNavMenuListProps = { children: ReactNode };

/** The list of the menu sheet's items (`AppNavMenuLink`). */
export function AppNavMenuList({ children }: AppNavMenuListProps) {
  return <ul className={appNavMenu().list()}>{children}</ul>;
}

export type AppNavMenuLinkProps = Omit<AppNavItemProps, "shows">;

/** An item of the menu sheet: a full-width row with the icon, the label and the count after it. */
export function AppNavMenuLink({ href, icon, label, current = false, badge }: AppNavMenuLinkProps) {
  return (
    <li>
      <Link
        href={href}
        {...(badge !== undefined && { "aria-label": badge.name })}
        aria-current={current ? "page" : undefined}
        variant="navigation"
        className={appNavMenu().link()}
      >
        {icon}
        {label}
        {badge !== undefined && <Badge count={badge.count} label={badge.label} />}
      </Link>
    </li>
  );
}

const appNavMenuSection = tv({
  base: "px-3",
  variants: {
    separator: {
      "3": "mt-3 border-t border-border pt-3",
      "4": "mt-4 border-t border-border pt-4",
    },
  },
});

export type AppNavMenuSectionProps = {
  /** A line above the section, with this spacing step on both of its sides; none for a section that follows on. */
  separator?: "3" | "4";
  children: ReactNode;
};

/** A block of the menu sheet under its items (the theme switch, the server's name and documents), indented as they are. */
export function AppNavMenuSection({ separator, children }: AppNavMenuSectionProps) {
  return <div className={appNavMenuSection({ separator })}>{children}</div>;
}

const appNavAccount = tv({
  base: "mx-2 mt-auto mb-2 hidden min-h-12 min-w-0 gap-3 rounded-lg px-3 py-2 font-normal text-fg data-hovered:bg-surface-sunken lg:flex",
});

export type AppNavAccountProps = {
  /** The account's profile. */
  href: string;
  /** The account's `Avatar`, `size="sm"` and `decorative`: the name next to it says whose it is. */
  avatar: ReactNode;
  /** The display name (the user's text, `dir="auto"`). */
  name: ReactNode;
  /** The handle (`@user`). */
  handle: string;
  /** Describes the link ("Your profile"). */
  description: string;
};

/**
 * The sidebar's last item (lg only): the signed-in account, a link to its profile named by its display name and
 * handle, a line each and cut to one line (web-client.md G7). The list's own profile item is the place, so this second
 * link to it has no `aria-current`.
 */
export function AppNavAccount({ href, avatar, name, handle, description }: AppNavAccountProps) {
  const descriptionId = useId();
  return (
    <>
      <Link href={href} aria-describedby={descriptionId} variant="navigation" className={appNavAccount()}>
        {avatar}
        <PostCardAuthor name={name} handle={handle} />
      </Link>
      {/* A description of a hidden element is still read (accname), and the element stays out of the tree. */}
      <span id={descriptionId} hidden>
        {description}
      </span>
    </>
  );
}
