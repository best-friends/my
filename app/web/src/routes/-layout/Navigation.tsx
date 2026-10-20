// SPDX-License-Identifier: AGPL-3.0-only
// The main navigation, one landmark for every width (web-client.md D1, D2, ADR-0039): a tab bar of five items
// on the bottom edge of phones, a side rail of icons with short labels on tablets (md), and a sidebar with full
// labels on desktops (lg). The link to the current section has aria-current="page".
//
// Below md the tab bar holds Home, Timelines, Notifications, Search and Menu, so that every label fits a fifth
// of a 320 px screen. Timelines opens the local timeline (or the federated one where the local one is off),
// whose page has both as tabs; Menu is a button that opens a sheet with the profile, the bookmarks, the settings and
// the color theme. From md the rail holds Home, Local, Federated, Notifications, Search, Profile, Bookmarks and Settings
// instead. Both are one list
// in one order, and each item hides at the widths where it does not belong (display: none, so it leaves the
// accessibility tree too).
//
// The notifications item shows the number of unread notifications as a badge on the icon's corner. The
// badge's digits are hidden from assistive technology; the link is named with the count instead
// ("Notifications, 3 unread"), a name that starts with the visible label (WCAG 2.5.3).
//
// A viewer whose roles have a permission of an admin screen also has Administration, after Settings in the rail
// and the sidebar and in the menu's sheet. While open reports or registrations await the moderators (FR-712),
// it shows their number as a badge the same way ("Administration, 3 pending"), and so does the tab bar's Menu
// button, which leads there ("Menu, 3 pending in administration"): the count is in the names and drawn as
// digits, never shown by color alone (WCAG 1.4.1).
//
// The local and federated timelines appear in the rail only while the instance serves them (FR-202). The tab
// bar keeps its five items whatever the instance serves: only the Timelines screen's tabs change.
//
// The sidebar (lg) also says where and who the user is: the server's name above the list and, below it, the
// signed-in account (avatar, display name and handle) as a link to its profile. Both are outside the list, so
// the list's links stay the same and in the same order at every width, and narrower layouts hide both.
import {
  AdminIcon,
  Avatar,
  Badge,
  BookmarkIcon,
  Button,
  DialogTrigger,
  FederatedIcon,
  HomeIcon,
  Link,
  LocalIcon,
  MenuIcon,
  NotificationsIcon,
  PostCardAuthor,
  ProfileIcon,
  SearchIcon,
  SettingsIcon,
  Sheet,
  TimelinesIcon,
} from "@my/ui";
import { useLocation, useRouter } from "@tanstack/react-router";
import { type ReactNode, useEffect, useId, useState } from "react";
import { useAdminEntry } from "../../features/admin";
import { DocumentLinks } from "../../features/documents";
import { useUnreadNotificationCount } from "../../features/notifications";
import { useViewer } from "../../features/session";
import { ThemeRadioGroup } from "../../features/settings";
import { type TimelineAvailability, useTimelineAvailability } from "../../features/timeline";
import { handleOf } from "../../lib/handle";
import { useInstanceName } from "../../lib/instanceName";
import { type ClientMessageKey, useT } from "../../lib/i18n";
import { followListPath, profilePath } from "../../lib/paths";

type Section = "/home" | "/timelines" | "/local" | "/public" | "/search" | "/notifications" | "/profile" | "/bookmarks" | "/settings" | "/admin";

type NavigationItem = {
  section: Section;
  label: ClientMessageKey;
  icon: ReactNode;
  /** Where the item shows: everywhere, only in the phones' tab bar, or only in the rail and sidebar from md. */
  shows: "always" | "tab-bar" | "rail";
  /** The timeline the item opens, offered only while the instance serves it. */
  timeline?: keyof TimelineAvailability;
  /** Whether the item is for a viewer whose roles have a permission of an admin screen only. */
  admin?: true;
};

