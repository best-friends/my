// SPDX-License-Identifier: AGPL-3.0-only
// The main navigation, one landmark for every width (web-client.md D1, D2): a tab bar on the bottom edge
// of phones, a side rail of icons with short labels on tablets (md), and a sidebar with full labels on
// desktops (lg). The link to the current section has aria-current="page".
import { Link } from "@my/ui";
import { useLocation } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { type ClientMessageKey, useT } from "../../lib/i18n";
import { HomeIcon, LocalIcon, NotificationsIcon, ProfileIcon, SettingsIcon } from "./icons";

type NavigationItem = {
  to: "/" | "/local" | "/notifications" | "/profile" | "/settings";
  label: ClientMessageKey;
  icon: ReactNode;
};

const items: readonly NavigationItem[] = [
  { to: "/", label: "client:shell.nav.home", icon: <HomeIcon /> },
  { to: "/local", label: "client:shell.nav.local", icon: <LocalIcon /> },
  { to: "/notifications", label: "client:shell.nav.notifications", icon: <NotificationsIcon /> },
  { to: "/profile", label: "client:shell.nav.profile", icon: <ProfileIcon /> },
  { to: "/settings", label: "client:shell.nav.settings", icon: <SettingsIcon /> },
];

/** Whether `pathname` is in the section of `to`: the page itself or one below it. Home is only "/". */
function inSection(pathname: string, to: NavigationItem["to"]): boolean {
  return to === "/" ? pathname === "/" : pathname === to || pathname.startsWith(`${to}/`);
}

export function Navigation() {
  const t = useT();
  const pathname = useLocation({ select: (location) => location.pathname });
  return (
    <nav
      aria-label={t("client:shell.nav.label")}
      className="fixed inset-x-0 bottom-0 z-10 border-t border-border bg-surface-raised px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pb-[env(safe-area-inset-bottom)] md:sticky md:top-0 md:bottom-auto md:h-svh md:border-t-0 md:border-e md:bg-surface md:px-0 md:pb-0"
    >
      <ul className="grid grid-cols-5 md:flex md:flex-col md:gap-1 md:p-2 lg:w-60">
        {items.map((item) => (
          <li key={item.to} className="min-w-0">
            <Link
              href={item.to}
              aria-current={inSection(pathname, item.to) ? "page" : undefined}
              variant="standalone"
              className="group flex min-h-16 flex-col justify-center gap-1 px-1 text-xs font-normal text-fg-muted no-underline data-hovered:bg-surface-sunken data-hovered:no-underline pointer-coarse:min-h-16 aria-[current=page]:font-semibold aria-[current=page]:text-accent md:w-20 md:rounded-lg lg:min-h-12 lg:w-auto lg:flex-row lg:justify-start lg:gap-3 lg:px-3 lg:text-base"
            >
              <span className="rounded-full px-4 py-0.5 group-aria-[current=page]:bg-accent group-aria-[current=page]:text-accent-fg lg:px-1">
                {item.icon}
              </span>
              <span className="max-w-full truncate lg:whitespace-normal">{t(item.label)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
