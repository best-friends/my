// SPDX-License-Identifier: AGPL-3.0-only
// The main navigation, one landmark for every width (web-client.md D1, D2): a tab bar on the bottom edge
// of phones, a side rail of icons with short labels on tablets (md), and a sidebar with full labels on
// desktops (lg). The link to the current section has aria-current="page".
//
// The notifications item shows the number of unread notifications as a badge on the icon's corner. The
// badge's digits are hidden from assistive technology; the link is named with the count instead
// ("Notifications, 3 unread"), a name that starts with the visible label (WCAG 2.5.3).
//
// The local and federated timelines appear only while the instance serves them (FR-202). With all seven
// items in the tab bar, the pill around the icon still fits a seventh of a 320 px screen.
import { Badge, Link } from "@my/ui";
import { useLocation } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useUnreadNotificationCount } from "../../features/notifications";
import { useViewer } from "../../features/session";
import { type TimelineAvailability, useTimelineAvailability } from "../../features/timeline";
import { type ClientMessageKey, useT } from "../../lib/i18n";
import { profilePath } from "../../lib/paths";
import { FederatedIcon, HomeIcon, LocalIcon, NotificationsIcon, ProfileIcon, SearchIcon, SettingsIcon } from "./icons";

type NavigationItem = {
  to: "/home" | "/local" | "/public" | "/search" | "/notifications" | "/profile" | "/settings";
  label: ClientMessageKey;
  icon: ReactNode;
  /** The timeline the item opens, offered only while the instance serves it. */
  timeline?: keyof TimelineAvailability;
};

const items: readonly NavigationItem[] = [
  { to: "/home", label: "client:shell.nav.home", icon: <HomeIcon /> },
  { to: "/local", label: "client:shell.nav.local", icon: <LocalIcon />, timeline: "local" },
  { to: "/public", label: "client:shell.nav.public", icon: <FederatedIcon />, timeline: "federated" },
  { to: "/search", label: "client:shell.nav.search", icon: <SearchIcon /> },
  { to: "/notifications", label: "client:shell.nav.notifications", icon: <NotificationsIcon /> },
  { to: "/profile", label: "client:shell.nav.profile", icon: <ProfileIcon /> },
  { to: "/settings", label: "client:shell.nav.settings", icon: <SettingsIcon /> },
];

/** Whether `pathname` is `path` or a page below it. */
function isUnder(pathname: string, path: string): boolean {
  return pathname === path || pathname.startsWith(`${path}/`);
}

/**
 * Whether `pathname` is in the section of `to`: the page itself or one below it. "/profile" redirects to
 * the viewer's own profile page, so that page and the pages below it are its section too.
 */
function inSection(pathname: string, to: NavigationItem["to"], ownProfile: string | null): boolean {
  return isUnder(pathname, to) || (to === "/profile" && ownProfile !== null && isUnder(pathname, ownProfile));
}

export function Navigation() {
  const t = useT();
  const pathname = useLocation({ select: (location) => location.pathname });
  const unread = useUnreadNotificationCount();
  const viewer = useViewer();
  const ownProfile = viewer.status === "signed-in" ? profilePath(viewer.account.acct) : null;
  const timelines = useTimelineAvailability();
  const shown = items.filter((item) => item.timeline === undefined || timelines[item.timeline]);
  return (
    <nav
      aria-label={t("client:shell.nav.label")}
      className="fixed inset-x-0 bottom-0 z-10 border-t border-border bg-surface-raised px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pb-[env(safe-area-inset-bottom)] md:sticky md:top-0 md:bottom-auto md:h-svh md:border-t-0 md:border-e md:bg-surface md:px-0 md:pb-0"
    >
      <ul className="grid auto-cols-fr grid-flow-col md:flex md:flex-col md:gap-1 md:p-2 lg:w-60">
        {shown.map((item) => {
          const badge = item.to === "/notifications" && unread > 0;
          return (
            <li key={item.to} className="min-w-0">
              <Link
                href={item.to}
                {...(badge ? { "aria-label": t("client:shell.nav.notifications_unread", { count: unread }) } : {})}
                aria-current={inSection(pathname, item.to, ownProfile) ? "page" : undefined}
                variant="standalone"
                className="group flex min-h-16 flex-col justify-center gap-1 px-1 text-xs font-normal text-fg-muted no-underline data-hovered:bg-surface-sunken data-hovered:no-underline pointer-coarse:min-h-16 aria-[current=page]:font-semibold aria-[current=page]:text-accent md:w-20 md:rounded-lg lg:min-h-12 lg:w-auto lg:flex-row lg:justify-start lg:gap-3 lg:px-3 lg:text-base"
              >
                <span className="relative rounded-full px-2 py-0.5 group-aria-[current=page]:bg-accent group-aria-[current=page]:text-accent-fg md:px-4 lg:px-1">
                  {item.icon}
                  {badge && (
                    <Badge
                      count={unread}
                      label={t("client:notifications.unread_count", { count: unread })}
                      className="absolute -top-1.5 -inset-e-1 ring-2 ring-surface-raised md:ring-surface"
                    />
                  )}
                </span>
                <span className="max-w-full truncate lg:whitespace-normal">{t(item.label)}</span>
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