const items: readonly NavigationItem[] = [
  { section: "/home", label: "client:shell.nav.home", icon: <HomeIcon />, shows: "always" },
  { section: "/timelines", label: "client:shell.nav.timelines", icon: <TimelinesIcon />, shows: "tab-bar" },
  { section: "/local", label: "client:shell.nav.local", icon: <LocalIcon />, shows: "rail", timeline: "local" },
  { section: "/public", label: "client:shell.nav.public", icon: <FederatedIcon />, shows: "rail", timeline: "federated" },
  { section: "/notifications", label: "client:shell.nav.notifications", icon: <NotificationsIcon />, shows: "always" },
  { section: "/search", label: "client:shell.nav.search", icon: <SearchIcon />, shows: "always" },
  { section: "/profile", label: "client:shell.nav.profile", icon: <ProfileIcon />, shows: "rail" },
  { section: "/bookmarks", label: "client:shell.nav.bookmarks", icon: <BookmarkIcon />, shows: "rail" },
  { section: "/settings", label: "client:shell.nav.settings", icon: <SettingsIcon />, shows: "rail" },
  { section: "/admin", label: "client:shell.nav.admin", icon: <AdminIcon />, shows: "rail", admin: true },
];

/** The items of the menu sheet: those of the rail that are not in the tab bar, other than the timelines (and Administration only for its viewers). */
const menuItems = items.filter((item) => item.shows === "rail" && item.timeline === undefined);

/**
 * The list item's classes for where the item shows (the tab bar is below md, the rail from md). In the tab bar
 * each item is as wide as its label and the room left is shared out evenly (flex: 1 1 auto), so that a long label
 * ("Notifications") takes the room that short ones ("Menu") leave and fits a 320 px screen on one line.
 */
const shownAt: Record<NavigationItem["shows"], string> = {
  always: "min-w-0 flex-auto md:flex-none",
  "tab-bar": "min-w-0 flex-auto md:hidden",
  rail: "hidden min-w-0 md:block",
};

/**
 * The look of an item of the tab bar and the rail, a link or the menu button. The label is never cut: where the
 * labels together are wider than the bar (a long translation), they wrap, inside a word if they must, so that
 * every label is read whole (ADR-0039). The icon is size-6 (web-client.md G11).
 */
const itemClass =
  "group flex min-h-16 w-full flex-col items-center justify-center gap-1 rounded-none px-0.5 text-xs font-normal text-fg-muted data-hovered:bg-surface-sunken pointer-coarse:min-h-16 short:min-h-12 aria-[current=page]:font-semibold aria-[current=page]:text-accent md:w-20 md:rounded-lg lg:min-h-12 lg:w-auto lg:flex-row lg:justify-start lg:gap-3 lg:px-3 lg:text-base [&_svg]:size-6 [&_svg]:shrink-0";

/** The pill around the icon, filled for the current item. */
const pillClass =
  "relative rounded-full px-2 py-0.5 group-aria-[current=page]:bg-accent group-aria-[current=page]:text-accent-fg md:px-4 lg:px-1";

/** A count on the corner of an item's icon. */
const badgeClass = "absolute -top-1.5 -inset-e-1 ring-2 ring-surface-raised md:ring-surface";

/** The label under the icon (beside it from lg). */
const labelClass = "max-w-full text-center leading-tight text-balance wrap-anywhere lg:text-start";

/** Whether `pathname` is `path` or a page below it. */
function isUnder(pathname: string, path: string): boolean {
  return pathname === path || pathname.startsWith(`${path}/`);
}

/**
 * Whether `pathname` is in `section`: the page itself or one below it. "/profile" redirects to the viewer's own
 * profile page, so that page and its lists of followers and follows are its section too. The viewer's own posts
 * (`/@acct/{id}`) are below the profile's address but are pages of posts, not of the profile. The timelines of
 * the tab bar are the local and the federated timeline.
 */
function inSection(pathname: string, section: Section, ownAcct: string | null): boolean {
  if (section === "/timelines") return isUnder(pathname, "/local") || isUnder(pathname, "/public");
  if (isUnder(pathname, section)) return true;
  if (section !== "/profile" || ownAcct === null) return false;
  return [profilePath(ownAcct), followListPath(ownAcct, "followers"), followListPath(ownAcct, "following")].includes(pathname);
}

/** Where the tab bar's Timelines goes: the local timeline, or the federated one where only it is served. */
function timelinesPath(timelines: TimelineAvailability): "/local" | "/public" {
  return !timelines.local && timelines.federated ? "/public" : "/local";
}

/** The badge of an item and the name of its link while it shows one, or null for an item without a count now. */
type ItemBadge = { count: number; badgeLabel: string; name: string } | null;

export function Navigation() {
  const t = useT();
  const instanceName = useInstanceName();
  const pathname = useLocation({ select: (location) => location.pathname });
  const unread = useUnreadNotificationCount();
  const adminEntry = useAdminEntry();
  const viewer = useViewer();
  const ownAcct = viewer.status === "signed-in" ? viewer.account.acct : null;
  const timelines = useTimelineAvailability();
  const shown = items.filter((item) => (item.timeline === undefined || timelines[item.timeline]) && (item.admin !== true || adminEntry.available));
  const badgeOf = (section: Section): ItemBadge => {
    if (section === "/notifications" && unread > 0) {
      return { count: unread, badgeLabel: t("client:notifications.unread_count", { count: unread }), name: t("client:shell.nav.notifications_unread", { count: unread }) };
    }
    if (section === "/admin" && adminEntry.available && adminEntry.pending > 0) {
      const count = adminEntry.pending;
      return { count, badgeLabel: t("client:admin.pending_count", { count }), name: t("client:shell.nav.admin_pending", { count }) };
    }
    return null;
  };
  const current = (section: Section) => (inSection(pathname, section, ownAcct) ? "page" : undefined);
  return (
    <nav
      aria-label={t("client:shell.nav.label")}
      className="fixed inset-x-0 bottom-0 z-10 border-t border-border bg-surface-raised px-[max(env(safe-area-inset-left),env(safe-area-inset-right))] pb-[env(safe-area-inset-bottom)] md:sticky md:top-0 md:bottom-auto md:flex md:h-svh md:flex-col md:overflow-y-auto md:border-t-0 md:border-e md:bg-surface md:px-0 md:pb-0 lg:w-60"
    >
      {/* The server's name is the administrator's text, in any language (web-client.md Y10, G9). */}
      <p dir="auto" className="mx-2 hidden px-3 pt-4 pb-2 text-lg font-semibold wrap-anywhere align-match-parent lg:block">
        {instanceName}
      </p>
      <ul className="flex md:flex-col md:gap-1 md:p-2">
        {shown.map((item) => {
          const badge = badgeOf(item.section);
          return (
            <li key={item.section} className={shownAt[item.shows]}>
              <Link
                href={item.section === "/timelines" ? timelinesPath(timelines) : item.section}
                {...(badge !== null && { "aria-label": badge.name })}
                aria-current={current(item.section)}
                variant="navigation"
                className={itemClass}
              >
                <span className={pillClass}>
                  {item.icon}
                  {badge !== null && <Badge count={badge.count} label={badge.badgeLabel} className={badgeClass} />}
                </span>
                <span className={labelClass}>{t(item.label)}</span>
              </Link>
            </li>
          );
        })}
        <li className={shownAt["tab-bar"]}>
          <NavigationMenu items={menuItems.filter((item) => item.admin !== true || adminEntry.available)} current={current} badgeOf={badgeOf} />
        </li>
      </ul>
      <ViewerLink />
    </nav>
  );
}

/**
 * The sidebar's last item: the signed-in account, a link to its profile named by its display name and handle,
 * each cut to one line (web-client.md G7). The list's Profile item is the place, so this second link to it has
 * no aria-current. It shows nothing until the account has loaded, and while signed out.
 */
function ViewerLink() {
  const t = useT();
  const viewer = useViewer();
  const descriptionId = useId();
  if (viewer.status !== "signed-in") return null;
  const { account } = viewer;
  const name = account.display_name || account.username;
  return (
    <>
      <Link
        href={profilePath(account.acct)}
        aria-describedby={descriptionId}
        variant="navigation"
        className="mx-2 mt-auto mb-2 hidden min-h-12 min-w-0 gap-3 rounded-lg px-3 py-2 font-normal text-fg data-hovered:bg-surface-sunken lg:flex"
      >
        <Avatar src={account.avatar_url} name={name} size="sm" decorative />
        {/* The display name and the handle a line each (the sidebar is no container), as a post's author shows them. */}
        <PostCardAuthor name={name} handle={handleOf(account.acct)} />
      </Link>
      {/* A description of a hidden element is still read (accname), and the element stays out of the tree. */}
      <span id={descriptionId} hidden>
        {t("client:shell.nav.viewer")}
      </span>
    </>
  );
}

type NavigationMenuProps = {
  /** The items of the sheet that the viewer may use. */
  items: readonly NavigationItem[];
  /** The `aria-current` of a section's link. */
  current: (section: Section) => "page" | undefined;
  /** The badge of a section's link, the same as in the rail. */
  badgeOf: (section: Section) => ItemBadge;
};

/**
 * The tab bar's Menu: a button that opens a sheet (a modal dialog, WAI-ARIA APG) with the items of the rail that
 * are not in the tab bar. The button is not a place, so it never has aria-current; the sheet marks the current
 * item. A navigation to another page closes the sheet when it starts, so that the new page's heading can take
 * focus (useViewChange); a navigation that the unsaved-changes guard holds does not start, and the sheet stays.
 * The button carries the counts of the sheet's items (what awaits the moderators), so that they show in the tab
 * bar too, and the sheet's items show them as the rail does. Under the items, the sheet switches the color theme
 * (the same control as the preferences page, in a row), and under that it links to the server's terms of service
 * and privacy policy, which the third column shows on wide screens, when the server publishes them.
 */
function NavigationMenu({ items, current, badgeOf }: NavigationMenuProps) {
  const t = useT();
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);
  const pending = items.reduce((sum, item) => sum + (badgeOf(item.section)?.count ?? 0), 0);
  // The router is outside React: subscribe to its navigations, and unsubscribe on unmount.
  useEffect(
    () =>
      router.subscribe("onBeforeNavigate", (event) => {
        if (event.pathChanged) setIsOpen(false);
      }),
    [router],
  );
  return (
    <DialogTrigger isOpen={isOpen} onOpenChange={setIsOpen}>
      <Button variant="ghost" className={itemClass} {...(pending > 0 && { "aria-label": t("client:shell.nav.menu_pending", { count: pending }) })}>
        <span className={pillClass}>
          <MenuIcon />
          {pending > 0 && <Badge count={pending} label={t("client:admin.pending_count", { count: pending })} className={badgeClass} />}
        </span>
        <span className={labelClass}>{t("client:shell.nav.menu")}</span>
      </Button>
      <Sheet title={t("client:shell.menu.title")} closeLabel={t("client:shell.menu.close")} isDismissable>
        <ul className="flex flex-col">
          {items.map((item) => {
            const badge = badgeOf(item.section);
            return (
              <li key={item.section}>
                <Link
                  href={item.section}
                  {...(badge !== null && { "aria-label": badge.name })}
                  aria-current={current(item.section)}
                  variant="navigation"
                  className="flex min-h-12 w-full gap-3 rounded-md px-3 text-base font-normal text-fg data-hovered:bg-surface-sunken aria-[current=page]:font-semibold aria-[current=page]:text-accent"
                >
                  {item.icon}
                  {t(item.label)}
                  {badge !== null && <Badge count={badge.count} label={badge.badgeLabel} />}
                </Link>
              </li>
            );
          })}
        </ul>
        {/* The theme is a setting of this browser, switched here too so that phones reach it in one step. */}
        <div className="mt-4 border-t border-border px-3 pt-4">
          <ThemeRadioGroup layout="sheet" />
        </div>
        <DocumentLinks placement="sheet" />
      </Sheet>
    </DialogTrigger>
  );
}
